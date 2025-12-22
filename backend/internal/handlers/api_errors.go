package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

func writeAPIError(c *gin.Context, err error) {
	if err == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Known sentinel errors
	if errors.Is(err, models.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// Safe typed validation / permission / conflict errors (do NOT echo raw errors).
	switch {
	case errors.Is(err, models.ErrInvalidJSON):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	case errors.Is(err, models.ErrInvalidCard):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card"})
		return
	case errors.Is(err, models.ErrNotAPlayer):
		c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
		return
	case errors.Is(err, models.ErrNotYourTurn):
		c.JSON(http.StatusConflict, gin.H{"error": "not your turn"})
		return
	case errors.Is(err, models.ErrNotInPeggingStage):
		c.JSON(http.StatusConflict, gin.H{"error": "not in pegging stage"})
		return
	case errors.Is(err, models.ErrWouldExceed31):
		c.JSON(http.StatusBadRequest, gin.H{"error": "move would exceed 31"})
		return
	case errors.Is(err, models.ErrCardNotInHand):
		c.JSON(http.StatusBadRequest, gin.H{"error": "card not in hand"})
		return
	case errors.Is(err, models.ErrNotInDiscardStage):
		c.JSON(http.StatusConflict, gin.H{"error": "not in discard stage"})
		return
	case errors.Is(err, models.ErrDiscardCardNotInHand):
		c.JSON(http.StatusBadRequest, gin.H{"error": "discard card not in hand"})
		return
	case errors.Is(err, models.ErrDiscardAlreadyCompleted):
		c.JSON(http.StatusConflict, gin.H{"error": "discard already completed"})
		return
	case errors.Is(err, models.ErrInvalidDiscardCount):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid discard count"})
		return
	case errors.Is(err, models.ErrInvalidPlayer):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player"})
		return
	case errors.Is(err, models.ErrInvalidPlayerPosition):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player position"})
		return
	case errors.Is(err, models.ErrUnknownMoveType):
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown move type"})
		return
	case errors.Is(err, models.ErrHasLegalPlay):
		c.JSON(http.StatusConflict, gin.H{"error": "you have a legal play"})
		return
	case errors.Is(err, models.ErrGameStateMissing):
		c.JSON(http.StatusConflict, gin.H{"error": "game state unavailable; recreate lobby"})
		return
	}

	// Unknown/internal errors: log details, return generic message.
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}


