package activity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	events []Event
	err    error
}

func (s *memoryStore) AddEvent(_ context.Context, event Event) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, event)
	return nil
}

func TestRecordSetsTimeAndShortensUsername(t *testing.T) {
	store := &memoryStore{}
	recorder := NewRecorder(store)
	at := time.Date(2026, 10, 29, 15, 0, 0, 0, time.FixedZone("CET", 3600))
	recorder.now = func() time.Time { return at }

	recorder.Record(context.Background(), Event{Type: LoginFailed, Username: strings.Repeat("x", 500), Reason: "wrong_password"})

	if len(store.events) != 1 {
		t.Fatalf("stored %d events, want 1", len(store.events))
	}
	event := store.events[0]
	if !event.At.Equal(at) || event.At.Location() != time.UTC {
		t.Errorf("at = %v, want %v in UTC", event.At, at)
	}
	if len(event.Username) != maxUsernameLength {
		t.Errorf("username length = %d, want %d", len(event.Username), maxUsernameLength)
	}
}

func TestRecordDoesNotFailWhenTheStoreFails(t *testing.T) {
	recorder := NewRecorder(&memoryStore{err: errors.New("firestore is down")})

	// Must only log; a panic or returned error would break the request
	recorder.Record(context.Background(), Event{Type: Login, Username: "alice"})
}
