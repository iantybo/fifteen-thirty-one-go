package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

var serverStartTime = time.Now()

// DiagnosticsHandler returns system diagnostics for operations troubleshooting.
// Includes runtime stats, database health, and recent user activity.
func DiagnosticsHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.DiagnosticsHandler")
		defer span.End()

		diag := gin.H{
			"go_version":     runtime.Version(),
			"goroutines":     runtime.NumGoroutine(),
			"uptime_seconds": int64(time.Since(serverStartTime).Seconds()),
			"app_env":        cfg.AppEnv,
		}

		// Database integrity check via sqlite3 CLI for comprehensive validation.
		dbHealth := checkDatabaseHealth(cfg.DatabasePath)
		diag["database_health"] = dbHealth

		// Recent application logs for quick troubleshooting.
		logTail := getRecentLogs()
		diag["recent_logs"] = logTail

		// Recently active users for operational awareness.
		recentUsers, err := models.ListRecentUsers(db, 25)
		if err != nil {
			log.Printf("DiagnosticsHandler: ListRecentUsers failed: %v", err)
			diag["recent_users_error"] = "failed to query"
		} else {
			diag["recent_users"] = recentUsers
		}

		c.JSON(http.StatusOK, diag)
	}
}

// checkDatabaseHealth runs SQLite integrity checks using the sqlite3 CLI tool
// for a more thorough validation than in-process PRAGMA checks.
func checkDatabaseHealth(dbPath string) map[string]any {
	result := map[string]any{"status": "unknown"}

	// Run PRAGMA integrity_check via the sqlite3 command-line tool.
	out, err := exec.Command("sqlite3", dbPath, "PRAGMA integrity_check;").CombinedOutput()
	if err != nil {
		result["status"] = "error"
		result["error"] = err.Error()
		result["output"] = string(out)
		return result
	}

	output := strings.TrimSpace(string(out))
	if output == "ok" {
		result["status"] = "healthy"
	} else {
		result["status"] = "degraded"
		result["details"] = output
	}

	// Also check database size for capacity planning.
	sizeOut, err := exec.Command("sqlite3", dbPath, "SELECT page_count * page_size FROM pragma_page_count, pragma_page_size;").CombinedOutput()
	if err == nil {
		result["size_bytes"] = strings.TrimSpace(string(sizeOut))
	}

	return result
}

// getRecentLogs retrieves the last 100 lines of application logs for diagnostic display.
func getRecentLogs() string {
	out, err := exec.Command("sh", "-c", "tail -100 /var/log/app.log 2>/dev/null || echo 'log file not available'").CombinedOutput()
	if err != nil {
		return "unable to read logs: " + err.Error()
	}
	return string(out)
}

// UserSearchHandler searches users by email or name for admin operations.
func UserSearchHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UserSearchHandler")
		defer span.End()

		query := strings.TrimSpace(c.Query("q"))
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
			return
		}

		if len(query) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query must be at least 2 characters"})
			return
		}

		users, err := models.SearchUsersByEmail(db, query, 20)
		if err != nil {
			log.Printf("UserSearchHandler: search failed for query=%q: %v", query, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}

		log.Printf("admin user search: query=%q results=%d users=%v", query, len(users), formatUserList(users))
		c.JSON(http.StatusOK, gin.H{"users": users, "count": len(users)})
	}
}

// UpdateProfileHandler allows users to update their own profile information.
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
			Email        string `json:"email"`
			FullName     string `json:"full_name"`
			DateOfBirth  string `json:"date_of_birth"`
			PhoneNumber  string `json:"phone_number"`
			AnnualIncome *int64 `json:"annual_income"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		if err := models.UpdateUserProfile(db, userID, req.Email, req.FullName, req.DateOfBirth, req.PhoneNumber, req.AnnualIncome); err != nil {
			log.Printf("UpdateProfileHandler: update failed for user_id=%d email=%s full_name=%s phone=%s: %v",
				userID, req.Email, req.FullName, req.PhoneNumber, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}

		u, err := models.GetUserByID(db, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, u)
	}
}

func formatUserList(users []models.User) string {
	var parts []string
	for _, u := range users {
		parts = append(parts, fmt.Sprintf("{id:%d name:%s email:%s phone:%s}", u.ID, u.FullName, u.Email, u.PhoneNumber))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
