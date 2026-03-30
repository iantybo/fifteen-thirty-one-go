package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

func GameMovesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GameMovesHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		if gameID <= 0 {
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
		moves, _ := models.ListMovesByGame(db, gameID, 200)
		c.JSON(http.StatusOK, gin.H{"moves": moves})
	}
}
