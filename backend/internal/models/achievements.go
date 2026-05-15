// Package models - achievements.go defines a small achievements system.
//
// The catalogue is hard-coded at compile time (the set of recognised
// achievements is finite and changes only with releases). Unlocks are stored
// in the user_achievements table; the evaluator inspects scoreboard rows to
// decide which achievements a user currently qualifies for, then upserts new
// unlocks.
package models

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// AchievementID is a stable string identifier for an achievement. Changing
// these breaks already-unlocked rows, so prefer additive changes.
type AchievementID string

const (
	AchFirstWin       AchievementID = "first_win"
	AchTenWins        AchievementID = "ten_wins"
	AchFiftyWins      AchievementID = "fifty_wins"
	AchHundredWins    AchievementID = "hundred_wins"
	AchTenPlayed      AchievementID = "ten_played"
	AchHundredPlayed  AchievementID = "hundred_played"
	AchWinRate60      AchievementID = "win_rate_60"
	AchWinRate75      AchievementID = "win_rate_75"
	AchNightOwl       AchievementID = "night_owl"
	AchEarlyBird      AchievementID = "early_bird"
)

// Achievement is the public-facing definition of an achievement.
type Achievement struct {
	ID          AchievementID `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"` // emoji or icon code
	Tier        string        `json:"tier"` // bronze, silver, gold, platinum
}

// UnlockedAchievement adds unlock metadata to an Achievement.
type UnlockedAchievement struct {
	Achievement
	UnlockedAt time.Time `json:"unlocked_at"`
}

// Catalogue is the canonical list of achievements served to clients.
var Catalogue = []Achievement{
	{ID: AchFirstWin, Title: "First Win", Description: "Win your very first game.", Icon: "🥇", Tier: "bronze"},
	{ID: AchTenWins, Title: "Ten Wins", Description: "Win 10 games.", Icon: "🏅", Tier: "silver"},
	{ID: AchFiftyWins, Title: "Half Century", Description: "Win 50 games.", Icon: "🏆", Tier: "gold"},
	{ID: AchHundredWins, Title: "Centurion", Description: "Win 100 games.", Icon: "👑", Tier: "platinum"},
	{ID: AchTenPlayed, Title: "Getting Started", Description: "Play 10 games.", Icon: "🎲", Tier: "bronze"},
	{ID: AchHundredPlayed, Title: "Regular", Description: "Play 100 games.", Icon: "🎮", Tier: "gold"},
	{ID: AchWinRate60, Title: "Skilled", Description: "Reach 60% win rate (min 20 games).", Icon: "🎯", Tier: "silver"},
	{ID: AchWinRate75, Title: "Elite", Description: "Reach 75% win rate (min 20 games).", Icon: "🌟", Tier: "platinum"},
	{ID: AchNightOwl, Title: "Night Owl", Description: "Finish a game between midnight and 4am UTC.", Icon: "🦉", Tier: "bronze"},
	{ID: AchEarlyBird, Title: "Early Bird", Description: "Finish a game between 5am and 8am UTC.", Icon: "🌅", Tier: "bronze"},
}

var catalogueByID map[AchievementID]Achievement

func init() {
	catalogueByID = make(map[AchievementID]Achievement, len(Catalogue))
	for _, a := range Catalogue {
		catalogueByID[a.ID] = a
	}
}

// LookupAchievement returns the catalogue entry for id, if any.
func LookupAchievement(id AchievementID) (Achievement, bool) {
	a, ok := catalogueByID[id]
	return a, ok
}

