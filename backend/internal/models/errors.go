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
	// We only detect UNIQUE violations via ExtendedCode.
	// If ExtendedCode is unavailable (or indicates a different constraint), this returns false.
	return se.ExtendedCode == sqlite3.ErrConstraintUnique
}
