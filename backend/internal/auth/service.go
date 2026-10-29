package auth

import (
	"context"
	"errors"
	"log"
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
	ErrInvalidRole        = errors.New("role must be user or admin")

	// Returned by the Store
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("username is already taken")
	ErrSessionNotFound = errors.New("session not found")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// Store is where users and sessions are saved. The Firestore implementation
// is in the repository package; tests use an in-memory one.
type Store interface {
	GetUser(ctx context.Context, username string) (*models.User, error)
	CreateUser(ctx context.Context, user models.User) error
	// UpdateUser loads the user, applies fn and saves the result in one
	// transaction, so concurrent updates (e.g. parallel failed logins) are never lost.
	// fn may run more than once if the transaction is retried.
	UpdateUser(ctx context.Context, username string, fn func(*models.User) error) error
	CreateSession(ctx context.Context, tokenHash string, session models.Session) error
	GetSession(ctx context.Context, tokenHash string) (*models.Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	// SetLastLogin only changes the user's last_login_at field
	SetLastLogin(ctx context.Context, username string, at time.Time) error
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	Role      string
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

// Register creates a regular user account. Anyone can register; admins can
// only be created on the server with CreateAdmin.
func (s *Service) Register(ctx context.Context, username, password string) (string, error) {
	return s.createUser(ctx, username, password, models.RoleUser)
}

// CreateAdmin is used by the create-admin command on the server.
func (s *Service) CreateAdmin(ctx context.Context, username, password string) error {
	_, err := s.createUser(ctx, username, password, models.RoleAdmin)
	return err
}

// CreateUser is used by admins in the web app to add an account with either role.
func (s *Service) CreateUser(ctx context.Context, username, password, role string) (string, error) {
	if role != models.RoleUser && role != models.RoleAdmin {
		return "", ErrInvalidRole
	}
	return s.createUser(ctx, username, password, role)
}

func (s *Service) createUser(ctx context.Context, username, password, role string) (string, error) {
	username = normalizeUsername(username)
	if !usernamePattern.MatchString(username) {
		return "", ErrInvalidUsername
	}
	if err := ValidatePasswordStrength(password); err != nil {
		return "", err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}

	err = s.store.CreateUser(ctx, models.User{
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    s.now(),
	})
	if err != nil {
		return "", err
	}
	return username, nil
}

// Login checks the password and returns a new session token.
func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	username = normalizeUsername(username)

	user, err := s.store.GetUser(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		CheckPassword(s.dummyHash, password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	now := s.now()

	// While locked, even the correct password is refused
	if now.Before(user.LockedUntil) {
		return nil, ErrAccountLocked
	}

	// The bcrypt check is slow, so it runs outside the transaction
	if !CheckPassword(user.PasswordHash, password) {
		return nil, s.recordFailedLogin(ctx, username, now)
	}

	// Successful login clears earlier failed attempts
	if user.FailedAttempts > 0 || !user.LockedUntil.IsZero() {
		err := s.store.UpdateUser(ctx, username, func(u *models.User) error {
			u.FailedAttempts = 0
			u.LockedUntil = time.Time{}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return nil, err
	}

	expiresAt := now.Add(SessionDuration)
	err = s.store.CreateSession(ctx, tokenHash, models.Session{
		Username:  username,
		Role:      user.Role,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	// Only for the admin user list, so a failed write must not fail the login
	if err := s.store.SetLastLogin(ctx, username, now); err != nil {
		log.Printf("[WARN] could not save last login for %q: %v", username, err)
	}
	return &LoginResult{Token: token, ExpiresAt: expiresAt, Role: user.Role}, nil
}

// recordFailedLogin counts a wrong password and locks the account on the 5th one.
// The counter is read and written in one transaction, so parallel wrong
// passwords are all counted. It returns the error to give back to the client.
func (s *Service) recordFailedLogin(ctx context.Context, username string, now time.Time) error {
	var locked bool

	err := s.store.UpdateUser(ctx, username, func(u *models.User) error {
		locked = false
		if now.Before(u.LockedUntil) {
			// Another request locked the account in the meantime
			locked = true
			return nil
		}

		u.FailedAttempts++
		if u.FailedAttempts >= MaxFailedAttempts {
			u.LockedUntil = now.Add(LockDuration)
			u.FailedAttempts = 0 // start counting again after the lock ends
			locked = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if locked {
		return ErrAccountLocked
	}
	return ErrInvalidCredentials
}

// ValidateSession returns the session if the token belongs to one that has not expired.
func (s *Service) ValidateSession(ctx context.Context, token string) (*models.Session, error) {
	if token == "" {
		return nil, ErrInvalidSession
	}

	tokenHash := HashToken(token)
	session, err := s.store.GetSession(ctx, tokenHash)
	if errors.Is(err, ErrSessionNotFound) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}

	if !s.now().Before(session.ExpiresAt) {
		// Clean up the expired session; the result does not matter
		_ = s.store.DeleteSession(ctx, tokenHash)
		return nil, ErrInvalidSession
	}
	return session, nil
}

// Logout ends the session, so the token can no longer be used.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, HashToken(token))
}
