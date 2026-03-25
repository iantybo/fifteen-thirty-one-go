package handlers

import (
	"database/sql"
	"strconv"

	ws "fifteen-thirty-one-go/backend/pkg/websocket"
)

// --- WebSocket room name helpers ---

// GameRoom returns the canonical room name for a game's realtime channel.
func GameRoom(gameID int64) string {
	return "game:" + strconv.FormatInt(gameID, 10)
}

// LobbyRoom returns the canonical room name for a specific lobby's channel.
func LobbyRoom(lobbyID int64) string {
	return "lobby:" + strconv.FormatInt(lobbyID, 10)
}

// GlobalLobbyRoom is the room used for server-wide presence broadcasts.
const GlobalLobbyRoom = "lobby:global"

// --- Hub wiring ---

// hubProvider is set by main at startup so HTTP handlers can broadcast realtime updates.
var hubProvider func() (*ws.Hub, bool)

func SetHubProvider(p func() (*ws.Hub, bool)) {
	hubProvider = p
}

func broadcastGameUpdate(db *sql.DB, gameID int64) {
	if hubProvider == nil {
		return
	}
	hub, ok := hubProvider()
	if !ok || hub == nil {
		return
	}
	snap, err := BuildGameSnapshotPublic(db, gameID)
	if err != nil {
		return
	}
	hub.Broadcast(GameRoom(gameID), "game_update", snap)
}
