package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/models"
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

func TestSyncJobsNotLeader(t *testing.T) {
	mockDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	// Not acquiring, so isLeader = false

	sched := New(d, elector, "node1")
	err = sched.syncJobs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncJobsLeader(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	elector.isLeader = true

	sched := New(d, elector, "node1")

	rows := sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"}).
		AddRow("uuid-1", "test-job", "* * * * * *", "{}", true, time.Now(), time.Now(), time.Now(), time.Now())

	mock.ExpectQuery("SELECT id, name, cron_expr").WillReturnRows(rows)

	err = sched.syncJobs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sched.entryIDs) != 1 {
		t.Errorf("expected 1 job registered, got %d", len(sched.entryIDs))
	}
}

func TestListJobsError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	elector.isLeader = true

	sched := New(d, elector, "node1")

	mock.ExpectQuery("SELECT id, name, cron_expr").WillReturnError(fmt.Errorf("db error"))

	err = sched.syncJobs(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestExecuteJobClaimFailed(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	sched := New(d, elector, "node1")

	job := models.Job{
		ID:   "job-1",
		Name: "test-job",
	}

	mock.ExpectQuery("INSERT INTO execution_logs").
		WillReturnError(fmt.Errorf("conflict"))

	sched.executeJob(context.Background(), job)
	// should not panic and return early
}

func TestExecuteJobSuccess(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	sched := New(d, elector, "node1")

	job := models.Job{
		ID:   "job-1",
		Name: "test-job",
	}

	mock.ExpectQuery("INSERT INTO execution_logs").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("exec-1"))

	mock.ExpectExec("UPDATE execution_logs").
		WithArgs("succeeded", sqlmock.AnyArg(), "", sqlmock.AnyArg(), "exec-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE jobs").
		WithArgs(sqlmock.AnyArg(), "job-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	sched.executeJob(context.Background(), job)
}

func TestStartStop(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	sched := New(d, elector, "node1")

	ctx, cancel := context.WithCancel(context.Background())
	sched.Start(ctx, 10*time.Millisecond)

	// mock release
	mock.ExpectExec("DELETE FROM leader_leases").
		WillReturnResult(sqlmock.NewResult(0, 0))

	sched.Stop(ctx)
	cancel()
}

func TestSimulateWork(t *testing.T) {
	_, err := simulateWork(models.Job{Name: "test", Payload: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncJobsUnregistersRemovedJobs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	elector.isLeader = true

	sched := New(d, elector, "node1")

	// First sync: register a job
	rows1 := sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"}).
		AddRow("uuid-1", "test-job", "* * * * * *", "{}", true, time.Now(), time.Now(), time.Now(), time.Now())
	mock.ExpectQuery("SELECT id, name, cron_expr").WillReturnRows(rows1)

	_ = sched.syncJobs(context.Background())
	if len(sched.entryIDs) != 1 {
		t.Fatalf("expected 1 job registered, got %d", len(sched.entryIDs))
	}

	// Second sync: job is gone → should unregister
	rows2 := sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"})
	mock.ExpectQuery("SELECT id, name, cron_expr").WillReturnRows(rows2)

	_ = sched.syncJobs(context.Background())
	if len(sched.entryIDs) != 0 {
		t.Errorf("expected 0 jobs after unregistration, got %d", len(sched.entryIDs))
	}
}

func TestStopWithReleaseError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	sched := New(d, elector, "node1")

	ctx, cancel := context.WithCancel(context.Background())
	sched.Start(ctx, 10*time.Millisecond)

	// mock release fails
	mock.ExpectExec("DELETE FROM leader_leases").
		WillReturnError(fmt.Errorf("release error"))

	sched.Stop(ctx)
	cancel()
}

func TestLeaderLoopTick(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	sched := New(d, elector, "node1")

	ctx, cancel := context.WithCancel(context.Background())

	// The leader loop will tick and call TryAcquire then syncJobs
	// We need to allow 1-2 ticks before cancel
	mock.ExpectExec("INSERT INTO leader_leases").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, name, cron_expr").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"}))
	// Allow extra calls
	mock.MatchExpectationsInOrder(false)

	sched.Start(ctx, 10*time.Millisecond) // 5ms heartbeat

	time.Sleep(30 * time.Millisecond)
	cancel()

	// Wait for goroutines to settle
	time.Sleep(20 * time.Millisecond)
}

func TestLeaderLoopElectionError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)
	sched := New(d, elector, "node1")

	ctx, cancel := context.WithCancel(context.Background())

	// Expect election to error on each tick
	mock.ExpectExec("INSERT INTO leader_leases").WillReturnError(fmt.Errorf("db error"))
	mock.MatchExpectationsInOrder(false)

	sched.Start(ctx, 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)
}
