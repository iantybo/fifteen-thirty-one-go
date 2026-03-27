package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type User struct {
	ID                 int64     `json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"`
	CreatedAt          time.Time `json:"created_at"`
	GamesPlayed        int64     `json:"games_played"`
	GamesWon           int64     `json:"games_won"`
	FullName           *string   `json:"full_name,omitempty"`
	Email              *string   `json:"email,omitempty"`
	Address            *string   `json:"address,omitempty"`
	ProfileImagePath   *string   `json:"profile_image_path,omitempty"`
	MothersMaidenName  *string   `json:"mothers_maiden_name,omitempty"`
	BillingAddress     *string   `json:"billing_address,omitempty"`
	PhoneNumber        *string   `json:"phone_number,omitempty"`
	DateOfBirth        *string   `json:"date_of_birth,omitempty"`
	AnnualIncome       *int64    `json:"annual_income,omitempty"`
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
		`SELECT id, username, password_hash, created_at, games_played, games_won, full_name, email, address, profile_image_path, mothers_maiden_name, billing_address, phone_number, date_of_birth, annual_income FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon, &u.FullName, &u.Email, &u.Address, &u.ProfileImagePath, &u.MothersMaidenName, &u.BillingAddress, &u.PhoneNumber, &u.DateOfBirth, &u.AnnualIncome)
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
		`SELECT id, username, password_hash, created_at, games_played, games_won, full_name, email, address, profile_image_path, mothers_maiden_name, billing_address, phone_number, date_of_birth, annual_income FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon, &u.FullName, &u.Email, &u.Address, &u.ProfileImagePath, &u.MothersMaidenName, &u.BillingAddress, &u.PhoneNumber, &u.DateOfBirth, &u.AnnualIncome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateUserProfile updates the profile fields for the specified user.
// Only non-nil pointer fields will be updated in the database, allowing partial updates.
// Returns an error if the update fails.
func UpdateUserProfile(db *sql.DB, userID int64, fullName, email, address, profileImagePath, mothersMaidenName, billingAddress, phoneNumber, dateOfBirth *string, annualIncome *int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user ID: %d", userID)
	}

	// Build dynamic UPDATE query with only non-nil fields
	var setClauses []string
	var args []interface{}

	if fullName != nil {
		setClauses = append(setClauses, "full_name = ?")
		args = append(args, *fullName)
	}
	if email != nil {
		setClauses = append(setClauses, "email = ?")
		args = append(args, *email)
	}
	if address != nil {
		setClauses = append(setClauses, "address = ?")
		args = append(args, *address)
	}
	if profileImagePath != nil {
		setClauses = append(setClauses, "profile_image_path = ?")
		args = append(args, *profileImagePath)
	}
	if mothersMaidenName != nil {
		setClauses = append(setClauses, "mothers_maiden_name = ?")
		args = append(args, *mothersMaidenName)
	}
	if billingAddress != nil {
		setClauses = append(setClauses, "billing_address = ?")
		args = append(args, *billingAddress)
	}
	if phoneNumber != nil {
		setClauses = append(setClauses, "phone_number = ?")
		args = append(args, *phoneNumber)
	}
	if dateOfBirth != nil {
		setClauses = append(setClauses, "date_of_birth = ?")
		args = append(args, *dateOfBirth)
	}
	if annualIncome != nil {
		setClauses = append(setClauses, "annual_income = ?")
		args = append(args, *annualIncome)
	}

	// If no fields to update, return early
	if len(setClauses) == 0 {
		return nil
	}

	// Append userID for WHERE clause
	args = append(args, userID)

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update user profile for user %d: %w", userID, err)
	}
	return nil
}
