package models

import (
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

func ListLobbies(db *sql.DB) ([]Lobby, error) {
	rows, err := db.Query(`SELECT id, name, host_id, max_players, current_players, status, created_at FROM lobbies ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Lobby
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
	l, err := GetLobbyByID(db, lobbyID)
	if err != nil {
		return nil, err
	}
	if l.Status != "waiting" {
		return nil, errors.New("lobby not joinable")
	}
	if l.CurrentPlayers >= l.MaxPlayers {
		return nil, errors.New("lobby full")
	}
	res, err := db.Exec(`UPDATE lobbies SET current_players = current_players + 1 WHERE id = ? AND status = 'waiting' AND current_players < max_players`, lobbyID)
	if err != nil {
		return nil, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if ra == 0 {
		return nil, errors.New("lobby full")
	}
	return GetLobbyByID(db, lobbyID)
}


