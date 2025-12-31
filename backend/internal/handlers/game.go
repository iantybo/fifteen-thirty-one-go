package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"
	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type moveRequest struct {
	Type string `json:"type"` // discard|play_card|go

	// discard: cards
	// play_card: card
	Cards []string `json:"cards,omitempty"`
	Card  string   `json:"card,omitempty"`
}

func GetGameHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		snap, err := BuildGameSnapshotForUser(db, gameID, userID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
				return
			}
			if errors.Is(err, models.ErrGameStateMissing) {
				c.JSON(http.StatusConflict, gin.H{"error": "game state unavailable; recreate lobby"})
				return
			}
			log.Printf("GetGameHandler BuildGameSnapshotForUser failed: game_id=%d user_id=%d err=%v", gameID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.JSON(http.StatusOK, snap)
	}
}

func MoveHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req moveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		resp, err := ApplyMove(db, gameID, userID, req)
		if err != nil {
			writeAPIError(c, err)
			return
		}
		// Single-player support: if bots are present, let them respond immediately so
		// the client can fetch a post-bot snapshot right away.
		if err := maybeRunBotTurns(db, gameID); err != nil {
			log.Printf("maybeRunBotTurns failed: game_id=%d err=%v", gameID, err)
		}
		// Realtime: notify all connected clients that the game changed.
		broadcastGameUpdate(db, gameID)
		c.JSON(http.StatusOK, resp)
	}
}

type countRequest struct {
	Kind   string `json:"kind"` // hand|crib
	Claim  int64  `json:"claim"`
	Final  bool   `json:"final"`
}

func CountHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req countRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		st, unlock, ok := defaultGameManager.GetLocked(gameID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "game not ready"})
			return
		}
		if st.Cut == nil {
			unlock()
			c.JSON(http.StatusBadRequest, gin.H{"error": "game not ready"})
			return
		}
		if st.Stage != "counting" {
			unlock()
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stage for counting"})
			return
		}
		// Copy the minimal read-only fields we need, then release the lock before DB work.
		cut := *st.Cut
		dealerIndex := st.DealerIndex
		keptHands := make([][]common.Card, len(st.KeptHands))
		for i := range st.KeptHands {
			keptHands[i] = append([]common.Card(nil), st.KeptHands[i]...)
		}
		crib := append([]common.Card(nil), st.Crib...)
		unlock()

		players, err := models.ListGamePlayersByGame(db, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		var pos int64 = -1
		for _, p := range players {
			if p.UserID == userID {
				pos = p.Position
				break
			}
		}
		if pos < 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
			return
		}

		var verified int64
		var breakdown any
		switch req.Kind {
		case "hand":
			posIdx := int(pos)
			if posIdx < 0 || posIdx >= len(keptHands) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player position"})
				return
			}
			bd := cribbage.ScoreHand(keptHands[posIdx], cut, false)
			verified = int64(bd.Total)
			breakdown = bd
		case "crib":
			if int(pos) != dealerIndex {
				c.JSON(http.StatusForbidden, gin.H{"error": "only dealer counts crib"})
				return
			}
			bd := cribbage.ScoreHand(crib, cut, true)
			verified = int64(bd.Total)
			breakdown = bd
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kind"})
			return
		}

		claim := req.Claim
		mt := "count_" + req.Kind
		if req.Final {
			mt = mt + "_final"
		}
		if req.Final {
			// Prevent duplicate final submissions for the same player/game/type.
			// Corrections should use the correction flow which marks the original as corrected.
			exists, err := models.HasUncorrectedMoveType(db, gameID, userID, mt)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}
			if exists {
				c.JSON(http.StatusConflict, gin.H{"error": "final count already submitted"})
				return
			}
		}
		if _, err := models.InsertMove(db, models.GameMove{
			GameID:        gameID,
			PlayerID:      userID,
			MoveType:      mt,
			ScoreClaimed:  &claim,
			ScoreVerified: &verified,
			IsCorrected:   false,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		prefs, err := models.GetUserPreferences(db, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"verified": verified, "breakdown": breakdown, "auto_count_mode": prefs.AutoCountMode})
	}
}

func QuitGameHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
			return
		}

		g, err := models.GetGameByID(db, gameID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		// Best-effort: mark game and lobby finished. This gives the UI a clean terminal state.
		_ = models.SetGameStatus(db, gameID, "finished")
		_ = models.SetLobbyStatus(db, g.LobbyID, "finished")

		// Drop in-memory runtime state so a future game doesn't accidentally reuse it.
		defaultGameManager.Delete(gameID)

		broadcastGameUpdate(db, gameID)
		c.Status(http.StatusNoContent)
	}
}

func NextHandHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
			return
		}

		players, err := models.ListGamePlayersByGame(db, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		st, unlock, err := ensureGameStateLocked(db, gameID, players)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "game not ready"})
			return
		}
		if st.Stage != "counting" {
			unlock()
			c.JSON(http.StatusConflict, gin.H{"error": "not in counting stage"})
			return
		}

		// Ready-up gate: during counting, both players must be ready before dealing the next hand.
		// (Bots are auto-ready.)
		myPos := -1
		for _, p := range players {
			if p.UserID == userID {
				myPos = int(p.Position)
				break
			}
		}
		if myPos < 0 || myPos >= st.Rules.MaxPlayers {
			unlock()
			c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
			return
		}
		if st.ReadyNextHand == nil || len(st.ReadyNextHand) != st.Rules.MaxPlayers {
			st.ReadyNextHand = make([]bool, st.Rules.MaxPlayers)
		}
		// Toggle readiness so users can un-ready if clicked accidentally.
		st.ReadyNextHand[myPos] = !st.ReadyNextHand[myPos]
		for _, p := range players {
			if p.IsBot {
				pos := int(p.Position)
				if pos >= 0 && pos < st.Rules.MaxPlayers {
					st.ReadyNextHand[pos] = true
				}
			}
		}
		allReady := true
		for _, p := range players {
			if p.IsBot {
				continue
			}
			pos := int(p.Position)
			if pos < 0 || pos >= st.Rules.MaxPlayers || !st.ReadyNextHand[pos] {
				allReady = false
				break
			}
		}

		baseVersion := st.Version
		working := cloneStateDeep(st)
		working.Version = baseVersion
		unlock()

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if allReady {
			// Advance dealer and deal next hand.
			working.DealerIndex = (working.DealerIndex + 1) % working.Rules.MaxPlayers
			if err := working.Deal(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "deal failed"})
				return
			}
			// Persist all newly dealt hands.
			for _, p := range players {
				posIdx := int(p.Position)
				if posIdx < 0 || posIdx >= len(working.Hands) {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid player position"})
					return
				}
				b, err := json.Marshal(working.Hands[posIdx])
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
					return
				}
				if err := models.UpdatePlayerHandTx(tx, gameID, p.UserID, string(b)); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
					return
				}
			}
		}
		sb, err := json.Marshal(working)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if err := models.UpdateGameStateTxCAS(tx, gameID, baseVersion, string(sb)); err != nil {
			writeAPIError(c, err)
			return
		}
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		committed = true

		// Re-apply to runtime state.
		working.Version = baseVersion + 1
		st2, unlock2, ok := defaultGameManager.GetLocked(gameID)
		if ok && st2 != nil {
			*st2 = working
			unlock2()
		}

		broadcastGameUpdate(db, gameID)
		c.Status(http.StatusNoContent)
	}
}

type correctRequest struct {
	MoveID int64 `json:"move_id"`
	NewClaim int64 `json:"new_claim"`
}

func CorrectHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req correctRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		// Minimal correction: append a correction move referencing the prior one.
		prev, err := models.GetMoveByID(db, req.MoveID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid move"})
				return
			}
			log.Printf("GetMoveByID failed: move_id=%d err=%v", req.MoveID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if prev.GameID != gameID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid move"})
			return
		}
		isHost := false
		var hostID int64
		if err := db.QueryRow(`SELECT l.host_id FROM games g JOIN lobbies l ON l.id = g.lobby_id WHERE g.id = ?`, gameID).Scan(&hostID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if hostID == userID {
			isHost = true
		}
		if prev.PlayerID != userID && !isHost {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot correct someone else's move"})
			return
		}
		// Reject attempts to correct a move that has already been corrected.
		// Note: run this after permission checks to avoid leaking state to unauthorized users.
		if prev.IsCorrected {
			c.JSON(http.StatusConflict, gin.H{"error": "move already corrected"})
			return
		}
		if strings.HasSuffix(prev.MoveType, "_final") && !isHost {
			c.JSON(http.StatusForbidden, gin.H{"error": "finalized counts require host correction"})
			return
		}
		if prev.ScoreVerified == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "move not correctable"})
			return
		}

		// Mark original move as corrected before inserting the correction (atomic via tx).
		tx, err := db.Begin()
		if err != nil {
			log.Printf("CorrectHandler begin tx failed: move_id=%d err=%v", req.MoveID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer tx.Rollback()

		if err := models.MarkMoveAsCorrectedTx(tx, req.MoveID); err != nil {
			log.Printf("MarkMoveAsCorrected failed: move_id=%d err=%v", req.MoveID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		verified := *prev.ScoreVerified
		newClaim := req.NewClaim
		if err := models.InsertMoveTx(tx, models.GameMove{
			GameID:        gameID,
			PlayerID:      userID,
			MoveType:      prev.MoveType + "_correct",
			ScoreClaimed:  &newClaim,
			ScoreVerified: &verified,
			IsCorrected:   false,
		}); err != nil {
			log.Printf("InsertMoveTx (correction) failed: game_id=%d move_id=%d err=%v", gameID, req.MoveID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if err := tx.Commit(); err != nil {
			log.Printf("CorrectHandler commit failed: move_id=%d err=%v", req.MoveID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"verified": verified})
	}
}

