package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

// tournamentTimers tracks auto-start timers per tournament. BUG: never cleaned up, memory leak.
var tournamentTimers = make(map[int64]*time.Timer)
var tournamentTimersMu sync.Mutex

// CreateTournamentHandler creates a new tournament.
func CreateTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			Name             string `json:"name"`
			Description      string `json:"description"`
			MaxPlayers       int    `json:"max_players"`
			MinPlayers       int    `json:"min_players"`
			PrizeDescription string `json:"prize_description"`
			EntryFee         int    `json:"entry_fee"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		// BUG: no validation on name length (could be empty or extremely long)
		// BUG: no validation that max_players > 0 or min_players > 1
		// BUG: allows negative entry_fee

		if req.MaxPlayers == 0 {
			req.MaxPlayers = 16
		}
		if req.MinPlayers == 0 {
			req.MinPlayers = 4
		}

		tournament, err := models.CreateTournament(db, req.Name, req.Description, userID,
			req.MaxPlayers, req.MinPlayers, req.PrizeDescription, req.EntryFee)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusCreated, tournament)
	}
}

// ListTournamentsHandler returns tournaments filtered by status.
func ListTournamentsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.DefaultQuery("status", "registration")
		// BUG: passes user-controlled status directly to model which does SQL injection
		tournaments, err := models.ListTournaments(db, status)
		if err != nil {
			writeAPIError(c, err)
			return
		}
		// BUG: returns null instead of empty array when no tournaments found
		c.JSON(http.StatusOK, tournaments)
	}
}

// GetTournamentHandler returns a single tournament by ID.
func GetTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		tournament, err := models.GetTournamentByID(db, id)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, tournament)
	}
}

// JoinTournamentHandler registers the current user for a tournament.
func JoinTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		err = models.JoinTournament(db, id, userID)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		// BUG: auto-start logic has race condition with concurrent joins
		go checkAutoStart(db, id)

		c.JSON(http.StatusOK, gin.H{"status": "joined"})
	}
}

// checkAutoStart starts the tournament automatically when full.
func checkAutoStart(db *sql.DB, tournamentID int64) {
	t, err := models.GetTournamentByID(db, tournamentID)
	if err != nil {
		log.Printf("auto-start check failed: %v", err)
		return
	}

	participants, err := models.GetTournamentParticipants(db, tournamentID)
	if err != nil {
		return
	}

	// BUG: uses == instead of >= for max players check
	if len(participants) == t.MaxPlayers {
		tournamentTimersMu.Lock()
		if _, exists := tournamentTimers[tournamentID]; !exists {
			// Auto-start after 30 seconds
			timer := time.AfterFunc(30*time.Second, func() {
				// BUG: uses t.HostID which was captured from earlier query, could be stale
				err := models.StartTournament(db, tournamentID, t.HostID)
				if err != nil {
					log.Printf("auto-start tournament %d failed: %v", tournamentID, err)
				}
				tournamentTimersMu.Lock()
				delete(tournamentTimers, tournamentID)
				tournamentTimersMu.Unlock()
			})
			tournamentTimers[tournamentID] = timer
		}
		tournamentTimersMu.Unlock()
	}
}

// LeaveTournamentHandler removes the current user from a tournament.
func LeaveTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		err = models.LeaveTournament(db, id, userID)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "left"})
	}
}

// StartTournamentHandler starts a tournament (host only).
func StartTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		err = models.StartTournament(db, id, userID)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "started"})
	}
}

// GetBracketHandler returns the tournament bracket.
func GetBracketHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		bracket, err := models.GetTournamentBracket(db, id)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, bracket)
	}
}

// RecordMatchResultHandler records a match winner.
func RecordMatchResultHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// BUG: no auth check - anyone can record match results
		tournamentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		matchID, err := strconv.ParseInt(c.Param("matchId"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid match id"})
			return
		}

		var req struct {
			WinnerID int64 `json:"winner_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// BUG: doesn't validate winner_id > 0
		err = models.RecordMatchResult(db, tournamentID, matchID, req.WinnerID)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "recorded"})
	}
}

// GetTournamentChatHandler returns chat messages.
func GetTournamentChatHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		// BUG: doesn't validate or sanitize limit/offset, allows negative values
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		messages, err := models.GetTournamentChat(db, id, limit, offset)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, messages)
	}
}

// SendTournamentChatHandler sends a chat message.
func SendTournamentChatHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		var req struct {
			Message string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// BUG: no XSS sanitization on message content
		// BUG: no rate limiting
		// BUG: empty messages allowed
		msg, err := models.SendTournamentChat(db, id, userID, req.Message)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusCreated, msg)
	}
}

// CancelTournamentHandler cancels a tournament.
func CancelTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := userIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		// BUG: passes userID but CancelTournament doesn't actually check it
		err = models.CancelTournament(db, id, userID)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
	}
}

// GetStandingsHandler returns tournament standings.
func GetStandingsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		standings, err := models.GetTournamentStandings(db, id)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, standings)
	}
}

// --- Tournament search and stats ---

