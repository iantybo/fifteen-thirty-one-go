package handlers

// resync_policy.go holds the acceptance policy for reconciling a client's
// optimistic action queue against authoritative server state.
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
//   - If the client is at the same revision as the server (or ahead, which
//     should not happen), nothing new has landed; the pending actions are still
//     genuinely outstanding and are returned as neither accepted nor rejected
//     (the client keeps replaying).
//
// This keeps the client and server from diverging without requiring the server
// to store per-action client ids. A future revision of this endpoint can record
// client_id on each move and return precise accepted/rejected sets; the wire
// contract already carries the ids to make that change non-breaking.

// classifyPending decides the fate of a single pending action given whether the
// client is behind the server. It is a pure helper for reconcilePending:
//
//   - When the client is not behind, there is nothing to resolve, so the action
//     is left outstanding (accept == false).
//   - When the client is behind, a well-formed action is accepted (resolved),
//     while an action with a blank client id can never be reconciled and so is
//     not accepted — reconcilePending surfaces those as rejected instead.
func classifyPending(p resyncPendingAction, clientBehind bool) (accept bool) {
	if !clientBehind {
		return false
	}
	return p.ClientID != ""
}

// reconcilePending implements the acceptance policy documented at the top of the
// file. It is a pure function of the request and the server revision so it can
// be unit-tested without a database. It always returns non-nil slices so the
// JSON response serializes to [] rather than null (the client treats null and
// [] identically, but [] is friendlier for consumers and tests).
func reconcilePending(req resyncRequest, serverRevision int64) (accepted, rejected []string) {
	accepted = make([]string, 0, len(req.Pending))
	rejected = make([]string, 0)

	clientBehind := revisionIsStale(req.LastRevision, serverRevision)
	if !clientBehind {
		// Client is up to date (or ahead, which should not happen): nothing to
		// reconcile. Leave the pending actions outstanding on the client.
		return accepted, rejected
	}

	// The server has advanced past the client: its optimistic actions are
	// superseded by the authoritative snapshot, so the client should stop
	// tracking them.
	for _, p := range req.Pending {
		if classifyPending(p, clientBehind) {
			accepted = append(accepted, p.ClientID)
			continue
		}
		// Defensive: a pending entry with no id can never be reconciled, so
		// surface it as rejected to prompt the client to drop it.
		rejected = append(rejected, p.ClientID)
	}
	return accepted, rejected
}
