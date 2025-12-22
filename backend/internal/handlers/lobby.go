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

		// Transaction: avoid orphaned lobby/game records on partial failure.
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer tx.Rollback()

		res, err := tx.Exec(
			`INSERT INTO lobbies(name, host_id, max_players, current_players, status) VALUES (?, ?, ?, 1, 'waiting')`,
			req.Name, hostID, int64(req.MaxPlayers),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		lobbyID, err := res.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		res, err = tx.Exec(`INSERT INTO games(lobby_id, status) VALUES (?, 'waiting')`, lobbyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		gameID, err := res.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if _, err := tx.Exec(
			`INSERT INTO game_players(game_id, user_id, position, is_bot, bot_difficulty) VALUES (?, ?, 0, 0, NULL)`,
			gameID, hostID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		l, err := models.GetLobbyByID(db, lobbyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		g, err := models.GetGameByID(db, gameID)
		if err != nil {
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
			if err := models.UpdatePlayerHand(db, g.ID, hostID, string(b)); err != nil {
				log.Printf("UpdatePlayerHand failed: game_id=%d user_id=%d err=%v", g.ID, hostID, err)
			}
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
			// Don't leak internal details; map known messages to safe ones.
			msg := "unable to join lobby"
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "lobby not found"})
				return
			}
			if err.Error() == "lobby full" {
				msg = "lobby full"
			} else if err.Error() == "lobby not joinable" {
				msg = "lobby not joinable"
			}
			log.Printf("JoinLobby failed: lobby_id=%d user_id=%d err=%v", lobbyID, userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		// Find game for lobby (assumes one game per lobby for now).
		// This is a small shortcut until we add explicit lobby membership and game start.
		var gameID int64
		if err := db.QueryRow(`SELECT id FROM games WHERE lobby_id = ? ORDER BY id DESC LIMIT 1`, lobbyID).Scan(&gameID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		st, unlock, ok := defaultGameManager.GetLocked(gameID)
		if !ok {
			// Avoid re-dealing a different game state (would desync hands).
			c.JSON(http.StatusConflict, gin.H{"error": "game state unavailable; recreate lobby"})
			return
		}
		defer unlock()

		nextPos, err := models.AddGamePlayerAutoPosition(db, gameID, userID, false, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Persist joining player's hand (best-effort).
		if int(nextPos) < len(st.Hands) {
			if b, err := json.Marshal(st.Hands[nextPos]); err == nil {
				if err := models.UpdatePlayerHand(db, gameID, userID, string(b)); err != nil {
					log.Printf("UpdatePlayerHand failed: game_id=%d user_id=%d err=%v", gameID, userID, err)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"lobby": l, "game_id": gameID})
	}
}


