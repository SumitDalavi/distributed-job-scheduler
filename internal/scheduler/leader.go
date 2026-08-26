package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
)

// LeaderElector manages distributed leader election using Postgres advisory locks.
// Only the elected leader runs the cron tick loop, preventing duplicate executions
// across multiple scheduler instances.
type LeaderElector struct {
	db       *db.DB
	nodeID   string
	leaseTTL time.Duration
	isLeader bool
}

// NewLeaderElector creates a new elector for the given node.
func NewLeaderElector(database *db.DB, nodeID string, leaseTTL time.Duration) *LeaderElector {
	return &LeaderElector{
		db:       database,
		nodeID:   nodeID,
		leaseTTL: leaseTTL,
	}
}

// TryAcquire attempts to claim or renew the leader lease using an UPSERT with
// a conditional check on expiry. Returns true if this node is now the leader.
func (le *LeaderElector) TryAcquire(ctx context.Context) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(le.leaseTTL)

	// Attempt to INSERT a new lease (first startup) or UPDATE our own lease.
	// If another node holds a valid (non-expired) lease, this is a no-op.
	query := `
		INSERT INTO leader_leases (id, leader_id, acquired_at, expires_at)
		VALUES ('singleton', $1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		  SET leader_id   = EXCLUDED.leader_id,
		      acquired_at = EXCLUDED.acquired_at,
		      expires_at  = EXCLUDED.expires_at
		WHERE leader_leases.expires_at < NOW()   -- lease expired: steal it
		   OR leader_leases.leader_id = $1;      -- we already own it: renew
	`
	res, err := le.db.ExecContext(ctx, query, le.nodeID, now, expiresAt)
	if err != nil {
		return false, fmt.Errorf("leader election upsert: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	le.isLeader = rowsAffected > 0

	if le.isLeader {
		log.Printf("[leader] node %s is leader (lease until %s)", le.nodeID, expiresAt.Format(time.RFC3339))
	} else {
		// Check who the current leader is for logging
		var currentLeader string
		_ = le.db.QueryRowContext(ctx, `SELECT leader_id FROM leader_leases WHERE id = 'singleton'`).Scan(&currentLeader)
		log.Printf("[leader] node %s is follower (leader: %s)", le.nodeID, currentLeader)
	}
	return le.isLeader, nil
}

// Release surrenders the leader lease, allowing another node to take over.
func (le *LeaderElector) Release(ctx context.Context) error {
	_, err := le.db.ExecContext(ctx,
		`DELETE FROM leader_leases WHERE id = 'singleton' AND leader_id = $1`, le.nodeID)
	return err
}

// IsLeader returns whether this node currently holds the leader lease.
func (le *LeaderElector) IsLeader() bool {
	return le.isLeader
}

// CurrentLeader returns the node ID of the current leader from the DB.
func (le *LeaderElector) CurrentLeader(ctx context.Context) (string, error) {
	var leaderID string
	var expiresAt time.Time
	err := le.db.QueryRowContext(ctx,
		`SELECT leader_id, expires_at FROM leader_leases WHERE id = 'singleton'`,
	).Scan(&leaderID, &expiresAt)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", nil // lease expired, no active leader
	}
	return leaderID, nil
}
