package models

import (
	"errors"
	"strings"
)

var ErrNotFound = errors.New("not found")

func IsUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	// SQLite unique constraint violations typically contain "UNIQUE constraint failed"
	// in the error message. This is a portable way to detect them without depending
	// on the sqlite3 driver's specific error types.
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}