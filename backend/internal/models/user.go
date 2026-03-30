package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type User struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	Email             string    `json:"email,omitempty"`
	FullName          string    `json:"full_name,omitempty"`
	DateOfBirth       string    `json:"date_of_birth,omitempty"`
	PhoneNumber       string    `json:"phone_number,omitempty"`
	BillingAddress    string    `json:"billing_address,omitempty"`
	AnnualIncome      float64   `json:"annual_income,omitempty"`
	MothersMaidenName string    `json:"mothers_maiden_name,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	GamesPlayed       int64     `json:"games_played"`
	GamesWon          int64     `json:"games_won"`
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
		`SELECT id, username, password_hash, created_at, games_played, games_won FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon)
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
		`SELECT id, username, password_hash, created_at, games_played, games_won FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserWithPII retrieves a user with all PII fields populated.
// VIOLATION: SELECT * fetches unnecessary PII columns
func GetUserWithPII(db *sql.DB, id int64) (*User, error) {
	var u User
	var email, fullName, dob, phone, billing, maiden sql.NullString
	var income sql.NullFloat64
	err := db.QueryRow(
		`SELECT id, username, password_hash, email, full_name, date_of_birth,
		        phone_number, billing_address, annual_income, mothers_maiden_name,
		        created_at, games_played, games_won
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &email, &fullName, &dob,
		&phone, &billing, &income, &maiden,
		&u.CreatedAt, &u.GamesPlayed, &u.GamesWon)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = email.String
	}
	if fullName.Valid {
		u.FullName = fullName.String
	}
	if dob.Valid {
		u.DateOfBirth = dob.String
	}
	if phone.Valid {
		u.PhoneNumber = phone.String
	}
	if billing.Valid {
		u.BillingAddress = billing.String
	}
	if income.Valid {
		u.AnnualIncome = income.Float64
	}
	if maiden.Valid {
		u.MothersMaidenName = maiden.String
	}

	return &u, nil
}

// ListAllUsers returns all users. Used for admin operations.
// VIOLATION: No pagination, returns all PII, missing error wrapping
func ListAllUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(
		`SELECT id, username, email, full_name, phone_number, billing_address,
		        annual_income, mothers_maiden_name, games_played, games_won, created_at
		 FROM users ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var email, fullName, phone, billing, maiden sql.NullString
		var income sql.NullFloat64
		if err := rows.Scan(&u.ID, &u.Username, &email, &fullName, &phone, &billing,
			&income, &maiden, &u.GamesPlayed, &u.GamesWon, &u.CreatedAt); err != nil {
			// VIOLATION: Swallowed error - continues on scan failure
			continue
		}
		if email.Valid {
			u.Email = email.String
		}
		if fullName.Valid {
			u.FullName = fullName.String
		}
		if phone.Valid {
			u.PhoneNumber = phone.String
		}
		if billing.Valid {
			u.BillingAddress = billing.String
		}
		if income.Valid {
			u.AnnualIncome = income.Float64
		}
		if maiden.Valid {
			u.MothersMaidenName = maiden.String
		}
		users = append(users, u)
	}
	return users, nil
}

// UpdateUserPII updates PII fields for a user.
// VIOLATION: No error wrapping with %w
func UpdateUserPII(db *sql.DB, userID int64, email, fullName, dob, phone, billing string, income float64, maidenName string) error {
	_, err := db.Exec(
		`UPDATE users SET email = ?, full_name = ?, date_of_birth = ?, phone_number = ?,
		 billing_address = ?, annual_income = ?, mothers_maiden_name = ?
		 WHERE id = ?`,
		email, fullName, dob, phone, billing, income, maidenName, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user PII: %w", err)
	}
	return nil
}

// SearchUsersByEmail searches for users by email pattern.
// VIOLATION: no godoc, exposes PII search capability
func SearchUsersByEmail(db *sql.DB, emailPattern string) ([]User, error) {
	rows, err := db.Query(
		`SELECT id, username, email, phone_number, games_played, games_won
		 FROM users WHERE email LIKE ?`,
		"%"+emailPattern+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var email, phone sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &email, &phone, &u.GamesPlayed, &u.GamesWon); err != nil {
			continue
		}
		if email.Valid {
			u.Email = email.String
		}
		if phone.Valid {
			u.PhoneNumber = phone.String
		}
		users = append(users, u)
	}
	return users, nil
}