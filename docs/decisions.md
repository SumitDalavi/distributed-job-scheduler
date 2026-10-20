# Decisions

## ADR-001: Postgres Advisory Locks vs Redis
**Date:** 2026-08-29  
**Status:** Accepted

**Context:**  
We need a distributed locking mechanism to ensure a scheduled job is executed by exactly one worker in a cluster.

**Decision:**  
We chose Postgres `pg_advisory_xact_lock` over Redis. 

**Consequences:**  
- ✅ Reduces infrastructure complexity (no Redis required).
- ✅ Transactions are atomic: lock and job status update occur in the same transaction.
- ⚠️ Potential contention on the database if the job frequency is extremely high (1000s per second).

## ADR-002: Progressive Credential Loading (PCL) Architecture
- **Context:** Production hardening required live backends (PostgreSQL, Redis, OpenAI, Anthropic), but removing mocks would break CI pipelines and create high developer onboarding friction.
- **Decision:** Implement Progressive Credential Loading. Keep in-memory and local mock heuristics as the default safe baseline, while wrapping external integrations in auto-promoting factory adapters triggered by environment variables.
- **Consequences:** 
  - *Pros:* 100% CI pass rate without requiring external secrets; zero setup friction for new contributors; progressive upgrade to production clusters.
  - *Cons:* Adds lightweight factory abstraction layers and runtime credential detection logic.
