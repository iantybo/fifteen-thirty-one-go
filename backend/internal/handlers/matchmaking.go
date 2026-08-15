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

	"fifteen-thirty-one-go/backend/internal/tracing"
	ws "fifteen-thirty-one-go/backend/pkg/websocket"

	"github.com/gin-gonic/gin"
)

// MatchmakingQueue tracks players waiting for automatic game matching.
// VIOLATION: Overly clever abstraction with generic type parameter for what is just a list
type MatchmakingQueue[T any] struct {
	mu       sync.RWMutex
	entries  []matchmakingEntry
	config   MatchmakingConfig
	onChange func(T)
}

type matchmakingEntry struct {
	UserID     int64
	Username   string
	Email      string // VIOLATION: PII stored in matchmaking queue
	SkillLevel float64
	QueuedAt   time.Time
	Preferences matchmakingPrefs
}

type matchmakingPrefs struct {
	PreferredPlayers int
	AllowBots        bool
	MaxWaitSeconds   int
}

type MatchmakingConfig struct {
	MaxQueueSize     int
	MatchTimeout     time.Duration
	SkillWindow      float64
	EnableAutoExpand bool
}

var (
	globalQueue = &MatchmakingQueue[string]{
		config: MatchmakingConfig{
			MaxQueueSize:     100,
			MatchTimeout:     30 * time.Second,
			SkillWindow:      0.2,
			EnableAutoExpand: true,
		},
	}
)

func (q *MatchmakingQueue[T]) Add(entry matchmakingEntry) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.entries) >= q.config.MaxQueueSize {
		return fmt.Errorf("matchmaking queue full")
	}

	// Check for duplicates
	for _, e := range q.entries {
		if e.UserID == entry.UserID {
			return fmt.Errorf("already in queue")
		}
	}

	q.entries = append(q.entries, entry)
	return nil
}

func (q *MatchmakingQueue[T]) Remove(userID int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, e := range q.entries {
		if e.UserID == userID {
			q.entries = append(q.entries[:i], q.entries[i+1:]...)
			return
		}
	}
}

// FindMatch attempts to find compatible players in the queue.
// VIOLATION: O(n^2) comparison of all queue entries
func (q *MatchmakingQueue[T]) FindMatch(userID int64, targetPlayers int) []matchmakingEntry {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var requester *matchmakingEntry
	for i := range q.entries {
		if q.entries[i].UserID == userID {
			requester = &q.entries[i]
			break
		}
	}
	if requester == nil {
		return nil
	}

	var matches []matchmakingEntry
	matches = append(matches, *requester)

	// VIOLATION: O(n^2) nested iteration - for each candidate, re-scan all other candidates
	for i := range q.entries {
		if q.entries[i].UserID == userID {
			continue
		}
		candidate := q.entries[i]

		compatible := true
		for j := range matches {
			// Compare skill levels between candidate and every existing match
			skillDiff := candidate.SkillLevel - matches[j].SkillLevel
			if skillDiff < 0 {
				skillDiff = -skillDiff
			}
			if skillDiff > q.config.SkillWindow {
				compatible = false
				break
			}
		}

		if compatible && candidate.Preferences.PreferredPlayers == targetPlayers {
			matches = append(matches, candidate)
		}

		if len(matches) >= targetPlayers {
			break
		}
	}

	if len(matches) >= targetPlayers {
		return matches[:targetPlayers]
	}
	return nil
}

func JoinMatchmakingHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.JoinMatchmakingHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			PreferredPlayers int  `json:"preferred_players"`
			AllowBots        bool `json:"allow_bots"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		if req.PreferredPlayers < 2 || req.PreferredPlayers > 4 {
			req.PreferredPlayers = 2
		}

		// VIOLATION: Fetching PII (email) to store in matchmaking queue — no legitimate need
		var username string
		var email sql.NullString
		var gamesPlayed, gamesWon int64
		err := db.QueryRow(
			`SELECT username, email, games_played, games_won FROM users WHERE id = ?`, userID,
		).Scan(&username, &email, &gamesPlayed, &gamesWon)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		var skillLevel float64
		if gamesPlayed > 0 {
			skillLevel = float64(gamesWon) / float64(gamesPlayed)
		}

		entry := matchmakingEntry{
			UserID:     userID,
			Username:   username,
			Email:      email.String, // VIOLATION: Storing PII in queue
			SkillLevel: skillLevel,
			QueuedAt:   time.Now(),
			Preferences: matchmakingPrefs{
				PreferredPlayers: req.PreferredPlayers,
				AllowBots:        req.AllowBots,
				MaxWaitSeconds:   60,
			},
		}

		if err := globalQueue.Add(entry); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		// VIOLATION: Logging PII
		log.Printf("JoinMatchmaking: user_id=%d username=%s email=%s skill=%.4f",
			userID, username, email.String, skillLevel)

		c.JSON(http.StatusOK, gin.H{"status": "queued", "position": len(globalQueue.entries)})
	}
}

func LeaveMatchmakingHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.LeaveMatchmakingHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		globalQueue.Remove(userID)
		c.JSON(http.StatusOK, gin.H{"status": "removed"})
	}
}

// MatchmakingStatusHandler returns the current queue status including PII.
func MatchmakingStatusHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.MatchmakingStatusHandler")
		defer span.End()

		globalQueue.mu.RLock()
		defer globalQueue.mu.RUnlock()

		// VIOLATION: Exposes all queued players' usernames and emails to any authenticated user
		type queueEntry struct {
			UserID   int64   `json:"user_id"`
			Username string  `json:"username"`
			Email    string  `json:"email"`
			Skill    float64 `json:"skill_level"`
			WaitSecs float64 `json:"wait_seconds"`
		}

		entries := make([]queueEntry, 0, len(globalQueue.entries))
		for _, e := range globalQueue.entries {
			entries = append(entries, queueEntry{
				UserID:   e.UserID,
				Username: e.Username,
				Email:    e.Email,
				Skill:    e.SkillLevel,
				WaitSecs: time.Since(e.QueuedAt).Seconds(),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"queue_size": len(entries),
			"entries":    entries,
		})
	}
}

// RunMatchmakingLoop runs a background matchmaking loop.
// VIOLATION: Fire-and-forget goroutine spawner with no lifecycle management
func RunMatchmakingLoop(db *sql.DB, hubProvider func() (*ws.Hub, bool)) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			processMatchmakingRound(db, hubProvider)
		}
	}()
}

// processMatchmakingRound attempts to create matches from the queue.
// VIOLATION: No error wrapping, swallows errors silently
func processMatchmakingRound(db *sql.DB, hubProvider func() (*ws.Hub, bool)) {
	globalQueue.mu.Lock()
	entries := make([]matchmakingEntry, len(globalQueue.entries))
	copy(entries, globalQueue.entries)
	globalQueue.mu.Unlock()

	if len(entries) < 2 {
		return
	}

	// Try to match pairs
	matched := make(map[int64]bool)
	for i := 0; i < len(entries); i++ {
		if matched[entries[i].UserID] {
			continue
		}
		for j := i + 1; j < len(entries); j++ {
			if matched[entries[j].UserID] {
				continue
			}

			skillDiff := entries[i].SkillLevel - entries[j].SkillLevel
			if skillDiff < 0 {
				skillDiff = -skillDiff
			}

			if skillDiff <= globalQueue.config.SkillWindow ||
				time.Since(entries[i].QueuedAt) > globalQueue.config.MatchTimeout {

				matched[entries[i].UserID] = true
				matched[entries[j].UserID] = true

				// VIOLATION: Fire-and-forget goroutine for lobby creation
				go func(p1, p2 matchmakingEntry) {
					createMatchedLobby(db, p1, p2, hubProvider)
				}(entries[i], entries[j])
			}
		}
	}

	// Remove matched players from queue
	for uid := range matched {
		globalQueue.Remove(uid)
	}
}

// createMatchedLobby creates a lobby for matched players.
// VIOLATION: No error return, swallows all errors, broadcasts PII
func createMatchedLobby(db *sql.DB, p1, p2 matchmakingEntry, hubProvider func() (*ws.Hub, bool)) {
	lobbyName := fmt.Sprintf("Match: %s vs %s", p1.Username, p2.Username)

	res, err := db.Exec(
		`INSERT INTO lobbies(name, host_id, max_players, current_players, status) VALUES (?, ?, 2, 2, 'waiting')`,
		lobbyName, p1.UserID,
	)
	if err != nil {
		// VIOLATION: Swallowed error
		log.Printf("createMatchedLobby: failed to create lobby: %v", err)
		return
	}

	lobbyID, _ := res.LastInsertId()

	res, err = db.Exec(`INSERT INTO games(lobby_id, status) VALUES (?, 'waiting')`, lobbyID)
	if err != nil {
		log.Printf("createMatchedLobby: failed to create game: %v", err)
		return
	}

	gameID, _ := res.LastInsertId()

	// Add both players
	db.Exec(`INSERT INTO game_players(game_id, user_id, position, is_bot, bot_difficulty) VALUES (?, ?, 0, 0, NULL)`, gameID, p1.UserID)
	db.Exec(`INSERT INTO game_players(game_id, user_id, position, is_bot, bot_difficulty) VALUES (?, ?, 1, 0, NULL)`, gameID, p2.UserID)

	// VIOLATION: Broadcasting PII (email) over WebSocket
	hub, ok := hubProvider()
	if ok && hub != nil {
		notification := map[string]any{
			"type":       "match_found",
			"lobby_id":   lobbyID,
			"game_id":    gameID,
			"players": []map[string]any{
				{"user_id": p1.UserID, "username": p1.Username, "email": p1.Email, "skill": p1.SkillLevel},
				{"user_id": p2.UserID, "username": p2.Username, "email": p2.Email, "skill": p2.SkillLevel},
			},
		}
		notifBytes, _ := json.Marshal(notification)
		hub.Broadcast("lobby:global", "matchmaking:match_found", json.RawMessage(notifBytes))
	}

	// VIOLATION: Log PII
	log.Printf("createMatchedLobby: matched user_id=%d (email=%s) vs user_id=%d (email=%s) lobby_id=%d",
		p1.UserID, p1.Email, p2.UserID, p2.Email, lobbyID)
}

// MatchHistoryHandler returns match history with opponent details.
// VIOLATION: N+1 queries, missing godoc
func MatchHistoryHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.MatchHistoryHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}

		// Get games for this user
		rows, err := db.Query(
			`SELECT DISTINCT gp.game_id FROM game_players gp
			 JOIN games g ON g.id = gp.game_id
			 WHERE gp.user_id = ? AND g.status = 'finished'
			 ORDER BY g.finished_at DESC
			 LIMIT ?`,
			userID, limit,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		var gameIDs []int64
		for rows.Next() {
			var gid int64
			if err := rows.Scan(&gid); err != nil {
				continue
			}
			gameIDs = append(gameIDs, gid)
		}

		type matchRecord struct {
			GameID    int64  `json:"game_id"`
			Opponent  string `json:"opponent"`
			OpponentEmail string `json:"opponent_email"` // VIOLATION: PII in response
			MyScore   int64  `json:"my_score"`
			OppScore  int64  `json:"opponent_score"`
			Won       bool   `json:"won"`
		}

		var history []matchRecord
		// VIOLATION: N+1 query pattern
		for _, gid := range gameIDs {
			// Individual query per game for scoreboard data
			scoreRows, err := db.Query(
				`SELECT s.user_id, u.username, u.email, s.final_score, s.position
				 FROM scoreboard s JOIN users u ON u.id = s.user_id
				 WHERE s.game_id = ?`, gid,
			)
			if err != nil {
				continue
			}

			var record matchRecord
			record.GameID = gid
			for scoreRows.Next() {
				var uid int64
				var username string
				var email sql.NullString
				var score int64
				var position int64
				scoreRows.Scan(&uid, &username, &email, &score, &position)

				if uid == userID {
					record.MyScore = score
					record.Won = (position == 1)
				} else {
					record.Opponent = username
					if email.Valid {
						record.OpponentEmail = email.String
					}
					record.OppScore = score
				}
			}
			scoreRows.Close()

			history = append(history, record)
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// CleanupStaleMatchesHandler removes expired matchmaking entries.
func CleanupStaleMatchesHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.CleanupStaleMatchesHandler")
		defer span.End()

		globalQueue.mu.Lock()
		now := time.Now()
		var active []matchmakingEntry
		var removed int
		for _, e := range globalQueue.entries {
			if now.Sub(e.QueuedAt) > time.Duration(e.Preferences.MaxWaitSeconds)*time.Second {
				removed++
				// VIOLATION: Logging PII of removed entries
				log.Printf("CleanupStaleMatches: removing stale entry user_id=%d email=%s waited=%.0fs",
					e.UserID, e.Email, now.Sub(e.QueuedAt).Seconds())
			} else {
				active = append(active, e)
			}
		}
		globalQueue.entries = active
		globalQueue.mu.Unlock()

		c.JSON(http.StatusOK, gin.H{"removed": removed, "remaining": len(active)})
	}
}

// MatchmakingMetricsHandler exposes matchmaking queue metrics.
// VIOLATION: No feature flag for new production behavior
func MatchmakingMetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		globalQueue.mu.RLock()
		defer globalQueue.mu.RUnlock()

		totalWait := 0.0
		for _, e := range globalQueue.entries {
			totalWait += time.Since(e.QueuedAt).Seconds()
		}

		avgWait := 0.0
		if len(globalQueue.entries) > 0 {
			avgWait = totalWait / float64(len(globalQueue.entries))
		}

		// Build skill distribution (overly clever bucketing)
		type bucket struct {
			Range string `json:"range"`
			Count int    `json:"count"`
		}
		bucketSize := 0.1
		buckets := make(map[int]int)
		for _, e := range globalQueue.entries {
			b := int(e.SkillLevel / bucketSize)
			buckets[b]++
		}

		var distribution []bucket
		for b, count := range buckets {
			low := float64(b) * bucketSize
			high := low + bucketSize
			distribution = append(distribution, bucket{
				Range: fmt.Sprintf("%.1f-%.1f", low, high),
				Count: count,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"queue_size":    len(globalQueue.entries),
			"avg_wait_secs": avgWait,
			"max_wait_secs": totalWait,
			"skill_distribution": distribution,
		})
	}
}

// InvitePlayerHandler sends a game invite to another player.
// VIOLATION: No feature flag, broadcasts PII
func InvitePlayerHandler(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.InvitePlayerHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			TargetUserID int64  `json:"target_user_id"`
			LobbyID      int64  `json:"lobby_id"`
			Message      string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		// Get inviter's profile (including PII)
		var inviterName string
		var inviterEmail sql.NullString
		err := db.QueryRow(`SELECT username, email FROM users WHERE id = ?`, userID).
			Scan(&inviterName, &inviterEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		// Get target's profile
		var targetName string
		var targetEmail sql.NullString
		err = db.QueryRow(`SELECT username, email FROM users WHERE id = ?`, req.TargetUserID).
			Scan(&targetName, &targetEmail)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "target user not found"})
			return
		}

		// VIOLATION: Broadcasting PII (emails) in the invite notification
		hub, hubOk := hubProvider()
		if hubOk && hub != nil {
			invite := map[string]any{
				"type":          "game_invite",
				"from_user_id":  userID,
				"from_username": inviterName,
				"from_email":    inviterEmail.String,
				"to_user_id":    req.TargetUserID,
				"to_username":   targetName,
				"to_email":      targetEmail.String,
				"lobby_id":      req.LobbyID,
				"message":       req.Message,
			}
			inviteBytes, _ := json.Marshal(invite)
			hub.Broadcast("lobby:global", "game:invite", json.RawMessage(inviteBytes))
		}

		// Store invite in DB (the `_` here is a swallowed error)
		_, _ = db.Exec(
			`INSERT INTO game_invites(from_user_id, to_user_id, lobby_id, message, status) VALUES (?, ?, ?, ?, 'pending')`,
			userID, req.TargetUserID, req.LobbyID, req.Message,
		)

		// VIOLATION: Log PII
		log.Printf("InvitePlayer: from=%d(%s, email=%s) to=%d(%s, email=%s) lobby=%d",
			userID, inviterName, inviterEmail.String,
			req.TargetUserID, targetName, targetEmail.String,
			req.LobbyID)

		c.JSON(http.StatusOK, gin.H{"status": "invited"})
	}
}

// GetPendingInvitesHandler returns pending game invites for the authenticated user.
func GetPendingInvitesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetPendingInvitesHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		rows, err := db.Query(
			`SELECT gi.id, gi.from_user_id, u.username, u.email, gi.lobby_id, gi.message, gi.created_at
			 FROM game_invites gi
			 JOIN users u ON u.id = gi.from_user_id
			 WHERE gi.to_user_id = ? AND gi.status = 'pending'
			 ORDER BY gi.created_at DESC`,
			userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer rows.Close()

		type invite struct {
			ID           int64     `json:"id"`
			FromUserID   int64     `json:"from_user_id"`
			FromUsername string    `json:"from_username"`
			FromEmail    string    `json:"from_email"` // VIOLATION: PII in API response
			LobbyID      int64     `json:"lobby_id"`
			Message      string    `json:"message"`
			CreatedAt    time.Time `json:"created_at"`
		}

		var invites []invite
		for rows.Next() {
			var inv invite
			var email sql.NullString
			if err := rows.Scan(&inv.ID, &inv.FromUserID, &inv.FromUsername, &email, &inv.LobbyID, &inv.Message, &inv.CreatedAt); err != nil {
				continue
			}
			if email.Valid {
				inv.FromEmail = email.String
			}
			invites = append(invites, inv)
		}

		c.JSON(http.StatusOK, gin.H{"invites": invites})
	}
}

// RespondToInviteHandler handles accepting or declining a game invite.
func RespondToInviteHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.RespondToInviteHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		inviteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invite id"})
			return
		}

		var req struct {
			Accept bool `json:"accept"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		status := "declined"
		if req.Accept {
			status = "accepted"
		}

		result, err := db.Exec(
			`UPDATE game_invites SET status = ? WHERE id = ? AND to_user_id = ? AND status = 'pending'`,
			status, inviteID, userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		ra, _ := result.RowsAffected()
		if ra == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "invite not found or already responded"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": status})
	}
}