// EnsureAchievementsSchema creates the user_achievements table if missing.
func EnsureAchievementsSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_achievements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			achievement_id TEXT NOT NULL,
			unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, achievement_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("EnsureAchievementsSchema: %w", err)
	}
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_user_achievements_user ON user_achievements(user_id)`)
	if err != nil {
		return fmt.Errorf("EnsureAchievementsSchema: idx: %w", err)
	}
	return nil
}

// UnlockSnapshot captures what a user has unlocked and what is still locked.
type UnlockSnapshot struct {
	Unlocked []UnlockedAchievement `json:"unlocked"`
	Locked   []Achievement         `json:"locked"`
}

// ListAchievementsForUser returns a snapshot of unlocked + locked
// achievements. The unlocked list is sorted by unlock time descending; the
// locked list follows the catalogue order.
func ListAchievementsForUser(ctx context.Context, db *sql.DB, userID int64) (*UnlockSnapshot, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT achievement_id, unlocked_at FROM user_achievements WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListAchievementsForUser: %w", err)
	}
	defer rows.Close()

	unlockedTimes := make(map[AchievementID]time.Time, len(Catalogue))
	for rows.Next() {
		var id AchievementID
		var ts time.Time
		if err := rows.Scan(&id, &ts); err != nil {
			return nil, fmt.Errorf("ListAchievementsForUser: scan: %w", err)
		}
		unlockedTimes[id] = ts
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListAchievementsForUser: iter: %w", err)
	}

	out := &UnlockSnapshot{
		Unlocked: make([]UnlockedAchievement, 0, len(unlockedTimes)),
		Locked:   make([]Achievement, 0, len(Catalogue)),
	}
	for _, a := range Catalogue {
		if ts, ok := unlockedTimes[a.ID]; ok {
			out.Unlocked = append(out.Unlocked, UnlockedAchievement{Achievement: a, UnlockedAt: ts})
		} else {
			out.Locked = append(out.Locked, a)
		}
	}
	sort.Slice(out.Unlocked, func(i, j int) bool {
		return out.Unlocked[i].UnlockedAt.After(out.Unlocked[j].UnlockedAt)
	})
	return out, nil
}

// PlayerStatsSnapshot is the subset of stats the evaluator consumes. Keeping
// it as a plain struct makes it cheap to construct from any data source
// (queries, fixtures, tests).
type PlayerStatsSnapshot struct {
	GamesPlayed int64
	GamesWon    int64
	// HourUTC is the hour of the most recently finished game in UTC.
	// Negative values indicate "no recent game".
	HourUTC int
}

// EvaluateForStats returns the set of achievement IDs that the given stats
// satisfy. It does not touch the database; pair with PersistUnlocks to write
// any newly-unlocked rows.
func EvaluateForStats(s PlayerStatsSnapshot) []AchievementID {
	out := make([]AchievementID, 0, 4)
	if s.GamesWon >= 1 {
		out = append(out, AchFirstWin)
	}
	if s.GamesWon >= 10 {
		out = append(out, AchTenWins)
	}
	if s.GamesWon >= 50 {
		out = append(out, AchFiftyWins)
	}
	if s.GamesWon >= 100 {
		out = append(out, AchHundredWins)
	}
	if s.GamesPlayed >= 10 {
		out = append(out, AchTenPlayed)
	}
	if s.GamesPlayed >= 100 {
		out = append(out, AchHundredPlayed)
	}
	if s.GamesPlayed >= 20 {
		rate := float64(s.GamesWon) / float64(s.GamesPlayed)
		if rate >= 0.60 {
			out = append(out, AchWinRate60)
		}
		if rate >= 0.75 {
			out = append(out, AchWinRate75)
		}
	}
	if s.HourUTC >= 0 && s.HourUTC < 4 {
		out = append(out, AchNightOwl)
	}
	if s.HourUTC >= 5 && s.HourUTC < 8 {
		out = append(out, AchEarlyBird)
	}
	return out
}

// PersistUnlocks inserts new (user, achievement) rows for ids that the user
// has not yet unlocked. Returns the IDs that were newly inserted.
func PersistUnlocks(ctx context.Context, db *sql.DB, userID int64, ids []AchievementID) ([]AchievementID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("PersistUnlocks: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO user_achievements (user_id, achievement_id)
		VALUES (?, ?)
		ON CONFLICT(user_id, achievement_id) DO NOTHING
	`)
	if err != nil {
		return nil, fmt.Errorf("PersistUnlocks: prepare: %w", err)
	}
	defer stmt.Close()

	newlyUnlocked := make([]AchievementID, 0, len(ids))
	for _, id := range ids {
		if _, ok := catalogueByID[id]; !ok {
			continue
		}
		res, err := stmt.ExecContext(ctx, userID, id)
		if err != nil {
			return nil, fmt.Errorf("PersistUnlocks: exec %s: %w", id, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			newlyUnlocked = append(newlyUnlocked, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("PersistUnlocks: commit: %w", err)
	}
	return newlyUnlocked, nil
}

// EvaluateAndPersistForUser pulls fresh stats from the scoreboard table for
// userID, evaluates achievements, and persists any new unlocks. Returns the
// IDs newly granted in this call so callers can broadcast a notification.
func EvaluateAndPersistForUser(ctx context.Context, db *sql.DB, userID int64) ([]AchievementID, error) {
	var snap PlayerStatsSnapshot
	err := db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS games_played,
			SUM(CASE WHEN position = 1 THEN 1 ELSE 0 END) AS games_won
		FROM scoreboard WHERE user_id = ?
	`, userID).Scan(&snap.GamesPlayed, &snap.GamesWon)
	if err != nil {
		return nil, fmt.Errorf("EvaluateAndPersistForUser: stats: %w", err)
	}

	// Hour of last finished game (UTC). Optional; -1 if none.
	snap.HourUTC = -1
	var lastTs sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT MAX(created_at) FROM scoreboard WHERE user_id = ?
	`, userID).Scan(&lastTs)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("EvaluateAndPersistForUser: last_ts: %w", err)
	}
	if lastTs.Valid {
		snap.HourUTC = lastTs.Time.UTC().Hour()
	}

	candidates := EvaluateForStats(snap)
	return PersistUnlocks(ctx, db, userID, candidates)
}
