package models

import "time"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User is stored in the "users" collection, with the username as document ID.
// Regular users and admins are both users; Role tells them apart.
type User struct {
	Username       string    `firestore:"username"`
	PasswordHash   string    `firestore:"password_hash"`
	Role           string    `firestore:"role"`
	FailedAttempts int       `firestore:"failed_attempts"`
	LockedUntil    time.Time `firestore:"locked_until"`
	CreatedAt      time.Time `firestore:"created_at"`

	// Days in a row with at least one sorted item. Dates are "2006-01-02"
	// in the bin's time zone (Europe/Amsterdam).
	CurrentStreak  int    `firestore:"current_streak"`
	LongestStreak  int    `firestore:"longest_streak"`
	LastSortedDate string `firestore:"last_sorted_date"`
}

// Session is stored in the "sessions" collection, with the SHA-256 hash
// of the session token as document ID.
type Session struct {
	Username  string    `firestore:"username"`
	Role      string    `firestore:"role"`
	CreatedAt time.Time `firestore:"created_at"`
	ExpiresAt time.Time `firestore:"expires_at"`
}
