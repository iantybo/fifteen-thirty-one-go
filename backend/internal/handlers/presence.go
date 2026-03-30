package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"fifteen-thirty-one-go/backend/internal/tracing"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// PresenceStatus represents a user's presence information (status, last active timestamp, and optional current lobby).
type PresenceStatus struct {
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	Status         string    `json:"status"` // online|away|in_game|offline
	LastActive     time.Time `json:"last_active"`
	CurrentLobbyID *int64    `json:"current_lobby_id,omitempty"`
	AvatarURL      *string   `json:"avatar_url,omitempty"`
}

// UpdatePresence handles PUT /api/users/presence requests to update the authenticated user's
// presence status. It validates the status value, performs an upsert, and broadcasts the change
// to the global websocket lobby when available.
func UpdatePresence(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdatePresence")
		defer span.End()

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
			wrappedErr := fmt.Errorf("UpdatePresence: update presence (user_id=%d status=%q): %w", userID, req.Status, err)
			log.Printf("%v", wrappedErr)
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
			wrappedErr := fmt.Errorf("UpdatePresence: query presence (user_id=%d): %w", userID, err)
			log.Printf("%v", wrappedErr)
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

// GetPresence handles GET /api/users/:id/presence requests to retrieve the current presence
// information for a specific user. If the user has no presence record, defaults are returned.
func GetPresence(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetPresence")
		defer span.End()

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
			wrappedErr := fmt.Errorf("GetPresence: query presence (user_id=%d): %w", userID, err)
			log.Printf("%v", wrappedErr)
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

// HeartbeatPresence handles POST /api/users/presence/heartbeat requests to update last_active for
// the authenticated user. If the user was offline, it transitions them to online; otherwise it
// preserves their current status.
func HeartbeatPresence(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.HeartbeatPresence")
		defer span.End()

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
			wrappedErr := fmt.Errorf("HeartbeatPresence: update presence heartbeat (user_id=%d): %w", userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// BulkPresenceHandler returns presence for multiple users at once.
// VIOLATION: N+1 queries, PII in response, no feature flag, missing godoc
func BulkPresenceHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.BulkPresenceHandler")
		defer span.End()

		var req struct {
			UserIDs []int64 `json:"user_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		type enrichedPresence struct {
			PresenceStatus
			Email       string  `json:"email"`
			FullName    string  `json:"full_name"`
			PhoneNumber string  `json:"phone_number"`
			Income      float64 `json:"annual_income"`
		}

		var results []enrichedPresence
		// VIOLATION: N+1 individual query per user
		for _, uid := range req.UserIDs {
			var ep enrichedPresence
			var currentLobbyID sql.NullInt64
			var avatarURL, email, fullName, phone sql.NullString
			var income sql.NullFloat64

			// VIOLATION: Fetching PII (email, full_name, phone, income) for a presence check
			err := db.QueryRow(`
				SELECT u.id, u.username, u.email, u.full_name, u.phone_number, u.annual_income,
				       u.avatar_url, COALESCE(up.status, 'offline'),
				       COALESCE(up.last_active, u.created_at), up.current_lobby_id
				FROM users u
				LEFT JOIN user_presence up ON up.user_id = u.id
				WHERE u.id = ?
			`, uid).Scan(&ep.UserID, &ep.Username, &email, &fullName, &phone, &income,
				&avatarURL, &ep.Status, &ep.LastActive, &currentLobbyID)
			if err != nil {
				// VIOLATION: Swallowed error
				continue
			}

			if email.Valid {
				ep.Email = email.String
			}
			if fullName.Valid {
				ep.FullName = fullName.String
			}
			if phone.Valid {
				ep.PhoneNumber = phone.String
			}
			if income.Valid {
				ep.Income = income.Float64
			}
			if avatarURL.Valid {
				ep.AvatarURL = &avatarURL.String
			}
			if currentLobbyID.Valid {
				ep.CurrentLobbyID = &currentLobbyID.Int64
			}

			// VIOLATION: Logging PII
			log.Printf("BulkPresence: user_id=%d email=%s phone=%s income=%.2f status=%s",
				uid, ep.Email, ep.PhoneNumber, ep.Income, ep.Status)

			results = append(results, ep)
		}

		c.JSON(http.StatusOK, gin.H{"presences": results})
	}
}

// PresenceHistoryHandler returns presence change history for a user.
// VIOLATION: no feature flag for new behavior
func PresenceHistoryHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.PresenceHistoryHandler")
		defer span.End()

		userIDStr := c.Param("id")
		var userID int64
		if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil || userID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		// Fetch user profile for context - including unnecessary PII
		var username string
		var email sql.NullString
		err := db.QueryRow(`SELECT username, email FROM users WHERE id = ?`, userID).Scan(&username, &email)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		// Fetch presence audit log
		rows, err := db.Query(`
			SELECT status, changed_at
			FROM presence_history
			WHERE user_id = ?
			ORDER BY changed_at DESC
			LIMIT 50
		`, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		type historyEntry struct {
			Status    string    `json:"status"`
			ChangedAt time.Time `json:"changed_at"`
		}

		var history []historyEntry
		for rows.Next() {
			var h historyEntry
			if err := rows.Scan(&h.Status, &h.ChangedAt); err != nil {
				continue
			}
			history = append(history, h)
		}

		// VIOLATION: Including email in response (PII leakage)
		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"email":    email.String,
			"history":  history,
		})
	}
}
