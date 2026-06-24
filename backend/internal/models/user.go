package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User represents an application user account and its game stats.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	GamesPlayed  int64     `json:"games_played"`
	GamesWon     int64     `json:"games_won"`
	Coins        int64     `json:"coins"`
}

func CreateUser(db *sql.DB, username, passwordHash string) (*User, error) {
	res, err := db.Exec(
		`INSERT INTO users(username, password_hash) VALUES (?, ?)`,
		username, passwordHash,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetUserByID(db, id)
}

func GetUserByID(db *sql.DB, id int64) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, username, password_hash, created_at, games_played, games_won, coins FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon, &u.Coins)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, username, password_hash, created_at, games_played, games_won, coins FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon, &u.Coins)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// AwardCoins increments the coins balance for the specified user by the given amount.
// Returns ErrNotFound if the user does not exist.
func AwardCoins(db *sql.DB, userID int64, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("amount must be non-negative: %d", amount)
	}
	res, err := db.Exec(`UPDATE users SET coins = coins + ? WHERE id = ?`, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update coins for user %d: %w", userID, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}

// AwardCoinsTx increments the coins balance for the specified user by the given amount
// within the provided transaction. Returns ErrNotFound if the user does not exist.
func AwardCoinsTx(tx *sql.Tx, userID int64, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("amount must be non-negative: %d", amount)
	}
	res, err := tx.Exec(`UPDATE users SET coins = coins + ? WHERE id = ?`, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update coins for user %d in transaction: %w", userID, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}

// AwardCoinsTxContext increments the coins balance for the specified user by the given amount
// within the provided transaction with context propagation. Returns ErrNotFound if the user does not exist.
func AwardCoinsTxContext(ctx context.Context, tx *sql.Tx, userID int64, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("amount must be non-negative: %d", amount)
	}
	res, err := tx.ExecContext(ctx, `UPDATE users SET coins = coins + ? WHERE id = ?`, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update coins for user %d in transaction: %w", userID, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}
