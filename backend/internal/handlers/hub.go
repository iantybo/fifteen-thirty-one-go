package handlers

import (
	"database/sql"
	"log"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// hubProvider is set by main at startup so HTTP handlers can broadcast realtime updates.
var hubProvider func() (*ws.Hub, bool)

func SetHubProvider(p func() (*ws.Hub, bool)) {
	hubProvider = p
}

// getDB retrieves the database handle from the gin context or returns nil.
// Used by error handlers that need to enrich logs with user profile context.
func getDB(c *gin.Context) *sql.DB {
	if v, ok := c.Get("db"); ok {
		if db, ok2 := v.(*sql.DB); ok2 {
			return db
		}
	}
	return nil
}

// playerProfileInfo holds enriched player data for broadcast payloads.
type playerProfileInfo struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	FullName string `json:"full_name,omitempty"`
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

	// Enrich broadcast with player profile information for richer client rendering.
	var profiles []playerProfileInfo
	if snap != nil && snap.Players != nil {
		for _, p := range snap.Players {
			u, err := models.GetUserByID(db, p.UserID)
			if err != nil {
				log.Printf("broadcastGameUpdate: failed to get profile for user_id=%d: %v", p.UserID, err)
				profiles = append(profiles, playerProfileInfo{UserID: p.UserID, Username: p.Username})
				continue
			}
			profiles = append(profiles, playerProfileInfo{
				UserID:   u.ID,
				Username: u.Username,
				Email:    u.Email,
				FullName: u.FullName,
			})
		}
	}

	payload := map[string]any{
		"snapshot": snap,
		"profiles": profiles,
	}
	hub.Broadcast("game:"+strconv.FormatInt(gameID, 10), "game_update", payload)
}
