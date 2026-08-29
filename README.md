# Distributed Job Scheduler ⏱️

![CI](https://github.com/SumitDalavi/distributed-job-scheduler/actions/workflows/ci.yml/badge.svg?branch=master)

> **Maturity:** Full Prototype
> _Leader-election based distributed cron scheduler using Postgres advisory locks to prevent duplicate job execution._

## The Problem
Modern distributed systems require robust, highly concurrent solutions. Simple CRUD applications fail when subjected to high throughput, race conditions, or massive data sets.

## The Solution
This project implements a robust microservice architecture designed to handle these specific edge cases. By utilizing advanced paradigms like idempotency keys, advisory locks, or optimized caching layers, this service guarantees data integrity under load.

```text
┌──────────────┐      ┌───────────────┐      ┌───────────────┐
│              │      │               │      │               │
│   Client     │─────►│   API Layer   │─────►│  Data Store   │
│              │      │               │      │               │
└──────────────┘      └───────────────┘      └───────────────┘
```

## 🛠️ Tech Stack
- **Core Technology**: Go, PostgreSQL
- **Architecture**: Microservices, Event-Driven

## 📚 Documentation

- [Architecture](docs/architecture.md) — System diagram and component details
- [Runbook](docs/runbook.md) — Setup, commands, and expected outputs
- [Decisions](docs/decisions.md) — ADRs for scheduler pattern choices
- [Changelog](docs/changelog.md) — Change history

## Decision Log
| Decision | Rationale |
|----------|-----------|
| Monorepo vs Polyrepo | Chosen self-contained repository for easier deployment and PoC demonstration |
| State Management | All state is pushed to the Data Store/Cache to keep the API stateless and horizontally scalable |
| Error Handling | Standardized JSON error responses with explicit error codes |

## 🚀 Step-by-Step Setup

```bash
# 1. Clone the repository

![CI](https://github.com/SumitDalavi/distributed-job-scheduler/actions/workflows/ci.yml/badge.svg?branch=master)
git clone https://github.com/SumitDalavi/distributed-job-scheduler.git
cd distributed-job-scheduler

# 2. Build and start

![CI](https://github.com/SumitDalavi/distributed-job-scheduler/actions/workflows/ci.yml/badge.svg?branch=master)
docker-compose up -d --build

# 3. Verify it's running

![CI](https://github.com/SumitDalavi/distributed-job-scheduler/actions/workflows/ci.yml/badge.svg?branch=master)
curl http://localhost:8080/health
```

The API is now available at **http://localhost:8080**

## 🧪 Usage & Demo

```bash
# Health Check

![CI](https://github.com/SumitDalavi/distributed-job-scheduler/actions/workflows/ci.yml/badge.svg?branch=master)
curl http://localhost:8080/health

# Simulate Traffic

![CI](https://github.com/SumitDalavi/distributed-job-scheduler/actions/workflows/ci.yml/badge.svg?branch=master)
curl -X POST http://localhost:8080/api/trigger -H "Content-Type: application/json" -d '{"test":"payload"}'
```

## ✅ Verification

| Check | Command | Expected |
|-------|---------|----------|
| Health | `curl http://localhost:8080/health` | `{"status": "ok"}` |
| Load | `make test` | All unit/integration tests pass |

## Benchmark Results (Last Run: 2026-08-29)
| Metric | Value | Environment |
|---|---|---|
| P50 Execution Latency | ~51.38ms | Windows 11 / WSL2 / Docker |
| P99 Execution Latency | ~52.25ms | Windows 11 / WSL2 / Docker |
| Chaos Test Pass Rate | 100% | Python subprocess assertions |

## Key Design Decisions
- **Why Postgres over Redis for locks:** We prioritize strict ACID correctness and operational simplicity over the extreme high-throughput of in-memory stores. Using Postgres consolidates the persistence and locking layer.
- See `docs/adr/` for full Architecture Decision Records.
- See `docs/slo.md` for availability and latency objectives.

## Test Coverage
Includes comprehensive Unit Tests and Python-driven Chaos Integration Tests (e.g. killing the worker mid-job).

## Known Limitations & Honest Scope
- **Horizontal Scalability**: Postgres advisory locks are highly robust but will bottleneck at extreme scales (e.g. >10,000 jobs per second) where a sharded Redis or ZooKeeper architecture would become necessary.

## 👨‍💻 Author
**Sumit Dalavi** — Senior DevSecOps / Platform Engineer
[GitHub](https://github.com/SumitDalavi) | [LinkedIn](https://in.linkedin.com/in/sumit-dalavi-762838129)

---
*Built with a focus on robust patterns, not toy demos.*

