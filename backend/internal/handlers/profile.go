package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fifteen-thirty-one-go/backend/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	maxUploadSize = 10 << 20 // 10MB
	uploadsDir    = "uploads/profile-images"
)

type ProfileUploadRequest struct {
	FullName          string `form:"full_name"`
	Email             string `form:"email"`
	Address           string `form:"address"`
	MothersMaidenName string `form:"mothers_maiden_name"`
	BillingAddress    string `form:"billing_address"`
	PhoneNumber       string `form:"phone_number"`
	DateOfBirth       string `form:"date_of_birth"`
	AnnualIncome      *int64 `form:"annual_income"`
}

// UploadProfileHandler handles profile field updates and optional profile image uploads.
// It expects multipart form data with optional fields: full_name, email, address, and profile_image.
func UploadProfileHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			log.Printf("profile upload: user_id not found in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		log.Printf("profile upload: processing request for user_id=%v", userID)

		// Parse multipart form with size limit
		if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
			log.Printf("profile upload: failed to parse multipart form for user_id=%v: %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "request too large or invalid form data"})
			return
		}

		var req ProfileUploadRequest
		if err := c.ShouldBind(&req); err != nil {
			log.Printf("profile upload: failed to bind form data for user_id=%v: %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form data"})
			return
		}

		log.Printf("profile upload: received data for user_id=%v - fields_present: email=%t, full_name=%t, address=%t, mothers_maiden=%t, billing_address=%t, phone=%t, dob=%t, income=%v",
			userID, req.Email != "", req.FullName != "", req.Address != "", req.MothersMaidenName != "", req.BillingAddress != "", req.PhoneNumber != "", req.DateOfBirth != "", req.AnnualIncome)

		// Handle image upload
		var profileImagePath *string
		file, header, err := c.Request.FormFile("profile_image")
		if err != nil && err != http.ErrMissingFile {
			log.Printf("profile upload: error reading file for user_id=%v: %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "error reading uploaded file"})
			return
		}

		if err == nil {
			defer file.Close()

			log.Printf("profile upload: processing image file for user_id=%v - size=%d bytes",
				userID, header.Size)

			// Validate file type
			contentType := header.Header.Get("Content-Type")
			if !isValidImageType(contentType) {
				log.Printf("profile upload: invalid content type for user_id=%v: %s", userID, contentType)
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image type, only jpg, jpeg, png, and gif are allowed"})
				return
			}

			// Create uploads directory if it doesn't exist
			if err := os.MkdirAll(uploadsDir, 0755); err != nil {
				log.Printf("profile upload: failed to create uploads directory for user_id=%v: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload directory"})
				return
			}

			// Generate secure filename using hash
			hash := sha256.New()
			if _, err := io.Copy(hash, file); err != nil {
				log.Printf("profile upload: failed to hash file for user_id=%v: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process image"})
				return
			}

			// Reset file pointer after hashing
			if _, err := file.Seek(0, 0); err != nil {
				log.Printf("profile upload: failed to reset file pointer for user_id=%v: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process image"})
				return
			}

			hashStr := hex.EncodeToString(hash.Sum(nil))

			// Derive extension from validated content type instead of user-supplied filename
			ext := getExtensionFromContentType(contentType)
			filename := fmt.Sprintf("%s%s", hashStr, ext)
			destPath := filepath.Join(uploadsDir, filename)

			// Save file
			out, err := os.Create(destPath)
			if err != nil {
				log.Printf("profile upload: failed to create file for user_id=%v: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image"})
				return
			}
			defer out.Close()

			if _, err := io.Copy(out, file); err != nil {
				log.Printf("profile upload: failed to write file for user_id=%v: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image"})
				return
			}

			profileImagePath = &destPath
			log.Printf("profile upload: successfully saved image for user_id=%v at path=%s", userID, destPath)
		}

		// Convert form fields to pointers (only non-empty values)
		var fullName, email, address, mothersMaidenName, billingAddress, phoneNumber, dateOfBirth *string
		if req.FullName != "" {
			fullName = &req.FullName
		}
		if req.Email != "" {
			email = &req.Email
		}
		if req.Address != "" {
			address = &req.Address
		}
		if req.MothersMaidenName != "" {
			mothersMaidenName = &req.MothersMaidenName
		}
		if req.BillingAddress != "" {
			billingAddress = &req.BillingAddress
		}
		if req.PhoneNumber != "" {
			phoneNumber = &req.PhoneNumber
		}
		if req.DateOfBirth != "" {
			dateOfBirth = &req.DateOfBirth
		}

		// Update user profile in database
		uid, ok := userID.(int64)
		if !ok {
			log.Printf("profile upload: invalid user_id type for user_id=%v", userID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if err := models.UpdateUserProfile(db, uid, fullName, email, address, profileImagePath, mothersMaidenName, billingAddress, phoneNumber, dateOfBirth, req.AnnualIncome); err != nil {
			log.Printf("profile upload: failed to update database for user_id=%v: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
			return
		}

		log.Printf("profile upload: successfully updated profile for user_id=%v", userID)

		c.JSON(http.StatusOK, gin.H{
			"message":            "profile updated successfully",
			"profile_image_path": profileImagePath,
		})
	}
}

func isValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
	}

	contentType = strings.ToLower(contentType)
	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}

func getExtensionFromContentType(contentType string) string {
	extMap := map[string]string{
		"image/jpeg": ".jpg",
		"image/jpg":  ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
	}

	contentType = strings.ToLower(contentType)
	if ext, ok := extMap[contentType]; ok {
		return ext
	}
	// Fallback to safe default
	return ".bin"
}
