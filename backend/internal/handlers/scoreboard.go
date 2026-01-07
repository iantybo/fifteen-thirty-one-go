package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

func ScoreboardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ScoreboardHandler")
		defer span.End()

		items, err := models.ListScoreboard(db, 50)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func UserStatsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UserStatsHandler")
		defer span.End()

		userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		if userID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		stats, err := models.GetUserStats(db, userID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// ActiveGamesHandler returns all games currently in progress with player information.
// This endpoint is used for the global scoreboard to show all active games.
func ActiveGamesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ActiveGamesHandler")
		defer span.End()

		games, err := models.ListActiveGames(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch active games"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"games": games})
	}
}
