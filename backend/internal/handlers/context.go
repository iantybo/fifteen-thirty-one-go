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


