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
