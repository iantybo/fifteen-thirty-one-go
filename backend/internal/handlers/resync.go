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
// ## Revision semantics
//
// A game's "revision" is defined as the number of durably-recorded moves for
// that game. It is monotonically non-decreasing and cheap to compute, which is
// all the client needs: it only compares revisions for ordering (newer vs.
// stale), never interprets the absolute value. Defining the revision in terms
// of persisted moves means it advances exactly when authoritative state changes.
//
// ## Acceptance policy
//
// The server does not (yet) persist client action ids alongside moves, so it
// cannot match a pending client_id to a specific stored move. Instead it uses a
// conservative, safe reconciliation rule:
//
//   - If the client's last_revision is behind the server's current revision,
//     the server has already advanced past the client's optimistic actions.
//     Those actions were either applied (their effect is now in the snapshot) or
//     superseded; either way the client should stop replaying them. We therefore
//     report every pending client_id as "accepted" (meaning: stop tracking it,
//     the authoritative snapshot supersedes your optimistic copy).
//
//   - If the client is at the same revision as the server, nothing new has
//     landed; the pending actions are still genuinely outstanding and are
//     returned as neither accepted nor rejected (the client keeps replaying).
//
// This keeps the client and server from diverging without requiring the server
// to store per-action client ids. A future revision of this endpoint can record
// client_id on each move and return precise accepted/rejected sets; the wire
// contract already carries the ids to make that change non-breaking.

type resyncPendingAction struct {
	ClientID string `json:"client_id"`
	Kind     string `json:"kind"`
}

type resyncRequest struct {
	// LastRevision is the client's last-known server revision (move count).
	LastRevision int64 `json:"last_revision"`
	// Pending is the client's outstanding action queue (client ids + kinds).
	Pending []resyncPendingAction `json:"pending"`
}

type resyncResponse struct {
	Snapshot *GameSnapshot `json:"snapshot"`
	Revision int64         `json:"revision"`
	Accepted []string      `json:"accepted"`
	Rejected []string      `json:"rejected"`
}

// gameRevision returns the authoritative revision for a game: the count of
// durably-recorded moves. A limit of -1 means "no limit" per ListMovesByGame's
// contract.
func gameRevision(db *sql.DB, gameID int64) (int64, error) {
	moves, err := models.ListMovesByGame(db, gameID, -1)
	if err != nil {
		return 0, err
	}
	return int64(len(moves)), nil
}

// ResyncHandler reconciles a client's optimistic action queue against the
// authoritative server state and returns the current snapshot plus the
// accepted/rejected client ids. See the file-level comment for the policy.
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

// reconcilePending implements the acceptance policy documented at the top of the
// file. It is a pure function of the request and the server revision so it can
// be unit-tested without a database. It always returns non-nil slices so the
// JSON response serializes to [] rather than null (the client treats null and
// [] identically, but [] is friendlier for consumers and tests).
func reconcilePending(req resyncRequest, serverRevision int64) (accepted, rejected []string) {
	accepted = make([]string, 0, len(req.Pending))
	rejected = make([]string, 0)

	// The server has advanced past the client: its optimistic actions are
	// superseded by the authoritative snapshot, so the client should stop
	// tracking them. We report them as accepted (i.e. "resolved").
	if req.LastRevision < serverRevision {
		for _, p := range req.Pending {
			if p.ClientID == "" {
				// Defensive: a pending entry with no id can never be reconciled,
				// so surface it as rejected to prompt the client to drop it.
				rejected = append(rejected, p.ClientID)
				continue
			}
			accepted = append(accepted, p.ClientID)
		}
		return accepted, rejected
	}

	// Client is up to date (or ahead, which should not happen): nothing to
	// reconcile. Leave the pending actions outstanding on the client.
	return accepted, rejected
}
