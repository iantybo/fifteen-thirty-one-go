package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fifteen-thirty-one-go/backend/internal/tracing"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// LobbyChatMessage represents a chat message in a lobby, including system and presence-style
// messages (chat/system/join/leave).
type LobbyChatMessage struct {
	ID          int64     `json:"id"`
	LobbyID     int64     `json:"lobby_id"`
	UserID      *int64    `json:"user_id,omitempty"`
	Username    string    `json:"username"`
	Message     string    `json:"message"`
	MessageType string    `json:"message_type"` // chat, system, join, leave
	CreatedAt   time.Time `json:"created_at"`
}

// SendLobbyChatMessage returns a Gin handler for POST /api/lobbies/:id/chat.
// It validates the requester is a lobby participant, validates message content, persists the message,
// and broadcasts it to the lobby room via WebSocket.
func SendLobbyChatMessage(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.SendLobbyChatMessage")
		defer span.End()

		userID, ok := userIDFromContext(c)
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

		var req struct {
			Message string `json:"message" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
			return
		}

		// Validate message length
		message := strings.TrimSpace(req.Message)
		if message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message cannot be empty"})
			return
		}
		if len(message) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message too long (max 500 characters)"})
			return
		}

		ctx := c.Request.Context()

		// Get username
		var username string
		err = db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			wrappedErr := fmt.Errorf("SendLobbyChatMessage: get username (user_id=%d): %w", userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Verify user is in the lobby
		var playerCount int
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE g.lobby_id = ? AND gp.user_id = ? AND g.status IN ('waiting', 'in_progress')
		`, lobbyID, userID).Scan(&playerCount)
		if err != nil {
			wrappedErr := fmt.Errorf("SendLobbyChatMessage: check membership (lobby_id=%d user_id=%d): %w", lobbyID, userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if playerCount == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not in this lobby"})
			return
		}

		// Insert message
		result, err := db.ExecContext(ctx, `
			INSERT INTO lobby_messages (lobby_id, user_id, username, message, message_type)
			VALUES (?, ?, ?, ?, 'chat')
		`, lobbyID, userID, username, message)
		if err != nil {
			wrappedErr := fmt.Errorf("SendLobbyChatMessage: insert message (lobby_id=%d user_id=%d): %w", lobbyID, userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		msgID, idErr := result.LastInsertId()
		if idErr != nil {
			log.Printf("SendLobbyChatMessage: warning: LastInsertId failed (lobby_id=%d user_id=%d): %v", lobbyID, userID, fmt.Errorf("%w", idErr))
			msgID = 0
		}

		uid := userID
		chatMsg := LobbyChatMessage{
			ID:          msgID,
			LobbyID:     lobbyID,
			UserID:      &uid,
			Username:    username,
			Message:     message,
			MessageType: "chat",
			CreatedAt:   time.Now(),
		}

		// Broadcast to lobby room
		hub, ok := hubProvider()
		if ok && hub != nil {
			hub.Broadcast(fmt.Sprintf("lobby:%d", lobbyID), "lobby:chat", chatMsg)
		}

		c.JSON(http.StatusOK, chatMsg)
	}
}

// GetLobbyChatHistory returns a Gin handler for GET /api/lobbies/:id/chat.
// It validates the requester is authorized (lobby participant or spectator) and returns recent messages.
func GetLobbyChatHistory(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetLobbyChatHistory")
		defer span.End()

		userID, ok := userIDFromContext(c)
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

		// Verify user is in the lobby or is a spectator
		var authorized int
		err = db.QueryRowContext(ctx, `
			SELECT 1
			FROM (
				SELECT gp.user_id
				FROM game_players gp
				JOIN games g ON g.id = gp.game_id
				WHERE g.lobby_id = ? AND gp.user_id = ? AND g.status IN ('waiting', 'in_progress')
				UNION
				SELECT ls.user_id
				FROM lobby_spectators ls
				WHERE ls.lobby_id = ? AND ls.user_id = ?
			) AS authorized_users
			LIMIT 1
		`, lobbyID, userID, lobbyID, userID).Scan(&authorized)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not in this lobby"})
			return
		}
		if err != nil {
			wrappedErr := fmt.Errorf("GetLobbyChatHistory: check authorization (lobby_id=%d user_id=%d): %w", lobbyID, userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Get chat history (last 100 messages)
		limit := 100
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
				limit = l
			}
		}

		rows, err := db.QueryContext(ctx, `
			SELECT id, lobby_id, user_id, username, message, message_type, created_at
			FROM lobby_messages
			WHERE lobby_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		`, lobbyID, limit)
		if err != nil {
			wrappedErr := fmt.Errorf("GetLobbyChatHistory: query messages (lobby_id=%d limit=%d): %w", lobbyID, limit, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		defer rows.Close()

		messages := []LobbyChatMessage{}
		scanErrors := 0
		for rows.Next() {
			var msg LobbyChatMessage
			var nullUserID sql.NullInt64
			err := rows.Scan(&msg.ID, &msg.LobbyID, &nullUserID, &msg.Username, &msg.Message, &msg.MessageType, &msg.CreatedAt)
			if err != nil {
				scanErrors++
				log.Printf("Error scanning chat message for lobby %d (row skipped): %v", lobbyID, err)
				continue
			}
			if nullUserID.Valid {
				msg.UserID = &nullUserID.Int64
			}
			messages = append(messages, msg)
		}
		if scanErrors > 0 {
			log.Printf("Warning: %d chat messages failed to scan for lobby %d", scanErrors, lobbyID)
		}
		if err := rows.Err(); err != nil {
			log.Printf("Error iterating chat messages for lobby %d: %v", lobbyID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Reverse to get chronological order
		for i := 0; i < len(messages)/2; i++ {
			j := len(messages) - 1 - i
			messages[i], messages[j] = messages[j], messages[i]
		}

		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

// handleLobbyChatWS handles WebSocket "lobby:send_message" events
func handleLobbyChatWS(hub *ws.Hub, client *ws.Client, db *sql.DB, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var req struct {
		LobbyID int64  `json:"lobby_id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &req); err != nil || req.LobbyID <= 0 {
		if err := sendDirect(client, "error", map[string]any{"error": "invalid chat payload"}); err != nil {
			log.Printf("sendDirect failed (invalid_chat): err=%v", err)
			client.Close()
		}
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" || len(message) > 500 {
		if err := sendDirect(client, "error", map[string]any{"error": "invalid message"}); err != nil {
			log.Printf("sendDirect failed (invalid_message): err=%v", err)
			client.Close()
		}
		return
	}

	// Get username
	var username string
	err := db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = ?", client.UserID).Scan(&username)
	if err != nil {
		wrappedErr := fmt.Errorf("handleLobbyChatWS: get username (user_id=%d): %w", client.UserID, err)
		log.Printf("%v", wrappedErr)
		if err := sendDirect(client, "error", map[string]any{"error": "internal error"}); err != nil {
			log.Printf("sendDirect failed (username_error): err=%v", err)
			client.Close()
		}
		return
	}

	// Verify user is in the lobby
	var playerCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_players gp
		JOIN games g ON g.id = gp.game_id
		WHERE g.lobby_id = ? AND gp.user_id = ? AND g.status IN ('waiting', 'in_progress')
	`, req.LobbyID, client.UserID).Scan(&playerCount)
	if err != nil || playerCount == 0 {
		if err != nil {
			wrappedErr := fmt.Errorf("handleLobbyChatWS: check membership (lobby_id=%d user_id=%d): %w", req.LobbyID, client.UserID, err)
			log.Printf("%v", wrappedErr)
		}
		if err := sendDirect(client, "error", map[string]any{"error": "not in lobby"}); err != nil {
			log.Printf("sendDirect failed (not_in_lobby): err=%v", err)
			client.Close()
		}
		return
	}

	// Insert message
	result, err := db.ExecContext(ctx, `
		INSERT INTO lobby_messages (lobby_id, user_id, username, message, message_type)
		VALUES (?, ?, ?, ?, 'chat')
	`, req.LobbyID, client.UserID, username, message)
	if err != nil {
		wrappedErr := fmt.Errorf("handleLobbyChatWS: insert message (lobby_id=%d user_id=%d): %w", req.LobbyID, client.UserID, err)
		log.Printf("%v", wrappedErr)
		if err := sendDirect(client, "error", map[string]any{"error": "internal error"}); err != nil {
			log.Printf("sendDirect failed (insert_error): err=%v", err)
			client.Close()
		}
		return
	}

	msgID, idErr := result.LastInsertId()
	if idErr != nil {
		log.Printf("handleLobbyChatWS: warning: LastInsertId failed (lobby_id=%d user_id=%d): %v", req.LobbyID, client.UserID, fmt.Errorf("%w", idErr))
		msgID = 0
	}

	chatMsg := LobbyChatMessage{
		ID:          msgID,
		LobbyID:     req.LobbyID,
		UserID:      &client.UserID,
		Username:    username,
		Message:     message,
		MessageType: "chat",
		CreatedAt:   time.Now(),
	}

	// Broadcast to lobby room
	hub.Broadcast(fmt.Sprintf("lobby:%d", req.LobbyID), "lobby:chat", chatMsg)
}

// SendSystemMessage inserts a system message into the lobby chat and broadcasts it via WebSocket if hub is provided.
// messageType defaults to "system" when empty.
func SendSystemMessage(ctx context.Context, db *sql.DB, hub *ws.Hub, lobbyID int64, message string, messageType string) error {
	if messageType == "" {
		messageType = "system"
	}

	result, err := db.ExecContext(ctx, `
		INSERT INTO lobby_messages (lobby_id, username, message, message_type)
		VALUES (?, 'System', ?, ?)
	`, lobbyID, message, messageType)
	if err != nil {
		return fmt.Errorf("failed to insert system message: %w", err)
	}

	msgID, idErr := result.LastInsertId()
	if idErr != nil {
		log.Printf("SendSystemMessage: warning: LastInsertId failed (lobby_id=%d): %v", lobbyID, fmt.Errorf("%w", idErr))
		msgID = 0
	}

	chatMsg := LobbyChatMessage{
		ID:          msgID,
		LobbyID:     lobbyID,
		Username:    "System",
		Message:     message,
		MessageType: messageType,
		CreatedAt:   time.Now(),
	}

	if hub != nil {
		hub.Broadcast(fmt.Sprintf("lobby:%d", lobbyID), "lobby:chat", chatMsg)
	}

	return nil
}
