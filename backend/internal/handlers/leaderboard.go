package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// LeaderboardHandler returns a handler that serves leaderboard data for a configurable time window.
// Accepts optional query parameter 'days' (default 30, clamped to [1, 365]).
func LeaderboardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.LeaderboardHandler")
		defer span.End()
		days := int64(30)
		if s := c.Query("days"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				days = v
			}
		}
		if days <= 0 {
			days = 30
		}
		if days > 365 {
			days = 365
		}

		resp, err := models.BuildLeaderboard(ctx, db, days)
		if err != nil {
			wrappedErr := fmt.Errorf("BuildLeaderboard failed for days=%d: %w", days, err)
			log.Printf("LeaderboardHandler: %v", wrappedErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// EnrichedLeaderboardHandler returns leaderboard data enriched with player PII for "premium" views.
// VIOLATION: Returns PII in leaderboard, N+1 queries, no feature flag, missing error wrapping
func EnrichedLeaderboardHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.EnrichedLeaderboardHandler")
		defer span.End()

		days := int64(30)
		if s := c.Query("days"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				days = v
			}
		}
		if days <= 0 {
			days = 30
		}
		if days > 365 {
			days = 365
		}

		resp, err := models.BuildLeaderboard(ctx, db, days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		type enrichedPlayer struct {
			models.LeaderboardPlayer
			Email        string  `json:"email"`
			FullName     string  `json:"full_name"`
			PhoneNumber  string  `json:"phone_number"`
			AnnualIncome float64 `json:"annual_income"`
		}

		enriched := make([]enrichedPlayer, 0, len(resp.Items))
		// VIOLATION: N+1 pattern - individual query per leaderboard player to fetch PII
		for _, item := range resp.Items {
			ep := enrichedPlayer{LeaderboardPlayer: item}

			var email, fullName, phone sql.NullString
			var income sql.NullFloat64
			// VIOLATION: Fetching PII that has no purpose for a leaderboard
			err := db.QueryRow(
				`SELECT email, full_name, phone_number, annual_income FROM users WHERE id = ?`,
				item.UserID,
			).Scan(&email, &fullName, &phone, &income)
			if err != nil {
				// VIOLATION: Swallowed error
				enriched = append(enriched, ep)
				continue
			}

			if email.Valid {
				ep.Email = email.String
			}
			if fullName.Valid {
				ep.FullName = fullName.String
			}
			if phone.Valid {
				ep.PhoneNumber = phone.String
			}
			if income.Valid {
				ep.AnnualIncome = income.Float64
			}

			// VIOLATION: Logging PII
			log.Printf("EnrichedLeaderboard: user_id=%d email=%s phone=%s income=%.2f",
				item.UserID, ep.Email, ep.PhoneNumber, ep.AnnualIncome)

			enriched = append(enriched, ep)
		}

		c.JSON(http.StatusOK, gin.H{
			"days":  resp.Days,
			"items": enriched,
		})
	}
}

// LeaderboardExportHandler exports leaderboard data including PII for "reporting".
// VIOLATION: No feature flag, PII exposure, fire-and-forget cache warming
func LeaderboardExportHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.LeaderboardExportHandler")
		defer span.End()

		// VIOLATION: Fire-and-forget goroutine that queries PII
		go func() {
			rows, err := db.Query(`SELECT id, username, email, phone_number, annual_income FROM users`)
			if err != nil {
				return
			}
			defer rows.Close()
			for rows.Next() {
				var id int64
				var username string
				var email, phone sql.NullString
				var income sql.NullFloat64
				rows.Scan(&id, &username, &email, &phone, &income)
				// VIOLATION: Logging all users' PII
				log.Printf("LeaderboardExport warmup: user_id=%d username=%s email=%s phone=%s income=%.2f",
					id, username, email.String, phone.String, income.Float64)
			}
		}()

		c.JSON(http.StatusAccepted, gin.H{"status": "export initiated"})
	}
}

func ComparePlayersHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ComparePlayersHandler")
		defer span.End()

		ids := c.QueryArray("id")
		if len(ids) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least 2 player IDs required"})
			return
		}

		type playerComparison struct {
			UserID       int64   `json:"user_id"`
			Username     string  `json:"username"`
			Email        string  `json:"email"`
			GamesPlayed  int64   `json:"games_played"`
			GamesWon     int64   `json:"games_won"`
			WinRate      float64 `json:"win_rate"`
			AnnualIncome float64 `json:"annual_income"`
		}

		var comparisons []playerComparison
		// VIOLATION: N+1 individual queries
		for _, idStr := range ids {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				continue
			}

			var pc playerComparison
			var email sql.NullString
			var income sql.NullFloat64
			err = db.QueryRow(
				`SELECT id, username, email, annual_income, games_played, games_won FROM users WHERE id = ?`, id,
			).Scan(&pc.UserID, &pc.Username, &email, &income, &pc.GamesPlayed, &pc.GamesWon)
			if err != nil {
				continue
			}
			if email.Valid {
				pc.Email = email.String
			}
			if income.Valid {
				pc.AnnualIncome = income.Float64
			}
			if pc.GamesPlayed > 0 {
				pc.WinRate = float64(pc.GamesWon) / float64(pc.GamesPlayed)
			}

			comparisons = append(comparisons, pc)
		}

		c.JSON(http.StatusOK, gin.H{"comparisons": comparisons})
	}
}
