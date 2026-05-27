package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

func GetPreferencesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetPreferencesHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		prefs, err := models.GetUserPreferences(db, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, prefs)
	}
}

type putPreferencesRequest struct {
	AutoCountMode *string `json:"auto_count_mode"`
	CardTheme     *string `json:"card_theme"`
}

func PutPreferencesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.PutPreferencesHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req putPreferencesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		if req.AutoCountMode == nil && req.CardTheme == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
			return
		}

		prefs, err := models.UpdateUserPreferencesTx(db, userID, req.AutoCountMode, req.CardTheme)
		if err != nil {
			if errors.Is(err, models.ErrInvalidMode) {
				log.Printf("PutPreferencesHandler invalid mode: user_id=%d err=%v", userID, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode"})
				return
			}
			if errors.Is(err, models.ErrInvalidCardTheme) {
				log.Printf("PutPreferencesHandler invalid card_theme: user_id=%d err=%v", userID, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card_theme"})
				return
			}
			log.Printf("UpdateUserPreferencesTx failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, prefs)
	}
}
