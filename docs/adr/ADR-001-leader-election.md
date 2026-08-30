# ADR-001: Leader Election Algorithm

## Status: Accepted

## Context
In a distributed cron scheduler, we must ensure that only a single node triggers the scheduled jobs at any given minute to prevent widespread duplicate executions. We evaluated Redis distributed locks (Redlock), ZooKeeper, and Postgres advisory locks.

## Decision
We chose **Postgres advisory locks**.

## Alternatives Considered
| Option | Pros | Cons | Why rejected |
|---|---|---|---|
| Redis (Redlock) | Fast, in-memory | Requires managing an additional stateful system | Added operational complexity for limited benefit |
| ZooKeeper | Industry standard for consensus | Very heavy, requires JVM, steep learning curve | Overkill for a simple scheduler |
| Postgres Advisory Locks | Zero additional infrastructure, strict consistency | Tied to DB connection health | **Selected** as we already use Postgres for job state |

## Consequences
- Positive: No additional infrastructure to deploy or monitor. Extremely reliable strict consistency.
- Negative: If the primary database goes down, scheduling stops entirely. Connection pooling must be carefully managed to avoid dropping the session that holds the lock.
- Trade-offs accepted: We accept database-tied availability in exchange for operational simplicity and strict consistency.

## References
- PostgreSQL Advisory Locks Documentation
