package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

type updateProfileRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// GetProfileHandler retrieves the authenticated user's profile
func GetProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.GetProfileHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		user, err := models.GetUserByID(db, userID)
		if err != nil {
			log.Printf("GetProfileHandler failed to get user: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// UpdateProfileHandler updates the authenticated user's profile settings
func UpdateProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateProfileHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req updateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		// Validate email format if provided
		if req.Email != nil && *req.Email != "" {
			trimmedEmail := strings.TrimSpace(*req.Email)
			if !emailRegex.MatchString(trimmedEmail) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
				return
			}
			req.Email = &trimmedEmail
		}

		// Validate display name length if provided
		if req.DisplayName != nil {
			trimmedName := strings.TrimSpace(*req.DisplayName)
			if len(trimmedName) > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "display name must be 100 characters or less"})
				return
			}
			req.DisplayName = &trimmedName
		}

		// Update profile
		if err := models.UpdateUserProfile(c.Request.Context(), db, userID, req.Email, req.DisplayName); err != nil {
			// Check for unique constraint violation on email
			if errors.Is(err, models.ErrEmailAlreadyExists) {
				c.JSON(http.StatusConflict, gin.H{"error": "email address is already in use"})
				return
			}
			log.Printf("UpdateProfileHandler failed: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		// Return updated user
		user, err := models.GetUserByID(db, userID)
		if err != nil {
			log.Printf("UpdateProfileHandler failed to get updated user: user_id=%d err=%v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}
