package main

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SumitDalavi/distributed-job-scheduler/config"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
)

func TestRunBadDB(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://invalid:invalid@localhost:1/invalid")
	defer os.Unsetenv("DATABASE_URL")

	if code := run(); code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestRunWithDBMigrationFailure(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	cfg := &config.Config{
		DatabaseURL:     "unused",
		Port:            "19999",
		NodeID:          "test-node",
		LeaseTTLSeconds: 30,
	}

	mock.ExpectExec("CREATE TABLE").WillReturnError(sqlmock.ErrCancelled)

	code := runWithDB(d, cfg)
	if code != 1 {
		t.Errorf("expected exit code 1 for migration failure, got %d", code)
	}
}

func TestRunWithDBSuccess(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	cfg := &config.Config{
		DatabaseURL:     "unused",
		Port:            "19998",
		NodeID:          "test-node",
		LeaseTTLSeconds: 30,
	}

	// Expect migration
	mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock the Release call when scheduler stops
	mock.ExpectExec("DELETE FROM leader_leases").WillReturnResult(sqlmock.NewResult(0, 0))

	// Signal shutdown after a very short time
	go func() {
		time.Sleep(50 * time.Millisecond)
		quit <- syscall.SIGTERM
	}()

	code := runWithDB(d, cfg)
	if code != 0 {
		t.Errorf("expected exit code 0 for successful run, got %d", code)
	}
}
