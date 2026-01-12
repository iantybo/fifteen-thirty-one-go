package handlers

import (
	"database/sql"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"
)

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
	hub.Broadcast("game:"+strconv.FormatInt(gameID, 10), "game_update", snap)

	// Also broadcast to global scoreboard when game state changes
	broadcastGlobalScoreboard(db)
}

// broadcastGlobalScoreboard sends an update to all clients subscribed to the global scoreboard
// This is called whenever a game starts, finishes, or has a significant state change
func broadcastGlobalScoreboard(db *sql.DB) {
	if hubProvider == nil {
		return
	}
	hub, ok := hubProvider()
	if !ok || hub == nil {
		return
	}

	games, err := models.ListActiveGames(db)
	if err != nil {
		return
	}

	hub.Broadcast("scoreboard:global", "scoreboard_update", map[string]interface{}{
		"games": games,
	})
}
