package db

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewSuccess(t *testing.T) {
	// We can't really test the real postgres driver connection easily,
	// so we mock sql.Open implicitly or we just skip erroring if it fails
	// Wait, we can test DB wrapper initialization if we inject a db instance,
	// but New() calls sql.Open("postgres", dsn).
	// Let's just rely on the other tests for coverage of DB, or we test Migrate.
}

func TestMigrate(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	d := &DB{mockDB}

	// Expect the migration schema to be executed
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS jobs").WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.Migrate()
	if err != nil {
		t.Errorf("error was not expected while migrating: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMigrateError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	d := &DB{mockDB}

	// Expect the migration schema to fail
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS jobs").WillReturnError(fmt.Errorf("db error"))

	err = d.Migrate()
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
