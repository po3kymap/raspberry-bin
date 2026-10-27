package models

import "time"

// Admin is stored in the "admins" collection, with the username as document ID.
type Admin struct {
	Username       string    `firestore:"username"`
	PasswordHash   string    `firestore:"password_hash"`
	FailedAttempts int       `firestore:"failed_attempts"`
	LockedUntil    time.Time `firestore:"locked_until"`
	CreatedAt      time.Time `firestore:"created_at"`
}

// AdminSession is stored in the "admin_sessions" collection,
// with the SHA-256 hash of the session token as document ID.
type AdminSession struct {
	Username  string    `firestore:"username"`
	CreatedAt time.Time `firestore:"created_at"`
	ExpiresAt time.Time `firestore:"expires_at"`
}
