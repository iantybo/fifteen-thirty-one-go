package models

import (
	"database/sql"
	"errors"
	"math"
	"time"
)

type ScoreboardEntry struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	GameID     int64     `json:"game_id"`
	FinalScore int64     `json:"final_score"`
	Position   int64     `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserStats struct {
	UserID      int64   `json:"user_id"`
	GamesPlayed int64   `json:"games_played"`
	GamesWon    int64   `json:"games_won"`
	WinRate     float64 `json:"win_rate"` // all-time [0..1]
	// CurrentWinStreak is the number of consecutive wins ending with the
	// player's most recent finished game (0 if the last game was a loss).
	CurrentWinStreak int64 `json:"current_win_streak"`
	// LongestWinStreak is the longest run of consecutive wins ever recorded.
	LongestWinStreak int64 `json:"longest_win_streak"`
	// BestScore is the highest final score the player has recorded.
	BestScore int64 `json:"best_score"`
	// AverageScore is the mean final score across finished games, rounded to
	// one decimal place.
	AverageScore float64 `json:"average_score"`
}

// computeStreaks walks a slice of finished-game results ordered from oldest to
// newest and returns the player's current and longest win streaks. A result of
// true represents a win.
func computeStreaks(wins []bool) (current, longest int64) {
	var run int64
	for _, won := range wins {
		if won {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	return run, longest
}

func InsertScoreboardEntry(db *sql.DB, userID, gameID, finalScore, position int64) (*ScoreboardEntry, error) {
	res, err := db.Exec(
		`INSERT INTO scoreboard(user_id, game_id, final_score, position) VALUES (?, ?, ?, ?)`,
		userID, gameID, finalScore, position,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	var e ScoreboardEntry
	if err := db.QueryRow(
		`SELECT id, user_id, game_id, final_score, position, created_at FROM scoreboard WHERE id = ?`,
		id,
	).Scan(&e.ID, &e.UserID, &e.GameID, &e.FinalScore, &e.Position, &e.CreatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

func ListScoreboard(db *sql.DB, limit int64) ([]ScoreboardEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.Query(
		`SELECT id, user_id, game_id, final_score, position, created_at FROM scoreboard ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScoreboardEntry
	for rows.Next() {
		var e ScoreboardEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.GameID, &e.FinalScore, &e.Position, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func GetUserStats(db *sql.DB, userID int64) (*UserStats, error) {
	var s UserStats
	s.UserID = userID
	if err := db.QueryRow(`SELECT games_played, games_won FROM users WHERE id = ?`, userID).Scan(&s.GamesPlayed, &s.GamesWon); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if s.GamesPlayed > 0 {
		s.WinRate = math.Round(float64(s.GamesWon)/float64(s.GamesPlayed)*1000) / 1000
	}

	// Derive streak and scoring records from the per-game scoreboard history,
	// ordered oldest-to-newest so the trailing run is the current streak.
	rows, err := db.Query(
		`SELECT final_score, position FROM scoreboard WHERE user_id = ? ORDER BY game_id ASC, id ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wins := make([]bool, 0)
	var totalScore int64
	var count int64
	for rows.Next() {
		var finalScore, position int64
		if err := rows.Scan(&finalScore, &position); err != nil {
			return nil, err
		}
		wins = append(wins, position == 1)
		totalScore += finalScore
		if count == 0 || finalScore > s.BestScore {
			s.BestScore = finalScore
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s.CurrentWinStreak, s.LongestWinStreak = computeStreaks(wins)
	if count > 0 {
		s.AverageScore = math.Round(float64(totalScore)/float64(count)*10) / 10
	}
	return &s, nil
}
