package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

func GameEventsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GameEventsHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			log.Printf("GameEventsHandler IsUserInGame failed: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		limit := int64(100)
		if s := c.Query("limit"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
				limit = v
			}
		}

		events, err := models.ListGameEvents(db, gameID, limit)
		if err != nil {
			log.Printf("GameEventsHandler ListGameEvents failed: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"events": events})
	}
}

func GameSnapshotHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GameSnapshotHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		snap, err := models.CaptureGameSnapshot(db, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "snapshot unavailable"})
			return
		}
		c.JSON(http.StatusOK, snap)
	}
}

func GameMoveStatsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GameMoveStatsHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		stats, err := models.ComputeGameMoveStats(db, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "stats unavailable"})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

func LobbyPlayersHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.LobbyPlayersHandler")
		defer span.End()

		lobbyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || lobbyID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lobby id"})
			return
		}

		players, err := models.GetLobbyPlayersWithStats(db, lobbyID)
		if err != nil {
			log.Printf("LobbyPlayersHandler GetLobbyPlayersWithStats failed: lobby_id=%d err=%v", lobbyID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"players": players})
	}
}

func RematchHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.RematchHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
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

		if err := models.ResetGameForRematch(c.Request.Context(), db, gameID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		defaultGameManager.Delete(gameID)

		go broadcastGameUpdate(db, gameID)

		c.JSON(http.StatusOK, gin.H{"ok": true, "game_id": gameID})
	}
}
