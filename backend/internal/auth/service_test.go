package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"raspberry-bin-api/internal/models"
)

// memoryStore is an in-memory Store so the tests do not need Firestore.
// The mutex plays the role of the Firestore transaction in UpdateUser.
type memoryStore struct {
	mu       sync.Mutex
	users    map[string]models.User
	sessions map[string]models.Session
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:    make(map[string]models.User),
		sessions: make(map[string]models.Session),
	}
}

func (s *memoryStore) GetUser(_ context.Context, username string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[username]
	if !ok {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

func (s *memoryStore) CreateUser(_ context.Context, user models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[user.Username]; ok {
		return ErrUserExists
	}
	s.users[user.Username] = user
	return nil
}

func (s *memoryStore) UpdateUser(_ context.Context, username string, fn func(*models.User) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[username]
	if !ok {
		return ErrUserNotFound
	}
	if err := fn(&user); err != nil {
		return err
	}
	s.users[username] = user
	return nil
}

func (s *memoryStore) CreateSession(_ context.Context, tokenHash string, session models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[tokenHash] = session
	return nil
}

func (s *memoryStore) GetSession(_ context.Context, tokenHash string) (*models.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenHash]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return &session, nil
}

func (s *memoryStore) DeleteSession(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenHash)
	return nil
}

func (s *memoryStore) SetLastLogin(_ context.Context, username string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[username]
	if !ok {
		return ErrUserNotFound
	}
	user.LastLoginAt = at
	s.users[username] = user
	return nil
}

const (
	testAdmin    = "admin"
	testUser     = "alice"
	testPassword = "correct-horse-42"
)

// newTestService returns a service with one admin and one user, and a clock
// the test can move forward.
func newTestService(t *testing.T) (*Service, *memoryStore, *time.Time) {
	t.Helper()

	store := newMemoryStore()
	service := NewService(store)

	clock := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }

	ctx := context.Background()
	if err := service.CreateAdmin(ctx, testAdmin, testPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Register(ctx, testUser, testPassword); err != nil {
		t.Fatal(err)
	}
	return service, store, &clock
}

func TestRegisterStoresHashAndUserRole(t *testing.T) {
	_, store, _ := newTestService(t)

	user := store.users[testUser]
	if user.PasswordHash == testPassword || !CheckPassword(user.PasswordHash, testPassword) {
		t.Error("password is not stored as a matching hash")
	}
	if user.Role != models.RoleUser {
		t.Errorf("registered role = %q, want %q", user.Role, models.RoleUser)
	}
	if store.users[testAdmin].Role != models.RoleAdmin {
		t.Error("CreateAdmin did not give the admin role")
	}
}

func TestRegisterRejectsBadInput(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	if _, err := service.Register(ctx, "bob", "weak"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password: error = %v, want %v", err, ErrWeakPassword)
	}
	if _, err := service.Register(ctx, "../evil", testPassword); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("bad username: error = %v, want %v", err, ErrInvalidUsername)
	}
	// Usernames are case-insensitive, so "ALICE" is the same account
	if _, err := service.Register(ctx, "ALICE", testPassword); !errors.Is(err, ErrUserExists) {
		t.Errorf("duplicate user: error = %v, want %v", err, ErrUserExists)
	}
}

func TestAdminCreatesUsersWithEitherRole(t *testing.T) {
	service, store, _ := newTestService(t)
	ctx := context.Background()

	for username, role := range map[string]string{"bob": models.RoleUser, "carol": models.RoleAdmin} {
		if _, err := service.CreateUser(ctx, username, testPassword, role); err != nil {
			t.Fatalf("create %s: %v", username, err)
		}
		user := store.users[username]
		if user.Role != role || !CheckPassword(user.PasswordHash, testPassword) {
			t.Errorf("%s: role = %q, password stored as hash = %v; want %q, true",
				username, user.Role, CheckPassword(user.PasswordHash, testPassword), role)
		}
	}

	if _, err := service.CreateUser(ctx, "dave", testPassword, "superuser"); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("unknown role: error = %v, want %v", err, ErrInvalidRole)
	}
	if _, err := service.CreateUser(ctx, "dave", "weak", models.RoleUser); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password: error = %v, want %v", err, ErrWeakPassword)
	}
	if _, err := service.CreateUser(ctx, testUser, testPassword, models.RoleUser); !errors.Is(err, ErrUserExists) {
		t.Errorf("existing user: error = %v, want %v", err, ErrUserExists)
	}
}

func TestLoginReturnsRole(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	for username, role := range map[string]string{testAdmin: models.RoleAdmin, testUser: models.RoleUser} {
		result, err := service.Login(ctx, username, testPassword)
		if err != nil {
			t.Fatalf("login %s: %v", username, err)
		}
		if result.Role != role {
			t.Errorf("login %s: role = %q, want %q", username, result.Role, role)
		}

		session, err := service.ValidateSession(ctx, result.Token)
		if err != nil || session.Username != username || session.Role != role {
			t.Errorf("ValidateSession = %+v, %v; want %s/%s", session, err, username, role)
		}
	}
}

