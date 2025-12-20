package handlers

import (
	"database/sql"
	"net/http"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

func GetPreferencesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDAny, _ := c.Get("userID")
		userID := userIDAny.(int64)
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
		userIDAny, _ := c.Get("userID")
		userID := userIDAny.(int64)

		var req putPreferencesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		if err := models.SetUserAutoCountMode(db, userID, req.AutoCountMode); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		prefs, _ := models.GetUserPreferences(db, userID)
		c.JSON(http.StatusOK, prefs)
	}
}


