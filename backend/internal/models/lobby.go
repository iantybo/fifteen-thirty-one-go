package models

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Lobby struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	HostID         int64     `json:"host_id"`
	MaxPlayers     int64     `json:"max_players"`
	CurrentPlayers int64     `json:"current_players"`
	Status         string    `json:"status"` // waiting|in_progress|finished
	CreatedAt      time.Time `json:"created_at"`
}

func CreateLobby(db *sql.DB, name string, hostID int64, maxPlayers int64) (*Lobby, error) {
	res, err := db.Exec(
		`INSERT INTO lobbies(name, host_id, max_players, current_players, status) VALUES (?, ?, ?, 1, 'waiting')`,
		name, hostID, maxPlayers,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetLobbyByID(db, id)
}

func GetLobbyByID(db *sql.DB, id int64) (*Lobby, error) {
	var l Lobby
	err := db.QueryRow(
		`SELECT id, name, host_id, max_players, current_players, status, created_at FROM lobbies WHERE id = ?`,
		id,
	).Scan(&l.ID, &l.Name, &l.HostID, &l.MaxPlayers, &l.CurrentPlayers, &l.Status, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func ListLobbies(db *sql.DB, limit, offset int64) ([]Lobby, error) {
	// Defensive defaults/caps to prevent unbounded reads.
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := db.Query(
		`SELECT id, name, host_id, max_players, current_players, status, created_at
		 FROM lobbies
		 WHERE status != 'finished'
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Lobby, 0)
	for rows.Next() {
		var l Lobby
		if err := rows.Scan(&l.ID, &l.Name, &l.HostID, &l.MaxPlayers, &l.CurrentPlayers, &l.Status, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// JoinLobby increments current_players if possible.
func JoinLobby(db *sql.DB, lobbyID int64) (*Lobby, error) {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	l, err := JoinLobbyTx(tx, lobbyID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return l, nil
}

// JoinLobbyTx increments current_players if possible, within a transaction.
// This allows callers to rollback the increment if subsequent steps fail.
func JoinLobbyTx(tx *sql.Tx, lobbyID int64) (*Lobby, error) {
	// Note: SQLite doesn't support SELECT ... FOR UPDATE. We rely on a write
	// statement inside this transaction to acquire the relevant lock before we
	// classify the failure, so concurrent joins can't skew the diagnosis.
	res, err := tx.Exec(`UPDATE lobbies SET current_players = current_players + 1 WHERE id = ? AND status = 'waiting' AND current_players < max_players`, lobbyID)
	if err != nil {
		return nil, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if ra == 0 {
		// Inspect the lobby under the same transaction to classify why the guarded
		// increment didn't apply.
		var status string
		var currentPlayers int64
		var maxPlayers int64
		err := tx.QueryRow(`SELECT status, current_players, max_players FROM lobbies WHERE id = ?`, lobbyID).Scan(&status, &currentPlayers, &maxPlayers)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if status != "waiting" {
			return nil, ErrLobbyNotJoinable
		}
		if currentPlayers >= maxPlayers {
			return nil, ErrLobbyFull
		}
		return nil, errors.New("unable to join lobby")
	}

	var l Lobby
	err = tx.QueryRow(
		`SELECT id, name, host_id, max_players, current_players, status, created_at FROM lobbies WHERE id = ?`,
		lobbyID,
	).Scan(&l.ID, &l.Name, &l.HostID, &l.MaxPlayers, &l.CurrentPlayers, &l.Status, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// DecrementLobbyCurrentPlayers decrements current_players by 1, but never below 0.
// Used as a compensating action when a join flow fails after incrementing.
func DecrementLobbyCurrentPlayers(db *sql.DB, lobbyID int64) error {
	_, err := db.Exec(
		`UPDATE lobbies
		 SET current_players = CASE WHEN current_players > 0 THEN current_players - 1 ELSE 0 END
		 WHERE id = ?`,
		lobbyID,
	)
	return err
}

func SetLobbyStatus(db *sql.DB, lobbyID int64, status string) error {
	if status != "waiting" && status != "in_progress" && status != "finished" {
		return errors.New("invalid status")
	}
	res, err := db.Exec(`UPDATE lobbies SET status = ? WHERE id = ?`, status, lobbyID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		// Disambiguate "no rows affected": lobby may not exist, or values were already set.
		var one int
		if err := db.QueryRow(`SELECT 1 FROM lobbies WHERE id = ?`, lobbyID).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		return nil
	}
	return nil
}

// SetLobbyStatusTx updates a lobby's status within the provided transaction.
// Valid status values are "waiting", "in_progress", and "finished".
// Returns ErrNotFound if the lobby does not exist.
func SetLobbyStatusTx(tx *sql.Tx, lobbyID int64, status string) error {
	if status != "waiting" && status != "in_progress" && status != "finished" {
		return errors.New("invalid status")
	}
	res, err := tx.Exec(`UPDATE lobbies SET status = ? WHERE id = ?`, status, lobbyID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		var one int
		if err := tx.QueryRow(`SELECT 1 FROM lobbies WHERE id = ?`, lobbyID).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		return nil
	}
	return nil
}
