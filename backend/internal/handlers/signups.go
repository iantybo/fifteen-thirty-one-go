package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/middleware"
	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

type createSignupRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Note  string `json:"note"`
}

type updateSignupStatusRequest struct {
	Status string `json:"status"`
}

// signupErrorStatus maps signup model errors to an HTTP status and client
// message. ok=false means the error is not a client fault.
func signupErrorStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, models.ErrInvalidSignupEmail):
		return http.StatusBadRequest, "a valid email address is required", true
	case errors.Is(err, models.ErrInvalidSignupName):
		return http.StatusBadRequest, "name is required and must be 80 characters or fewer", true
	case errors.Is(err, models.ErrInvalidSignupNote):
		return http.StatusBadRequest, "note must be 500 characters or fewer", true
	case errors.Is(err, models.ErrInvalidSignupStatus):
		return http.StatusBadRequest, "invalid status", true
	case errors.Is(err, models.ErrSignupEmailTaken):
		return http.StatusConflict, "that email is already signed up", true
	case errors.Is(err, models.ErrSignupNotFound):
		return http.StatusNotFound, "signup not found", true
	}
	return 0, "", false
}

// logSignupError reports a signup failure, correlating it with the request id.
// The email is deliberately not logged: it is user-supplied PII and the
// request id is enough to tie a support report back to this line.
func logSignupError(c *gin.Context, op string, err error) {
	slog.Error("signup operation failed",
		"request_id", middleware.RequestIDFromContext(c),
		"op", op,
		"error", err,
	)
}

// respondSignupError writes the mapped client error, or a 500 for server faults.
func respondSignupError(c *gin.Context, op string, err error) {
	if status, msg, ok := signupErrorStatus(err); ok {
		slog.Info("signup rejected",
			"request_id", middleware.RequestIDFromContext(c),
			"op", op,
			"status", status,
			"reason", msg,
		)
		c.JSON(status, gin.H{"error": msg})
		return
	}
	logSignupError(c, op, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
}

// signupIDParam parses the :signupId path parameter.
func signupIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("signupId"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signup id"})
		return 0, false
	}
	return id, true
}

// CreateSignupHandler accepts a public signup submission.
func CreateSignupHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.CreateSignupHandler")
		defer span.End()

		var req createSignupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		signup, err := models.CreateSignup(ctx, db, models.SignupInput{
			Email: req.Email,
			Name:  req.Name,
			Note:  req.Note,
		})
		if err != nil {
			respondSignupError(c, "CreateSignup", err)
			return
		}

		// Log the id rather than the email so the audit trail carries no PII.
		slog.Info("signup created",
			"request_id", middleware.RequestIDFromContext(c),
			"signup_id", signup.ID,
			"status", signup.Status,
		)
		c.JSON(http.StatusCreated, gin.H{"signup": signup})
	}
}

// ListSignupsHandler returns signups. Auth-gated: signups contain email
// addresses, so this must not be public.
func ListSignupsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.ListSignupsHandler")
		defer span.End()

		status := c.Query("status")
		limit := 0
		if raw := c.Query("limit"); raw != "" {
			v, err := strconv.Atoi(raw)
			if err != nil || v <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			limit = v
		}

		signups, err := models.ListSignups(ctx, db, status, limit)
		if err != nil {
			respondSignupError(c, "ListSignups", err)
			return
		}
		counts, err := models.CountSignupsByStatus(ctx, db)
		if err != nil {
			respondSignupError(c, "CountSignupsByStatus", err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"signups": signups, "counts": counts})
	}
}

// UpdateSignupStatusHandler moves a signup through its lifecycle. Auth-gated.
func UpdateSignupStatusHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.UpdateSignupStatusHandler")
		defer span.End()

		id, ok := signupIDParam(c)
		if !ok {
			return
		}

		var req updateSignupStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		signup, err := models.UpdateSignupStatus(ctx, db, id, req.Status)
		if err != nil {
			respondSignupError(c, "UpdateSignupStatus", err)
			return
		}

		actorID, _ := userIDFromContext(c)
		slog.Info("signup status updated",
			"request_id", middleware.RequestIDFromContext(c),
			"signup_id", signup.ID,
			"status", signup.Status,
			"actor_user_id", actorID,
		)
		c.JSON(http.StatusOK, gin.H{"signup": signup})
	}
}
