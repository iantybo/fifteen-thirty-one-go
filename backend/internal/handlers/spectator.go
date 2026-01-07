package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// SpectatorInfo represents a user spectating a lobby.
type SpectatorInfo struct {
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	JoinedAt  time.Time `json:"joined_at"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

// JoinAsSpectator handles POST /api/lobbies/:id/spectate and adds the authenticated user as a spectator.
func JoinAsSpectator(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
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

		lobbyIDStr := c.Param("id")
		lobbyID, err := strconv.ParseInt(lobbyIDStr, 10, 64)
		if err != nil || lobbyID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lobby id"})
			return
		}

		ctx := c.Request.Context()

		// Check if lobby exists and allows spectators
		var allowSpectators bool
		var lobbyStatus string
		err = db.QueryRowContext(ctx, `
			SELECT allow_spectators, status
			FROM lobbies
			WHERE id = ?
		`, lobbyID).Scan(&allowSpectators, &lobbyStatus)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "lobby not found"})
			return
		}
		if err != nil {
			wrappedErr := fmt.Errorf("JoinAsSpectator: checking lobby (lobby_id=%d): %w", lobbyID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if lobbyStatus == "finished" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot spectate a finished lobby"})
			return
		}

		if !allowSpectators {
			c.JSON(http.StatusForbidden, gin.H{"error": "this lobby does not allow spectators"})
			return
		}

		// Check if user is already a player in this lobby
		var playerCount int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE g.lobby_id = ? AND gp.user_id = ? AND g.status IN ('waiting', 'in_progress')
		`, lobbyID, userID).Scan(&playerCount)
		if err != nil {
			log.Printf("Error checking player status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if playerCount > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "you are already a player in this lobby"})
			return
		}

		// Get username
		var username string
		var avatarURL sql.NullString
		err = db.QueryRowContext(ctx, "SELECT username, avatar_url FROM users WHERE id = ?", userID).Scan(&username, &avatarURL)
		if err != nil {
			log.Printf("Error getting user info: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Insert spectator (ON CONFLICT DO NOTHING for idempotency)
		_, err = db.ExecContext(ctx, `
			INSERT INTO lobby_spectators (lobby_id, user_id)
			VALUES (?, ?)
			ON CONFLICT(lobby_id, user_id) DO NOTHING
		`, lobbyID, userID)
		if err != nil {
			log.Printf("Error inserting spectator: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		var joinedAt time.Time
		// If the spectator already existed (ON CONFLICT DO NOTHING), use the stored joined_at.
		// If this read fails unexpectedly, surface it rather than masking DB issues.
		if err := db.QueryRowContext(ctx, `SELECT joined_at FROM lobby_spectators WHERE lobby_id = ? AND user_id = ?`, lobbyID, userID).Scan(&joinedAt); err != nil {
			log.Printf("Error retrieving spectator joined_at (lobby_id=%d user_id=%d): %v", lobbyID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		spectator := SpectatorInfo{
			UserID:   userID,
			Username: username,
			JoinedAt: joinedAt,
		}
		if avatarURL.Valid {
			spectator.AvatarURL = &avatarURL.String
		}

		// Broadcast spectator joined event
		hub, ok := hubProvider()
		if ok && hub != nil {
			hub.Broadcast(fmt.Sprintf("lobby:%d", lobbyID), "lobby:spectator_joined", spectator)

			// Send system message
			_ = SendSystemMessage(ctx, db, hub, lobbyID, fmt.Sprintf("%s is now spectating", username), "join")
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "spectator": spectator})
	}
}

// LeaveAsSpectator handles DELETE /api/lobbies/:id/spectate and removes the authenticated user from spectators.
func LeaveAsSpectator(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
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

		lobbyIDStr := c.Param("id")
		lobbyID, err := strconv.ParseInt(lobbyIDStr, 10, 64)
		if err != nil || lobbyID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lobby id"})
			return
		}

		ctx := c.Request.Context()

		// Get username before deleting
		var username string
		err = db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			log.Printf("Error getting username: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Delete spectator
		result, err := db.ExecContext(ctx, `
			DELETE FROM lobby_spectators
			WHERE lobby_id = ? AND user_id = ?
		`, lobbyID, userID)
		if err != nil {
			log.Printf("Error removing spectator: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "you are not spectating this lobby"})
			return
		}

		// Broadcast spectator left event
		hub, ok := hubProvider()
		if ok && hub != nil {
			hub.Broadcast(fmt.Sprintf("lobby:%d", lobbyID), "lobby:spectator_left", map[string]any{
				"user_id":  userID,
				"username": username,
			})

			// Send system message
			_ = SendSystemMessage(ctx, db, hub, lobbyID, fmt.Sprintf("%s stopped spectating", username), "leave")
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// GetSpectators handles GET /api/lobbies/:id/spectators and returns the lobby's current spectator list.
func GetSpectators(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lobbyIDStr := c.Param("id")
		lobbyID, err := strconv.ParseInt(lobbyIDStr, 10, 64)
		if err != nil || lobbyID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lobby id"})
			return
		}

		ctx := c.Request.Context()
		rows, err := db.QueryContext(ctx, `
			SELECT ls.user_id, u.username, ls.joined_at, u.avatar_url
			FROM lobby_spectators ls
			JOIN users u ON u.id = ls.user_id
			WHERE ls.lobby_id = ?
			ORDER BY ls.joined_at ASC
		`, lobbyID)
		if err != nil {
			log.Printf("Error querying spectators: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		defer rows.Close()

		spectators := []SpectatorInfo{}
		for rows.Next() {
			var spec SpectatorInfo
			var avatarURL sql.NullString
			err := rows.Scan(&spec.UserID, &spec.Username, &spec.JoinedAt, &avatarURL)
			if err != nil {
				log.Printf("Error scanning spectator: %v", err)
				continue
			}
			if avatarURL.Valid {
				spec.AvatarURL = &avatarURL.String
			}
			spectators = append(spectators, spec)
		}

		c.JSON(http.StatusOK, gin.H{"spectators": spectators})
	}
}
