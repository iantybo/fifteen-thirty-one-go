package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func userIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get("userID")
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case float64:
		// defensive: some decoders store numbers as float64
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}