func TestLoginWithWrongPasswordOrUnknownUser(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	if _, err := service.Login(ctx, testUser, "wrong-password-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password: error = %v, want %v", err, ErrInvalidCredentials)
	}
	// Unknown user gets the same error, so usernames cannot be discovered
	if _, err := service.Login(ctx, "nobody", testPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user: error = %v, want %v", err, ErrInvalidCredentials)
	}
}

// RAD security requirement: after 5 wrong passwords, logins are blocked for 5 minutes.
func TestLoginLocksAfterFiveFailedAttempts(t *testing.T) {
	service, _, clock := newTestService(t)
	ctx := context.Background()

	for i := 1; i < MaxFailedAttempts; i++ {
		if _, err := service.Login(ctx, testAdmin, "wrong-password-1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v, want %v", i, err, ErrInvalidCredentials)
		}
	}

	// 5th wrong attempt locks the account
	if _, err := service.Login(ctx, testAdmin, "wrong-password-1"); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("attempt 5: error = %v, want %v", err, ErrAccountLocked)
	}

	// Even the correct password is refused while locked
	*clock = clock.Add(LockDuration - time.Second)
	if _, err := service.Login(ctx, testAdmin, testPassword); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("correct password while locked: error = %v, want %v", err, ErrAccountLocked)
	}

	// After 5 minutes the correct password works again
	*clock = clock.Add(2 * time.Second)
	if _, err := service.Login(ctx, testAdmin, testPassword); err != nil {
		t.Fatalf("login after lock expired: %v", err)
	}
}

// Many wrong passwords at the same moment must all be counted (the race
// condition from the STD), so an attacker cannot get more than 5 guesses.
func TestParallelFailedLoginsAreAllCounted(t *testing.T) {
	service, store, _ := newTestService(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < MaxFailedAttempts-1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service.Login(ctx, testAdmin, "wrong-password-1")
		}()
	}
	wg.Wait()

	if got := store.users[testAdmin].FailedAttempts; got != MaxFailedAttempts-1 {
		t.Fatalf("FailedAttempts = %d after %d parallel wrong passwords", got, MaxFailedAttempts-1)
	}
	if _, err := service.Login(ctx, testAdmin, "wrong-password-1"); !errors.Is(err, ErrAccountLocked) {
		t.Errorf("next wrong password: error = %v, want %v", err, ErrAccountLocked)
	}
}

func TestSuccessfulLoginResetsFailedAttempts(t *testing.T) {
	service, store, _ := newTestService(t)
	ctx := context.Background()

	service.Login(ctx, testUser, "wrong-password-1")
	service.Login(ctx, testUser, "wrong-password-1")
	if _, err := service.Login(ctx, testUser, testPassword); err != nil {
		t.Fatal(err)
	}

	if got := store.users[testUser].FailedAttempts; got != 0 {
		t.Errorf("FailedAttempts = %d after successful login, want 0", got)
	}
}

func TestSessionExpires(t *testing.T) {
	service, _, clock := newTestService(t)
	ctx := context.Background()

	result, err := service.Login(ctx, testUser, testPassword)
	if err != nil {
		t.Fatal(err)
	}

	*clock = clock.Add(SessionDuration)
	if _, err := service.ValidateSession(ctx, result.Token); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("expired session: error = %v, want %v", err, ErrInvalidSession)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	service, _, _ := newTestService(t)
	ctx := context.Background()

	result, err := service.Login(ctx, testUser, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(ctx, result.Token); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ValidateSession(ctx, result.Token); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("session after logout: error = %v, want %v", err, ErrInvalidSession)
	}
}

func TestSessionStoredOnlyAsHash(t *testing.T) {
	service, store, _ := newTestService(t)

	result, err := service.Login(context.Background(), testUser, testPassword)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := store.sessions[result.Token]; ok {
		t.Error("raw token is used as the database key")
	}
	if _, ok := store.sessions[HashToken(result.Token)]; !ok {
		t.Error("session is not stored under the token hash")
	}
}

func TestLoginRecordsLastLogin(t *testing.T) {
	service, store, clock := newTestService(t)
	ctx := context.Background()

	result, err := service.Login(ctx, testUser, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	loginTime := *clock
	if got := store.users[testUser].LastLoginAt; !got.Equal(loginTime) {
		t.Fatalf("last login = %v, want %v", got, loginTime)
	}

	// Using the session later is not a login
	*clock = clock.Add(time.Hour)
	if _, err := service.ValidateSession(ctx, result.Token); err != nil {
		t.Fatal(err)
	}
	if got := store.users[testUser].LastLoginAt; !got.Equal(loginTime) {
		t.Errorf("last login after a request = %v, want still %v", got, loginTime)
	}

	// A wrong password is not a login either
	if _, err := service.Login(ctx, testUser, "wrong-password-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: error = %v", err)
	}
	if got := store.users[testUser].LastLoginAt; !got.Equal(loginTime) {
		t.Errorf("last login after a wrong password = %v, want still %v", got, loginTime)
	}
}
