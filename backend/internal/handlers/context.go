package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// userIDFromContext extracts the authenticated user ID from the Gin context.
// It checks both "userID" (preferred) and "user_id" (legacy middleware compat).
func userIDFromContext(c *gin.Context) (int64, bool) {
	for _, key := range []string{"userID", "user_id"} {
		v, ok := c.Get(key)
		if !ok || v == nil {
			continue
		}
		if id, ok := v.(int64); ok && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// requireUserID extracts the authenticated user ID or writes a 401 response.
// Returns 0 and false if the response was already written.
func requireUserID(c *gin.Context) (int64, bool) {
	id, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}
	return id, true
}

// parseIDParam parses a route parameter as a positive int64. On failure it
// writes a 400 response with the given label (e.g. "game", "lobby") and
// returns 0, false.
func parseIDParam(c *gin.Context, param, label string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(param), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + label + " id"})
		return 0, false
	}
	return id, true
}
