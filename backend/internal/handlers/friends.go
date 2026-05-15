// Package handlers - friends.go wires the friends/blocks model into HTTP
// endpoints. All routes require an authenticated user and use the shared
// helpers in lobby_helpers.go where applicable.
package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"fifteen-thirty-one-go/backend/internal/models"
	"fifteen-thirty-one-go/backend/internal/tracing"

	"github.com/gin-gonic/gin"
)

// SendFriendRequestHandler handles POST /api/friends/requests with body
// {"user_id": <int>}. It returns the resulting friendship row.
func SendFriendRequestHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.SendFriendRequest")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			UserID int64 `json:"user_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
			return
		}

		f, err := models.SendFriendRequest(ctx, db, userID, req.UserID)
		switch {
		case errors.Is(err, models.ErrSelfFriendship):
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot befriend yourself"})
			return
		case errors.Is(err, models.ErrAlreadyFriends):
			c.JSON(http.StatusConflict, gin.H{"error": "already friends"})
			return
		case errors.Is(err, models.ErrRequestExists):
			c.JSON(http.StatusConflict, gin.H{"error": "pending request already exists"})
			return
		case errors.Is(err, models.ErrBlocked):
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot send request — blocked"})
			return
		case err != nil:
			log.Printf("SendFriendRequestHandler: requester=%d other=%d: %v", userID, req.UserID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"request": f})
	}
}

// AcceptFriendRequestHandler handles POST /api/friends/requests/:id/accept.
func AcceptFriendRequestHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.AcceptFriendRequest")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
			return
		}

		f, err := models.AcceptFriendRequest(ctx, db, id, userID)
		switch {
		case errors.Is(err, models.ErrRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
			return
		case errors.Is(err, models.ErrNotAuthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "not your request"})
			return
		case err != nil:
			log.Printf("AcceptFriendRequestHandler: id=%d actor=%d: %v", id, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"friendship": f})
	}
}

// DeclineFriendRequestHandler handles POST /api/friends/requests/:id/decline.
func DeclineFriendRequestHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.DeclineFriendRequest")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
			return
		}

		err = models.DeclineFriendRequest(ctx, db, id, userID)
		switch {
		case errors.Is(err, models.ErrRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
			return
		case errors.Is(err, models.ErrNotAuthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "not your request"})
			return
		case err != nil:
			log.Printf("DeclineFriendRequestHandler: id=%d actor=%d: %v", id, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// RemoveFriendHandler handles DELETE /api/friends/:user_id.
func RemoveFriendHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.RemoveFriend")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		otherID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil || otherID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		if err := models.RemoveFriend(ctx, db, userID, otherID); err != nil {
			log.Printf("RemoveFriendHandler: actor=%d other=%d: %v", userID, otherID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// ListFriendsHandler handles GET /api/friends and returns three lists:
// accepted friends, incoming pending requests, and outgoing pending requests.
func ListFriendsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.ListFriends")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		friends, err := models.ListFriends(ctx, db, userID)
		if err != nil {
			log.Printf("ListFriendsHandler: friends user=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		incoming, err := models.ListIncomingRequests(ctx, db, userID)
		if err != nil {
			log.Printf("ListFriendsHandler: incoming user=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		outgoing, err := models.ListOutgoingRequests(ctx, db, userID)
		if err != nil {
			log.Printf("ListFriendsHandler: outgoing user=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"friends":  friends,
			"incoming": incoming,
			"outgoing": outgoing,
		})
	}
}

// BlockUserHandler handles POST /api/friends/blocks with body {"user_id": ...}.
func BlockUserHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.BlockUser")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			UserID int64 `json:"user_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
			return
		}

		err := models.BlockUser(ctx, db, userID, req.UserID)
		switch {
		case errors.Is(err, models.ErrSelfFriendship):
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot block yourself"})
			return
		case err != nil:
			log.Printf("BlockUserHandler: blocker=%d blocked=%d: %v", userID, req.UserID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// UnblockUserHandler handles DELETE /api/friends/blocks/:user_id.
func UnblockUserHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.UnblockUser")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		otherID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil || otherID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		if err := models.UnblockUser(ctx, db, userID, otherID); err != nil {
			log.Printf("UnblockUserHandler: actor=%d other=%d: %v", userID, otherID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// ListBlockedHandler handles GET /api/friends/blocks.
func ListBlockedHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracing.StartSpan(c.Request.Context(), "handlers.ListBlocked")
		defer span.End()

		userID, ok := userIDFromContext(c)
		if !ok || userID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		blocks, err := models.ListBlocked(ctx, db, userID)
		if err != nil {
			log.Printf("ListBlockedHandler: user=%d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"blocked": blocks})
	}
}
