package scheduler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Scheduler drives the distributed job execution loop.
// It uses LeaderElector so only one node in a cluster executes jobs.
type Scheduler struct {
	db       *db.DB
	electer  *LeaderElector
	nodeID   string
	cron     *cron.Cron
	entryIDs map[string]cron.EntryID
	mu       sync.Mutex
}

// New creates a Scheduler backed by the given database and elector.
func New(database *db.DB, elector *LeaderElector, nodeID string) *Scheduler {
	return &Scheduler{
		db:       database,
		electer:  elector,
		nodeID:   nodeID,
		cron:     cron.New(cron.WithSeconds()),
		entryIDs: make(map[string]cron.EntryID),
	}
}

// Start begins the leadership heartbeat loop and cron engine.
func (s *Scheduler) Start(ctx context.Context, leaseTTL time.Duration) {
	s.cron.Start()
	go s.leaderLoop(ctx, leaseTTL/2) // heartbeat at half the TTL
}

// Stop gracefully shuts down the cron engine and releases the lease.
func (s *Scheduler) Stop(ctx context.Context) {
	s.cron.Stop()
	if err := s.electer.Release(ctx); err != nil {
		log.Printf("[scheduler] warn: failed to release leader lease: %v", err)
	}
	log.Println("[scheduler] stopped")
}

// leaderLoop periodically tries to acquire the leader lease and re-syncs jobs.
func (s *Scheduler) leaderLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			wasLeader := s.electer.IsLeader()
			isLeader, err := s.electer.TryAcquire(ctx)
			if err != nil {
				log.Printf("[scheduler] election error: %v", err)
				continue
			}
			// Re-sync job schedule when leadership changes
			if isLeader != wasLeader || isLeader {
				if err := s.syncJobs(ctx); err != nil {
					log.Printf("[scheduler] job sync error: %v", err)
				}
			}
		}
	}
}

// syncJobs loads all enabled jobs from DB and registers them in the cron engine.
// Jobs already registered are skipped; removed/disabled jobs are unregistered.
func (s *Scheduler) syncJobs(ctx context.Context) error {
	if !s.electer.IsLeader() {
		// Clear all cron entries if we're not the leader
		s.mu.Lock()
		for id, entryID := range s.entryIDs {
			s.cron.Remove(entryID)
			delete(s.entryIDs, id)
		}
		s.mu.Unlock()
		return nil
	}

	jobs, err := s.listJobs(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	active := make(map[string]bool)
	for _, job := range jobs {
		active[job.ID] = true
		if _, exists := s.entryIDs[job.ID]; !exists {
			j := job // capture loop variable
			entryID, err := s.cron.AddFunc(j.CronExpr, func() {
				s.executeJob(context.Background(), j)
			})
			if err != nil {
				log.Printf("[scheduler] invalid cron for job %s (%s): %v", j.Name, j.CronExpr, err)
				continue
			}
			s.entryIDs[j.ID] = entryID
			log.Printf("[scheduler] registered job %s (%s)", j.Name, j.CronExpr)
		}
	}

	// Unregister jobs that are no longer active/enabled
	for id, entryID := range s.entryIDs {
		if !active[id] {
			s.cron.Remove(entryID)
			delete(s.entryIDs, id)
			log.Printf("[scheduler] unregistered job %s", id)
		}
	}
	return nil
}

// executeJob runs a single job with idempotency guarantees and execution logging.
func (s *Scheduler) executeJob(ctx context.Context, job models.Job) {
	// Build a deterministic idempotency key from (job_id, scheduled_minute)
	tick := time.Now().UTC().Truncate(time.Minute)
	raw := fmt.Sprintf("%s:%d", job.ID, tick.Unix())
	hash := sha256.Sum256([]byte(raw))
	idempKey := fmt.Sprintf("%x", hash[:8])

	// Atomically claim execution via INSERT ... ON CONFLICT DO NOTHING
	var execID string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO execution_logs (id, job_id, idempotency_key, status, executor_node)
		VALUES ($1, $2, $3, 'running', $4)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`, uuid.New().String(), job.ID, idempKey, s.nodeID).Scan(&execID)

	if err != nil || execID == "" {
		// Another node already claimed this execution tick — skip
		log.Printf("[scheduler] job %s already claimed for tick %s, skipping", job.Name, tick)
		return
	}

	log.Printf("[scheduler] executing job %s (exec=%s)", job.Name, execID)
	start := time.Now()

	// ── Actual job work ──────────────────────────────────────────────────────
	// In a real system this dispatches to a handler registry by job name/type.
	// Here we simulate work with a sleep proportional to payload length.
	output, jobErr := simulateWork(job)
	// ─────────────────────────────────────────────────────────────────────────

	finished := time.Now()
	status := models.StatusSucceeded
	errorMsg := ""
	if jobErr != nil {
		status = models.StatusFailed
		errorMsg = jobErr.Error()
	}

	_, updateErr := s.db.ExecContext(ctx, `
		UPDATE execution_logs
		SET status=$1, output=$2, error=$3, finished_at=$4
		WHERE id=$5
	`, status, output, errorMsg, finished, execID)
	if updateErr != nil {
		log.Printf("[scheduler] failed to update exec log %s: %v", execID, updateErr)
	}

	// Update the job's last_run_at
	_, _ = s.db.ExecContext(ctx, `UPDATE jobs SET last_run_at=$1, updated_at=NOW() WHERE id=$2`, start, job.ID)

	log.Printf("[scheduler] job %s finished in %s (status=%s)", job.Name, time.Since(start), status)
}

// simulateWork is a placeholder for real job execution dispatch.
// Replace this with a handler registry keyed on job.Name.
func simulateWork(job models.Job) (string, error) {
	time.Sleep(50 * time.Millisecond) // simulate processing
	return fmt.Sprintf("job '%s' executed with payload: %s", job.Name, job.Payload), nil
}

// listJobs fetches all enabled jobs from Postgres.
func (s *Scheduler) listJobs(ctx context.Context) ([]models.Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, cron_expr, payload, enabled, created_at, updated_at, last_run_at, next_run_at
		FROM jobs WHERE enabled = TRUE ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(&j.ID, &j.Name, &j.CronExpr, &j.Payload, &j.Enabled,
			&j.CreatedAt, &j.UpdatedAt, &j.LastRunAt, &j.NextRunAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
