package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"raspberry-bin-api/internal/models"
)

const (
	// RAD: after 5 wrong passwords, block logins for 5 minutes
	MaxFailedAttempts = 5
	LockDuration      = 5 * time.Minute

	SessionDuration = 8 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("too many failed attempts, try again later")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrInvalidUsername    = errors.New("username must be 3-32 characters: a-z, 0-9, '.', '_' or '-'")

	// Returned by the Store
	ErrAdminNotFound   = errors.New("admin not found")
	ErrAdminExists     = errors.New("admin already exists")
	ErrSessionNotFound = errors.New("session not found")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// Store is where admins and sessions are saved. The Firestore implementation
// is in the repository package; tests use an in-memory one.
type Store interface {
	GetAdmin(ctx context.Context, username string) (*models.Admin, error)
	CreateAdmin(ctx context.Context, admin models.Admin) error
	UpdateLoginState(ctx context.Context, username string, failedAttempts int, lockedUntil time.Time) error
	CreateSession(ctx context.Context, tokenHash string, session models.AdminSession) error
	GetSession(ctx context.Context, tokenHash string) (*models.AdminSession, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

type Service struct {
	store Store
	now   func() time.Time

	// Hash checked when the username does not exist, so that a wrong
	// username takes as long as a wrong password and an attacker cannot
	// find out which usernames exist by timing the response.
	dummyHash string
}

func NewService(store Store) *Service {
	dummyHash, err := HashPassword("dummy-password-for-timing-0")
	if err != nil {
		panic(err)
	}
	return &Service{store: store, now: time.Now, dummyHash: dummyHash}
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// CreateAdmin checks the username and password strength, hashes the password and saves the admin.
func (s *Service) CreateAdmin(ctx context.Context, username, password string) error {
	username = normalizeUsername(username)
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	if err := ValidatePasswordStrength(password); err != nil {
		return err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	return s.store.CreateAdmin(ctx, models.Admin{
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    s.now(),
	})
}

// Login checks the password and returns a new session token.
func (s *Service) Login(ctx context.Context, username, password string) (token string, expiresAt time.Time, err error) {
	username = normalizeUsername(username)

	admin, err := s.store.GetAdmin(ctx, username)
	if errors.Is(err, ErrAdminNotFound) {
		CheckPassword(s.dummyHash, password)
		return "", time.Time{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", time.Time{}, err
	}

	now := s.now()

	// While locked, even the correct password is refused
	if now.Before(admin.LockedUntil) {
		return "", time.Time{}, ErrAccountLocked
	}

	if !CheckPassword(admin.PasswordHash, password) {
		return "", time.Time{}, s.recordFailedLogin(ctx, admin, now)
	}

	// Successful login clears earlier failed attempts
	if admin.FailedAttempts > 0 || !admin.LockedUntil.IsZero() {
		if err := s.store.UpdateLoginState(ctx, username, 0, time.Time{}); err != nil {
			return "", time.Time{}, err
		}
	}

	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt = now.Add(SessionDuration)
	err = s.store.CreateSession(ctx, tokenHash, models.AdminSession{
		Username:  username,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// recordFailedLogin counts a wrong password and locks the account on the 5th one.
// It returns the error the caller should give back to the client.
func (s *Service) recordFailedLogin(ctx context.Context, admin *models.Admin, now time.Time) error {
	attempts := admin.FailedAttempts + 1
	var lockedUntil time.Time

	if attempts >= MaxFailedAttempts {
		lockedUntil = now.Add(LockDuration)
		attempts = 0 // start counting again after the lock ends
	}

	if err := s.store.UpdateLoginState(ctx, admin.Username, attempts, lockedUntil); err != nil {
		return err
	}
	if !lockedUntil.IsZero() {
		return ErrAccountLocked
	}
	return ErrInvalidCredentials
}

// ValidateSession returns the admin's username if the token belongs to a session that has not expired.
func (s *Service) ValidateSession(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", ErrInvalidSession
	}

	tokenHash := HashToken(token)
	session, err := s.store.GetSession(ctx, tokenHash)
	if errors.Is(err, ErrSessionNotFound) {
		return "", ErrInvalidSession
	}
	if err != nil {
		return "", err
	}

	if !s.now().Before(session.ExpiresAt) {
		// Clean up the expired session; the result does not matter
		_ = s.store.DeleteSession(ctx, tokenHash)
		return "", ErrInvalidSession
	}
	return session.Username, nil
}

// Logout ends the session, so the token can no longer be used.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, HashToken(token))
}
