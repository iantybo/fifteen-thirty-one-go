package models

import (
	"errors"
	"strings"
)

var ErrNotFound = errors.New("not found")

func IsUniqueConstraint(err error) bool {
	// sqlite3 driver error strings are stable enough for this check.
	// We'll tighten this later if needed.
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}