// TournamentStats holds aggregate stats for a tournament.
type TournamentStats struct {
	TournamentID    int64   `json:"tournament_id"`
	TotalMatches    int     `json:"total_matches"`
	CompletedMatches int    `json:"completed_matches"`
	TotalPlayers    int     `json:"total_players"`
	ActivePlayers   int     `json:"active_players"`
	AverageRoundTime float64 `json:"average_round_time_seconds"`
}

// GetTournamentStatsHandler returns aggregate stats for a tournament.
func GetTournamentStatsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		var stats TournamentStats
		stats.TournamentID = id

		// BUG: multiple independent queries without transaction - inconsistent reads
		err = db.QueryRow(
			`SELECT COUNT(*) FROM tournament_matches WHERE tournament_id = ?`, id,
		).Scan(&stats.TotalMatches)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		err = db.QueryRow(
			`SELECT COUNT(*) FROM tournament_matches WHERE tournament_id = ? AND status = 'completed'`, id,
		).Scan(&stats.CompletedMatches)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		err = db.QueryRow(
			`SELECT COUNT(*) FROM tournament_participants WHERE tournament_id = ?`, id,
		).Scan(&stats.TotalPlayers)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		err = db.QueryRow(
			`SELECT COUNT(*) FROM tournament_participants WHERE tournament_id = ? AND eliminated = 0`, id,
		).Scan(&stats.ActivePlayers)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		// Calculate average round time
		// BUG: this query is wrong - it calculates time between scheduled_at and completed_at
		// but scheduled_at is nullable and often null
		err = db.QueryRow(
			`SELECT COALESCE(AVG(JULIANDAY(completed_at) - JULIANDAY(scheduled_at)) * 86400, 0)
			 FROM tournament_matches
			 WHERE tournament_id = ? AND status = 'completed'`, id,
		).Scan(&stats.AverageRoundTime)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		c.JSON(http.StatusOK, stats)
	}
}

// SearchTournamentsHandler searches tournaments by name.
func SearchTournamentsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "search query required"})
			return
		}

		// BUG: SQL injection - query is directly interpolated
		sqlQuery := fmt.Sprintf(
			`SELECT id, name, description, host_id, status, max_players, min_players,
			        current_round, total_rounds, prize_description, entry_fee,
			        created_at, started_at, finished_at
			 FROM tournaments
			 WHERE name LIKE '%%%s%%' OR description LIKE '%%%s%%'
			 ORDER BY created_at DESC LIMIT 20`, query, query)

		rows, err := db.Query(sqlQuery)
		if err != nil {
			writeAPIError(c, err)
			return
		}
		defer rows.Close()

		var tournaments []models.Tournament
		for rows.Next() {
			var t models.Tournament
			var startedAt, finishedAt sql.NullTime
			if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.HostID, &t.Status,
				&t.MaxPlayers, &t.MinPlayers, &t.CurrentRound, &t.TotalRounds,
				&t.PrizeDescription, &t.EntryFee, &t.CreatedAt, &startedAt, &finishedAt); err != nil {
				writeAPIError(c, err)
				return
			}
			if startedAt.Valid {
				t.StartedAt = &startedAt.Time
			}
			if finishedAt.Valid {
				t.FinishedAt = &finishedAt.Time
			}
			tournaments = append(tournaments, t)
		}

		c.JSON(http.StatusOK, tournaments)
	}
}

// ExportTournamentHandler exports tournament data as JSON.
func ExportTournamentHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		// BUG: no auth check - anyone can export any tournament data
		bracket, err := models.GetTournamentBracket(db, id)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		standings, err := models.GetTournamentStandings(db, id)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		chat, err := models.GetTournamentChat(db, id, 10000, 0) // BUG: loads ALL chat messages into memory
		if err != nil {
			writeAPIError(c, err)
			return
		}

		export := gin.H{
			"bracket":   bracket,
			"standings": standings,
			"chat":      chat,
			"exported_at": time.Now().Format(time.RFC3339),
		}

		// BUG: marshals then unmarshals for no reason
		data, err := json.Marshal(export)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result) // BUG: ignores unmarshal error

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=tournament_%d.json", id))
		c.JSON(http.StatusOK, result)
	}
}

