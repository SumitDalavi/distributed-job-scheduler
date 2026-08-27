package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/db"
	"github.com/SumitDalavi/distributed-job-scheduler/internal/scheduler"
	"github.com/gorilla/mux"
)

func setupTestHandler(t *testing.T) (*Handler, sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}

	d := &db.DB{DB: mockDB}
	elector := scheduler.NewLeaderElector(d, "node-1", time.Second)
	h := New(d, elector, "node-1")

	return h, mock, func() { mockDB.Close() }
}

func TestHealth(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnRows(sqlmock.NewRows([]string{"leader_id", "expires_at"}).AddRow("node-1", time.Now().Add(time.Minute)))

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetLeader(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnRows(sqlmock.NewRows([]string{"leader_id", "expires_at"}).AddRow("node-2", time.Now().Add(time.Minute)))

	req := httptest.NewRequest("GET", "/api/v1/leader", nil)
	rr := httptest.NewRecorder()
	h.GetLeader(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetLeaderError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT leader_id, expires_at FROM leader_leases").
		WillReturnError(fmt.Errorf("db error"))

	req := httptest.NewRequest("GET", "/api/v1/leader", nil)
	rr := httptest.NewRecorder()
	h.GetLeader(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestListJobs(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"}).
		AddRow("id1", "job1", "* * * * *", "{}", true, time.Now(), time.Now(), nil, nil)
	mock.ExpectQuery("SELECT id, name, cron_expr").WillReturnRows(rows)

	req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()
	h.ListJobs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestListJobsError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, name, cron_expr").WillReturnError(fmt.Errorf("db error"))

	req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	rr := httptest.NewRecorder()
	h.ListJobs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestCreateJob(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	body := []byte(`{"name":"test","cron_expr":"* * * * *"}`)
	req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs(sqlmock.AnyArg(), "test", "* * * * *", "{}").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"}).
			AddRow("new-id", "test", "* * * * *", "{}", true, time.Now(), time.Now(), nil, nil))

	h.CreateJob(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}
}

func TestCreateJobInvalid(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer([]byte(`{`)))
	rr := httptest.NewRecorder()
	h.CreateJob(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json")
	}

	req = httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer([]byte(`{"name":""}`)))
	rr = httptest.NewRecorder()
	h.CreateJob(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields")
	}
}

func TestCreateJobError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	body := []byte(`{"name":"test","cron_expr":"* * * * *"}`)
	req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	mock.ExpectQuery("INSERT INTO jobs").WillReturnError(fmt.Errorf("db error"))

	h.CreateJob(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestGetJob(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "name", "cron_expr", "payload", "enabled", "created_at", "updated_at", "last_run_at", "next_run_at"}).
		AddRow("id1", "job1", "* * * * *", "{}", true, time.Now(), time.Now(), nil, nil)
	mock.ExpectQuery("SELECT id, name, cron_expr").WithArgs("1").WillReturnRows(rows)

	req := httptest.NewRequest("GET", "/api/v1/jobs/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.GetJob(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetJobNotFound(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, name, cron_expr").WithArgs("1").WillReturnError(sql.ErrNoRows)

	req := httptest.NewRequest("GET", "/api/v1/jobs/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.GetJob(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGetJobError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, name, cron_expr").WithArgs("1").WillReturnError(fmt.Errorf("db error"))

	req := httptest.NewRequest("GET", "/api/v1/jobs/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.GetJob(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDeleteJob(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectExec("DELETE FROM jobs").WithArgs("1").WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("DELETE", "/api/v1/jobs/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.DeleteJob(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestDeleteJobNotFound(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectExec("DELETE FROM jobs").WithArgs("1").WillReturnResult(sqlmock.NewResult(0, 0))

	req := httptest.NewRequest("DELETE", "/api/v1/jobs/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.DeleteJob(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDeleteJobError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectExec("DELETE FROM jobs").WithArgs("1").WillReturnError(fmt.Errorf("db error"))

	req := httptest.NewRequest("DELETE", "/api/v1/jobs/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.DeleteJob(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestEnableJob(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectExec("UPDATE jobs").WithArgs(true, "1").WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("PUT", "/api/v1/jobs/1/enable", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.EnableJob(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestDisableJob(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectExec("UPDATE jobs").WithArgs(false, "1").WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("PUT", "/api/v1/jobs/1/disable", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.DisableJob(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestSetJobEnabledError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectExec("UPDATE jobs").WithArgs(true, "1").WillReturnError(fmt.Errorf("db error"))

	req := httptest.NewRequest("PUT", "/api/v1/jobs/1/enable", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.EnableJob(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestGetJobLogs(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "job_id", "idempotency_key", "status", "output", "error", "started_at", "finished_at", "executor_node"}).
		AddRow("exec-1", "1", "key1", "success", "out", "", time.Now(), time.Now(), "node-1")
	mock.ExpectQuery("SELECT id, job_id, idempotency_key").WithArgs("1").WillReturnRows(rows)

	req := httptest.NewRequest("GET", "/api/v1/jobs/1/logs", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.GetJobLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetJobLogsError(t *testing.T) {
	h, mock, cleanup := setupTestHandler(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, job_id, idempotency_key").WithArgs("1").WillReturnError(fmt.Errorf("db error"))

	req := httptest.NewRequest("GET", "/api/v1/jobs/1/logs", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rr := httptest.NewRecorder()
	h.GetJobLogs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()

	r := mux.NewRouter()
	h.RegisterRoutes(r)
	if r.GetRoute("health") == nil {
		// Just ensure it doesn't panic
	}
}
