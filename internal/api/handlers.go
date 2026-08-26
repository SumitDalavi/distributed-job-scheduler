package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/models"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/scheduler"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Handler struct {
	db      *db.DB
	electer *scheduler.LeaderElector
	nodeID  string
}

func New(database *db.DB, elector *scheduler.LeaderElector, nodeID string) *Handler {
	return &Handler{db: database, electer: elector, nodeID: nodeID}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", h.Health).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/leader", h.GetLeader).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/jobs", h.ListJobs).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/jobs", h.CreateJob).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/jobs/{id}", h.GetJob).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/jobs/{id}", h.DeleteJob).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/jobs/{id}/enable", h.EnableJob).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/jobs/{id}/disable", h.DisableJob).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/jobs/{id}/logs", h.GetJobLogs).Methods(http.MethodGet)
}

// Health returns the node's health and current leadership status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	leader, _ := h.electer.CurrentLeader(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"node_id":   h.nodeID,
		"is_leader": h.electer.IsLeader(),
		"leader":    leader,
		"time":      time.Now().UTC(),
	})
}

// GetLeader returns the current cluster leader.
func (h *Handler) GetLeader(w http.ResponseWriter, r *http.Request) {
	leader, err := h.electer.CurrentLeader(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query leader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"leader": leader})
}

// ListJobs returns all registered jobs.
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, cron_expr, payload, enabled, created_at, updated_at, last_run_at, next_run_at
		FROM jobs ORDER BY created_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		rows.Scan(&j.ID, &j.Name, &j.CronExpr, &j.Payload, &j.Enabled,
			&j.CreatedAt, &j.UpdatedAt, &j.LastRunAt, &j.NextRunAt)
		jobs = append(jobs, j)
	}
	writeJSON(w, http.StatusOK, jobs)
}

// CreateJob registers a new cron job.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		CronExpr string `json:"cron_expr"`
		Payload  string `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.CronExpr == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expr are required")
		return
	}
	if req.Payload == "" {
		req.Payload = "{}"
	}

	var job models.Job
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO jobs (id, name, cron_expr, payload) VALUES ($1, $2, $3, $4)
		RETURNING id, name, cron_expr, payload, enabled, created_at, updated_at, last_run_at, next_run_at
	`, uuid.New().String(), req.Name, req.CronExpr, req.Payload,
	).Scan(&job.ID, &job.Name, &job.CronExpr, &job.Payload, &job.Enabled,
		&job.CreatedAt, &job.UpdatedAt, &job.LastRunAt, &job.NextRunAt)

	if err != nil {
		log.Printf("[api] create job error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

// GetJob returns a single job by ID.
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var job models.Job
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, cron_expr, payload, enabled, created_at, updated_at, last_run_at, next_run_at
		FROM jobs WHERE id=$1
	`, id).Scan(&job.ID, &job.Name, &job.CronExpr, &job.Payload, &job.Enabled,
		&job.CreatedAt, &job.UpdatedAt, &job.LastRunAt, &job.NextRunAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// DeleteJob removes a job and its execution history.
func (h *Handler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := h.db.ExecContext(r.Context(), `DELETE FROM jobs WHERE id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EnableJob re-enables a disabled job.
func (h *Handler) EnableJob(w http.ResponseWriter, r *http.Request) {
	h.setJobEnabled(w, r, true)
}

// DisableJob pauses job execution without deleting it.
func (h *Handler) DisableJob(w http.ResponseWriter, r *http.Request) {
	h.setJobEnabled(w, r, false)
}

func (h *Handler) setJobEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id := mux.Vars(r)["id"]
	_, err := h.db.ExecContext(r.Context(),
		`UPDATE jobs SET enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// GetJobLogs returns the execution history for a specific job.
func (h *Handler) GetJobLogs(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, job_id, idempotency_key, status, output, error, started_at, finished_at, executor_node
		FROM execution_logs WHERE job_id=$1 ORDER BY started_at DESC LIMIT 50
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var logs []models.ExecutionLog
	for rows.Next() {
		var l models.ExecutionLog
		rows.Scan(&l.ID, &l.JobID, &l.IdempotencyKey, &l.Status,
			&l.Output, &l.Error, &l.StartedAt, &l.FinishedAt, &l.ExecutorNode)
		logs = append(logs, l)
	}
	writeJSON(w, http.StatusOK, logs)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[api] encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
