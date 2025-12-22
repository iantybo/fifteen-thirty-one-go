package auth

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// bcrypt truncates passwords at 72 bytes. We enforce this explicitly to avoid
	// user confusion and inconsistent login behavior.
	bcryptMaxPasswordBytes = 72
	minPasswordChars       = 8
)

type PasswordValidationError struct {
	msg string
}

func (e PasswordValidationError) Error() string { return e.msg }

func IsPasswordValidationError(err error) bool {
	if err == nil {
		return false
	}
	var v PasswordValidationError
	return errors.As(err, &v)
}

// HashPassword hashes a plaintext password using bcrypt.
//
// Validation:
// - Must be at least minPasswordChars characters.
// - Must be <= bcryptMaxPasswordBytes bytes when encoded as UTF-8.
//   (bcrypt truncates inputs beyond 72 bytes.)
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", PasswordValidationError{msg: "password required"}
	}
	if utf8.RuneCountInString(plain) < minPasswordChars {
		return "", PasswordValidationError{msg: fmt.Sprintf("password must be at least %d characters", minPasswordChars)}
	}
	if len([]byte(plain)) > bcryptMaxPasswordBytes {
		return "", PasswordValidationError{msg: fmt.Sprintf("password too long: bcrypt only supports up to %d bytes (UTF-8); shorten the password", bcryptMaxPasswordBytes)}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func ComparePasswordHash(hash string, plain string) error {
	if plain == "" {
		return fmt.Errorf("password required")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}


