package models

import (
	"database/sql"
	"errors"
)

func IsUserInGame(db *sql.DB, userID int64, gameID int64) (bool, error) {
	var exists int
	err := db.QueryRow(
		`SELECT 1 FROM game_players WHERE game_id = ? AND user_id = ? LIMIT 1`,
		gameID, userID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
