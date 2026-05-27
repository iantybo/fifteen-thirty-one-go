package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// allowedReactionEmojis caps reactions to a small curated set. This avoids
// arbitrary text injection while still being "cool".
var allowedReactionEmojis = map[string]struct{}{
	"👍": {}, "👎": {}, "😂": {}, "😮": {}, "😢": {}, "🔥": {},
	"🎉": {}, "♠️": {}, "♥️": {}, "♦️": {}, "♣️": {}, "🤔": {},
	"😎": {}, "🍀": {}, "💯": {},
}

type postReactionRequest struct {
	Emoji string `json:"emoji"`
}

// reactionRateLimiter keeps a per-user-per-game timestamp of the last reaction
// to throttle spam. A small in-memory map is sufficient for the vertical slice.
type reactionRateLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

// reactionRateLimiterCleanupTTL is how long an entry is retained before being
// considered stale and eligible for eviction.
const reactionRateLimiterCleanupTTL = 1 * time.Hour

func (r *reactionRateLimiter) allow(key string, now time.Time, interval time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Opportunistically evict stale entries to prevent unbounded growth of
	// the in-memory map across long-lived processes.
	if len(r.last) >= 128 {
		cutoff := now.Add(-reactionRateLimiterCleanupTTL)
		for k, t := range r.last {
			if t.Before(cutoff) {
				delete(r.last, k)
			}
		}
	}
	if last, ok := r.last[key]; ok && now.Sub(last) < interval {
		return false
	}
	r.last[key] = now
	return true
}

var reactionLimiter = &reactionRateLimiter{last: map[string]time.Time{}}

const reactionMinInterval = 750 * time.Millisecond

// PostReactionHandler accepts an emoji reaction from a player and broadcasts
// it to all subscribers of the game room.
func PostReactionHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.PostReactionHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		var req postReactionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		if req.Emoji == "" || utf8.RuneCountInString(req.Emoji) > 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid emoji"})
			return
		}
		if _, allowed := allowedReactionEmojis[req.Emoji]; !allowed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "emoji not allowed"})
			return
		}

		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			log.Printf("PostReactionHandler IsUserInGame failed: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		key := strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(gameID, 10)
		if !reactionLimiter.allow(key, time.Now(), reactionMinInterval) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "slow down"})
			return
		}

		broadcastErr := broadcastReaction(gameID, userID, req.Emoji)
		if broadcastErr != nil {
			// Reactions are best-effort: the user's request itself succeeded,
			// but we surface broadcast issues in logs and on the response so
			// they aren't silent.
			log.Printf("PostReactionHandler broadcast failed: game_id=%d user_id=%d err=%v", gameID, userID, broadcastErr)
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":               true,
			"broadcast_failed": broadcastErr != nil,
		})
	}
}

func broadcastReaction(gameID, userID int64, emoji string) error {
	hub, ok := getHubProvider()
	if !ok || hub == nil {
		return errors.New("hub not available")
	}
	hub.Broadcast("game:"+strconv.FormatInt(gameID, 10), "reaction", gin.H{
		"user_id": userID,
		"emoji":   emoji,
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
	})
	return nil
}
