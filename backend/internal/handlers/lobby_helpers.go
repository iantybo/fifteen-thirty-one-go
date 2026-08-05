package handlers

import (
	"context"
	"database/sql"
	"strconv"
)

// lobbyRoomKey returns the websocket room name for a given lobby.
// Centralised so the format ("lobby:<id>") cannot drift between handlers and
// so callers avoid repeated fmt.Sprintf allocations on hot broadcast paths.
func lobbyRoomKey(lobbyID int64) string {
	return "lobby:" + strconv.FormatInt(lobbyID, 10)
}

// getUsername looks up a user's username by id. Returns sql.ErrNoRows if the
// user does not exist.
func getUsername(ctx context.Context, db *sql.DB, userID int64) (string, error) {
	var username string
	err := db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	return username, err
}

// getUserDisplay looks up a user's username and avatar URL by id.
func getUserDisplay(ctx context.Context, db *sql.DB, userID int64) (string, sql.NullString, error) {
	var username string
	var avatar sql.NullString
	err := db.QueryRowContext(ctx, "SELECT username, avatar_url FROM users WHERE id = ?", userID).Scan(&username, &avatar)
	return username, avatar, err
}

// isActiveLobbyMember reports whether userID is a player in an active
// (waiting or in_progress) game within the given lobby.
func isActiveLobbyMember(ctx context.Context, db *sql.DB, lobbyID, userID int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_players gp
		JOIN games g ON g.id = gp.game_id
		WHERE g.lobby_id = ? AND gp.user_id = ? AND g.status IN ('waiting', 'in_progress')
	`, lobbyID, userID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
