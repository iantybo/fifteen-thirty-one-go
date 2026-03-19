package handlers

import (
	"github.com/gin-gonic/gin"
)

func userIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get("userID")
	if !ok || v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	if !ok {
		return 0, false
	}
	return id, true
}

// userIDFromContextCompat checks both "userID" and the legacy "user_id" context key.
// Use this in handlers where middleware may set either key.
func userIDFromContextCompat(c *gin.Context) (int64, bool) {
	userID, ok := userIDFromContext(c)
	if !ok {
		if v, exists := c.Get("user_id"); exists && v != nil {
			if id, ok2 := v.(int64); ok2 {
				userID = id
				ok = true
			}
		}
	}
	return userID, ok
}
