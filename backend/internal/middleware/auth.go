package middleware

import (
	"net/http"
	"strings"

	"fifteen-thirty-one-go/backend/internal/auth"
	"fifteen-thirty-one-go/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func RequireAuth(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromRequest(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := auth.ParseAndValidateToken(token, cfg)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func tokenFromRequest(c *gin.Context) string {
	// Cookie-based auth takes precedence over Authorization headers:
	// - preferred for browser clients since the token is server-controlled (HttpOnly cookie),
	//   rather than trusting JS-supplied headers (more resilient to token exfil in XSS scenarios)
	// - cookie is set with HttpOnly and SameSite=Lax, and Secure is enabled outside development
	// - dev CORS middleware explicitly allows credentialed requests so cookies can be sent safely
	if v, err := c.Cookie("fto_token"); err == nil {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	// Authorization: Bearer <token>
	authz := c.GetHeader("Authorization")
	if authz != "" {
		parts := strings.SplitN(authz, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}


