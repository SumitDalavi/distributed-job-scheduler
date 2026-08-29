# Runbook — distributed-job-scheduler
> Last updated: 2026-08-29

## Prerequisites
| Tool | Required Version | How to check |
|---|---|---|
| Go | >= 1.21 | `go version` |
| Docker & Compose | Latest | `docker-compose version` |

## Quick Start
```bash
# Start DB and 3 worker instances
docker-compose up --scale worker=3 -d

# Check health
curl http://localhost:8080/health
```

## Run Tests
```bash
# Unit tests
go test ./... -v

# E2E Failover Test
bash tests/integration/test_failover.sh
```

Expected output:
```
?       github.com/SumitDalavi/distributed-job-scheduler/cmd/api        [no test files]
ok      github.com/SumitDalavi/distributed-job-scheduler/internal/scheduler     0.015s
```

## Environment Variables
| Variable | Default | Purpose |
|---|---|---|
| DB_DSN | `postgres://user:pass@db:5432/scheduler` | Connection string |
| SERVER_PORT | `8080` | HTTP port |

## Common Failure Modes
| Symptom | Cause | Fix |
|---|---|---|
| `failed to acquire lock` (all workers) | Postgres is down or unreachable | Check DB container health `docker-compose logs db` |
| `duplicate job execution` | Missing `pg_advisory_xact_lock` call | Ensure the transaction is committing/rolling back correctly. |