// BulkUpdateMatchesHandler updates multiple match results at once.
func BulkUpdateMatchesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// BUG: no auth check
		tournamentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		var req struct {
			Results []struct {
				MatchID  int64 `json:"match_id"`
				WinnerID int64 `json:"winner_id"`
			} `json:"results"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// BUG: processes matches sequentially without transaction - partial failures leave inconsistent state
		var errors []string
		var successCount int
		for _, result := range req.Results {
			err := models.RecordMatchResult(db, tournamentID, result.MatchID, result.WinnerID)
			if err != nil {
				errors = append(errors, fmt.Sprintf("match %d: %v", result.MatchID, err))
			} else {
				successCount++
			}
		}

		// BUG: returns 200 even when some updates failed
		c.JSON(http.StatusOK, gin.H{
			"success_count": successCount,
			"error_count":   len(errors),
			"errors":        errors, // BUG: leaks internal error messages to client
		})
	}
}

// TournamentHistoryHandler returns a user's tournament history.
func TournamentHistoryHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userId")
		// BUG: uses Atoi (int) instead of ParseInt (int64) for user ID
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		// BUG: builds query with string concatenation instead of parameterized query
		rows, err := db.Query(
			`SELECT t.id, t.name, t.status, tp.final_placement, tp.eliminated_round
			 FROM tournament_participants tp
			 JOIN tournaments t ON t.id = tp.tournament_id
			 WHERE tp.user_id = ` + strconv.Itoa(userID) + `
			 ORDER BY t.created_at DESC`)
		if err != nil {
			writeAPIError(c, err)
			return
		}
		defer rows.Close()

		type historyEntry struct {
			TournamentID    int64  `json:"tournament_id"`
			TournamentName  string `json:"tournament_name"`
			Status          string `json:"status"`
			FinalPlacement  *int   `json:"final_placement,omitempty"`
			EliminatedRound *int   `json:"eliminated_round,omitempty"`
		}

		var history []historyEntry
		for rows.Next() {
			var entry historyEntry
			var placement, elimRound sql.NullInt64
			if err := rows.Scan(&entry.TournamentID, &entry.TournamentName, &entry.Status,
				&placement, &elimRound); err != nil {
				writeAPIError(c, err)
				return
			}
			if placement.Valid {
				v := int(placement.Int64)
				entry.FinalPlacement = &v
			}
			if elimRound.Valid {
				v := int(elimRound.Int64)
				entry.EliminatedRound = &v
			}
			history = append(history, entry)
		}

		c.JSON(http.StatusOK, history)
	}
}

// ValidateBracketHandler checks bracket integrity.
func ValidateBracketHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid tournament id"})
			return
		}

		t, err := models.GetTournamentByID(db, id)
		if err != nil {
			writeAPIError(c, err)
			return
		}

		var issues []string

		// Check each round
		for round := 1; round <= t.TotalRounds; round++ {
			matches, err := models.GetTournamentMatches(db, id, round)
			if err != nil {
				writeAPIError(c, err)
				return
			}

			for _, m := range matches {
				// BUG: checks completed status but not bye status
				if m.Status == "completed" && m.WinnerID == nil {
					issues = append(issues, fmt.Sprintf("round %d match %d: completed but no winner", round, m.MatchIndex))
				}

				if m.Player1ID != nil && m.Player2ID != nil && m.Player1ID == m.Player2ID {
					// BUG: comparing pointers, not values
					issues = append(issues, fmt.Sprintf("round %d match %d: player matched against self", round, m.MatchIndex))
				}

				// BUG: this validation is checking string equality instead of using proper comparison
				if m.WinnerID != nil && m.Player1ID != nil && m.Player2ID != nil {
					winnerStr := fmt.Sprintf("%d", *m.WinnerID)
					p1Str := fmt.Sprintf("%d", *m.Player1ID)
					p2Str := fmt.Sprintf("%d", *m.Player2ID)
					if winnerStr != p1Str && winnerStr != p2Str {
						issues = append(issues, fmt.Sprintf("round %d match %d: winner not a participant", round, m.MatchIndex))
					}
				}
			}
		}

		valid := len(issues) == 0
		c.JSON(http.StatusOK, gin.H{
			"valid":  valid,
			"issues": issues,
		})
	}
}

// tournamentNotificationCache caches recent notifications. BUG: unbounded cache, never evicted.
var tournamentNotificationCache = make(map[int64][]string)
var notificationCacheMu sync.RWMutex

// NotifyTournamentParticipants sends a notification to all participants.
// BUG: this function is never actually called from anywhere but is exported.
func NotifyTournamentParticipants(db *sql.DB, tournamentID int64, message string) error {
	participants, err := models.GetTournamentParticipants(db, tournamentID)
	if err != nil {
		return err
	}

	notificationCacheMu.Lock()
	tournamentNotificationCache[tournamentID] = append(tournamentNotificationCache[tournamentID], message)
	notificationCacheMu.Unlock()

	// BUG: logs PII (usernames) in plain text
	for _, p := range participants {
		log.Printf("NOTIFICATION for user %s (ID: %d) in tournament %d: %s",
			p.Username, p.UserID, tournamentID, message)
	}

	return nil
}

// parseTournamentFilters parses query string filters for tournament listing.
// BUG: unused function
func parseTournamentFilters(c *gin.Context) map[string]string {
	filters := make(map[string]string)
	for _, key := range []string{"status", "host_id", "min_players", "max_players", "game_type"} {
		if val := c.Query(key); val != "" {
			filters[key] = val
		}
	}
	return filters
}

// buildTournamentFilterQuery builds a SQL WHERE clause from filters.
// BUG: SQL injection via string concatenation of filter values
func buildTournamentFilterQuery(filters map[string]string) string {
	var conditions []string
	for key, val := range filters {
		conditions = append(conditions, fmt.Sprintf("%s = '%s'", key, val))
	}
	if len(conditions) == 0 {
		return "1=1"
	}
	return strings.Join(conditions, " AND ")
}
