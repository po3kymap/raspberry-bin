// Package activity keeps the activity log that admins see: logins, logouts,
// registrations, scans and admin actions. It complements the [AUDIT] lines in
// the server log, which only someone with access to the server can read.
package activity

import (
	"context"
	"log"
	"time"
)

// Event types
const (
	Login       = "login"
	LoginFailed = "login_failed"
	Logout      = "logout"
	Register    = "register"
	UserCreated = "user_created"
	UserDeleted = "user_deleted"
	Scan        = "scan"
	Lid         = "lid"
	BinConnect  = "bin_connect"
)

// Longest username stored for a failed login, which can be anything the client sent
const maxUsernameLength = 64

// Event is one entry of the log. Only the fields that fit the type are set.
type Event struct {
	// Firestore document ID; filled when the log is read, not stored in the document
	ID   string    `json:"id" firestore:"-"`
	Type string    `json:"type" firestore:"type"`
	At   time.Time `json:"at" firestore:"at"`
	// Who did it: the user who logged in or scanned, or the admin
	Username string `json:"username" firestore:"username"`
	// The account an admin created or deleted
	Target string `json:"target,omitempty" firestore:"target,omitempty"`
	Role   string `json:"role,omitempty" firestore:"role,omitempty"`
	BinID  string `json:"bin_id,omitempty" firestore:"bin_id,omitempty"`

	// Scans: the scan document, the photo in Firebase Storage and the result
	ScanID     string  `json:"scan_id,omitempty" firestore:"scan_id,omitempty"`
	ImageID    string  `json:"image_id,omitempty" firestore:"image_id,omitempty"`
	Category   string  `json:"category,omitempty" firestore:"category,omitempty"`
	Confidence float64 `json:"confidence,omitempty" firestore:"confidence,omitempty"`
	CanSort    bool    `json:"can_sort" firestore:"can_sort"`
	// Why a scan was not accepted, or for a failed login "wrong_password" / "locked"
	Reason string `json:"reason,omitempty" firestore:"reason,omitempty"`

	// Lid commands
	Compartment string `json:"compartment,omitempty" firestore:"compartment,omitempty"`
	Action      string `json:"action,omitempty" firestore:"action,omitempty"`

	// Client IP of logins and admin actions, to notice attacks
	IP string `json:"ip,omitempty" firestore:"ip,omitempty"`
}

type Store interface {
	AddEvent(ctx context.Context, event Event) error
}

type Recorder struct {
	store Store
	now   func() time.Time
}

func NewRecorder(store Store) *Recorder {
	return &Recorder{store: store, now: time.Now}
}

// Record saves the event with the current time. The log is extra information,
// so a failed write is only logged and never fails the request that caused it.
func (r *Recorder) Record(ctx context.Context, event Event) {
	event.At = r.now().UTC()
	if len(event.Username) > maxUsernameLength {
		event.Username = event.Username[:maxUsernameLength]
	}
	if err := r.store.AddEvent(ctx, event); err != nil {
		log.Printf("[WARN] could not save %s activity for %q: %v", event.Type, event.Username, err)
	}
}
