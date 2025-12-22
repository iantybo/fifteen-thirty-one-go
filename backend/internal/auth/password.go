package auth

import (
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

// HashPassword hashes a plaintext password using bcrypt.
//
// Validation:
// - Must be at least minPasswordChars characters.
// - Must be <= bcryptMaxPasswordBytes bytes when encoded as UTF-8.
//   (bcrypt truncates inputs beyond 72 bytes.)
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", fmt.Errorf("password required")
	}
	if utf8.RuneCountInString(plain) < minPasswordChars {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordChars)
	}
	if len([]byte(plain)) > bcryptMaxPasswordBytes {
		return "", fmt.Errorf("password too long: bcrypt only supports up to %d bytes (UTF-8); shorten the password", bcryptMaxPasswordBytes)
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


