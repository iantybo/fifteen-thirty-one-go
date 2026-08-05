package handlers

// resync_response.go holds small, pure helpers for shaping the resync response.
// Extracted so ResyncHandler stays focused on request handling and so these
// helpers can be unit-tested without a database or gin context.

// ensureNonNilIDs returns the input slice, or an empty (non-nil) slice when the
// input is nil. This guarantees the JSON encoder emits `[]` rather than `null`
// for the accepted/rejected fields, which is friendlier for the TypeScript
// client (see frontend/src/sync/types.ts#ResyncResponse) and for tests.
func ensureNonNilIDs(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// dedupeIDs returns a new slice containing the first occurrence of each id,
// preserving order. The reconciliation policy should never produce duplicates,
// but this is a cheap defensive guard against a client sending the same
// client_id twice in its pending list.
func dedupeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// partitionPending splits a pending list into those with a well-formed client
// id and those without, using isValidClientID (client_id.go). Callers use this
// to reject structurally-invalid entries early.
func partitionPending(pending []resyncPendingAction) (valid, invalid []resyncPendingAction) {
	valid = make([]resyncPendingAction, 0, len(pending))
	invalid = make([]resyncPendingAction, 0)
	for _, p := range pending {
		if isValidClientID(p.ClientID) {
			valid = append(valid, p)
		} else {
			invalid = append(invalid, p)
		}
	}
	return valid, invalid
}
