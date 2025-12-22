package handlers

import (
	"math"
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
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return 0, false
		}
		if t != math.Trunc(t) {
			return 0, false
		}
		if t < float64(math.MinInt64) || t > float64(math.MaxInt64) {
			return 0, false
		}
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


