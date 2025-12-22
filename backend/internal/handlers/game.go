package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
		defer unlock()
		if st.Cut == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "game not ready"})
			return
		}

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
			if posIdx < 0 || posIdx >= len(st.KeptHands) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player position"})
				return
			}
			bd := cribbage.ScoreHand(st.KeptHands[posIdx], *st.Cut, false)
			verified = int64(bd.Total)
			breakdown = bd
		case "crib":
			if int(pos) != st.DealerIndex {
				c.JSON(http.StatusForbidden, gin.H{"error": "only dealer counts crib"})
				return
			}
			bd := cribbage.ScoreHand(st.Crib, *st.Cut, true)
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
		if err != nil || prev.GameID != gameID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid move"})
			return
		}
		isHost := false
		var hostID int64
		if err := db.QueryRow(`SELECT l.host_id FROM games g JOIN lobbies l ON l.id = g.lobby_id WHERE g.id = ?`, gameID).Scan(&hostID); err != nil {
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
		if strings.HasSuffix(prev.MoveType, "_final") && !isHost {
			c.JSON(http.StatusForbidden, gin.H{"error": "finalized counts require host correction"})
			return
		}
		if prev.ScoreVerified == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "move not correctable"})
			return
		}

		verified := *prev.ScoreVerified
		newClaim := req.NewClaim
		if _, err := models.InsertMove(db, models.GameMove{
			GameID:        gameID,
			PlayerID:      userID,
			MoveType:      prev.MoveType + "_correct",
			ScoreClaimed:  &newClaim,
			ScoreVerified: &verified,
			IsCorrected:   true,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"verified": verified})
	}
}

