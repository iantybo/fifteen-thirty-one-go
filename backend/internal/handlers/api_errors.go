package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

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

	// Safe string-mapped validation / permission / conflict errors (do NOT echo raw errors).
	switch {
	case strings.Contains(err.Error(), "invalid json"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	case strings.Contains(err.Error(), "invalid card"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card"})
		return
	case strings.Contains(err.Error(), "not a player"):
		c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
		return
	case strings.Contains(err.Error(), "not your turn"):
		c.JSON(http.StatusConflict, gin.H{"error": "not your turn"})
		return
	case strings.Contains(err.Error(), "not in pegging stage"):
		c.JSON(http.StatusConflict, gin.H{"error": "not in pegging stage"})
		return
	case strings.Contains(err.Error(), "would exceed 31"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "move would exceed 31"})
		return
	case strings.Contains(err.Error(), "card not in hand"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "card not in hand"})
		return
	case strings.Contains(err.Error(), "not in discard stage"):
		c.JSON(http.StatusConflict, gin.H{"error": "not in discard stage"})
		return
	case strings.Contains(err.Error(), "discard card not in hand"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "discard card not in hand"})
		return
	}

	// Unknown/internal errors: log details, return generic message.
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}


