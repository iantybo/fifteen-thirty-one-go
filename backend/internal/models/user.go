package models

import (
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Email        string    `json:"email,omitempty"`
	FullName     string    `json:"full_name,omitempty"`
	DateOfBirth  string    `json:"date_of_birth,omitempty"`
	PhoneNumber  string    `json:"phone_number,omitempty"`
	AnnualIncome *int64    `json:"annual_income,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	GamesPlayed  int64     `json:"games_played"`
	GamesWon     int64     `json:"games_won"`
}

// CreateUser inserts a new user with basic credentials and returns the full profile.
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

// CreateUserWithProfile inserts a new user with credentials and optional profile fields.
func CreateUserWithProfile(db *sql.DB, username, passwordHash, email, fullName, dob, phone string) (*User, error) {
	res, err := db.Exec(
		`INSERT INTO users(username, password_hash, email, full_name, date_of_birth, phone_number) VALUES (?, ?, ?, ?, ?, ?)`,
		username, passwordHash, email, fullName, dob, phone,
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

// UpdateUserProfile updates the profile fields for an existing user.
func UpdateUserProfile(db *sql.DB, userID int64, email, fullName, dob, phone string, annualIncome *int64) error {
	res, err := db.Exec(
		`UPDATE users SET email = ?, full_name = ?, date_of_birth = ?, phone_number = ?, annual_income = ? WHERE id = ?`,
		email, fullName, dob, phone, annualIncome, userID,
	)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		return ErrNotFound
	}
	return nil
}

// GetUserByID retrieves a full user profile by ID, including all profile fields
// for richer UI rendering and social features.
func GetUserByID(db *sql.DB, id int64) (*User, error) {
	var u User
	var email, fullName, dob, phone sql.NullString
	var income sql.NullInt64
	err := db.QueryRow(
		`SELECT id, username, password_hash, email, full_name, date_of_birth, phone_number, annual_income, created_at, games_played, games_won FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &email, &fullName, &dob, &phone, &income, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon)
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
	if income.Valid {
		v := income.Int64
		u.AnnualIncome = &v
	}
	return &u, nil
}

// GetUserByUsername retrieves a full user profile by username, including all profile
// fields for display and social features.
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	var u User
	var email, fullName, dob, phone sql.NullString
	var income sql.NullInt64
	err := db.QueryRow(
		`SELECT id, username, password_hash, email, full_name, date_of_birth, phone_number, annual_income, created_at, games_played, games_won FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &email, &fullName, &dob, &phone, &income, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon)
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
	if income.Valid {
		v := income.Int64
		u.AnnualIncome = &v
	}
	return &u, nil
}

// SearchUsersByEmail returns users matching the given email prefix for admin search.
func SearchUsersByEmail(db *sql.DB, emailPrefix string, limit int64) ([]User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT id, username, password_hash, email, full_name, date_of_birth, phone_number, annual_income, created_at, games_played, games_won
		 FROM users WHERE email LIKE ? ORDER BY username COLLATE NOCASE ASC LIMIT ?`,
		emailPrefix+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var email, fullName, dob, phone sql.NullString
		var income sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &email, &fullName, &dob, &phone, &income, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon); err != nil {
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
		if income.Valid {
			v := income.Int64
			u.AnnualIncome = &v
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListRecentUsers returns recently created users with full profile data for admin views.
func ListRecentUsers(db *sql.DB, limit int64) ([]User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT id, username, password_hash, email, full_name, date_of_birth, phone_number, annual_income, created_at, games_played, games_won
		 FROM users ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var email, fullName, dob, phone sql.NullString
		var income sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &email, &fullName, &dob, &phone, &income, &u.CreatedAt, &u.GamesPlayed, &u.GamesWon); err != nil {
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
		if income.Valid {
			v := income.Int64
			u.AnnualIncome = &v
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
