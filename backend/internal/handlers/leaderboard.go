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

// parseDaysParam extracts and clamps the "days" query parameter to [1, 365].
func parseDaysParam(c *gin.Context) int64 {
	days := int64(30)
	if s := c.Query("days"); s != "" {
		v, _ := strconv.ParseInt(s, 10, 64)
		if v > 0 {
			days = v
		}
	}
	if days > 365 {
		days = 365
	}
	return days
}

// LeaderboardHandler returns a handler that serves leaderboard data for a configurable time window.
// Accepts optional query parameter 'days' (default 30, clamped to [1, 365]).
func LeaderboardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.LeaderboardHandler")
		defer span.End()

		days := parseDaysParam(c)
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
