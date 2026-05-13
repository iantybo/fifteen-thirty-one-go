package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// parseDaysParam parses and validates the 'days' query parameter.
// Returns 30 when the parameter is absent (empty string).
// Returns HTTP-400-worthy errors for non-integer or non-positive values.
// Values above 365 are clamped to 365; one year is the maximum supported
// leaderboard window to keep query cost bounded on large datasets.
func parseDaysParam(raw string) (int64, error) {
	if raw == "" {
		return 30, nil
	}

	days, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid days parameter: must be a positive integer: %w", err)
	}
	if days <= 0 {
		return 0, errors.New("invalid days parameter: must be a positive integer")
	}
	// Cap at one year. Queries beyond 365 days provide diminishing value and
	// impose unbounded cost on the leaderboard aggregation query.
	if days > 365 {
		return 365, nil
	}

	return days, nil
}

// LeaderboardHandler returns a handler that serves leaderboard data for a configurable time window.
// Accepts optional query parameter 'days' (default 30, clamped to [1, 365]).
func LeaderboardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.LeaderboardHandler")
		defer span.End()

		days, err := parseDaysParam(c.Query("days"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
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
