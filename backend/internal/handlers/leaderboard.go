package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// LeaderboardHandler returns a handler that serves leaderboard data for a configurable time window.
// Accepts optional query parameter 'days' (default 30, clamped to [1, 365]).
func LeaderboardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.LeaderboardHandler")
		defer span.End()
		days := int64(30)
		if s := c.Query("days"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				days = v
			}
		}
		if days <= 0 {
			days = 30
		}
		if days > 365 {
			days = 365
		}

		resp, err := models.BuildLeaderboard(ctx, db, days)
		if err != nil {
			wrappedErr := fmt.Errorf("BuildLeaderboard failed for days=%d: %w", days, err)
			log.Printf("LeaderboardHandler: %v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}
