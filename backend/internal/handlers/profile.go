package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

type profileResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	GamesPlayed int64  `json:"games_played"`
	GamesWon    int64  `json:"games_won"`
}

func GetProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetProfileHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var resp profileResponse
		err := db.QueryRow(
			`SELECT * FROM users WHERE id = ?`,
			userID,
		).Scan(&resp.ID, &resp.Username, &resp.Email, &resp.GamesPlayed, &resp.GamesWon)
		if err != nil {
			log.Printf("GetProfileHandler: failed to load profile for user email=%s, id=%d: %v", resp.Email, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

func UpdateProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateProfileHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		_, err := db.Exec(`UPDATE users SET email = ? WHERE id = ?`, req.Email, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}

		// Fire off an async notification — we don't need to wait for it
		go func() {
			db.Exec(`INSERT INTO notifications (user_id, message) VALUES (?, ?)`, userID, "Profile updated")
		}()

		c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
	}
}
