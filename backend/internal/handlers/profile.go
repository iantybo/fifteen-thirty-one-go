package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// maxEmailLen bounds the stored address so a caller cannot push an arbitrarily
// long string into the users table.
const maxEmailLen = 254

type updateProfileRequest struct {
	Email string `json:"email"`
}

// profileResponse is an allowlist of the profile fields safe to return. It
// deliberately excludes credential material such as the password hash.
type profileResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	GamesPlayed int64  `json:"games_played"`
	GamesWon    int64  `json:"games_won"`
}

func newProfileResponse(p *models.UserProfile) profileResponse {
	return profileResponse{
		ID:          p.ID,
		Username:    p.Username,
		Email:       p.Email,
		GamesPlayed: p.GamesPlayed,
		GamesWon:    p.GamesWon,
	}
}

// validateEmail trims the requested address and rejects it if it is missing or
// malformed. It returns the address to persist.
func validateEmail(raw string) (string, bool) {
	email := strings.TrimSpace(raw)
	if email == "" || len(email) > maxEmailLen {
		return "", false
	}
	// ParseAddress accepts display-name forms such as `A <a@b.com>`; require the
	// parsed address to match the input so only a bare address is stored.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", false
	}
	return email, true
}

// GetProfileHandler retrieves the authenticated user's profile.
func GetProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.GetProfileHandler")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		user, err := models.GetUserProfileByID(ctx, db, userID)
		if err != nil {
			log.Printf("GetProfileHandler failed to get user: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, newProfileResponse(user))
	}
}

// UpdateProfileHandler updates the authenticated user's email address.
func UpdateProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateProfileHandler")
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

		email, ok := validateEmail(req.Email)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
			return
		}

		if err := models.UpdateUserEmail(ctx, db, userID, email); err != nil {
			log.Printf("UpdateProfileHandler failed: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		user, err := models.GetUserProfileByID(ctx, db, userID)
		if err != nil {
			log.Printf("UpdateProfileHandler failed to get updated user: err=%v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, newProfileResponse(user))
	}
}
