package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// DB wraps the standard sql.DB with helper methods.
type DB struct {
	*sql.DB
}

// New opens a Postgres connection and verifies it with a ping.
func New(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	if err = sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	log.Println("[db] connected to postgres")
	return &DB{sqlDB}, nil
}

// Migrate runs all SQL migrations.
func (db *DB) Migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name           TEXT NOT NULL UNIQUE,
		cron_expr      TEXT NOT NULL,
		payload        TEXT NOT NULL DEFAULT '{}',
		enabled        BOOLEAN NOT NULL DEFAULT TRUE,
		created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_run_at    TIMESTAMPTZ,
		next_run_at    TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS execution_logs (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
		idempotency_key TEXT NOT NULL UNIQUE,
		status          TEXT NOT NULL DEFAULT 'pending',
		output          TEXT,
		error           TEXT,
		started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		finished_at     TIMESTAMPTZ,
		executor_node   TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS leader_leases (
		id          TEXT PRIMARY KEY DEFAULT 'singleton',
		leader_id   TEXT NOT NULL,
		acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		expires_at  TIMESTAMPTZ NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_exec_logs_job_id ON execution_logs(job_id);
	CREATE INDEX IF NOT EXISTS idx_exec_logs_status ON execution_logs(status);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Println("[db] migrations applied")
	return nil
}
