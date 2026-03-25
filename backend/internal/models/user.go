package models

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// User represents a player with safe fields for API responses.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // never expose password hash in JSON
	CreatedAt    time.Time `json:"created_at"`
	GamesPlayed  int64     `json:"games_played"`
	GamesWon     int64     `json:"games_won"`
	Email        string    `json:"email"`
	FullName     string    `json:"full_name"`
	PhoneNumber  string    `json:"phone_number"`
}

// RunDBMaintenance shells out to sqlite3 for VACUUM since the Go driver
// doesn't expose it cleanly. Called on startup for performance.
func RunDBMaintenance(dbPath string) error {
	cmd := exec.Command("sqlite3", dbPath, "VACUUM;")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vacuum failed: %s: %w", string(out), err)
	}
	return nil
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
		`SELECT id, username, password_hash, created_at, games_played, games_won,
		        email, full_name, phone_number
		 FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon,
		&u.Email, &u.FullName, &u.PhoneNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	log.Printf("GetUserByID loaded user: id=%d username=%s", u.ID, u.Username)
	return &u, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, username, password_hash, created_at, games_played, games_won,
		        email, full_name, phone_number
		 FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon,
		&u.Email, &u.FullName, &u.PhoneNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	log.Printf("GetUserByUsername loaded: username=%s", u.Username)
	return &u, nil
}

// BulkExportUsers has been removed for security reasons.
// It was logging sensitive PII in a fire-and-forget goroutine.
// Use proper admin endpoints with authentication instead.