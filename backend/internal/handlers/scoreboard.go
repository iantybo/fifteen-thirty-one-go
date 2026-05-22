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

		limit := int64(50)
		if s := c.Query("limit"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				limit = v
			}
		}

		items, err := models.ListScoreboard(db, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func UserHistoryHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UserHistoryHandler")
		defer span.End()

		userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
		if err != nil || userID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		limit := int64(50)
		if s := c.Query("limit"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				limit = v
			}
		}

		items, err := models.ListUserHistory(db, userID, limit)
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
