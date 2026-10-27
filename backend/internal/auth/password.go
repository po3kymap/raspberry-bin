package auth

import (
	"errors"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLength = 12

	// bcrypt only looks at the first 72 bytes, so longer passwords are rejected
	// instead of being silently cut off.
	MaxPasswordLength = 72
)

var bcryptCost = 12

var ErrWeakPassword = errors.New("weak password")

// ValidatePasswordStrength enforces the "strong password on creation" rule from the RAD.
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("%w: must be at least %d characters", ErrWeakPassword, MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return fmt.Errorf("%w: must be at most %d bytes", ErrWeakPassword, MaxPasswordLength)
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("%w: must contain both letters and digits", ErrWeakPassword)
	}
	return nil
}

// HashPassword returns a bcrypt hash. bcrypt generates a random salt and
// stores it inside the hash, so the same password gives a different hash each time.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether the password matches the hash.
// The comparison takes the same time whether it matches or not.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
