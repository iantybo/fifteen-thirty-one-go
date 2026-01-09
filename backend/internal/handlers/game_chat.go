package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// GameChatMessage represents a chat message in a game.
// It includes sender identity, message content, message type, and the persisted timestamp.
type GameChatMessage struct {
	ID          int64     `json:"id"`
	GameID      int64     `json:"game_id"`
	UserID      *int64    `json:"user_id,omitempty"`
	Username    string    `json:"username"`
	Message     string    `json:"message"`
	MessageType string    `json:"message_type"` // chat, system
	CreatedAt   time.Time `json:"created_at"`
}

func insertGameChatMessage(ctx context.Context, db *sql.DB, gameID int64, userID int64, username string, message string) (msgID int64, createdAt time.Time, err error) {
	// Prefer RETURNING so the API response exactly matches persisted values.
	// SQLite supports RETURNING from 3.35+; if unavailable, fall back gracefully.
	var returningErr error
	{
		var id int64
		var ts time.Time
		row := db.QueryRowContext(ctx, `
			INSERT INTO game_messages (game_id, user_id, username, message, message_type)
			VALUES (?, ?, ?, ?, 'chat')
			RETURNING id, created_at
		`, gameID, userID, username, message)
		if scanErr := row.Scan(&id, &ts); scanErr == nil {
			return id, ts, nil
		} else {
			returningErr = fmt.Errorf("insertGameChatMessage: returning scan failed (game_id=%d user_id=%d): %w", gameID, userID, scanErr)
		}
	}

	// Fallback: insert, then read back the DB timestamp.
	result, execErr := db.ExecContext(ctx, `
		INSERT INTO game_messages (game_id, user_id, username, message, message_type)
		VALUES (?, ?, ?, ?, 'chat')
	`, gameID, userID, username, message)
	if execErr != nil {
		return 0, time.Time{}, fmt.Errorf(
			"insertGameChatMessage: exec insert (game_id=%d user_id=%d): %w",
			gameID, userID, errors.Join(execErr, returningErr),
		)
	}

	id, idErr := result.LastInsertId()
	if idErr != nil {
		return 0, time.Time{}, fmt.Errorf(
			"insertGameChatMessage: get last insert id (game_id=%d user_id=%d): %w",
			gameID, userID, errors.Join(idErr, returningErr),
		)
	}

	var ts time.Time
	if err := db.QueryRowContext(ctx, `SELECT created_at FROM game_messages WHERE id = ?`, id).Scan(&ts); err != nil {
		return id, time.Time{}, fmt.Errorf(
			"insertGameChatMessage: fetch created_at (game_id=%d user_id=%d msg_id=%d): %w",
			gameID, userID, id, errors.Join(err, returningErr),
		)
	}

	return id, ts, nil
}

