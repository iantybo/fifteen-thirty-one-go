package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

// Signup validation errors. Handlers map these to 4xx responses.
var (
	ErrInvalidSignupEmail  = errors.New("invalid signup email")
	ErrInvalidSignupName   = errors.New("invalid signup name")
	ErrInvalidSignupNote   = errors.New("invalid signup note")
	ErrInvalidSignupStatus = errors.New("invalid signup status")
	ErrSignupEmailTaken    = errors.New("signup email already registered")
	ErrSignupNotFound      = errors.New("signup not found")
)

const (
	maxSignupEmailLen = 254 // RFC 5321 practical maximum
	maxSignupNameLen  = 80
	maxSignupNoteLen  = 500
	// maxSignupPageSize bounds list responses.
	maxSignupPageSize = 100
)

// SignupStatus values, mirroring the CHECK constraint in migration 008.
const (
	SignupPending  = "pending"
	SignupInvited  = "invited"
	SignupAccepted = "accepted"
	SignupRejected = "rejected"
)

// Signup is an expression of interest captured by the public signup form.
type Signup struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Note      string    `json:"note,omitempty"`
	Status    string    `json:"status"`
	UserID    *int64    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SignupInput carries the fields accepted from the signup form.
type SignupInput struct {
	Email string
	Name  string
	Note  string
}

// IsValidSignupStatus reports whether s is a permitted status value.
func IsValidSignupStatus(s string) bool {
	switch s {
	case SignupPending, SignupInvited, SignupAccepted, SignupRejected:
		return true
	}
	return false
}

// normalizeSignupEmail lowercases and validates an email address. Addresses are
// stored normalized so the UNIQUE constraint is effectively case-insensitive.
func normalizeSignupEmail(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > maxSignupEmailLen {
		return "", ErrInvalidSignupEmail
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", ErrInvalidSignupEmail
	}
	// Reject display-name forms ("Bob <bob@x.com>") so the stored value is a
	// bare address.
	if addr.Address != s {
		return "", ErrInvalidSignupEmail
	}
	// A bare address must contain exactly one "@" with content on both sides.
	at := strings.LastIndex(addr.Address, "@")
	if at <= 0 || at == len(addr.Address)-1 {
		return "", ErrInvalidSignupEmail
	}
	// Require a dot in the domain so obvious non-deliverables are rejected.
	if !strings.Contains(addr.Address[at+1:], ".") {
		return "", ErrInvalidSignupEmail
	}
	return strings.ToLower(addr.Address), nil
}

// hasControlChars reports whether s contains control characters, which would
// corrupt log output and rendering.
func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// normalizeSignupInput validates and normalizes a signup payload.
func normalizeSignupInput(in SignupInput) (SignupInput, error) {
	var out SignupInput
	var err error

	if out.Email, err = normalizeSignupEmail(in.Email); err != nil {
		return SignupInput{}, err
	}

	out.Name = strings.TrimSpace(in.Name)
	if out.Name == "" || len([]rune(out.Name)) > maxSignupNameLen || hasControlChars(out.Name) {
		return SignupInput{}, ErrInvalidSignupName
	}

	// Notes are optional; newlines are allowed but other controls are not.
	out.Note = strings.TrimSpace(in.Note)
	if len([]rune(out.Note)) > maxSignupNoteLen {
		return SignupInput{}, ErrInvalidSignupNote
	}
	for _, r := range out.Note {
		if (r < 0x20 && r != '\n' && r != '\r' && r != '\t') || r == 0x7f {
			return SignupInput{}, ErrInvalidSignupNote
		}
	}

	return out, nil
}

const signupColumns = `id, email, name, COALESCE(note, ''), status, user_id, created_at, updated_at`

func scanSignup(row interface {
	Scan(dest ...any) error
}) (*Signup, error) {
	var s Signup
	var userID sql.NullInt64
	if err := row.Scan(&s.ID, &s.Email, &s.Name, &s.Note, &s.Status, &userID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	if userID.Valid {
		id := userID.Int64
		s.UserID = &id
	}
	return &s, nil
}

// CreateSignup validates and inserts a signup.
func CreateSignup(ctx context.Context, db *sql.DB, in SignupInput) (*Signup, error) {
	n, err := normalizeSignupInput(in)
	if err != nil {
		return nil, err
	}

	res, err := db.ExecContext(ctx,
		`INSERT INTO signups(email, name, note) VALUES (?, ?, ?)`,
		n.Email, n.Name, nullIfEmpty(n.Note),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSignupEmailTaken
		}
		return nil, fmt.Errorf("insert signup: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id for signup: %w", err)
	}
	return GetSignup(ctx, db, id)
}

// GetSignup loads a single signup by id.
func GetSignup(ctx context.Context, db *sql.DB, id int64) (*Signup, error) {
	s, err := scanSignup(db.QueryRowContext(ctx, `SELECT `+signupColumns+` FROM signups WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSignupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan signup (id=%d): %w", id, err)
	}
	return s, nil
}

// ListSignups returns signups newest first, optionally filtered by status.
// limit is clamped to [1, maxSignupPageSize].
func ListSignups(ctx context.Context, db *sql.DB, status string, limit int) ([]Signup, error) {
	if status != "" && !IsValidSignupStatus(status) {
		return nil, ErrInvalidSignupStatus
	}
	if limit <= 0 || limit > maxSignupPageSize {
		limit = maxSignupPageSize
	}

	query := `SELECT ` + signupColumns + ` FROM signups`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query signups (status=%q): %w", status, err)
	}
	defer rows.Close()

	signups := []Signup{}
	for rows.Next() {
		s, err := scanSignup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan signup row: %w", err)
		}
		signups = append(signups, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signups: %w", err)
	}
	return signups, nil
}

// CountSignupsByStatus returns the number of signups per status.
func CountSignupsByStatus(ctx context.Context, db *sql.DB) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT status, COUNT(*) FROM signups GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count signups by status: %w", err)
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("scan signup count: %w", err)
		}
		out[status] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signup counts: %w", err)
	}
	return out, nil
}

// UpdateSignupStatus moves a signup to a new status.
func UpdateSignupStatus(ctx context.Context, db *sql.DB, id int64, status string) (*Signup, error) {
	if !IsValidSignupStatus(status) {
		return nil, ErrInvalidSignupStatus
	}

	res, err := db.ExecContext(ctx,
		`UPDATE signups SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update signup status (id=%d): %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("update signup status rows affected (id=%d): %w", id, err)
	}
	if affected == 0 {
		return nil, ErrSignupNotFound
	}
	return GetSignup(ctx, db, id)
}
