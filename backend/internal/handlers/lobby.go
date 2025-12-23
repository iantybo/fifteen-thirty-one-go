package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"fifteen-thirty-one-go/backend/internal/game/common"
	"fifteen-thirty-one-go/backend/internal/game/cribbage"
	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

func syncRuntimeStateFromDB(gameID int64, nextPos int, stateVersion int64, stateJSON, handJSON string) error {
	// After commit, briefly acquire the in-memory lock to keep runtime state aligned with DB.
	// No DB operations while holding this lock.
	if strings.TrimSpace(stateJSON) == "" {
		// Explicitly log that we skipped runtime alignment (operators should see this).
		log.Printf(
			"syncRuntimeStateFromDB: state_json missing/empty; no runtime sync attempted: game_id=%d next_pos=%d state_json_len=%d hand_json_len=%d",
			gameID, nextPos, len(stateJSON), len(handJSON),
		)
		return nil
	}

	var handCards []common.Card
	handCardsOK := false
	reloadFullState := false
	var restored cribbage.State
	restoredOK := false

	var returnedErr error

	// Unmarshal hand; if it fails, fall back to full state unmarshal.
	if strings.TrimSpace(handJSON) != "" {
		if err := json.Unmarshal([]byte(handJSON), &handCards); err != nil {
			// Do not leave stale in-memory state when DB already has the joining player's hand.
			// Reload from the persisted game state snapshot instead.
			log.Printf(
				"syncRuntimeStateFromDB: hand_json unmarshal failed; attempting reload from state_json (best-effort): game_id=%d next_pos=%d err=%v hand_json_len=%d",
				gameID, nextPos, err, len(handJSON),
			)
			returnedErr = fmt.Errorf("hand_json unmarshal failed: %w", err)

			if err := json.Unmarshal([]byte(stateJSON), &restored); err != nil {
				// DB transaction already committed the join; don't return an HTTP error here.
				// Log and continue without modifying runtime state; caller must proceed.
				log.Printf(
					"syncRuntimeStateFromDB: state_json unmarshal failed during runtime reload after commit (best-effort; skipping runtime sync): game_id=%d next_pos=%d err=%v state_json_len=%d",
					gameID, nextPos, err, len(stateJSON),
				)
				// Join succeeded in DB; degrade gracefully. Next request will attempt recovery from DB snapshot.
				log.Printf("syncRuntimeStateFromDB continuing despite unmarshal failure; DB state is authoritative: game_id=%d next_pos=%d", gameID, nextPos)
				reloadFullState = false
				returnedErr = fmt.Errorf("%v; state_json unmarshal failed: %w", returnedErr, err)
			} else {
				restored.Version = stateVersion
				reloadFullState = true
				restoredOK = true
			}
		} else {
			handCardsOK = true
		}
	}

	st, unlock, ok := defaultGameManager.GetLocked(gameID)
	if ok {
		defer unlock()
		if reloadFullState {
			*st = restored
			log.Printf("syncRuntimeStateFromDB: runtime state reloaded from DB snapshot after hand decode failure: game_id=%d next_pos=%d", gameID, nextPos)
		} else if handCardsOK && nextPos >= 0 && nextPos < len(st.Hands) {
			st.Hands[nextPos] = handCards
		}
		return returnedErr
	}

	// Restore full state from DB snapshot if runtime state is missing (e.g., restart).
	if !reloadFullState {
		if err := json.Unmarshal([]byte(stateJSON), &restored); err != nil {
			// DB transaction already committed the join; don't return an HTTP error here.
			// Log and continue without installing runtime state.
			log.Printf(
				"syncRuntimeStateFromDB: state_json unmarshal failed while restoring missing runtime state after commit (best-effort; runtime state remains missing): game_id=%d next_pos=%d err=%v state_json_len=%d",
				gameID, nextPos, err, len(stateJSON),
			)
			if returnedErr == nil {
				returnedErr = fmt.Errorf("state_json unmarshal failed: %w", err)
			}
		} else {
			restored.Version = stateVersion
			restoredOK = true
		}
	}

	// Only install runtime state if we successfully decoded a snapshot.
	if restoredOK {
		defaultGameManager.Set(gameID, &restored)
	}
	return returnedErr
}

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
		// Keep handler defaults consistent with models.ListLobbies, and avoid the
		// common "LIMIT 0 returns empty set" pitfall.
		limit := int64(50)
		offset := int64(0)
		if v := strings.TrimSpace(c.Query("limit")); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			if n < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			limit = n
		}
		if v := strings.TrimSpace(c.Query("offset")); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
				return
			}
			if n < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
				return
			}
			offset = n
		}
		lobbies, err := models.ListLobbies(db, limit, offset)
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
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			req.Name = "Lobby"
		}
		if len(req.Name) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name must be <= 100 characters"})
			return
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

		// Initialize in-memory engine state BEFORE commit so we don't create DB rows
		// without a corresponding in-memory state if dealing fails.
		st := cribbage.NewState(req.MaxPlayers)
		if err := st.Deal(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "game init error"})
			return
		}
		// Persist the host's initial hand for UI convenience (idempotent; should be empty here).
		if b, err := json.Marshal(st.Hands[0]); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		} else {
			if _, err := models.UpdatePlayerHandIfEmptyTx(tx, gameID, hostID, string(b)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}
		}
		// Persist full engine state for restart recovery.
		if sb, err := json.Marshal(st); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		} else {
			if err := models.UpdateGameStateTx(tx, gameID, string(sb)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}
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

		// Fresh game: UpdateGameStateTx has incremented from 0 -> 1.
		st.Version = 1
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

		// Transaction: increment lobby count + add game player together or not at all.
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer tx.Rollback()

		l, err := models.JoinLobbyTx(tx, lobbyID)
		if err != nil {
			// Don't leak internal details; map known messages to safe ones.
			msg := "unable to join lobby"
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "lobby not found"})
				return
			}
			if errors.Is(err, models.ErrLobbyFull) {
				msg = "lobby full"
			} else if errors.Is(err, models.ErrLobbyNotJoinable) {
				msg = "lobby not joinable"
			}
			log.Printf("JoinLobbyTx failed: lobby_id=%d user_id=%d err=%v", lobbyID, userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		// Find game for lobby (assumes one game per lobby for now).
		// This is a small shortcut until we add explicit lobby membership and game start.
		var gameID int64
		if err := tx.QueryRow(`SELECT id FROM games WHERE lobby_id = ? ORDER BY id DESC LIMIT 1`, lobbyID).Scan(&gameID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		nextPos, err := models.AddGamePlayerAutoPositionTx(tx, gameID, userID, l.MaxPlayers, false, nil)
		if err != nil {
			log.Printf("AddGamePlayerAutoPositionTx failed: game_id=%d user_id=%d err=%v", gameID, userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to join game"})
			return
		}

		// Persist joining player's initial hand WITHOUT taking the in-memory state lock.
		// Use the persisted engine state in DB (if present) to keep lock ordering DB -> memory.
		var handJSON string
		var stateJSON string
		var stateVersion int64
		var s sql.NullString
		var v sql.NullInt64
		if err := tx.QueryRow(`SELECT state_json, state_version FROM games WHERE id = ?`, gameID).Scan(&s, &v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if v.Valid {
			stateVersion = v.Int64
		}
		if s.Valid && strings.TrimSpace(s.String) != "" {
			stateJSON = s.String

			var restored cribbage.State
			if err := json.Unmarshal([]byte(stateJSON), &restored); err != nil {
				log.Printf("JoinLobbyHandler restore state_json unmarshal failed: game_id=%d err=%v state_json_len=%d", gameID, err, len(stateJSON))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}
			restored.Version = stateVersion
			if int(nextPos) >= 0 && int(nextPos) < len(restored.Hands) {
				if b, err := json.Marshal(restored.Hands[nextPos]); err == nil {
					handJSON = string(b)
					if _, err := models.UpdatePlayerHandIfEmptyTx(tx, gameID, userID, handJSON); err != nil {
						log.Printf("UpdatePlayerHandIfEmptyTx failed: game_id=%d user_id=%d err=%v", gameID, userID, err)
						c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
						return
					}
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
					return
				}
			} else {
				// This indicates a mismatch between the persisted engine state and the assigned position.
				log.Printf(
					"JoinLobbyHandler: position out of bounds while persisting player hand: game_id=%d user_id=%d next_pos=%d hands_len=%d",
					gameID, userID, nextPos, len(restored.Hands),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "position out of bounds"})
				return
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		resp := gin.H{"lobby": l, "game_id": gameID, "joined_persisted": true, "realtime_sync": "ok"}
		if err := syncRuntimeStateFromDB(gameID, int(nextPos), stateVersion, stateJSON, handJSON); err != nil {
			log.Printf(
				"JoinLobbyHandler: runtime state sync encountered errors after commit (best-effort; continuing): game_id=%d user_id=%d next_pos=%d err=%v",
				gameID, userID, nextPos, err,
			)
			resp["realtime_sync"] = "failed"
		}

		c.JSON(http.StatusOK, resp)
	}
}



