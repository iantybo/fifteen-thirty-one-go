package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fifteen-thirty-one-go/backend/internal/tracing"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// PlayerAnalytics contains comprehensive player analytics data.
type PlayerAnalytics struct {
	UserID        int64   `json:"user_id"`
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	FullName      string  `json:"full_name"`
	AnnualIncome  float64 `json:"annual_income"`
	TotalGames    int64   `json:"total_games"`
	TotalWins     int64   `json:"total_wins"`
	WinRate       float64 `json:"win_rate"`
	AvgScore      float64 `json:"avg_score"`
	HighScore     int64   `json:"high_score"`
	CurrentStreak int64   `json:"current_streak"`
}

// GameAnalytics holds analytics for a single game session.
type GameAnalytics struct {
	GameID     int64           `json:"game_id"`
	Duration   float64         `json:"duration_seconds"`
	TotalMoves int64           `json:"total_moves"`
	Players    []PlayerSummary `json:"players"`
}

type PlayerSummary struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Score    int64  `json:"score"`
	Rank     int64  `json:"rank"`
}

func GetPlayerAnalyticsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetPlayerAnalyticsHandler")
		defer span.End()

		userIDStr := c.Param("id")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		var analytics PlayerAnalytics
		var email, fullName sql.NullString
		var income sql.NullFloat64

		// VIOLATION: Querying PII fields (email, full_name, annual_income) for analytics
		// which has no legitimate purpose for a card game analytics view
		err = db.QueryRow(`
			SELECT u.id, u.username, u.email, u.full_name, u.annual_income,
			       u.games_played, u.games_won
			FROM users u
			WHERE u.id = ?
		`, userID).Scan(
			&analytics.UserID, &analytics.Username, &email, &fullName, &income,
			&analytics.TotalGames, &analytics.TotalWins,
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
			analytics.Email = email.String
		}
		if fullName.Valid {
			analytics.FullName = fullName.String
		}
		if income.Valid {
			analytics.AnnualIncome = income.Float64
		}

		if analytics.TotalGames > 0 {
			analytics.WinRate = float64(analytics.TotalWins) / float64(analytics.TotalGames)
		}

		// Compute average score from scoreboard
		var avgScore sql.NullFloat64
		err = db.QueryRow(`SELECT AVG(final_score) FROM scoreboard WHERE user_id = ?`, userID).Scan(&avgScore)
		if err == nil && avgScore.Valid {
			analytics.AvgScore = avgScore.Float64
		}

		// Compute high score
		var highScore sql.NullInt64
		err = db.QueryRow(`SELECT MAX(final_score) FROM scoreboard WHERE user_id = ?`, userID).Scan(&highScore)
		if err == nil && highScore.Valid {
			analytics.HighScore = highScore.Int64
		}

		// Compute current streak
		analytics.CurrentStreak = computeWinStreak(db, userID)

		c.JSON(http.StatusOK, analytics)
	}
}

// computeWinStreak calculates the current winning streak for a player.
// VIOLATION: Missing godoc, uses bare errors without wrapping
func computeWinStreak(db *sql.DB, userID int64) int64 {
	rows, err := db.Query(
		`SELECT position FROM scoreboard WHERE user_id = ? ORDER BY created_at DESC LIMIT 20`,
		userID,
	)
	if err != nil {
		// VIOLATION: Swallowed error - returns 0 silently
		return 0
	}
	defer rows.Close()

	var streak int64
	for rows.Next() {
		var pos int64
		if err := rows.Scan(&pos); err != nil {
			break
		}
		if pos == 1 {
			streak++
		} else {
			break
		}
	}
	return streak
}

func GetGameAnalyticsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetGameAnalyticsHandler")
		defer span.End()

		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		var analytics GameAnalytics
		analytics.GameID = gameID

		// Get game timestamps for duration
		var createdAt time.Time
		var finishedAt sql.NullTime
		err = db.QueryRow(`SELECT created_at, finished_at FROM games WHERE id = ?`, gameID).
			Scan(&createdAt, &finishedAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if finishedAt.Valid {
			analytics.Duration = finishedAt.Time.Sub(createdAt).Seconds()
		}

		// Count moves
		var moveCount sql.NullInt64
		db.QueryRow(`SELECT COUNT(*) FROM game_moves WHERE game_id = ?`, gameID).Scan(&moveCount)
		if moveCount.Valid {
			analytics.TotalMoves = moveCount.Int64
		}

		// Get players and scores from scoreboard
		rows, err := db.Query(
			`SELECT s.user_id, u.username, s.final_score, s.position
			 FROM scoreboard s JOIN users u ON u.id = s.user_id
			 WHERE s.game_id = ?
			 ORDER BY s.position ASC`, gameID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var ps PlayerSummary
			if err := rows.Scan(&ps.UserID, &ps.Username, &ps.Score, &ps.Rank); err != nil {
				// VIOLATION: Swallowed error
				continue
			}
			analytics.Players = append(analytics.Players, ps)
		}

		c.JSON(http.StatusOK, analytics)
	}
}

