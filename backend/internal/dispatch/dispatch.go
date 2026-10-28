// Package dispatch connects the web app and the Pi, which never talk directly:
//
//  1. A user starts a scan: a scan request is created and a "capture" command
//     is queued for that bin.
//  2. The Pi long-polls for commands, takes the photo and sends it with the
//     request ID; the request is "claimed" so it can be used only once.
//  3. When the photo is classified the request is completed, and the web app,
//     which long-polls the request, gets the result at once.
//
// Requests and commands live only a few seconds, so they are kept in memory
// instead of in Firestore. This works because there is one backend instance;
// pending requests are lost if it restarts.
package dispatch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"raspberry-bin-api/internal/models"
)

const (
	StatusPending    = "pending"    // waiting for the photo from the Pi
	StatusProcessing = "processing" // photo received, being classified
	StatusDone       = "done"
	StatusFailed     = "failed"

	CommandCapture  = "capture"
	CommandOpenLid  = "open_lid"
	CommandCloseLid = "close_lid"

	ReasonTimeout = "timeout"
)

var (
	ErrBinBusy          = errors.New("a scan is already in progress for this bin")
	ErrUserBusy         = errors.New("you already have a scan in progress")
	ErrRequestNotFound  = errors.New("scan request not found")
	ErrRequestNotActive = errors.New("scan request is not waiting for a photo")
	ErrWrongBin         = errors.New("scan request belongs to another bin")
)

type Config struct {
	// How long the Pi has to send the photo after the user starts a scan
	PhotoTimeout time.Duration
	// Upper limit for classifying a received photo
	ProcessingTimeout time.Duration
	// How long a finished result can still be read
	ResultTTL time.Duration
	// How long an admin lid command waits for the Pi before it is dropped
	CommandTTL time.Duration
}

func DefaultConfig() Config {
	return Config{
		PhotoTimeout:      15 * time.Second,
		ProcessingTimeout: 60 * time.Second,
		ResultTTL:         5 * time.Minute,
		CommandTTL:        10 * time.Second,
	}
}

type Command struct {
	Command     string `json:"command"`
	RequestID   string `json:"request_id,omitempty"`
	Compartment string `json:"compartment,omitempty"`

	expiresAt time.Time
}

// Snapshot is what the web app gets for GET /api/scan-requests/{id}.
// "processing" is shown as "pending": for the user both mean "wait".
type Snapshot struct {
	Status string             `json:"status"`
	Result *models.ScanResult `json:"result,omitempty"`
	Reason string             `json:"reason,omitempty"`
}

type request struct {
	id       string
	binID    string
	username string
	status   string
	result   *models.ScanResult
	reason   string

	deadline   time.Time // photo or processing deadline, depending on status
	finishedAt time.Time
	done       chan struct{} // closed when the request is done or failed
}

type Dispatcher struct {
	mu  sync.Mutex
	cfg Config

	requests     map[string]*request
	activeByBin  map[string]string // bin ID -> request ID
	activeByUser map[string]string // username -> request ID
	commands     map[string][]Command
	// Closed and replaced whenever a command is queued for the bin,
	// which wakes every Pi request waiting on it.
	wakeups map[string]chan struct{}
}

func New(cfg Config) *Dispatcher {
	d := &Dispatcher{
		cfg:          cfg,
		requests:     make(map[string]*request),
		activeByBin:  make(map[string]string),
		activeByUser: make(map[string]string),
		commands:     make(map[string][]Command),
		wakeups:      make(map[string]chan struct{}),
	}
	go d.janitor()
	return d
}

func newRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateScanRequest starts a scan for a user at a bin. Only one scan may be
// active per bin and per user at a time. The check and the creation happen
// under one lock, so two users pressing "Start Scan" together cannot both get it.
func (d *Dispatcher) CreateScanRequest(binID, username string) (string, error) {
	id, err := newRequestID()
	if err != nil {
		return "", err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.expireLocked(time.Now())

	if _, busy := d.activeByBin[binID]; busy {
		return "", ErrBinBusy
	}
	if _, busy := d.activeByUser[username]; busy {
		return "", ErrUserBusy
	}

	now := time.Now()
	d.requests[id] = &request{
		id:       id,
		binID:    binID,
		username: username,
		status:   StatusPending,
		deadline: now.Add(d.cfg.PhotoTimeout),
		done:     make(chan struct{}),
	}
	d.activeByBin[binID] = id
	d.activeByUser[username] = id

	d.enqueueLocked(binID, Command{Command: CommandCapture, RequestID: id, expiresAt: now.Add(d.cfg.PhotoTimeout)})
	return id, nil
}

// EnqueueCommand queues an admin command (open_lid / close_lid) for the bin.
func (d *Dispatcher) EnqueueCommand(binID string, cmd Command) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cmd.expiresAt = time.Now().Add(d.cfg.CommandTTL)
	d.enqueueLocked(binID, cmd)
}

func (d *Dispatcher) enqueueLocked(binID string, cmd Command) {
	d.commands[binID] = append(d.commands[binID], cmd)
	if ch, ok := d.wakeups[binID]; ok {
		close(ch)
		delete(d.wakeups, binID)
	}
}

