package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/game/cribbage"
	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type createLobbyRequest struct {
	Name       string `json:"name"`
	MaxPlayers int    `json:"max_players"`
}

type createLobbyResponse struct {
	Lobby *models.Lobby `json:"lobby"`
	Game  *models.Game  `json:"game"`
}

func ListLobbiesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lobbies, err := models.ListLobbies(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"lobbies": lobbies})
	}
}

func CreateLobbyHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createLobbyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		if req.MaxPlayers < 2 || req.MaxPlayers > 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_players must be 2-4"})
			return
		}
		if req.Name == "" {
			req.Name = "Lobby"
		}

		hostID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		l, err := models.CreateLobby(db, req.Name, hostID, int64(req.MaxPlayers))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		g, err := models.CreateGame(db, l.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if err := models.AddGamePlayer(db, g.ID, hostID, 0, false, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		// Initialize in-memory engine state.
		st := cribbage.NewState(req.MaxPlayers)
		if err := st.Deal(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "game init error"})
			return
		}
		// Persist the host's initial hand for UI convenience.
		if b, err := json.Marshal(st.Hands[0]); err == nil {
			_ = models.UpdatePlayerHand(db, g.ID, hostID, string(b))
		}
		defaultGameManager.Set(g.ID, st)

		c.JSON(http.StatusCreated, createLobbyResponse{Lobby: l, Game: g})
	}
}

func JoinLobbyHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lobbyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lobby id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		l, err := models.JoinLobby(db, lobbyID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Find game for lobby (assumes one game per lobby for now).
		// This is a small shortcut until we add explicit lobby membership and game start.
		var gameID int64
		if err := db.QueryRow(`SELECT id FROM games WHERE lobby_id = ? ORDER BY id DESC LIMIT 1`, lobbyID).Scan(&gameID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		st, ok := defaultGameManager.Get(gameID)
		if !ok {
			// Recreate minimal state if missing.
			st = cribbage.NewState(int(l.MaxPlayers))
			if err := st.Deal(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "game init error"})
				return
			}
			defaultGameManager.Set(gameID, st)
		}

		nextPos, err := models.AddGamePlayerAutoPosition(db, gameID, userID, false, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Persist joining player's hand (best-effort).
		if int(nextPos) < len(st.Hands) {
			if b, err := json.Marshal(st.Hands[nextPos]); err == nil {
				_ = models.UpdatePlayerHand(db, gameID, userID, string(b))
			}
		}

		c.JSON(http.StatusOK, gin.H{"lobby": l, "game_id": gameID})
	}
}