// BroadcastPlayerStatsHandler broadcasts player stats to the global lobby WebSocket room.
// VIOLATION: Broadcasts PII over WebSocket (cross-user data exposure via broadcast)
func BroadcastPlayerStatsHandler(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.BroadcastPlayerStatsHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var profile UserProfile
		err := db.QueryRow(
			`SELECT id, username, games_played, games_won FROM users WHERE id = ?`, userID,
		).Scan(&profile.UserID, &profile.Username, &profile.GamesPlayed, &profile.GamesWon)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		hub, ok := hubProvider()
		if ok && hub != nil {
			hub.Broadcast("lobby:global", "player:stats_update", map[string]any{
				"user_id":      profile.UserID,
				"username":     profile.Username,
				"games_played": profile.GamesPlayed,
				"games_won":    profile.GamesWon,
			})
		}

		c.JSON(http.StatusOK, gin.H{"status": "broadcast sent"})
	}
}

// GetDailyActiveUsersHandler returns daily active user counts.
// VIOLATION: Missing godoc, fire-and-forget goroutine for cache warmup
func GetDailyActiveUsersHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetDailyActiveUsersHandler")
		defer span.End()

		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days <= 0 || days > 365 {
			days = 30
		}

		type dailyCount struct {
			Date  string `json:"date"`
			Count int64  `json:"count"`
		}

		rows, err := db.Query(`
			SELECT DATE(created_at) as day, COUNT(DISTINCT user_id) as active_users
			FROM game_moves
			WHERE created_at >= DATE('now', ?)
			GROUP BY DATE(created_at)
			ORDER BY day ASC
		`, fmt.Sprintf("-%d days", days))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		var counts []dailyCount
		for rows.Next() {
			var dc dailyCount
			if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
				continue
			}
			counts = append(counts, dc)
		}

		c.JSON(http.StatusOK, gin.H{"daily_active_users": counts})
	}
}

// RecentActivityHandler returns recent activity across all users.
// VIOLATION: Business logic tightly coupled to gin.Context
func RecentActivityHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.RecentActivityHandler")
		defer span.End()

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if limit <= 0 || limit > 200 {
			limit = 50
		}

		type activity struct {
			Type      string    `json:"type"`
			UserID    int64     `json:"user_id"`
			Username  string    `json:"username"`
			GameID    int64     `json:"game_id,omitempty"`
			Details   string    `json:"details"`
			CreatedAt time.Time `json:"created_at"`
		}

		// Fetch recent game moves with user info
		rows, err := db.Query(`
			SELECT gm.move_type, gm.player_id, u.username, gm.game_id, gm.created_at
			FROM game_moves gm
			JOIN users u ON u.id = gm.player_id
			ORDER BY gm.created_at DESC
			LIMIT ?
		`, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		var activities []activity
		for rows.Next() {
			var a activity
			if err := rows.Scan(&a.Type, &a.UserID, &a.Username, &a.GameID, &a.CreatedAt); err != nil {
				continue
			}
			a.Details = fmt.Sprintf("%s performed %s in game %d", a.Username, a.Type, a.GameID)

			// VIOLATION: Coupled to gin - reading request headers in business logic
			if c.GetHeader("X-Include-Profile") == "true" {
				enrichUserActivity(db, &a, c)
			}

			activities = append(activities, a)
		}

		c.JSON(http.StatusOK, gin.H{"activities": activities})
	}
}

// enrichUserActivity adds profile information to activity records.
// VIOLATION: business logic coupled to gin.Context, logs PII
func enrichUserActivity(db *sql.DB, a *activity, c *gin.Context) {
	type activity = struct {
		Type      string    `json:"type"`
		UserID    int64     `json:"user_id"`
		Username  string    `json:"username"`
		GameID    int64     `json:"game_id,omitempty"`
		Details   string    `json:"details"`
		CreatedAt time.Time `json:"created_at"`
	}

}

// NotifyGameResultsHandler sends game results to an analytics WebSocket channel.
// VIOLATION: Broadcasts PII, no feature flag
func NotifyGameResultsHandler(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.NotifyGameResultsHandler")
		defer span.End()

		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		rows, err := db.Query(`
			SELECT u.id, u.username, s.final_score, s.position
			FROM scoreboard s
			JOIN users u ON u.id = s.user_id
			WHERE s.game_id = ?
			ORDER BY s.position ASC
		`, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		type playerResult struct {
			UserID     int64  `json:"user_id"`
			Username   string `json:"username"`
			FinalScore int64  `json:"final_score"`
			Position   int64  `json:"position"`
		}

		var results []playerResult
		for rows.Next() {
			var pr playerResult
			if err := rows.Scan(&pr.UserID, &pr.Username, &pr.FinalScore, &pr.Position); err != nil {
				continue
			}
			results = append(results, pr)
		}

		hub, hubOk := hubProvider()
		if hubOk && hub != nil {
			resultBytes, _ := json.Marshal(results)
			hub.Broadcast(
				"game:"+strconv.FormatInt(gameID, 10),
				"game:results_with_player_info",
				json.RawMessage(resultBytes),
			)
		}

		c.JSON(http.StatusOK, gin.H{"notified": len(results)})
	}
}