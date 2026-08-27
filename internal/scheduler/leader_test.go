package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
)

func TestLeaderElectorAcquireSuccess(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)

	mock.ExpectExec("INSERT INTO leader_leases").
		WithArgs("node1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	isLeader, err := elector.TryAcquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLeader {
		t.Errorf("expected to be leader")
	}
	if !elector.IsLeader() {
		t.Errorf("expected IsLeader() to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestLeaderElectorAcquireFail(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)

	mock.ExpectExec("INSERT INTO leader_leases").
		WithArgs("node1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	// It also queries for the current leader
	mock.ExpectQuery("SELECT leader_id FROM leader_leases").
		WillReturnRows(sqlmock.NewRows([]string{"leader_id"}).AddRow("node2"))

	isLeader, err := elector.TryAcquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isLeader {
		t.Errorf("expected not to be leader")
	}
}

func TestLeaderElectorAcquireError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)

	mock.ExpectExec("INSERT INTO leader_leases").
		WithArgs("node1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(fmt.Errorf("db error"))

	isLeader, err := elector.TryAcquire(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if isLeader {
		t.Errorf("expected not to be leader")
	}
}

func TestLeaderElectorRelease(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)

	mock.ExpectExec("DELETE FROM leader_leases").
		WithArgs("node1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = elector.Release(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLeaderElectorCurrentLeader(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	d := &db.DB{DB: mockDB}
	elector := NewLeaderElector(d, "node1", time.Second)

	// Test 1: Leader exists and is valid
	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnRows(sqlmock.NewRows([]string{"leader_id", "expires_at"}).AddRow("node2", time.Now().Add(1*time.Minute)))

	leader, err := elector.CurrentLeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if leader != "node2" {
		t.Errorf("expected node2, got %s", leader)
	}

	// Test 2: No rows
	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnError(sql.ErrNoRows)

	leader, err = elector.CurrentLeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if leader != "" {
		t.Errorf("expected empty leader")
	}

	// Test 3: Expired lease
	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnRows(sqlmock.NewRows([]string{"leader_id", "expires_at"}).AddRow("node2", time.Now().Add(-1*time.Minute)))

	leader, err = elector.CurrentLeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if leader != "" {
		t.Errorf("expected empty leader because lease expired")
	}

	// Test 4: General error
	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnError(fmt.Errorf("db error"))

	_, err = elector.CurrentLeader(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}
