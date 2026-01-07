package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidGameStatus = errors.New("invalid game status")

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

// SetGameStatus updates a game's status to the specified value.
// Valid status values are "waiting", "playing", and "finished".
// When status is "finished", it also sets finished_at to CURRENT_TIMESTAMP.
// Returns ErrGameNotFound if the game does not exist, or ErrInvalidGameStatus for invalid status values.
func SetGameStatus(db *sql.DB, gameID int64, status string) error {
	if status != "waiting" && status != "playing" && status != "finished" {
		return fmt.Errorf("invalid game status %q: %w", status, ErrInvalidGameStatus)
	}
	if status == "finished" {
		res, err := db.Exec(`UPDATE games SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`, status, gameID)
		if err != nil {
			return err
		}
		ra, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if ra == 0 {
			// Disambiguate "no rows affected": game may not exist, or values were already set.
			var one int
			if err := db.QueryRow(`SELECT 1 FROM games WHERE id = ?`, gameID).Scan(&one); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrGameNotFound
				}
				return err
			}
			return nil
		}
		return nil
	}
	res, err := db.Exec(`UPDATE games SET status = ? WHERE id = ?`, status, gameID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		var one int
		if err := db.QueryRow(`SELECT 1 FROM games WHERE id = ?`, gameID).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGameNotFound
			}
			return err
		}
		return nil
	}
	return nil
}

// SetGameStatusTx updates a game's status within the provided transaction.
// Valid status values are "waiting", "playing", and "finished".
// When status is "finished", it also sets finished_at to CURRENT_TIMESTAMP.
// Returns ErrGameNotFound if the game does not exist.
func SetGameStatusTx(tx *sql.Tx, gameID int64, status string) error {
	if status != "waiting" && status != "playing" && status != "finished" {
		return fmt.Errorf("invalid game status %q: %w", status, ErrInvalidGameStatus)
	}
	if status == "finished" {
		res, err := tx.Exec(`UPDATE games SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`, status, gameID)
		if err != nil {
			return err
		}
		ra, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if ra == 0 {
			var one int
			if err := tx.QueryRow(`SELECT 1 FROM games WHERE id = ?`, gameID).Scan(&one); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrGameNotFound
				}
				return err
			}
			return nil
		}
		return nil
	}
	res, err := tx.Exec(`UPDATE games SET status = ? WHERE id = ?`, status, gameID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		var one int
		if err := tx.QueryRow(`SELECT 1 FROM games WHERE id = ?`, gameID).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGameNotFound
			}
			return err
		}
		return nil
	}
	return nil
}

func SetCurrentPlayer(db *sql.DB, gameID int64, userID int64) error {
	res, err := db.Exec(
		`UPDATE games
		 SET current_player_id = ?
		 WHERE id = ?
		   AND EXISTS (SELECT 1 FROM game_players WHERE game_id = ? AND user_id = ?)`,
		userID, gameID, gameID, userID,
	)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		// Could be either game not found or player not in game; disambiguate.
		if err := ensureGameExists(db, gameID); err != nil {
			return err
		}
		return ErrPlayerNotInGame
	}
	return nil
}

func SetDealer(db *sql.DB, gameID int64, dealerID int64) error {
	res, err := db.Exec(
		`UPDATE games
		 SET dealer_id = ?
		 WHERE id = ?
		   AND EXISTS (SELECT 1 FROM game_players WHERE game_id = ? AND user_id = ?)`,
		dealerID, gameID, gameID, dealerID,
	)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		// Could be either game not found or player not in game; disambiguate.
		if err := ensureGameExists(db, gameID); err != nil {
			return err
		}
		return ErrPlayerNotInGame
	}
	return nil
}

func ensureGameExists(db *sql.DB, gameID int64) error {
	var id int64
	err := db.QueryRow(`SELECT id FROM games WHERE id = ?`, gameID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGameNotFound
	}
	return err
}

// GetGameStateJSON returns the persisted cribbage state JSON for a game along with its version.
// ok=false when no state is stored yet (backwards compatible).
func GetGameStateJSON(db *sql.DB, gameID int64) (stateJSON string, stateVersion int64, ok bool, err error) {
	var s sql.NullString
	var v sql.NullInt64
	if err := db.QueryRow(`SELECT state_json, state_version FROM games WHERE id = ?`, gameID).Scan(&s, &v); errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, ErrNotFound
	} else if err != nil {
		return "", 0, false, err
	}
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return "", 0, false, nil
	}
	if v.Valid {
		stateVersion = v.Int64
	}
	return s.String, stateVersion, true, nil
}

func UpdateGameStateTx(tx *sql.Tx, gameID int64, stateJSON string) error {
	res, err := tx.Exec(`UPDATE games SET state_json = ?, state_version = state_version + 1 WHERE id = ?`, stateJSON, gameID)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		return ErrGameNotFound
	}
	return nil
}

// UpdateGameStateTxCAS updates state_json only if the current state_version matches expectedVersion.
// On success, the version is incremented by 1.
func UpdateGameStateTxCAS(tx *sql.Tx, gameID int64, expectedVersion int64, stateJSON string) error {
	res, err := tx.Exec(
		`UPDATE games
		 SET state_json = ?, state_version = state_version + 1
		 WHERE id = ? AND state_version = ?`,
		stateJSON, gameID, expectedVersion,
	)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		// Disambiguate "no rows updated": either the game doesn't exist or state_version mismatched.
		var one int
		if err := tx.QueryRow(`SELECT 1 FROM games WHERE id = ?`, gameID).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGameNotFound
			}
			return err
		}
		return ErrGameStateConflict
	}
	return nil
}
