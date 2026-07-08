package handlers

import (
	"database/sql"

	"fifteen-thirty-one-go/backend/internal/models"
)

// resync_revision.go defines a game's "revision" and the pure comparisons the
// client relies on for ordering.
//
// A game's revision is the number of durably-recorded moves for that game. It is
// monotonically non-decreasing and cheap to compute, which is all the client
// needs: it only compares revisions for ordering (newer vs. stale), never
// interprets the absolute value. Defining the revision in terms of persisted
// moves means it advances exactly when authoritative state changes.

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

// revisionIsNewer reports whether incoming reflects strictly more authoritative
// state than current (i.e. the server has advanced past what the caller last
// saw). It is a pure comparison with no side effects.
func revisionIsNewer(incoming, current int64) bool {
	return incoming > current
}

// revisionIsStale reports whether incoming is behind current, meaning the
// caller's view has been superseded by newer authoritative state. It is the
// strict complement of revisionIsNewer on the "not equal" partition:
// revisionIsNewer, revisionIsStale, and equality partition every pair of
// revisions into exactly one case.
func revisionIsStale(incoming, current int64) bool {
	return incoming < current
}
