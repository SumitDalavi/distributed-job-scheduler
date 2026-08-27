package models

import "time"

// JobStatus represents the lifecycle state of a job execution.
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusSucceeded JobStatus = "succeeded"
	StatusFailed    JobStatus = "failed"
)

// Job is a registered cron job stored in Postgres.
type Job struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CronExpr  string     `json:"cron_expr"`
	Payload   string     `json:"payload"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
}

// ExecutionLog records a single execution attempt for a job.
type ExecutionLog struct {
	ID             string     `json:"id"`
	JobID          string     `json:"job_id"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         JobStatus  `json:"status"`
	Output         string     `json:"output,omitempty"`
	Error          string     `json:"error,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	ExecutorNode   string     `json:"executor_node"`
}

// LeaderLease represents the distributed leader election lease.
type LeaderLease struct {
	LeaderID   string    `json:"leader_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
