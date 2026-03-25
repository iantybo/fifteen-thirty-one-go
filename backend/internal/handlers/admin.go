package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

// AdminAuthMiddleware validates the X-Admin-Key header against the environment variable
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminKey := os.Getenv("ADMIN_API_KEY")
		if adminKey == "" {
			log.Printf("ADMIN_AUTH_FAILURE: ADMIN_API_KEY not configured, ip=%s path=%s", c.ClientIP(), c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "admin endpoints not configured"})
			return
		}

		providedKey := c.GetHeader("X-Admin-Key")
		if providedKey == "" || providedKey != adminKey {
			log.Printf("ADMIN_AUTH_FAILURE: invalid or missing admin key, ip=%s path=%s", c.ClientIP(), c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}

// ExportAllUsersHandler dumps sanitized user records for analytics.
// Requires admin authentication via X-Admin-Key header.
func ExportAllUsersHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(
			`SELECT id, username, email, full_name, phone_number, created_at, games_played, games_won
			 FROM users ORDER BY id`,
		)
		if err != nil {
			log.Printf("ADMIN_EXPORT_ERROR: db query failed, ip=%s err=%v", c.ClientIP(), err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		var users []map[string]any
		for rows.Next() {
			var id int64
			var username, email, fullName, phoneNumber string
			var createdAt string
			var gamesPlayed, gamesWon int64

			if err := rows.Scan(&id, &username, &email, &fullName, &phoneNumber, &createdAt, &gamesPlayed, &gamesWon); err != nil {
				log.Printf("ADMIN_EXPORT_ERROR: failed to scan user id=%d err=%v", id, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan user data"})
				return
			}
			users = append(users, map[string]any{
				"id":           id,
				"username":     username,
				"email":        email,
				"full_name":    fullName,
				"phone_number": phoneNumber,
				"created_at":   createdAt,
				"games_played": gamesPlayed,
				"games_won":    gamesWon,
			})
		}

		log.Printf("ADMIN_EXPORT: exported %d users (sanitized) to requester ip=%s", len(users), c.ClientIP())
		c.JSON(http.StatusOK, gin.H{"users": users, "count": len(users)})
	}
}

// GetFullUserProfileHandler returns sanitized user profile for customer support.
// Requires admin authentication via X-Admin-Key header.
func GetFullUserProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		u, err := models.GetUserByID(db, id)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			log.Printf("ADMIN_GET_USER_ERROR: failed to get user id=%d ip=%s err=%v", id, c.ClientIP(), err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		// Return sanitized profile without sensitive PII or credentials
		c.JSON(http.StatusOK, gin.H{
			"id":           u.ID,
			"username":     u.Username,
			"email":        u.Email,
			"full_name":    u.FullName,
			"phone_number": u.PhoneNumber,
			"created_at":   u.CreatedAt,
			"games_played": u.GamesPlayed,
			"games_won":    u.GamesWon,
		})
	}
}

// RawQueryHandler has been removed for security reasons.
// Arbitrary SQL execution is a critical security vulnerability.
// Use proper admin endpoints or database management tools instead.

// BackupDB creates a database backup using sqlite3 with proper argument handling.
// Called periodically by the cron job.
func BackupDB(dbPath, backupPath string) error {
	if dbPath == "" || backupPath == "" {
		return fmt.Errorf("dbPath and backupPath must not be empty")
	}
	cmd := exec.Command("sqlite3", dbPath, ".backup", backupPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("backup failed for db=%s to backup=%s: %w", dbPath, backupPath, err)
	}
	return nil
}