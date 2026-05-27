package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidMode is returned when an invalid auto-count mode is provided.
var ErrInvalidMode = errors.New("invalid mode")

// ErrInvalidCardTheme is returned when an invalid card theme is provided.
var ErrInvalidCardTheme = errors.New("invalid card theme")

// DefaultCardTheme is the card theme used when no preference is set.
const DefaultCardTheme = "classic"

// validCardThemes enumerates allowed card_theme values.
// IMPORTANT: Keep in sync with the CHECK constraint in
// migrations/007_card_theme.sql.
var validCardThemes = map[string]struct{}{
	"classic": {},
	"neon":    {},
	"minimal": {},
	"emoji":   {},
}

// IsValidCardTheme returns true if the given theme is in the allowed set.
// Valid themes: classic, neon, minimal, emoji.
func IsValidCardTheme(theme string) bool {
	_, ok := validCardThemes[theme]
	return ok
}

type UserPreferences struct {
	UserID        int64     `json:"user_id"`
	AutoCountMode string    `json:"auto_count_mode"` // off|suggest|auto
	CardTheme     string    `json:"card_theme"`      // classic|neon|minimal|emoji
	UpdatedAt     time.Time `json:"updated_at"`
}

func GetUserPreferences(db *sql.DB, userID int64) (*UserPreferences, error) {
	var p UserPreferences
	err := db.QueryRow(`SELECT user_id, auto_count_mode, card_theme, updated_at FROM user_preferences WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.AutoCountMode, &p.CardTheme, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &UserPreferences{UserID: userID, AutoCountMode: "suggest", CardTheme: DefaultCardTheme, UpdatedAt: time.Now().UTC()}, nil
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

	var p UserPreferences
	err = tx.QueryRow(`SELECT user_id, auto_count_mode, card_theme, updated_at FROM user_preferences WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.AutoCountMode, &p.CardTheme, &p.UpdatedAt)
	if err != nil {
		// Extremely defensive: after an upsert, the row should exist.
		// Preserve GetUserPreferences semantics if it somehow doesn't.
		if errors.Is(err, sql.ErrNoRows) {
			// Preserve the upsert; commit even though the SELECT returned no rows.
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit user_preferences tx: %w", err)
			}
			return &UserPreferences{UserID: userID, AutoCountMode: mode, CardTheme: DefaultCardTheme, UpdatedAt: time.Now().UTC()}, nil
		}
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit user_preferences tx: %w", err)
	}
	return &p, nil
}

// UpdateUserPreferencesTx atomically updates the provided preference fields
// (auto-count mode and/or card theme) in a single transaction and returns the
// resulting preferences. Passing nil for a field leaves it unchanged.
// Returns ErrInvalidMode or ErrInvalidCardTheme if any provided value is invalid.
func UpdateUserPreferencesTx(db *sql.DB, userID int64, mode *string, theme *string) (*UserPreferences, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid userID: %d", userID)
	}
	if mode == nil && theme == nil {
		// Nothing to update; just return the current preferences.
		return GetUserPreferences(db, userID)
	}
	if mode != nil && *mode != "off" && *mode != "suggest" && *mode != "auto" {
		return nil, ErrInvalidMode
	}
	if theme != nil && !IsValidCardTheme(*theme) {
		return nil, ErrInvalidCardTheme
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	// Resolve effective insert values: provided values override defaults so
	// that a brand-new row receives a sane preference for the untouched field.
	insertMode := "suggest"
	if mode != nil {
		insertMode = *mode
	}
	insertTheme := DefaultCardTheme
	if theme != nil {
		insertTheme = *theme
	}

	// Build ON CONFLICT update clause to touch only the provided fields.
	updateClause := ""
	if mode != nil {
		updateClause += "auto_count_mode = excluded.auto_count_mode, "
	}
	if theme != nil {
		updateClause += "card_theme = excluded.card_theme, "
	}
	updateClause += "updated_at = datetime('now','utc')"

	stmt := `INSERT INTO user_preferences(user_id, auto_count_mode, card_theme) VALUES (?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET ` + updateClause

	if _, err := tx.Exec(stmt, userID, insertMode, insertTheme); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	var p UserPreferences
	err = tx.QueryRow(`SELECT user_id, auto_count_mode, card_theme, updated_at FROM user_preferences WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.AutoCountMode, &p.CardTheme, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit user_preferences tx: %w", err)
			}
			return &UserPreferences{UserID: userID, AutoCountMode: insertMode, CardTheme: insertTheme, UpdatedAt: time.Now().UTC()}, nil
		}
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit user_preferences tx: %w", err)
	}
	return &p, nil
}

// SetUserCardThemeAndGetPreferencesTx updates the user's card theme preference and
// returns the updated preferences, atomically.
func SetUserCardThemeAndGetPreferencesTx(db *sql.DB, userID int64, theme string) (*UserPreferences, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid userID: %d", userID)
	}
	if !IsValidCardTheme(theme) {
		return nil, ErrInvalidCardTheme
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(
		`INSERT INTO user_preferences(user_id, card_theme) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET card_theme = excluded.card_theme, updated_at = datetime('now','utc')`,
		userID, theme,
	); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	var p UserPreferences
	err = tx.QueryRow(`SELECT user_id, auto_count_mode, card_theme, updated_at FROM user_preferences WHERE user_id = ?`, userID).
		Scan(&p.UserID, &p.AutoCountMode, &p.CardTheme, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit user_preferences tx: %w", err)
			}
			return &UserPreferences{UserID: userID, AutoCountMode: "suggest", CardTheme: theme, UpdatedAt: time.Now().UTC()}, nil
		}
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit user_preferences tx: %w", err)
	}
	return &p, nil
}
