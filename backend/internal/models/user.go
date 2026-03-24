package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type User struct {
	ID             int64     `json:"id"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"-"`
	WalletAddress  *string   `json:"wallet_address,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	GamesPlayed    int64     `json:"games_played"`
	GamesWon       int64     `json:"games_won"`
}

// WalletOnlyPasswordSentinel is a bcrypt hash used as password_hash for wallet-only users.
// No plaintext password matches it, so they cannot use password login.
const WalletOnlyPasswordSentinel = "$2a$10$CwTycUXWue0Thq9StjUM0uJ8lvZ9i8a9kaI0s5momkGLumZ5qX6e."

// NormalizeWalletAddress validates and returns the wallet address in lowercase for storage and lookup.
// Expects 0x followed by 40 hex digits.
func NormalizeWalletAddress(addr string) (string, error) {
	addr = strings.TrimSpace(strings.ToLower(addr))
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		return "", errors.New("invalid wallet address length or format")
	}
	for i := 2; i < 42; i++ {
		c := addr[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return "", errors.New("invalid wallet address: not hex")
	}
	return addr, nil
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

func CreateWalletUser(db *sql.DB, walletAddress, username string) (*User, error) {
	normalized, err := NormalizeWalletAddress(walletAddress)
	if err != nil {
		return nil, err
	}
	res, err := db.Exec(
		`INSERT INTO users(username, password_hash, wallet_address) VALUES (?, ?, ?)`,
		username, WalletOnlyPasswordSentinel, normalized,
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

func scanUserRow(row *sql.Row) (*User, error) {
	var u User
	var wallet sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &wallet, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon)
	if err != nil {
		return nil, err
	}
	if wallet.Valid && wallet.String != "" {
		u.WalletAddress = &wallet.String
	}
	return &u, nil
}

func GetUserByID(db *sql.DB, id int64) (*User, error) {
	row := db.QueryRow(
		`SELECT id, username, password_hash, wallet_address, created_at, games_played, games_won FROM users WHERE id = ?`,
		id,
	)
	u, err := scanUserRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	row := db.QueryRow(
		`SELECT id, username, password_hash, wallet_address, created_at, games_played, games_won FROM users WHERE username = ?`,
		username,
	)
	u, err := scanUserRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByWalletAddress(db *sql.DB, walletAddress string) (*User, error) {
	normalized, err := NormalizeWalletAddress(walletAddress)
	if err != nil {
		return nil, err
	}
	row := db.QueryRow(
		`SELECT id, username, password_hash, wallet_address, created_at, games_played, games_won FROM users WHERE wallet_address = ?`,
		normalized,
	)
	u, err := scanUserRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}
