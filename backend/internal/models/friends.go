// Package models defines the persistence layer for application entities.
// This file implements a lightweight friends/blocked-users feature:
//
//   - send a friend request from user A to user B
//   - accept or decline an incoming request
//   - list incoming, outgoing, and accepted relationships
//   - remove an accepted friendship
//   - block / unblock a user (one-directional)
//
// Relationships are stored in two tables: friendships (one row per pair,
// canonicalised so user_a < user_b) and user_blocks (asymmetric, one row per
// block direction). Callers are expected to initialise the schema once at
// startup via EnsureFriendsSchema.
package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// FriendshipStatus enumerates the lifecycle states of a friendship row.
type FriendshipStatus string

const (
	FriendshipPending  FriendshipStatus = "pending"
	FriendshipAccepted FriendshipStatus = "accepted"
	FriendshipDeclined FriendshipStatus = "declined"
)

// Friendship is the canonical representation of a friendship row. The
// "Requester" is the user who initiated the request; "Other" is the
// counterparty. For accepted friendships either user may be the requester.
type Friendship struct {
	ID          int64            `json:"id"`
	RequesterID int64            `json:"requester_id"`
	OtherID     int64            `json:"other_id"`
	Status      FriendshipStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// FriendSummary is a lightweight view used by list endpoints.
type FriendSummary struct {
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	Since     time.Time `json:"since"`
}

// FriendRequestView represents a pending request from the recipient's PoV.
type FriendRequestView struct {
	ID          int64     `json:"id"`
	FromUserID  int64     `json:"from_user_id"`
	FromUsername string   `json:"from_username"`
	CreatedAt   time.Time `json:"created_at"`
}

// Errors returned by the friends API.
var (
	ErrSelfFriendship       = errors.New("friends: cannot befriend yourself")
	ErrAlreadyFriends       = errors.New("friends: already friends")
	ErrRequestExists        = errors.New("friends: pending request already exists")
	ErrBlocked              = errors.New("friends: relationship is blocked")
	ErrRequestNotFound      = errors.New("friends: request not found")
	ErrNotAuthorized        = errors.New("friends: not authorized")
)

// EnsureFriendsSchema creates the tables this module depends on if they do
// not already exist. Safe to call repeatedly.
func EnsureFriendsSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS friendships (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			requester_id INTEGER NOT NULL,
			other_id INTEGER NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('pending','accepted','declined')),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			-- Enforce canonical ordering so a pair appears at most once.
			CHECK(requester_id <> other_id),
			UNIQUE(requester_id, other_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_friendships_other ON friendships(other_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_friendships_requester ON friendships(requester_id, status)`,
		`CREATE TABLE IF NOT EXISTS user_blocks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			blocker_id INTEGER NOT NULL,
			blocked_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(blocker_id, blocked_id),
			CHECK(blocker_id <> blocked_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked ON user_blocks(blocked_id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("EnsureFriendsSchema: %w", err)
		}
	}
	return nil
}

// SendFriendRequest creates a pending row from requester to other. Returns
// ErrAlreadyFriends if the pair are already accepted friends, ErrRequestExists
// if there is already a pending row in either direction, or ErrBlocked if
// either party has blocked the other.
func SendFriendRequest(ctx context.Context, db *sql.DB, requesterID, otherID int64) (*Friendship, error) {
	if requesterID == otherID {
		return nil, ErrSelfFriendship
	}

	if blocked, err := isEitherBlocked(ctx, db, requesterID, otherID); err != nil {
		return nil, err
	} else if blocked {
		return nil, ErrBlocked
	}

	// Existing relationship check, in either direction.
	existing, err := findFriendshipEitherDirection(ctx, db, requesterID, otherID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		switch existing.Status {
		case FriendshipAccepted:
			return nil, ErrAlreadyFriends
		case FriendshipPending:
			return nil, ErrRequestExists
		case FriendshipDeclined:
			// Allow re-requesting after a decline: bump status and timestamps.
			_, err := db.ExecContext(ctx, `
				UPDATE friendships
				SET requester_id = ?, other_id = ?, status = 'pending', updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, requesterID, otherID, existing.ID)
			if err != nil {
				return nil, fmt.Errorf("SendFriendRequest: re-request: %w", err)
			}
			return loadFriendshipByID(ctx, db, existing.ID)
		}
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO friendships (requester_id, other_id, status)
		VALUES (?, ?, 'pending')
	`, requesterID, otherID)
	if err != nil {
		return nil, fmt.Errorf("SendFriendRequest: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return loadFriendshipByID(ctx, db, id)
}

// AcceptFriendRequest transitions a pending request to accepted. The actor
// must be the recipient (i.e. the "other_id" on the row).
func AcceptFriendRequest(ctx context.Context, db *sql.DB, requestID, actorID int64) (*Friendship, error) {
	f, err := loadFriendshipByID(ctx, db, requestID)
	if err != nil {
		return nil, err
	}
	if f.Status != FriendshipPending {
		return nil, ErrRequestNotFound
	}
	if f.OtherID != actorID {
		return nil, ErrNotAuthorized
	}
	_, err = db.ExecContext(ctx, `
		UPDATE friendships SET status = 'accepted', updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, requestID)
	if err != nil {
		return nil, fmt.Errorf("AcceptFriendRequest: %w", err)
	}
	return loadFriendshipByID(ctx, db, requestID)
}

// DeclineFriendRequest transitions a pending request to declined. The actor
// must be the recipient.
func DeclineFriendRequest(ctx context.Context, db *sql.DB, requestID, actorID int64) error {
	f, err := loadFriendshipByID(ctx, db, requestID)
	if err != nil {
		return err
	}
	if f.Status != FriendshipPending {
		return ErrRequestNotFound
	}
	if f.OtherID != actorID {
		return ErrNotAuthorized
	}
	_, err = db.ExecContext(ctx, `
		UPDATE friendships SET status = 'declined', updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, requestID)
	if err != nil {
		return fmt.Errorf("DeclineFriendRequest: %w", err)
	}
	return nil
}

// RemoveFriend deletes the accepted friendship between two users. It is a
// no-op if no accepted friendship exists.
func RemoveFriend(ctx context.Context, db *sql.DB, actorID, otherID int64) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM friendships
		WHERE status = 'accepted'
		  AND ((requester_id = ? AND other_id = ?) OR (requester_id = ? AND other_id = ?))
	`, actorID, otherID, otherID, actorID)
	if err != nil {
		return fmt.Errorf("RemoveFriend: %w", err)
	}
	return nil
}

// ListFriends returns all accepted friendships for userID.
func ListFriends(ctx context.Context, db *sql.DB, userID int64) ([]FriendSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.username, u.avatar_url, f.updated_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.requester_id = ? THEN f.other_id ELSE f.requester_id END
		WHERE f.status = 'accepted' AND (f.requester_id = ? OR f.other_id = ?)
		ORDER BY u.username COLLATE NOCASE ASC
	`, userID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("ListFriends: %w", err)
	}
	defer rows.Close()

	out := make([]FriendSummary, 0, 16)
	for rows.Next() {
		var s FriendSummary
		var avatar sql.NullString
		if err := rows.Scan(&s.UserID, &s.Username, &avatar, &s.Since); err != nil {
			return nil, fmt.Errorf("ListFriends: scan: %w", err)
		}
		if avatar.Valid {
			s.AvatarURL = &avatar.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListIncomingRequests returns pending requests addressed to userID.
func ListIncomingRequests(ctx context.Context, db *sql.DB, userID int64) ([]FriendRequestView, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT f.id, f.requester_id, u.username, f.created_at
		FROM friendships f
		JOIN users u ON u.id = f.requester_id
		WHERE f.status = 'pending' AND f.other_id = ?
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListIncomingRequests: %w", err)
	}
	defer rows.Close()

	out := make([]FriendRequestView, 0, 8)
	for rows.Next() {
		var r FriendRequestView
		if err := rows.Scan(&r.ID, &r.FromUserID, &r.FromUsername, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListIncomingRequests: scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListOutgoingRequests returns pending requests initiated by userID.
func ListOutgoingRequests(ctx context.Context, db *sql.DB, userID int64) ([]FriendRequestView, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT f.id, f.other_id, u.username, f.created_at
		FROM friendships f
		JOIN users u ON u.id = f.other_id
		WHERE f.status = 'pending' AND f.requester_id = ?
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListOutgoingRequests: %w", err)
	}
	defer rows.Close()

	out := make([]FriendRequestView, 0, 8)
	for rows.Next() {
		var r FriendRequestView
		if err := rows.Scan(&r.ID, &r.FromUserID, &r.FromUsername, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListOutgoingRequests: scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BlockUser establishes a one-directional block from blocker to blocked, and
// removes any pending or accepted friendship between them.
func BlockUser(ctx context.Context, db *sql.DB, blockerID, blockedID int64) error {
	if blockerID == blockedID {
		return ErrSelfFriendship
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("BlockUser: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id)
		VALUES (?, ?)
		ON CONFLICT(blocker_id, blocked_id) DO NOTHING
	`, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("BlockUser: insert: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM friendships
		WHERE (requester_id = ? AND other_id = ?) OR (requester_id = ? AND other_id = ?)
	`, blockerID, blockedID, blockedID, blockerID)
	if err != nil {
		return fmt.Errorf("BlockUser: clean friendships: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("BlockUser: commit: %w", err)
	}
	return nil
}

// UnblockUser removes a block, if one exists.
func UnblockUser(ctx context.Context, db *sql.DB, blockerID, blockedID int64) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM user_blocks WHERE blocker_id = ? AND blocked_id = ?
	`, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("UnblockUser: %w", err)
	}
	return nil
}

// ListBlocked returns all users blocked by blockerID.
func ListBlocked(ctx context.Context, db *sql.DB, blockerID int64) ([]FriendSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.username, u.avatar_url, b.created_at
		FROM user_blocks b
		JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = ?
		ORDER BY u.username COLLATE NOCASE ASC
	`, blockerID)
	if err != nil {
		return nil, fmt.Errorf("ListBlocked: %w", err)
	}
	defer rows.Close()

	out := make([]FriendSummary, 0, 8)
	for rows.Next() {
		var s FriendSummary
		var avatar sql.NullString
		if err := rows.Scan(&s.UserID, &s.Username, &avatar, &s.Since); err != nil {
			return nil, fmt.Errorf("ListBlocked: scan: %w", err)
		}
		if avatar.Valid {
			s.AvatarURL = &avatar.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// AreFriends reports whether a and b have an accepted friendship.
func AreFriends(ctx context.Context, db *sql.DB, a, b int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM friendships
		WHERE status = 'accepted'
		  AND ((requester_id = ? AND other_id = ?) OR (requester_id = ? AND other_id = ?))
	`, a, b, b, a).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("AreFriends: %w", err)
	}
	return n > 0, nil
}

// internal helpers --------------------------------------------------------

func loadFriendshipByID(ctx context.Context, db *sql.DB, id int64) (*Friendship, error) {
	var f Friendship
	err := db.QueryRowContext(ctx, `
		SELECT id, requester_id, other_id, status, created_at, updated_at
		FROM friendships WHERE id = ?
	`, id).Scan(&f.ID, &f.RequesterID, &f.OtherID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loadFriendshipByID: %w", err)
	}
	return &f, nil
}

func findFriendshipEitherDirection(ctx context.Context, db *sql.DB, a, b int64) (*Friendship, error) {
	var f Friendship
	err := db.QueryRowContext(ctx, `
		SELECT id, requester_id, other_id, status, created_at, updated_at
		FROM friendships
		WHERE (requester_id = ? AND other_id = ?) OR (requester_id = ? AND other_id = ?)
		LIMIT 1
	`, a, b, b, a).Scan(&f.ID, &f.RequesterID, &f.OtherID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func isEitherBlocked(ctx context.Context, db *sql.DB, a, b int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_blocks
		WHERE (blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)
	`, a, b, b, a).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("isEitherBlocked: %w", err)
	}
	return n > 0, nil
}
