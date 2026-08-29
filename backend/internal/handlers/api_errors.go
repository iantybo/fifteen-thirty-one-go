package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"fifteen-thirty-one-go/backend/internal/models"

	"github.com/gin-gonic/gin"
)

// apiErrorMapping maps a sentinel error to an HTTP status code and safe
// client-facing message. Order does not matter; lookup is linear.
type apiErrorMapping struct {
	sentinel error
	status   int
	message  string
}

var apiErrorMappings = []apiErrorMapping{
	// Not-found family
	{models.ErrNotFound, http.StatusNotFound, "not found"},
	{models.ErrGameNotFound, http.StatusNotFound, "not found"},
	{sql.ErrNoRows, http.StatusNotFound, "not found"},

	// Bad-request (validation)
	{models.ErrInvalidJSON, http.StatusBadRequest, "invalid json"},
	{models.ErrInvalidCard, http.StatusBadRequest, "invalid card"},
	{models.ErrWouldExceed31, http.StatusBadRequest, "move would exceed 31"},
	{models.ErrCardNotInHand, http.StatusBadRequest, "card not in hand"},
	{models.ErrDiscardCardNotInHand, http.StatusBadRequest, "discard card not in hand"},
	{models.ErrInvalidDiscardCount, http.StatusBadRequest, "invalid discard count"},
	{models.ErrInvalidPlayer, http.StatusBadRequest, "invalid player"},
	{models.ErrInvalidPlayerPosition, http.StatusBadRequest, "invalid player position"},
	{models.ErrUnknownMoveType, http.StatusBadRequest, "unknown move type"},

	// Forbidden (permissions)
	{models.ErrNotAPlayer, http.StatusForbidden, "not a player"},
	{models.ErrLobbyNotJoinable, http.StatusForbidden, "lobby not joinable"},
	{models.ErrPlayerNotInGame, http.StatusForbidden, "player not in game"},

	// Conflict (state)
	{models.ErrNotYourTurn, http.StatusConflict, "not your turn"},
	{models.ErrNotInPeggingStage, http.StatusConflict, "not in pegging stage"},
	{models.ErrNotInDiscardStage, http.StatusConflict, "not in discard stage"},
	{models.ErrDiscardAlreadyCompleted, http.StatusConflict, "discard already completed"},
	{models.ErrHasLegalPlay, http.StatusConflict, "you have a legal play"},
	{models.ErrGameStateMissing, http.StatusConflict, "game state unavailable; recreate lobby"},
	{models.ErrGameStateConflict, http.StatusConflict, "game state changed; retry"},
	{models.ErrLobbyFull, http.StatusConflict, "lobby full"},
}

func writeAPIError(c *gin.Context, err error) {
	if err == nil {
		log.Printf("BUG: writeAPIError called with nil error")
		panic("writeAPIError called with nil error")
	}

	for _, m := range apiErrorMappings {
		if errors.Is(err, m.sentinel) {
			c.AbortWithStatusJSON(m.status, gin.H{"error": m.message})
			return
		}
	}

	// Unknown/internal errors: log details, return generic message.
	log.Printf("internal error: %v", err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
