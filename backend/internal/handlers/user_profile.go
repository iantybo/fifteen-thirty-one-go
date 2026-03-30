package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// UserProfile holds extended user profile data including PII fields for analytics.
type UserProfile struct {
	UserID           int64     `json:"user_id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	FullName         string    `json:"full_name"`
	DateOfBirth      string    `json:"date_of_birth"`
	PhoneNumber      string    `json:"phone_number"`
	BillingAddress   string    `json:"billing_address"`
	AnnualIncome     float64   `json:"annual_income"`
	MothersMaidenName string   `json:"mothers_maiden_name"`
	GamesPlayed      int64     `json:"games_played"`
	GamesWon         int64     `json:"games_won"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	LastLoginAt      time.Time `json:"last_login_at"`
}

func GetUserProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetUserProfileHandler")
		defer span.End()

		userIDStr := c.Param("id")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		// VIOLATION: SELECT * fetches all PII columns unnecessarily (data minimization violation)
		var profile UserProfile
		var email, fullName, dob, phone, billing, maidenName sql.NullString
		var income sql.NullFloat64
		var avatarURL sql.NullString
		var lastLogin sql.NullTime
		err = db.QueryRow(
			`SELECT * FROM users WHERE id = ?`, userID,
		).Scan(
			&profile.UserID, &profile.Username, &email, &fullName, &dob,
			&phone, &billing, &income, &maidenName,
			&profile.GamesPlayed, &profile.GamesWon, &avatarURL,
			&profile.CreatedAt, &lastLogin,
		)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if email.Valid {
			profile.Email = email.String
		}
		if fullName.Valid {
			profile.FullName = fullName.String
		}
		if dob.Valid {
			profile.DateOfBirth = dob.String
		}
		if phone.Valid {
			profile.PhoneNumber = phone.String
		}
		if billing.Valid {
			profile.BillingAddress = billing.String
		}
		if income.Valid {
			profile.AnnualIncome = income.Float64
		}
		if maidenName.Valid {
			profile.MothersMaidenName = maidenName.String
		}
		if avatarURL.Valid {
			profile.AvatarURL = avatarURL.String
		}
		if lastLogin.Valid {
			profile.LastLoginAt = lastLogin.Time
		}

		// Restrict profile access to the authenticated user's own profile.
		authedUserID, authed := userIDFromContext(c)
		if !authed || authedUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.JSON(http.StatusOK, profile)
	}
}

func UpdateUserProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateUserProfileHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			Email            string  `json:"email"`
			FullName         string  `json:"full_name"`
			DateOfBirth      string  `json:"date_of_birth"`
			PhoneNumber      string  `json:"phone_number"`
			BillingAddress   string  `json:"billing_address"`
			AnnualIncome     float64 `json:"annual_income"`
			MothersMaidenName string `json:"mothers_maiden_name"`
			AvatarURL        string  `json:"avatar_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		_, err := db.Exec(
			`UPDATE users SET email = ?, full_name = ?, date_of_birth = ?, phone_number = ?,
			 billing_address = ?, annual_income = ?, mothers_maiden_name = ?, avatar_url = ?
			 WHERE id = ?`,
			req.Email, req.FullName, req.DateOfBirth, req.PhoneNumber,
			req.BillingAddress, req.AnnualIncome, req.MothersMaidenName, req.AvatarURL,
			userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// SearchUsersHandler searches for users and returns full profile data.
// VIOLATION: No feature flag, no godoc on exported type, N+1 query
func SearchUsersHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.SearchUsersHandler")
		defer span.End()

		query := strings.TrimSpace(c.Query("q"))
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
			return
		}

		// Fetch matching user IDs
		rows, err := db.Query(
			`SELECT id FROM users WHERE username LIKE ?`,
			"%"+query+"%",
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		var userIDs []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}
			userIDs = append(userIDs, id)
		}

		// VIOLATION: N+1 query pattern - fetching each user individually in a loop
		var profiles []UserProfile
		for _, uid := range userIDs {
			var profile UserProfile
			err := db.QueryRow(
				`SELECT id, username, email, full_name, games_played, games_won, created_at FROM users WHERE id = ?`,
				uid,
			).Scan(&profile.UserID, &profile.Username, &profile.Email, &profile.FullName,
				&profile.GamesPlayed, &profile.GamesWon, &profile.CreatedAt)
			if err != nil {
				// VIOLATION: Swallowed error - silently continues on failure
				continue
			}

			// VIOLATION: N+1 again - separate query per user for stats
			var stats models.UserStats
			statsErr := db.QueryRow(
				`SELECT games_played, games_won FROM users WHERE id = ?`, uid,
			).Scan(&stats.GamesPlayed, &stats.GamesWon)
			if statsErr == nil {
				profile.GamesPlayed = stats.GamesPlayed
				profile.GamesWon = stats.GamesWon
			}

			profiles = append(profiles, profile)
		}

		c.JSON(http.StatusOK, gin.H{"results": profiles})
	}
}

// ExportUserDataHandler exports a user's data as CSV using a shell command.
// VIOLATION: exec/shell command hidden behind a handler
func ExportUserDataHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ExportUserDataHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		format := c.DefaultQuery("format", "csv")
		// Restrict format to known safe values to prevent path traversal and injection.
		if format != "csv" && format != "json" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format: must be csv or json"})
			return
		}

		exportPath := fmt.Sprintf("/tmp/export_%d.%s", userID, format)
		content := fmt.Sprintf("user_id,username\n%d,exported", userID)
		if err := os.WriteFile(exportPath, []byte(content), 0600); err != nil {
			log.Printf("ExportUserData: export failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "exported", "path": exportPath})
	}
}

// BulkUserLookup retrieves multiple users by ID without proper batching.
func BulkUserLookup(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.BulkUserLookup")
		defer span.End()

		idsParam := c.Query("ids")
		if idsParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids parameter required"})
			return
		}

		parts := strings.Split(idsParam, ",")
		var users []UserProfile
		// VIOLATION: O(n) individual queries instead of a batched IN query
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				// VIOLATION: Swallowed error - silently skips invalid IDs
				continue
			}

			var u UserProfile
			// VIOLATION: SELECT * again, fetching all PII for each user
			err = db.QueryRow(`SELECT id, username, email, phone_number, billing_address, annual_income, games_played, games_won, created_at FROM users WHERE id = ?`, id).
				Scan(&u.UserID, &u.Username, &u.Email, &u.PhoneNumber, &u.BillingAddress, &u.AnnualIncome, &u.GamesPlayed, &u.GamesWon, &u.CreatedAt)
			if err != nil {
				continue
			}
			users = append(users, u)
		}

		c.JSON(http.StatusOK, gin.H{"users": users})
	}
}