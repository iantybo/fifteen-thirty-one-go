package handlers

import (
	"database/sql"
	"strconv"

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
}


