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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Known sentinel errors
	if errors.Is(err, models.ErrNotFound) || errors.Is(err, models.ErrGameNotFound) || errors.Is(err, sql.ErrNoRows) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// Safe typed validation / permission / conflict errors (do NOT echo raw errors).
	switch {
	case errors.Is(err, models.ErrInvalidJSON):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	case errors.Is(err, models.ErrInvalidCard):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid card"})
		return
	case errors.Is(err, models.ErrNotAPlayer):
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a player"})
		return
	case errors.Is(err, models.ErrNotYourTurn):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "not your turn"})
		return
	case errors.Is(err, models.ErrNotInPeggingStage):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "not in pegging stage"})
		return
	case errors.Is(err, models.ErrWouldExceed31):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "move would exceed 31"})
		return
	case errors.Is(err, models.ErrCardNotInHand):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "card not in hand"})
		return
	case errors.Is(err, models.ErrNotInDiscardStage):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "not in discard stage"})
		return
	case errors.Is(err, models.ErrDiscardCardNotInHand):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "discard card not in hand"})
		return
	case errors.Is(err, models.ErrDiscardAlreadyCompleted):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "discard already completed"})
		return
	case errors.Is(err, models.ErrInvalidDiscardCount):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid discard count"})
		return
	case errors.Is(err, models.ErrInvalidPlayer):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid player"})
		return
	case errors.Is(err, models.ErrInvalidPlayerPosition):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid player position"})
		return
	case errors.Is(err, models.ErrUnknownMoveType):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "unknown move type"})
		return
	case errors.Is(err, models.ErrHasLegalPlay):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "you have a legal play"})
		return
	case errors.Is(err, models.ErrGameStateMissing):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "game state unavailable; recreate lobby"})
		return
	}

	// Unknown/internal errors: log details, return generic message.
	log.Printf("internal error: %v", err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}


