package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"fifteen-thirty-one-go/backend/internal/auth"
	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

type meResponse struct {
	User *models.User `json:"user"`
}

// fakeHash is a constant bcrypt hash used to normalize login timing when a user
// lookup fails or the username does not exist.
const fakeHash = "$2a$10$CwTycUXWue0Thq9StjUM0uJ8lvZ9i8a9kaI0s5momkGLumZ5qX6e."

func RegisterHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.RegisterHandler")
		defer span.End()

		var req authRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		uLen := utf8.RuneCountInString(req.Username)
		if uLen < 3 || uLen > 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 3-32 characters"})
			return
		}
		// Do not TrimSpace passwords: leading/trailing spaces are valid characters.
		if utf8.RuneCountInString(req.Password) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
			return
		}

		if _, err := models.GetUserByUsername(db, req.Username); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
			return
		} else if !errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			if auth.IsPasswordValidationError(err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "password hash error"})
			return
		}
		u, err := models.CreateUser(db, req.Username, hash)
		if err != nil {
			if models.IsUniqueConstraint(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		// Create default preferences (best-effort).
		_ = models.SetUserAutoCountMode(db, u.ID, "suggest")

		token, err := auth.GenerateToken(u.ID, u.Username, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"})
			return
		}

		setAuthCookie(c, cfg, token)
		c.JSON(http.StatusCreated, authResponse{Token: token, User: u})
	}
}

func LoginHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.LoginHandler")
		defer span.End()

		var req authRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
			return
		}

		u, err := models.GetUserByUsername(db, req.Username)
		pwHash := fakeHash
		userFound := false
		if err == nil {
			pwHash = u.PasswordHash
			userFound = true
		} else if errors.Is(err, models.ErrNotFound) {
			// Keep pwHash=fakeHash and continue to the bcrypt check to normalize timing.
			userFound = false
		} else {
			// Real DB error: return 500 (don't mask as invalid credentials).
			log.Printf("LoginHandler GetUserByUsername failed: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		// Always run bcrypt comparison exactly once per request to normalize timing.
		// Return 401 only for invalid credentials (including user-not-found after timing-normalized compare).
		if cmpErr := auth.ComparePasswordHash(pwHash, req.Password); cmpErr != nil || !userFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, err := auth.GenerateToken(u.ID, u.Username, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"})
			return
		}
		setAuthCookie(c, cfg, token)
		c.JSON(http.StatusOK, authResponse{Token: token, User: u})
	}
}

func MeHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.MeHandler")
		defer span.End()

		token := tokenFromHeaderOrCookie(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ParseAndValidateToken(token, cfg)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		u, err := models.GetUserByID(db, claims.UserID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found or unauthorized"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		c.JSON(http.StatusOK, meResponse{User: u})
	}
}

type walletChallengeRequest struct {
	WalletAddress string `json:"wallet_address"`
}

type walletChallengeResponse struct {
	Challenge string `json:"challenge"`
}

type walletVerifyRequest struct {
	WalletAddress string `json:"wallet_address"`
	Challenge     string `json:"challenge"`
	Signature     string `json:"signature"`
}

// WalletChallengeHandler returns a time-bound challenge for the client to sign with personal_sign.
func WalletChallengeHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.WalletChallengeHandler")
		defer span.End()

		var req walletChallengeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		challenge, err := auth.GenerateChallenge(cfg, req.WalletAddress)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, walletChallengeResponse{Challenge: challenge})
	}
}

func usernameFromWalletAddress(normalizedAddr string) string {
	if len(normalizedAddr) < 10 {
		return normalizedAddr
	}
	return normalizedAddr[:6] + "..." + normalizedAddr[len(normalizedAddr)-4:]
}

// WalletVerifyHandler verifies a signed challenge, then logs in or registers a wallet user.
func WalletVerifyHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.WalletVerifyHandler")
		defer span.End()

		var req walletVerifyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(req.WalletAddress) == "" || strings.TrimSpace(req.Signature) == "" || strings.TrimSpace(req.Challenge) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "wallet_address, challenge, and signature required"})
			return
		}

		if err := auth.VerifyChallenge(cfg, req.Challenge, req.WalletAddress); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		recovered, err := auth.RecoverAddress(req.Challenge, req.Signature)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}

		normalized, err := models.NormalizeWalletAddress(req.WalletAddress)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !strings.EqualFold(recovered, normalized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "signature does not match wallet address"})
			return
		}

		u, err := models.GetUserByWalletAddress(db, normalized)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			log.Printf("WalletVerifyHandler GetUserByWalletAddress failed: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if errors.Is(err, models.ErrNotFound) {
			base := usernameFromWalletAddress(normalized)
			// base is at most 14 runes (0x + 4 hex + "..." + 4 hex), so with "_<n>" suffix we stay under the 32 rune cap.
			username := base
			for attempt := 0; attempt < 50; attempt++ {
				if attempt > 0 {
					username = fmt.Sprintf("%s_%d", base, attempt)
				}
				if utf8.RuneCountInString(username) > 32 {
					username = username[:32]
				}
				_, errUser := models.GetUserByUsername(db, username)
				if errors.Is(errUser, models.ErrNotFound) {
					u, err = models.CreateWalletUser(db, normalized, username)
					if err != nil {
						if models.IsUniqueConstraint(err) {
							continue
						}
						log.Printf("WalletVerifyHandler CreateWalletUser failed: err=%v", err)
						c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
						return
					}
					_ = models.SetUserAutoCountMode(db, u.ID, "suggest")
					break
				}
				if errUser != nil {
					log.Printf("WalletVerifyHandler GetUserByUsername failed: err=%v", errUser)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
					return
				}
			}
			if u == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "could not allocate username"})
				return
			}
		}

		token, err := auth.GenerateToken(u.ID, u.Username, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"})
			return
		}
		setAuthCookie(c, cfg, token)
		c.JSON(http.StatusOK, authResponse{Token: token, User: u})
	}
}

func LogoutHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.LogoutHandler")
		defer span.End()

		// Clear cookie regardless of auth status.
		clearAuthCookie(c, cfg)
		c.Status(http.StatusNoContent)
	}
}

func setAuthCookie(c *gin.Context, cfg config.Config, token string) {
	// JWT TTL already enforced server-side; cookie lifetime is best-effort for UX.
	maxAge := int(cfg.JWTTTL.Seconds())
	secure := cfg.AppEnv != "development"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.AuthCookieName, token, maxAge, "/", "", secure, true)
}

func clearAuthCookie(c *gin.Context, cfg config.Config) {
	secure := cfg.AppEnv != "development"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.AuthCookieName, "", -1, "/", "", secure, true)
}

func tokenFromHeaderOrCookie(c *gin.Context) string {
	// Cookie first (preferred for browser clients).
	if v, err := c.Cookie(auth.AuthCookieName); err == nil {
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
