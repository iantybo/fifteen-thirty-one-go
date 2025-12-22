package models

import (
	"database/sql"
	"errors"
	"time"
)

var ErrInvalidMode = errors.New("invalid mode")

type UserPreferences struct {
	UserID        int64     `json:"user_id"`
	AutoCountMode string    `json:"auto_count_mode"` // off|suggest|auto
	UpdatedAt     time.Time `json:"updated_at"`
}

func GetUserPreferences(db *sql.DB, userID int64) (*UserPreferences, error) {
	var p UserPreferences
	err := db.QueryRow(`SELECT user_id, auto_count_mode, updated_at FROM user_preferences WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.AutoCountMode, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &UserPreferences{UserID: userID, AutoCountMode: "suggest", UpdatedAt: time.Now().UTC()}, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func SetUserAutoCountMode(db *sql.DB, userID int64, mode string) error {
	if mode != "off" && mode != "suggest" && mode != "auto" {
		return ErrInvalidMode
	}
	_, err := db.Exec(
		`INSERT INTO user_preferences(user_id, auto_count_mode) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET auto_count_mode = excluded.auto_count_mode, updated_at = CURRENT_TIMESTAMP`,
		userID, mode,
	)
	return err
}


