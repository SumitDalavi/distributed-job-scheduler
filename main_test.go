package main

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSchedulerRunAcquiresLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Mock acquiring lock successfully
	mock.ExpectQuery("SELECT pg_try_advisory_lock").
		WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	
	// Mock releasing lock
	mock.ExpectExec("SELECT pg_advisory_unlock").
		WillReturnResult(sqlmock.NewResult(1, 1))

	scheduler := NewScheduler(db)
	locked, err := scheduler.Run()

	if err != nil {
		t.Errorf("error was not expected while running scheduler: %s", err)
	}
	if !locked {
		t.Errorf("expected to acquire lock")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSchedulerRunStandby(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Mock acquiring lock fails (another holds it)
	mock.ExpectQuery("SELECT pg_try_advisory_lock").
		WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))

	scheduler := NewScheduler(db)
	locked, err := scheduler.Run()

	if err != nil {
		t.Errorf("error was not expected while running scheduler: %s", err)
	}
	if locked {
		t.Errorf("did not expect to acquire lock")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
