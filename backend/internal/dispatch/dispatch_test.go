package dispatch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"raspberry-bin-api/internal/models"
)

// Short timeouts so the tests run quickly
func testDispatcher() *Dispatcher {
	return New(Config{
		PhotoTimeout:      200 * time.Millisecond,
		ProcessingTimeout: 500 * time.Millisecond,
		ResultTTL:         time.Second,
		CommandTTL:        200 * time.Millisecond,
	})
}

var result = models.ScanResult{Category: "PMD", Confidence: 0.97, CanSort: true}

func TestFullScanFlow(t *testing.T) {
	d := testDispatcher()
	ctx := context.Background()

	id, err := d.CreateScanRequest("bin-27", "alice")
	if err != nil {
		t.Fatal(err)
	}

	// The Pi gets the capture command
	cmd, ok := d.NextCommand(ctx, "bin-27", 0)
	if !ok || cmd.Command != CommandCapture || cmd.RequestID != id {
		t.Fatalf("NextCommand = %+v, %v; want capture for %s", cmd, ok, id)
	}

	// The Pi sends the photo
	username, err := d.Claim(id, "bin-27")
	if err != nil || username != "alice" {
		t.Fatalf("Claim = %q, %v", username, err)
	}
	if snap, _ := d.Wait(ctx, id, "alice", 0); snap.Status != StatusPending {
		t.Errorf("status while processing = %q, want pending", snap.Status)
	}

	d.Complete(id, result)

	snap, err := d.Wait(ctx, id, "alice", 0)
	if err != nil || snap.Status != StatusDone || snap.Result == nil || snap.Result.Category != "PMD" {
		t.Errorf("Wait = %+v, %v; want done with PMD", snap, err)
	}
}

// Two users pressing "Start Scan" at the same moment: only one may get the bin.
func TestOnlyOneScanPerBin(t *testing.T) {
	d := testDispatcher()

	var wg sync.WaitGroup
	var mu sync.Mutex
	created, busy := 0, 0
	for _, user := range []string{"alice", "bob", "carol", "dave"} {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()
			_, err := d.CreateScanRequest("bin-27", user)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				created++
			} else if errors.Is(err, ErrBinBusy) {
				busy++
			}
		}(user)
	}
	wg.Wait()

	if created != 1 || busy != 3 {
		t.Errorf("created = %d, busy = %d; want 1 and 3", created, busy)
	}
}

func TestOnlyOneScanPerUser(t *testing.T) {
	d := testDispatcher()

	if _, err := d.CreateScanRequest("bin-1", "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateScanRequest("bin-2", "alice"); !errors.Is(err, ErrUserBusy) {
		t.Errorf("second scan of the same user: error = %v, want %v", err, ErrUserBusy)
	}
}

func TestRequestCanBeClaimedOnlyOnceAndOnlyForItsBin(t *testing.T) {
	d := testDispatcher()
	id, _ := d.CreateScanRequest("bin-27", "alice")

	if _, err := d.Claim(id, "bin-99"); !errors.Is(err, ErrWrongBin) {
		t.Errorf("claim for another bin: error = %v, want %v", err, ErrWrongBin)
	}
	if _, err := d.Claim("00000000000000000000000000000000", "bin-27"); !errors.Is(err, ErrRequestNotFound) {
		t.Errorf("unknown request: error = %v, want %v", err, ErrRequestNotFound)
	}
	if _, err := d.Claim(id, "bin-27"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Claim(id, "bin-27"); !errors.Is(err, ErrRequestNotActive) {
		t.Errorf("second photo for the same request: error = %v, want %v", err, ErrRequestNotActive)
	}
}

// Nobody can read someone else's result by guessing the request ID.
func TestOtherUserCannotReadRequest(t *testing.T) {
	d := testDispatcher()
	id, _ := d.CreateScanRequest("bin-27", "alice")

	if _, err := d.Wait(context.Background(), id, "mallory", 0); !errors.Is(err, ErrRequestNotFound) {
		t.Errorf("other user: error = %v, want %v", err, ErrRequestNotFound)
	}
}

func TestPhotoTimeoutFailsRequestAndFreesBin(t *testing.T) {
	d := testDispatcher()
	ctx := context.Background()
	id, _ := d.CreateScanRequest("bin-27", "alice")

	// Long polling wakes up at the deadline and reports the timeout
	start := time.Now()
	snap, err := d.Wait(ctx, id, "alice", 5*time.Second)
	if err != nil || snap.Status != StatusFailed || snap.Reason != ReasonTimeout {
		t.Fatalf("Wait = %+v, %v; want failed/timeout", snap, err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout reported after %v, want right after the 200 ms deadline", elapsed)
	}

	// A late photo is refused, the stale capture is not handed out, and the bin is free again
	if _, err := d.Claim(id, "bin-27"); !errors.Is(err, ErrRequestNotActive) {
		t.Errorf("late photo: error = %v, want %v", err, ErrRequestNotActive)
	}
	if cmd, ok := d.NextCommand(ctx, "bin-27", 0); ok {
		t.Errorf("stale command handed out: %+v", cmd)
	}
	if _, err := d.CreateScanRequest("bin-27", "bob"); err != nil {
		t.Errorf("bin still busy after timeout: %v", err)
	}
}

// Long polling: the waiting Pi gets a command as soon as it is queued.
func TestNextCommandWakesUpWhenCommandArrives(t *testing.T) {
	d := testDispatcher()

	go func() {
		time.Sleep(50 * time.Millisecond)
		d.EnqueueCommand("bin-27", Command{Command: CommandOpenLid, Compartment: "PMD"})
	}()

	start := time.Now()
	cmd, ok := d.NextCommand(context.Background(), "bin-27", 5*time.Second)
	if !ok || cmd.Command != CommandOpenLid || cmd.Compartment != "PMD" {
		t.Fatalf("NextCommand = %+v, %v; want open_lid PMD", cmd, ok)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("command arrived after %v, want right after it was queued", elapsed)
	}
}

func TestNextCommandTimesOut(t *testing.T) {
	d := testDispatcher()

	if cmd, ok := d.NextCommand(context.Background(), "bin-27", 100*time.Millisecond); ok {
		t.Errorf("got %+v, want no command", cmd)
	}
}

func TestNextCommandStopsWhenPiDisconnects(t *testing.T) {
	d := testDispatcher()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, ok := d.NextCommand(ctx, "bin-27", 5*time.Second); ok {
		t.Error("got a command, want none")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("returned after %v, want right after the client disconnected", elapsed)
	}
}

func TestExpiredLidCommandIsDropped(t *testing.T) {
	d := testDispatcher()
	d.EnqueueCommand("bin-27", Command{Command: CommandOpenLid, Compartment: "PMD"})

	time.Sleep(300 * time.Millisecond) // longer than CommandTTL

	if cmd, ok := d.NextCommand(context.Background(), "bin-27", 0); ok {
		t.Errorf("expired command handed out: %+v", cmd)
	}
}
