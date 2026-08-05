// Package handlers - achievements.go exposes the achievements module via HTTP.
package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// GetCatalogueHandler handles GET /api/achievements/catalogue and returns the
// static list of all known achievements.
func GetCatalogueHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetAchievementCatalogue")
		defer span.End()
		c.JSON(http.StatusOK, gin.H{"achievements": models.Catalogue})
	}
}

// GetMyAchievementsHandler handles GET /api/achievements (current user).
func GetMyAchievementsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.GetMyAchievements")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		snap, err := models.ListAchievementsForUser(ctx, db, userID)
		if err != nil {
			log.Printf("GetMyAchievementsHandler: user=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, snap)
	}
}

// GetUserAchievementsHandler handles GET /api/users/:user_id/achievements.
// It is public — anyone can view another user's unlocked achievements.
func GetUserAchievementsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.GetUserAchievements")
		defer span.End()

		// Param name "id" matches the existing /users/:id/* group.
		uid, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || uid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		snap, err := models.ListAchievementsForUser(ctx, db, uid)
		if err != nil {
			log.Printf("GetUserAchievementsHandler: user=%d: %v", uid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, snap)
	}
}

// EvaluateMyAchievementsHandler handles POST /api/achievements/evaluate. It
// recomputes the user's achievement set from current stats and returns any
// newly-unlocked IDs. Intended to be called from the game-finalize path or
// from a periodic background job; exposed via HTTP for easy debugging.
func EvaluateMyAchievementsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.EvaluateMyAchievements")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		newlyUnlocked, err := models.EvaluateAndPersistForUser(ctx, db, userID)
		if err != nil {
			log.Printf("EvaluateMyAchievementsHandler: user=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		// Hydrate the IDs into full catalogue rows for the client.
		out := make([]models.Achievement, 0, len(newlyUnlocked))
		for _, id := range newlyUnlocked {
			if a, ok := models.LookupAchievement(id); ok {
				out = append(out, a)
			}
		}
		c.JSON(http.StatusOK, gin.H{"newly_unlocked": out})
	}
}
