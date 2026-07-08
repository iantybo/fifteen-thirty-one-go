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

// resync.go implements the server side of the client's Optimistic Sync Engine
// (see frontend/src/sync/). The client applies game actions optimistically and
// queues them durably; when it (re)connects or receives a game_update, it calls
// this endpoint to reconcile its outstanding actions against the authoritative
// server state.
//
// The contract, mirrored by frontend/src/sync/types.ts#ResyncResponse:
//
//	POST /api/games/:id/resync
//	  request:  { last_revision: number, pending: [{ client_id, kind }] }
//	  response: { snapshot, revision, accepted: [client_id], rejected: [client_id] }
//
// The wire types live in resync_types.go, the revision definition and
// comparisons in resync_revision.go, and the reconciliation policy in
// resync_policy.go.

// ResyncHandler reconciles a client's optimistic action queue against the
// authoritative server state and returns the current snapshot plus the
// accepted/rejected client ids. See resync_policy.go for the policy.
func ResyncHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracing.StartSpan(c.Request.Context(), "handlers.ResyncHandler")
		defer span.End()

		gameID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || gameID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}
		userID, ok := userIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req resyncRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}

		// Only participants may resync a game (prevents leaking snapshots to
		// arbitrary users). Mirrors the guard in QuitGameHandler/NextHandHandler.
		isParticipant, err := models.IsUserInGame(db, userID, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if !isParticipant {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a player"})
			return
		}

		revision, err := gameRevision(db, gameID)
		if err != nil {
			log.Printf("ResyncHandler gameRevision failed: game_id=%d err=%v", gameID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		snap, err := BuildGameSnapshotForUser(db, gameID, userID)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
				return
			}
			if errors.Is(err, models.ErrGameStateMissing) {
				c.JSON(http.StatusConflict, gin.H{"error": "game state unavailable; recreate lobby"})
				return
			}
			log.Printf("ResyncHandler BuildGameSnapshotForUser failed: game_id=%d user_id=%d err=%v", gameID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		accepted, rejected := reconcilePending(req, revision)

		c.JSON(http.StatusOK, resyncResponse{
			Snapshot: snap,
			Revision: revision,
			Accepted: accepted,
			Rejected: rejected,
		})
	}
}
