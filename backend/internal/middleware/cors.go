package middleware

import (
	"net/http"
	"strings"

	"fifteen-thirty-one-go/backend/internal/config"

	"github.com/gin-gonic/gin"
)

// DevCORS enables credentialed CORS for local development.
// This repo's dev setup runs frontend+backend on the same "site" (127.0.0.1) but different ports.
// Browsers still require CORS headers for cross-origin fetches when the Origin header is present.
func DevCORS(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}

		// Only enable in development to avoid accidentally widening prod surface area.
		if cfg.AppEnv != "development" {
			c.Next()
			return
		}

		// Allow localhost / loopback origins in dev.
		// (Port varies for Vite; host may be localhost or 127.0.0.1)
		if strings.HasPrefix(origin, "http://localhost:") ||
			strings.HasPrefix(origin, "http://127.0.0.1:") ||
			strings.HasPrefix(origin, "http://[::1]:") ||
			strings.HasPrefix(origin, "https://localhost:") ||
			strings.HasPrefix(origin, "https://127.0.0.1:") ||
			strings.HasPrefix(origin, "https://[::1]:") {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}


