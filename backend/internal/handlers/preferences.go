package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

func GetPreferencesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
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
	AutoCountMode string `json:"auto_count_mode"`
}

func PutPreferencesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		prefs, err := models.SetUserAutoCountModeAndGetPreferencesTx(db, userID, req.AutoCountMode)
		if err != nil {
			if errors.Is(err, models.ErrInvalidMode) {
				log.Printf("PutPreferencesHandler invalid mode: user_id=%d err=%v", userID, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode"})
				return
			}
			log.Printf("SetUserAutoCountModeAndGetPreferencesTx failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, prefs)
	}
}


