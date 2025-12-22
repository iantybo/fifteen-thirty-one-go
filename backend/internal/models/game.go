package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Game struct {
	ID              int64      `json:"id"`
	LobbyID         int64      `json:"lobby_id"`
	Status          string     `json:"status"` // waiting|playing|finished
	CurrentPlayerID *int64     `json:"current_player_id,omitempty"`
	DealerID        *int64     `json:"dealer_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

func CreateGame(db *sql.DB, lobbyID int64) (*Game, error) {
	res, err := db.Exec(`INSERT INTO games(lobby_id, status) VALUES (?, 'waiting')`, lobbyID)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetGameByID(db, id)
}

func GetGameByID(db *sql.DB, id int64) (*Game, error) {
	var g Game
	var current sql.NullInt64
	var dealer sql.NullInt64
	var finished sql.NullTime
	err := db.QueryRow(
		`SELECT id, lobby_id, status, current_player_id, dealer_id, created_at, finished_at FROM games WHERE id = ?`,
		id,
	).Scan(&g.ID, &g.LobbyID, &g.Status, &current, &dealer, &g.CreatedAt, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if current.Valid {
		v := current.Int64
		g.CurrentPlayerID = &v
	}
	if dealer.Valid {
		v := dealer.Int64
		g.DealerID = &v
	}
	if finished.Valid {
		v := finished.Time
		g.FinishedAt = &v
	}
	return &g, nil
}

func SetGameStatus(db *sql.DB, gameID int64, status string) error {
	if status != "waiting" && status != "playing" && status != "finished" {
		return errors.New("invalid status")
	}
	if status == "finished" {
		_, err := db.Exec(`UPDATE games SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`, status, gameID)
		return err
	}
	_, err := db.Exec(`UPDATE games SET status = ? WHERE id = ?`, status, gameID)
	return err
}

func SetCurrentPlayer(db *sql.DB, gameID int64, userID int64) error {
	if err := ensurePlayerInGame(db, gameID, userID); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE games SET current_player_id = ? WHERE id = ?`, userID, gameID)
	return err
}

func SetDealer(db *sql.DB, gameID int64, dealerID int64) error {
	if err := ensurePlayerInGame(db, gameID, dealerID); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE games SET dealer_id = ? WHERE id = ?`, dealerID, gameID)
	return err
}

func ensurePlayerInGame(db *sql.DB, gameID int64, userID int64) error {
	var one int
	err := db.QueryRow(`SELECT 1 FROM game_players WHERE game_id = ? AND user_id = ?`, gameID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPlayerNotInGame
	}
	return err
}

// GetGameStateJSON returns the persisted cribbage state JSON for a game.
// ok=false when no state is stored yet (backwards compatible).
func GetGameStateJSON(db *sql.DB, gameID int64) (stateJSON string, ok bool, err error) {
	var s sql.NullString
	if err := db.QueryRow(`SELECT state_json FROM games WHERE id = ?`, gameID).Scan(&s); errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNotFound
	} else if err != nil {
		return "", false, err
	}
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return "", false, nil
	}
	return s.String, true, nil
}

func UpdateGameStateTx(tx *sql.Tx, gameID int64, stateJSON string) error {
	_, err := tx.Exec(`UPDATE games SET state_json = ? WHERE id = ?`, stateJSON, gameID)
	return err
}


