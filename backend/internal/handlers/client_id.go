package handlers

import "strconv"

// client_id.go parses the stable client-action ids minted by the frontend.
//
// The format is "c:<gameId>:<userId>:<seq>" and is the shared contract with
// frontend/src/sync/clientId.ts (see parseClientId / isClientId there). A client
// id is:
//   - stable across retries (the same logical action keeps its id), and
//   - deterministic (no randomness), so tests are reproducible.
//
// The server currently uses these ids opaquely (for reconciliation bookkeeping),
// but parsing them lets us validate and, in future, correlate actions to the
// game/user that produced them. Keep this in sync with the frontend format.

// clientIDPrefix is the leading segment of every client id ("c").
const clientIDPrefix = "c"

// parseClientID splits a client id of the form "c:<gameId>:<userId>:<seq>" into
// its numeric components. It returns ok == false for any malformed input:
// wrong prefix, wrong number of segments, or non-numeric fields. The numeric
// components are only meaningful when ok is true.
func parseClientID(id string) (gameID, userID, seq int64, ok bool) {
	// Expect exactly four colon-separated segments: prefix, gameId, userId, seq.
	first := indexByte(id, ':')
	if first < 0 || id[:first] != clientIDPrefix {
		return 0, 0, 0, false
	}
	rest := id[first+1:]

	g, rest, ok := nextInt(rest)
	if !ok {
		return 0, 0, 0, false
	}
	u, rest, ok := nextInt(rest)
	if !ok {
		return 0, 0, 0, false
	}
	// The final segment must be a lone integer with no trailing separators.
	if indexByte(rest, ':') >= 0 {
		return 0, 0, 0, false
	}
	s, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	return g, u, s, true
}

// isValidClientID reports whether id is a well-formed client id. It is the Go
// counterpart of isClientId in frontend/src/sync/clientId.ts.
func isValidClientID(id string) bool {
	_, _, _, ok := parseClientID(id)
	return ok
}

// nextInt parses the integer segment before the next ':' in s, returning the
// parsed value and the remainder after that ':'. It requires a ':' to be present
// so it can only be used for non-final segments.
func nextInt(s string) (val int64, rest string, ok bool) {
	i := indexByte(s, ':')
	if i < 0 {
		return 0, "", false
	}
	n, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return n, s[i+1:], true
}

// indexByte returns the index of the first occurrence of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
