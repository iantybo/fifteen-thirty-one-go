package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// PresenceStatus represents user presence information
type PresenceStatus struct {
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	Status         string    `json:"status"` // online, away, in_game, offline
	LastActive     time.Time `json:"last_active"`
	CurrentLobbyID *int64    `json:"current_lobby_id,omitempty"`
	AvatarURL      *string   `json:"avatar_url,omitempty"`
}

// UpdatePresence handles PUT /api/users/presence
func UpdatePresence(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			// Backwards compatible: some middleware sets "user_id".
			if v, exists := c.Get("user_id"); exists && v != nil {
				if id, ok2 := v.(int64); ok2 {
					userID = id
					ok = true
				}
			}
		}
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			Status string `json:"status" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
			return
		}

		// Validate status
		validStatuses := map[string]bool{
			"online":  true,
			"away":    true,
			"in_game": true,
			"offline": true,
		}
		if !validStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status (must be online, away, in_game, or offline)"})
			return
		}

		// Update or insert presence
		_, err := db.Exec(`
			INSERT INTO user_presence (user_id, status, last_active)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET
				status = excluded.status,
				last_active = CURRENT_TIMESTAMP
		`, userID, req.Status)
		if err != nil {
			log.Printf("Error updating presence: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Get updated presence info
		var presence PresenceStatus
		var currentLobbyID sql.NullInt64
		var avatarURL sql.NullString
		var username string
		err = db.QueryRow(`
			SELECT u.id, u.username, u.avatar_url,
			       COALESCE(up.status, 'offline'),
			       COALESCE(up.last_active, CURRENT_TIMESTAMP),
			       up.current_lobby_id
			FROM users u
			LEFT JOIN user_presence up ON up.user_id = u.id
			WHERE u.id = ?
		`, userID).Scan(&presence.UserID, &username, &avatarURL, &presence.Status, &presence.LastActive, &currentLobbyID)
		if err != nil {
			log.Printf("Error getting presence: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		presence.Username = username
		if avatarURL.Valid {
			presence.AvatarURL = &avatarURL.String
		}
		if currentLobbyID.Valid {
			presence.CurrentLobbyID = &currentLobbyID.Int64
		}

		// Broadcast presence change to global lobby
		hub, ok := hubProvider()
		if ok && hub != nil {
			hub.Broadcast("lobby:global", "player:presence_changed", presence)
		}

		c.JSON(http.StatusOK, presence)
	}
}

// GetPresence handles GET /api/users/:id/presence
func GetPresence(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("id")
		var userID int64
		if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil || userID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		var presence PresenceStatus
		var currentLobbyID sql.NullInt64
		var avatarURL sql.NullString
		var username string
		err := db.QueryRow(`
			SELECT u.id, u.username, u.avatar_url, COALESCE(up.status, 'offline'), COALESCE(up.last_active, u.created_at), up.current_lobby_id
			FROM users u
			LEFT JOIN user_presence up ON up.user_id = u.id
			WHERE u.id = ?
		`, userID).Scan(&presence.UserID, &username, &avatarURL, &presence.Status, &presence.LastActive, &currentLobbyID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if err != nil {
			log.Printf("Error getting presence: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		presence.Username = username
		if avatarURL.Valid {
			presence.AvatarURL = &avatarURL.String
		}
		if currentLobbyID.Valid {
			presence.CurrentLobbyID = &currentLobbyID.Int64
		}

		c.JSON(http.StatusOK, presence)
	}
}

// HeartbeatPresence handles POST /api/users/presence/heartbeat
func HeartbeatPresence(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			if v, exists := c.Get("user_id"); exists && v != nil {
				if id, ok2 := v.(int64); ok2 {
					userID = id
					ok = true
				}
			}
		}
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Update last_active timestamp
		_, err := db.Exec(`
			INSERT INTO user_presence (user_id, status, last_active)
			VALUES (?, 'online', CURRENT_TIMESTAMP)
			ON CONFLICT(user_id) DO UPDATE SET
				last_active = CURRENT_TIMESTAMP,
				status = CASE WHEN user_presence.status = 'offline' THEN 'online' ELSE user_presence.status END
		`, userID)
		if err != nil {
			log.Printf("Error updating presence heartbeat: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}
