package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"fifteen-thirty-one-go/backend/internal/auth"
	"fifteen-thirty-one-go/backend/internal/config"
	"fifteen-thirty-one-go/backend/internal/models"

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

func RegisterHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		c.JSON(http.StatusCreated, authResponse{Token: token, User: u})
	}
}

func LoginHandler(db *sql.DB, cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		if err := auth.ComparePasswordHash(u.PasswordHash, req.Password); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, err := auth.GenerateToken(u.ID, u.Username, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"})
			return
		}
		c.JSON(http.StatusOK, authResponse{Token: token, User: u})
	}
}


