package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"raspberry-bin-api/internal/models"
)

// memoryStore is an in-memory Store so the tests do not need Firestore.
type memoryStore struct {
	admins   map[string]models.Admin
	sessions map[string]models.AdminSession
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		admins:   make(map[string]models.Admin),
		sessions: make(map[string]models.AdminSession),
	}
}

func (s *memoryStore) GetAdmin(_ context.Context, username string) (*models.Admin, error) {
	admin, ok := s.admins[username]
	if !ok {
		return nil, ErrAdminNotFound
	}
	return &admin, nil
}

func (s *memoryStore) CreateAdmin(_ context.Context, admin models.Admin) error {
	if _, ok := s.admins[admin.Username]; ok {
		return ErrAdminExists
	}
	s.admins[admin.Username] = admin
	return nil
}

func (s *memoryStore) UpdateLoginState(_ context.Context, username string, failedAttempts int, lockedUntil time.Time) error {
	admin := s.admins[username]
	admin.FailedAttempts = failedAttempts
	admin.LockedUntil = lockedUntil
	s.admins[username] = admin
	return nil
}

func (s *memoryStore) CreateSession(_ context.Context, tokenHash string, session models.AdminSession) error {
	s.sessions[tokenHash] = session
	return nil
}

func (s *memoryStore) GetSession(_ context.Context, tokenHash string) (*models.AdminSession, error) {
	session, ok := s.sessions[tokenHash]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return &session, nil
}

func (s *memoryStore) DeleteSession(_ context.Context, tokenHash string) error {
	delete(s.sessions, tokenHash)
	return nil
}

const (
	testUsername = "admin"
	testPassword = "correct-horse-42"
)

// newTestService returns a service with one admin and a clock the test can move forward.
func newTestService(t *testing.T) (*Service, *memoryStore, *time.Time) {
	t.Helper()

	store := newMemoryStore()
	service := NewService(store)

	clock := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }

	if err := service.CreateAdmin(context.Background(), testUsername, testPassword); err != nil {
		t.Fatal(err)
	}
	return service, store, &clock
}

func TestCreateAdminStoresHashNotPassword(t *testing.T) {
	_, store, _ := newTestService(t)

	admin := store.admins[testUsername]
	if admin.PasswordHash == testPassword {
		t.Fatal("password is stored in plain text")
	}
	if !CheckPassword(admin.PasswordHash, testPassword) {
		t.Error("stored hash does not match the password")
	}
}

func TestCreateAdminRejectsBadInput(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	if err := service.CreateAdmin(ctx, "other", "weak"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password: error = %v, want %v", err, ErrWeakPassword)
	}
	if err := service.CreateAdmin(ctx, "../evil", testPassword); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("bad username: error = %v, want %v", err, ErrInvalidUsername)
	}
	if err := service.CreateAdmin(ctx, testUsername, testPassword); !errors.Is(err, ErrAdminExists) {
		t.Errorf("duplicate admin: error = %v, want %v", err, ErrAdminExists)
	}
}

func TestLoginWithCorrectPassword(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	token, _, err := service.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	username, err := service.ValidateSession(ctx, token)
	if err != nil || username != testUsername {
		t.Errorf("ValidateSession = %q, %v; want %q, nil", username, err, testUsername)
	}
}

func TestLoginWithWrongPasswordOrUnknownUser(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	if _, _, err := service.Login(ctx, testUsername, "wrong-password-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password: error = %v, want %v", err, ErrInvalidCredentials)
	}
	// Unknown user gets the same error, so usernames cannot be discovered
	if _, _, err := service.Login(ctx, "nobody", testPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user: error = %v, want %v", err, ErrInvalidCredentials)
	}
}

// RAD security requirement: after 5 wrong passwords, logins are blocked for 5 minutes.
func TestLoginLocksAfterFiveFailedAttempts(t *testing.T) {
	service, _, clock := newTestService(t)
	ctx := context.Background()

	for i := 1; i < MaxFailedAttempts; i++ {
		if _, _, err := service.Login(ctx, testUsername, "wrong-password-1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v, want %v", i, err, ErrInvalidCredentials)
		}
	}

	// 5th wrong attempt locks the account
	if _, _, err := service.Login(ctx, testUsername, "wrong-password-1"); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("attempt 5: error = %v, want %v", err, ErrAccountLocked)
	}

	// Even the correct password is refused while locked
	*clock = clock.Add(LockDuration - time.Second)
	if _, _, err := service.Login(ctx, testUsername, testPassword); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("correct password while locked: error = %v, want %v", err, ErrAccountLocked)
	}

	// After 5 minutes the correct password works again
	*clock = clock.Add(2 * time.Second)
	if _, _, err := service.Login(ctx, testUsername, testPassword); err != nil {
		t.Fatalf("login after lock expired: %v", err)
	}
}

func TestSuccessfulLoginResetsFailedAttempts(t *testing.T) {
	service, store, _ := newTestService(t)
	ctx := context.Background()

	service.Login(ctx, testUsername, "wrong-password-1")
	service.Login(ctx, testUsername, "wrong-password-1")
	if _, _, err := service.Login(ctx, testUsername, testPassword); err != nil {
		t.Fatal(err)
	}

	if got := store.admins[testUsername].FailedAttempts; got != 0 {
		t.Errorf("FailedAttempts = %d after successful login, want 0", got)
	}
}

func TestSessionExpires(t *testing.T) {
	service, _, clock := newTestService(t)
	ctx := context.Background()

	token, _, err := service.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatal(err)
	}

	*clock = clock.Add(SessionDuration)
	if _, err := service.ValidateSession(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("expired session: error = %v, want %v", err, ErrInvalidSession)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	token, _, err := service.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ValidateSession(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("session after logout: error = %v, want %v", err, ErrInvalidSession)
	}
}

func TestSessionStoredOnlyAsHash(t *testing.T) {
	service, store, _ := newTestService(t)

	token, _, err := service.Login(context.Background(), testUsername, testPassword)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := store.sessions[token]; ok {
		t.Error("raw token is used as the database key")
	}
	if _, ok := store.sessions[HashToken(token)]; !ok {
		t.Error("session is not stored under the token hash")
	}
}
