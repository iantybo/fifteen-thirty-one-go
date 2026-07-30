package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// maxStreakGames bounds how many finished games are scanned per request so a
// long-lived account cannot force an unbounded read.
const maxStreakGames = 1000

// streaksResponse reports a player's win/loss streaks derived from the
// scoreboard table. Current is positive for an ongoing win streak, negative for
// an ongoing losing streak, and zero when the player has no finished games.
type streaksResponse struct {
	UserID       int64 `json:"user_id"`
	GamesScanned int64 `json:"games_scanned"`
	Current      int64 `json:"current"`
	LongestWin   int64 `json:"longest_win"`
	LongestLoss  int64 `json:"longest_loss"`
}

// StreaksHandler returns the authenticated user's streak summary. An optional
// `userId` path parameter is honoured when the route provides one, so the same
// handler can serve both /me/streaks and /streaks/:userId.
func StreaksHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.StreaksHandler")
		defer span.End()

		userID, ok := streakTargetUserID(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		resp, err := buildStreaks(ctx, db, userID)
		if err != nil {
			log.Printf("StreaksHandler failed for userID=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

// streakTargetUserID resolves which player to report on. A well-formed positive
// `userId` path parameter wins; otherwise the request falls back to the
// authenticated caller.
func streakTargetUserID(c *gin.Context) (int64, bool) {
	raw := c.Param("userId")
	if raw == "" {
		return userIDFromContext(c)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// buildStreaks walks the player's finished games from newest to oldest, tracking
// the in-progress streak and the longest run of each outcome. A scoreboard row
// with position = 1 counts as a win.
func buildStreaks(ctx context.Context, db *sql.DB, userID int64) (*streaksResponse, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT position
		 FROM scoreboard
		 WHERE user_id = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		userID,
		maxStreakGames,
	)
	if err != nil {
		return nil, fmt.Errorf("buildStreaks: querying scoreboard: %w", err)
	}
	defer rows.Close()

	resp := &streaksResponse{UserID: userID}
	// run tracks the length of the streak currently being consumed, and runWon
	// whether that streak is made of wins.
	var run int64
	var runWon bool
	for rows.Next() {
		var position int64
		if err := rows.Scan(&position); err != nil {
			return nil, fmt.Errorf("buildStreaks: scanning scoreboard row: %w", err)
		}
		won := position == 1

		if run == 0 || won != runWon {
			run = 0
			runWon = won
		}
		run++
		resp.GamesScanned++

		// The first row is the most recent game, so the first run encountered is
		// the current streak.
		if resp.GamesScanned == run {
			if won {
				resp.Current = run
			} else {
				resp.Current = -run
			}
		}
		if won && run > resp.LongestWin {
			resp.LongestWin = run
		}
		if !won && run > resp.LongestLoss {
			resp.LongestLoss = run
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("buildStreaks: iterating scoreboard rows: %w", err)
	}

	return resp, nil
}
