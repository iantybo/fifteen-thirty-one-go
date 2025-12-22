package models

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

var ErrNotFound = errors.New("not found")

func IsUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	var se sqlite3.Error
	if !errors.As(err, &se) {
		return false
	}
	// Prefer a precise UNIQUE check when available via ExtendedCode.
	if se.ExtendedCode == sqlite3.ErrConstraintUnique {
		return true
	}
	// Fallback to generic constraint code (covers older/driver-wrapped cases).
	return se.Code == sqlite3.ErrConstraint
}


