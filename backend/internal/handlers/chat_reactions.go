// Package handlers - chat_reactions.go implements emoji reactions on lobby
// chat messages.
//
// Reactions are stored in lobby_message_reactions; each (message_id, user_id,
// emoji) tuple is unique. Toggling the same emoji removes the row, matching
// the UX convention of "tap-to-toggle".
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"fifteen-thirty-one-go/backend/internal/tracing"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// allowedReactionEmoji is a curated whitelist. We deliberately do not allow
// arbitrary user-supplied emoji to keep the UI tidy and the index small.
var allowedReactionEmoji = map[string]struct{}{
	"👍": {}, "❤️": {}, "😂": {}, "🎉": {}, "🤔": {}, "😢": {}, "🔥": {}, "👏": {},
}

// ReactionView is the per-emoji rollup returned alongside chat messages.
type ReactionView struct {
	Emoji   string  `json:"emoji"`
	Count   int     `json:"count"`
	UserIDs []int64 `json:"user_ids"`
	Reacted bool    `json:"reacted"` // whether the requesting user has reacted
}

// EnsureReactionsSchema creates the reactions table if missing.
func EnsureReactionsSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS lobby_message_reactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			emoji TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(message_id, user_id, emoji)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reactions_message ON lobby_message_reactions(message_id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("EnsureReactionsSchema: %w", err)
		}
	}
	return nil
}

// ToggleReactionHandler handles POST /api/lobbies/:id/chat/:msg_id/react with
// body {"emoji": "👍"}. Posting the same emoji a second time removes it.
func ToggleReactionHandler(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.ToggleReaction")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		lobbyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || lobbyID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lobby id"})
			return
		}
		msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
		if err != nil || msgID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
			return
		}

		var req struct {
			Emoji string `json:"emoji" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "emoji is required"})
			return
		}
		emoji := strings.TrimSpace(req.Emoji)
		if !isValidEmoji(emoji) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported emoji"})
			return
		}

		// Authorise: the actor must be in the lobby or a spectator.
		ok, err = canActInLobby(ctx, db, lobbyID, userID)
		if err != nil {
			log.Printf("ToggleReactionHandler: auth check failed: lobby_id=%d user_id=%d err=%v", lobbyID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not in this lobby"})
			return
		}

		// Confirm the message belongs to the lobby (defence in depth).
		var owningLobby int64
		err = db.QueryRowContext(ctx, `SELECT lobby_id FROM lobby_messages WHERE id = ?`, msgID).Scan(&owningLobby)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		case err != nil:
			log.Printf("ToggleReactionHandler: lookup message: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if owningLobby != lobbyID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message does not belong to lobby"})
			return
		}

		// Toggle: try delete first; if no row was affected, insert.
		res, err := db.ExecContext(ctx, `
			DELETE FROM lobby_message_reactions
			WHERE message_id = ? AND user_id = ? AND emoji = ?
		`, msgID, userID, emoji)
		if err != nil {
			log.Printf("ToggleReactionHandler: delete: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		var added bool
		if n, _ := res.RowsAffected(); n == 0 {
			_, err = db.ExecContext(ctx, `
				INSERT INTO lobby_message_reactions (message_id, user_id, emoji)
				VALUES (?, ?, ?)
			`, msgID, userID, emoji)
			if err != nil {
				log.Printf("ToggleReactionHandler: insert: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			added = true
		}

		// Rebuild the reaction rollup for this message and broadcast it.
		views, err := loadReactionsForMessage(ctx, db, msgID, userID)
		if err != nil {
			log.Printf("ToggleReactionHandler: rebuild: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		hub, hubOK := hubProvider()
		if hubOK && hub != nil {
			hub.Broadcast(lobbyRoomKey(lobbyID), "lobby:reaction", gin.H{
				"message_id": msgID,
				"reactions":  views,
				"actor":      userID,
				"emoji":      emoji,
				"added":      added,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"message_id": msgID,
			"reactions":  views,
			"added":      added,
		})
	}
}

// GetMessageReactionsHandler handles GET /api/lobbies/:id/chat/:msg_id/reactions.
func GetMessageReactionsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.GetMessageReactions")
		defer span.End()

		userID, _ := userIDFromContext(c)
		msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
		if err != nil || msgID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
			return
		}

		views, err := loadReactionsForMessage(ctx, db, msgID, userID)
		if err != nil {
			log.Printf("GetMessageReactionsHandler: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"reactions": views})
	}
}

// isValidEmoji performs the cheap checks. We tolerate variation selectors
// (e.g. the ❤️ U+FE0F suffix) by allowing strings up to 16 bytes that match
// the allow-list exactly.
func isValidEmoji(s string) bool {
	if s == "" || len(s) > 16 || !utf8.ValidString(s) {
		return false
	}
	_, ok := allowedReactionEmoji[s]
	return ok
}

// canActInLobby returns whether userID is either a player or a spectator in
// the given lobby. Reused by the toggle handler and could be extended for
// future write-paths.
func canActInLobby(ctx context.Context, db *sql.DB, lobbyID, userID int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT 1
		FROM (
			SELECT gp.user_id
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE g.lobby_id = ? AND gp.user_id = ? AND g.status IN ('waiting', 'in_progress')
			UNION
			SELECT ls.user_id FROM lobby_spectators ls
			WHERE ls.lobby_id = ? AND ls.user_id = ?
		) AS allowed
		LIMIT 1
	`, lobbyID, userID, lobbyID, userID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("canActInLobby: %w", err)
	}
	return n == 1, nil
}

// loadReactionsForMessage returns the rollup of reactions on a message. The
// viewerID is used to populate the Reacted flag; pass 0 for anonymous calls.
func loadReactionsForMessage(ctx context.Context, db *sql.DB, msgID, viewerID int64) ([]ReactionView, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT emoji, user_id
		FROM lobby_message_reactions
		WHERE message_id = ?
		ORDER BY emoji, created_at ASC
	`, msgID)
	if err != nil {
		return nil, fmt.Errorf("loadReactionsForMessage: query: %w", err)
	}
	defer rows.Close()

	byEmoji := make(map[string]*ReactionView, 4)
	for rows.Next() {
		var emoji string
		var uid int64
		if err := rows.Scan(&emoji, &uid); err != nil {
			return nil, fmt.Errorf("loadReactionsForMessage: scan: %w", err)
		}
		v, ok := byEmoji[emoji]
		if !ok {
			v = &ReactionView{Emoji: emoji, UserIDs: make([]int64, 0, 2)}
			byEmoji[emoji] = v
		}
		v.UserIDs = append(v.UserIDs, uid)
		v.Count++
		if uid == viewerID {
			v.Reacted = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loadReactionsForMessage: iter: %w", err)
	}

	out := make([]ReactionView, 0, len(byEmoji))
	for _, v := range byEmoji {
		out = append(out, *v)
	}
	return out, nil
}
