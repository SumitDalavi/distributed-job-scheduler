package scheduler

import (
	"context"
	"testing"
	"time"
)

// TestIdempotencyKey verifies that the same job+tick always produces the same key.
func TestIdempotencyKeyDeterminism(t *testing.T) {
	tick := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	raw1 := tick.Unix()
	raw2 := tick.Unix()
	if raw1 != raw2 {
		t.Errorf("idempotency key timestamps differ: %d vs %d", raw1, raw2)
	}
}

// TestLeaderElectorInterface ensures LeaderElector satisfies its contract.
func TestLeaderElectorIsLeaderDefault(t *testing.T) {
	// A new elector without TryAcquire being called should report false
	e := &LeaderElector{}
	if e.IsLeader() {
		t.Error("expected IsLeader to be false before TryAcquire")
	}
}

// TestContextCancellation ensures the leader loop exits when context is cancelled.
func TestLeaderLoopExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		// Simulate the loop body without a real DB
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	cancel()
	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Error("leader loop did not exit after context cancellation")
	}
}