// NextCommand returns the oldest command for the bin. If there is none it
// waits up to `wait` for one (long polling) and returns false on timeout or
// when the Pi disconnects.
func (d *Dispatcher) NextCommand(ctx context.Context, binID string, wait time.Duration) (*Command, bool) {
	deadline := time.Now().Add(wait)

	for {
		d.mu.Lock()
		if cmd, ok := d.popCommandLocked(binID, time.Now()); ok {
			d.mu.Unlock()
			return &cmd, true
		}
		ch, ok := d.wakeups[binID]
		if !ok {
			ch = make(chan struct{})
			d.wakeups[binID] = ch
		}
		d.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, false
		}

		timer := time.NewTimer(remaining)
		select {
		case <-ch:
			timer.Stop() // a command was queued; loop and take it
		case <-timer.C:
			return nil, false
		case <-ctx.Done():
			timer.Stop()
			return nil, false
		}
	}
}

func (d *Dispatcher) popCommandLocked(binID string, now time.Time) (Command, bool) {
	queue := d.commands[binID]
	for len(queue) > 0 {
		cmd := queue[0]
		queue = queue[1:]

		if now.After(cmd.expiresAt) {
			continue
		}
		// A capture for a request that already timed out is useless
		if cmd.Command == CommandCapture {
			if req, ok := d.requests[cmd.RequestID]; !ok || req.status != StatusPending {
				continue
			}
		}

		d.setQueueLocked(binID, queue)
		return cmd, true
	}
	d.setQueueLocked(binID, queue)
	return Command{}, false
}

func (d *Dispatcher) setQueueLocked(binID string, queue []Command) {
	if len(queue) == 0 {
		delete(d.commands, binID)
	} else {
		d.commands[binID] = queue
	}
}

// Claim is called when the Pi sends a photo. It returns the username the scan
// belongs to and marks the request as processing, so the same request ID
// cannot be used for a second photo.
func (d *Dispatcher) Claim(requestID, binID string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.expireLocked(time.Now())

	req, ok := d.requests[requestID]
	if !ok {
		return "", ErrRequestNotFound
	}
	if req.binID != binID {
		return "", ErrWrongBin
	}
	if req.status != StatusPending {
		return "", ErrRequestNotActive
	}

	req.status = StatusProcessing
	req.deadline = time.Now().Add(d.cfg.ProcessingTimeout)
	return req.username, nil
}

// Complete stores the classification result and wakes the waiting web app.
func (d *Dispatcher) Complete(requestID string, result models.ScanResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if req, ok := d.requests[requestID]; ok && req.status == StatusProcessing {
		req.result = &result
		d.finishLocked(req, StatusDone, "")
	}
}

// Fail ends a request with a reason, e.g. when saving the scan failed.
func (d *Dispatcher) Fail(requestID, reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if req, ok := d.requests[requestID]; ok && (req.status == StatusPending || req.status == StatusProcessing) {
		d.finishLocked(req, StatusFailed, reason)
	}
}

func (d *Dispatcher) finishLocked(req *request, status, reason string) {
	req.status = status
	req.reason = reason
	req.finishedAt = time.Now()
	close(req.done)

	if d.activeByBin[req.binID] == req.id {
		delete(d.activeByBin, req.binID)
	}
	if d.activeByUser[req.username] == req.id {
		delete(d.activeByUser, req.username)
	}
}

// Wait returns the request's state. If it is not finished yet, it waits up to
// `wait` for the result (long polling). A request of another user is reported
// as not found, so nobody can read someone else's result by guessing an ID.
func (d *Dispatcher) Wait(ctx context.Context, requestID, username string, wait time.Duration) (Snapshot, error) {
	d.mu.Lock()
	d.expireLocked(time.Now())
	req, ok := d.requests[requestID]
	if !ok || req.username != username {
		d.mu.Unlock()
		return Snapshot{}, ErrRequestNotFound
	}
	done, deadline := req.done, req.deadline
	d.mu.Unlock()

	if wait > 0 {
		// Also wake up at the deadline, so a timeout is reported right away
		until := time.Until(deadline) + 10*time.Millisecond
		if until < wait {
			wait = until
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-done:
			case <-timer.C:
			case <-ctx.Done():
			}
			timer.Stop()
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.expireLocked(time.Now())
	return snapshotLocked(req), nil
}

func snapshotLocked(req *request) Snapshot {
	switch req.status {
	case StatusDone:
		return Snapshot{Status: StatusDone, Result: req.result}
	case StatusFailed:
		return Snapshot{Status: StatusFailed, Reason: req.reason}
	default:
		return Snapshot{Status: StatusPending}
	}
}

// expireLocked fails requests whose photo or processing time ran out and
// forgets finished requests after ResultTTL.
func (d *Dispatcher) expireLocked(now time.Time) {
	for id, req := range d.requests {
		switch req.status {
		case StatusPending, StatusProcessing:
			if now.After(req.deadline) {
				d.finishLocked(req, StatusFailed, ReasonTimeout)
			}
		default:
			if now.Sub(req.finishedAt) > d.cfg.ResultTTL {
				delete(d.requests, id)
			}
		}
	}
}

func (d *Dispatcher) janitor() {
	for {
		time.Sleep(time.Second)
		d.mu.Lock()
		d.expireLocked(time.Now())
		d.mu.Unlock()
	}
}
