package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidMode = errors.New("invalid mode")

type UserPreferences struct {
	UserID        int64  `json:"user_id"`
	AutoCountMode string `json:"auto_count_mode"` // off|suggest|auto
	// ActiveDeck is the selected deck skin: "builtin:<id>", a custom deck id, or
	// "" for the default deck.
	ActiveDeck string    `json:"active_deck"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// preferencesSelect reads every preference column. active_deck is nullable, so
// it is coalesced to the empty string ("use the default deck").
const preferencesSelect = `SELECT user_id, auto_count_mode, COALESCE(active_deck, ''), updated_at
	FROM user_preferences WHERE user_id = ?`

// scanPreferencesRow scans a preferencesSelect row. It works with both *sql.Row
// and *sql.Tx rows.
func scanPreferencesRow(row interface {
	Scan(dest ...any) error
}) (*UserPreferences, error) {
	var p UserPreferences
	if err := row.Scan(&p.UserID, &p.AutoCountMode, &p.ActiveDeck, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// defaultPreferences returns the values used when a user has no stored row.
func defaultPreferences(userID int64) *UserPreferences {
	return &UserPreferences{
		UserID:        userID,
		AutoCountMode: "suggest",
		ActiveDeck:    "",
		UpdatedAt:     time.Now().UTC(),
	}
}

// GetUserPreferences returns the stored preferences for userID. If no row
// exists it returns the default preferences rather than sql.ErrNoRows.
func GetUserPreferences(db *sql.DB, userID int64) (*UserPreferences, error) {
	p, err := scanPreferencesRow(db.QueryRow(preferencesSelect, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return defaultPreferences(userID), nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func SetUserAutoCountMode(db *sql.DB, userID int64, mode string) error {
	if mode != "off" && mode != "suggest" && mode != "auto" {
		return ErrInvalidMode
	}
	_, err := db.Exec(
		`INSERT INTO user_preferences(user_id, auto_count_mode) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET auto_count_mode = excluded.auto_count_mode, updated_at = datetime('now','utc')`,
		userID, mode,
	)
	return err
}

// SetUserAutoCountModeAndGetPreferencesTx updates the user's auto-count preference and
// then returns the updated preferences, atomically.
func SetUserAutoCountModeAndGetPreferencesTx(db *sql.DB, userID int64, mode string) (*UserPreferences, error) {
	if mode != "off" && mode != "suggest" && mode != "auto" {
		return nil, ErrInvalidMode
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(
		`INSERT INTO user_preferences(user_id, auto_count_mode) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET auto_count_mode = excluded.auto_count_mode, updated_at = datetime('now','utc')`,
		userID, mode,
	); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	p, err := scanPreferencesRow(tx.QueryRow(preferencesSelect, userID))
	if err != nil {
		// Extremely defensive: after an upsert, the row should exist.
		// Preserve GetUserPreferences semantics if it somehow doesn't.
		if errors.Is(err, sql.ErrNoRows) {
			// Preserve the upsert; commit even though the SELECT returned no rows.
			// (This should be unreachable, but keeps DB changes consistent with caller intent.)
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit user_preferences tx: %w", err)
			}
			p := defaultPreferences(userID)
			p.AutoCountMode = mode
			return p, nil
		}
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}