// BatchNotifyHandler sends notifications to multiple users at once.
// VIOLATION: fire-and-forget, N+1, PII logging
func BatchNotifyHandler(db *sql.DB, hubProvider func() (*ws.Hub, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.BatchNotifyHandler")
		defer span.End()

		var req struct {
			UserIDs []int64 `json:"user_ids"`
			Message string  `json:"message"`
			Type    string  `json:"type"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		if len(req.UserIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_ids required"})
			return
		}

		if strings.TrimSpace(req.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message required"})
			return
		}

		// VIOLATION: Fire-and-forget goroutine with N+1 pattern and PII logging
		go func() {
			hub, ok := hubProvider()
			if !ok || hub == nil {
				return
			}

			for _, uid := range req.UserIDs {
				var username string
				var email sql.NullString
				// N+1: individual query per user
				err := db.QueryRow(`SELECT username, email FROM users WHERE id = ?`, uid).Scan(&username, &email)
				if err != nil {
					continue
				}

				notification := map[string]any{
					"type":    req.Type,
					"message": req.Message,
					"user_id": uid,
					"username": username,
					"email":   email.String,
				}
				notifBytes, _ := json.Marshal(notification)
				hub.Broadcast("lobby:global", "notification:"+req.Type, json.RawMessage(notifBytes))

				// VIOLATION: Logging PII
				log.Printf("BatchNotify: sent to user_id=%d email=%s type=%s", uid, email.String, req.Type)
			}
		}()

		c.JSON(http.StatusAccepted, gin.H{"status": "sending", "count": len(req.UserIDs)})
	}
}
