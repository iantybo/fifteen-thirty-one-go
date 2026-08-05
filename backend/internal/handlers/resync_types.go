package handlers

// resync_types.go holds the wire types for the resync endpoint. They mirror the
// client-side contract in frontend/src/sync/types.ts#ResyncResponse:
//
//	POST /api/games/:id/resync
//	  request:  { last_revision: number, pending: [{ client_id, kind }] }
//	  response: { snapshot, revision, accepted: [client_id], rejected: [client_id] }
//
// See resync.go for the endpoint behavior and resync_policy.go for the
// acceptance policy that consumes these types.

// resyncPendingAction is one entry in the client's outstanding optimistic action
// queue. ClientID is the stable, deterministic id minted by the frontend (see
// client_id.go for the shared format); Kind is the action's move kind (e.g.
// "play_card", "go", "discard") and is currently informational only.
type resyncPendingAction struct {
	ClientID string `json:"client_id"`
	Kind     string `json:"kind"`
}

// resyncRequest is the body of a resync call: the client's last-known revision
// plus its outstanding action queue.
type resyncRequest struct {
	// LastRevision is the client's last-known server revision (move count).
	LastRevision int64 `json:"last_revision"`
	// Pending is the client's outstanding action queue (client ids + kinds).
	Pending []resyncPendingAction `json:"pending"`
}

// resyncResponse is the reconciled result returned to the client: the current
// authoritative snapshot and revision, plus the sets of pending client ids the
// server accepted (resolved) and rejected (should be dropped). Accepted and
// Rejected are always non-nil so they serialize to [] rather than null.
type resyncResponse struct {
	Snapshot *GameSnapshot `json:"snapshot"`
	Revision int64         `json:"revision"`
	Accepted []string      `json:"accepted"`
	Rejected []string      `json:"rejected"`
}
