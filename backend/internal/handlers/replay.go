package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/game/cribbage"
	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

type replayGameInfo struct {
	ID         int64  `json:"id"`
	LobbyID    int64  `json:"lobby_id"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type replayPlayer struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Position int64  `json:"position"`
	IsBot    bool   `json:"is_bot"`
}

type replayResponse struct {
	Game    replayGameInfo           `json:"game"`
	Players []replayPlayer           `json:"players"`
	Rounds  []cribbage.RoundSummary  `json:"rounds"`
	Moves   []models.GameMove        `json:"moves"`
}

// ReplayHandler returns the full replay data for a finished game.
// Any authenticated user can view a finished game replay (cards are non-secret post-game).
func ReplayHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ReplayHandler")
		defer span.End()

		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		game, err := models.GetGameByID(db, gameID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
				return
			}
			log.Printf("ReplayHandler GetGameByID failed: game_id=%d err=%v", gameID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if game.Status != "finished" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "game is not finished"})
			return
		}

		// Fetch state_json and extract round history.
		stateJSON, _, ok, err := models.GetGameStateJSON(db, gameID)
		if err != nil {
			log.Printf("ReplayHandler GetGameStateJSON failed: game_id=%d err=%v", gameID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		var rounds []cribbage.RoundSummary
		if ok && stateJSON != "" {
			var st cribbage.State
			if err := json.Unmarshal([]byte(stateJSON), &st); err != nil {
				log.Printf("ReplayHandler unmarshal state failed: game_id=%d err=%v", gameID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			rounds = st.History
		}
		if rounds == nil {
			rounds = []cribbage.RoundSummary{}
		}

		// Fetch players.
		gamePlayers, err := models.ListGamePlayersByGameContext(c.Request.Context(), db, gameID)
		if err != nil {
			log.Printf("ReplayHandler ListGamePlayersByGameContext failed: game_id=%d err=%v", gameID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		players := make([]replayPlayer, len(gamePlayers))
		for i, gp := range gamePlayers {
			players[i] = replayPlayer{
				UserID:   gp.UserID,
				Username: gp.Username,
				Position: gp.Position,
				IsBot:    gp.IsBot,
			}
		}

		// Fetch all moves in chronological order.
		moves, err := models.ListAllMovesByGameAsc(db, gameID)
		if err != nil {
			log.Printf("ReplayHandler ListAllMovesByGameAsc failed: game_id=%d err=%v", gameID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if moves == nil {
			moves = []models.GameMove{}
		}

		info := replayGameInfo{
			ID:        game.ID,
			LobbyID:   game.LobbyID,
			CreatedAt: game.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if game.FinishedAt != nil {
			info.FinishedAt = game.FinishedAt.Format("2006-01-02T15:04:05Z")
		}

		c.JSON(http.StatusOK, replayResponse{
			Game:    info,
			Players: players,
			Rounds:  rounds,
			Moves:   moves,
		})
	}
}
