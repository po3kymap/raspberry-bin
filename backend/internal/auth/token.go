package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewSessionToken creates a random session token.
// The token goes to the client; only its hash is stored in the database,
// so someone who reads the database still cannot log in with it.
func NewSessionToken() (token string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the SHA-256 hash of a token. A fast hash is enough here
// (unlike passwords) because the token is 32 random bytes and cannot be guessed.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
