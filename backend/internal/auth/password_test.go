package auth

import (
	"errors"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	// The real cost makes every hash take ~0.25 s; tests only check behaviour
	bcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}

func TestHashPasswordDoesNotStorePlainText(t *testing.T) {
	password := "correct-horse-42"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, password) {
		t.Error("hash contains the plain-text password")
	}
	if !CheckPassword(hash, password) {
		t.Error("correct password was rejected")
	}
	if CheckPassword(hash, "wrong-horse-42") {
		t.Error("wrong password was accepted")
	}
}

func TestHashPasswordUsesSalt(t *testing.T) {
	// Same password twice must give different hashes thanks to the random salt
	first, _ := HashPassword("correct-horse-42")
	second, _ := HashPassword("correct-horse-42")

	if first == second {
		t.Error("two hashes of the same password are equal; salt is missing")
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"correct-horse-42", true},
		{"short1", false},
		{"onlylettersnodigits", false},
		{"123456789012345", false},
		{strings.Repeat("a1", 37), false}, // 74 bytes, over the bcrypt limit
	}

	for _, tt := range tests {
		err := ValidatePasswordStrength(tt.password)
		if (err == nil) != tt.valid {
			t.Errorf("ValidatePasswordStrength(%q) error = %v, want valid = %v", tt.password, err, tt.valid)
		}
		if err != nil && !errors.Is(err, ErrWeakPassword) {
			t.Errorf("error %v is not ErrWeakPassword", err)
		}
	}
}

func TestSessionTokensAreRandomAndHashed(t *testing.T) {
	token1, hash1, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	token2, _, _ := NewSessionToken()

	if token1 == token2 {
		t.Error("two session tokens are equal")
	}
	if hash1 == token1 {
		t.Error("stored hash equals the token itself")
	}
	if HashToken(token1) != hash1 {
		t.Error("HashToken does not match the hash from NewSessionToken")
	}
}
