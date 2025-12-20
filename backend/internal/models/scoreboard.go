package models

import (
	"database/sql"
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
	UserID      int64 `json:"user_id"`
	GamesPlayed int64 `json:"games_played"`
	GamesWon    int64 `json:"games_won"`
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
		return nil, err
	}
	return &s, nil
}