// SendGameChatMessage returns a Gin handler for POST /api/games/:id/chat.
// It validates the requester is a game participant, validates message content, persists the message,
// and broadcasts it to the game room via WebSocket.
func SendGameChatMessage(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.SendGameChatMessage")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		gameIDStr := c.Param("id")
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		var req struct {
			Message string `json:"message" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
			return
		}

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

		// Verify user is in the game.
		var playerCount int
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_players WHERE game_id = ? AND user_id = ?`, gameID, userID).Scan(&playerCount)
		if err != nil {
			wrappedErr := fmt.Errorf("SendGameChatMessage: check membership (game_id=%d user_id=%d): %w", gameID, userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if playerCount == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not in this game"})
			return
		}

		// Get username.
		var username string
		err = db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			wrappedErr := fmt.Errorf("SendGameChatMessage: get username (user_id=%d): %w", userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Insert message.
		msgID, createdAt, err := insertGameChatMessage(ctx, db, gameID, userID, username, message)
		if err != nil {
			wrappedErr := fmt.Errorf("SendGameChatMessage: insert message (game_id=%d user_id=%d): %w", gameID, userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		uid := userID
		chatMsg := GameChatMessage{
			ID:          msgID,
			GameID:      gameID,
			UserID:      &uid,
			Username:    username,
			Message:     message,
			MessageType: "chat",
			CreatedAt:   createdAt,
		}

		// Broadcast to game room.
		hub, ok := hubProvider()
		if ok && hub != nil {
			hub.Broadcast(fmt.Sprintf("game:%d", gameID), "game:chat", chatMsg)
		}

		c.JSON(http.StatusOK, chatMsg)
	}
}

// GetGameChatHistory returns a Gin handler for GET /api/games/:id/chat.
// It validates the requester is a game participant and returns recent messages.
func GetGameChatHistory(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetGameChatHistory")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		gameIDStr := c.Param("id")
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		ctx := c.Request.Context()

		// Verify user is in the game.
		var playerCount int
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_players WHERE game_id = ? AND user_id = ?`, gameID, userID).Scan(&playerCount)
		if err != nil {
			wrappedErr := fmt.Errorf("GetGameChatHistory: check membership (game_id=%d user_id=%d): %w", gameID, userID, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if playerCount == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not in this game"})
			return
		}

		// Get chat history (last 100 messages).
		limit := 100
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
				limit = l
			}
		}

		rows, err := db.QueryContext(ctx, `
			SELECT id, game_id, user_id, username, message, message_type, created_at
			FROM game_messages
			WHERE game_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		`, gameID, limit)
		if err != nil {
			wrappedErr := fmt.Errorf("GetGameChatHistory: query messages (game_id=%d limit=%d): %w", gameID, limit, err)
			log.Printf("%v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		defer rows.Close()

		messages := []GameChatMessage{}
		scanErrors := 0
		for rows.Next() {
			var msg GameChatMessage
			var nullUserID sql.NullInt64
			err := rows.Scan(&msg.ID, &msg.GameID, &nullUserID, &msg.Username, &msg.Message, &msg.MessageType, &msg.CreatedAt)
			if err != nil {
				scanErrors++
				log.Printf("Error scanning chat message for game %d (row skipped): %v", gameID, err)
				continue
			}
			if nullUserID.Valid {
				msg.UserID = &nullUserID.Int64
			}
			messages = append(messages, msg)
		}
		if scanErrors > 0 {
			log.Printf("Warning: %d chat messages failed to scan for game %d", scanErrors, gameID)
		}
		if err := rows.Err(); err != nil {
			log.Printf("Error iterating chat messages for game %d: %v", gameID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Reverse to get chronological order.
		for i := 0; i < len(messages)/2; i++ {
			j := len(messages) - 1 - i
			messages[i], messages[j] = messages[j], messages[i]
		}

		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

// handleGameChatWS handles WebSocket "game:send_message" events.
func handleGameChatWS(hub *ws.Hub, client *ws.Client, db *sql.DB, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var req struct {
		GameID  int64  `json:"game_id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &req); err != nil || req.GameID <= 0 {
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

	// Verify user is in the game.
	var playerCount int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_players WHERE game_id = ? AND user_id = ?`, req.GameID, client.UserID).Scan(&playerCount)
	if err != nil || playerCount == 0 {
		if err != nil {
			wrappedErr := fmt.Errorf("handleGameChatWS: check membership (game_id=%d user_id=%d): %w", req.GameID, client.UserID, err)
			log.Printf("%v", wrappedErr)
		}
		if err := sendDirect(client, "error", map[string]any{"error": "not in game"}); err != nil {
			log.Printf("sendDirect failed (not_in_game): err=%v", err)
			client.Close()
		}
		return
	}

	// Get username.
	var username string
	err = db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = ?", client.UserID).Scan(&username)
	if err != nil {
		wrappedErr := fmt.Errorf("handleGameChatWS: get username (user_id=%d): %w", client.UserID, err)
		log.Printf("%v", wrappedErr)
		if err := sendDirect(client, "error", map[string]any{"error": "internal error"}); err != nil {
			log.Printf("sendDirect failed (username_error): err=%v", err)
			client.Close()
		}
		return
	}

	// Insert message.
	msgID, createdAt, err := insertGameChatMessage(ctx, db, req.GameID, client.UserID, username, message)
	if err != nil {
		wrappedErr := fmt.Errorf("handleGameChatWS: insert message (game_id=%d user_id=%d): %w", req.GameID, client.UserID, err)
		log.Printf("%v", wrappedErr)
		if err := sendDirect(client, "error", map[string]any{"error": "internal error"}); err != nil {
			log.Printf("sendDirect failed (insert_error): err=%v", err)
			client.Close()
		}
		return
	}

	chatMsg := GameChatMessage{
		ID:          msgID,
		GameID:      req.GameID,
		UserID:      &client.UserID,
		Username:    username,
		Message:     message,
		MessageType: "chat",
		CreatedAt:   createdAt,
	}

	hub.Broadcast(fmt.Sprintf("game:%d", req.GameID), "game:chat", chatMsg)
}


