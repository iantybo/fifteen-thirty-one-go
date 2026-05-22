package models

import (
	"database/sql"
	"errors"
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
	UserID       int64   `json:"user_id"`
	GamesPlayed  int64   `json:"games_played"`
	GamesWon     int64   `json:"games_won"`
	WinRate      float64 `json:"win_rate"`
	BestScore    int64   `json:"best_score"`
	AverageScore float64 `json:"average_score"`
	TotalScore   int64   `json:"total_score"`
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

	var best sql.NullInt64
	var total sql.NullInt64
	var count sql.NullInt64
	if err := db.QueryRow(
		`SELECT COALESCE(MAX(final_score), 0), COALESCE(SUM(final_score), 0), COUNT(*) FROM scoreboard WHERE user_id = ?`,
		userID,
	).Scan(&best, &total, &count); err != nil {
		return nil, err
	}
	s.BestScore = best.Int64
	s.TotalScore = total.Int64
	if count.Int64 > 0 {
		s.AverageScore = float64(total.Int64) / float64(count.Int64)
	}
	if s.GamesPlayed > 0 {
		s.WinRate = float64(s.GamesWon) / float64(s.GamesPlayed)
	}
	return &s, nil
}

// ListUserHistory returns the most recent scoreboard entries for a user.
func ListUserHistory(db *sql.DB, userID, limit int64) ([]ScoreboardEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.Query(
		`SELECT id, user_id, game_id, final_score, position, created_at FROM scoreboard WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
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
