# ADR-002: Execution Semantics

## Status: Accepted

## Context
When scheduling and executing background jobs, we must decide between at-most-once, at-least-once, and exactly-once execution semantics.

## Decision
We chose **Exactly-once semantics** (achieved via unique constraint idempotency keys).

## Alternatives Considered
| Option | Pros | Cons | Why rejected |
|---|---|---|---|
| At-most-once | Very simple to implement (fire and forget) | High likelihood of dropped jobs during worker crashes | Unacceptable for critical payment/state jobs |
| At-least-once | Guarantees execution, easy to retry | Can result in duplicate executions | Pushes the burden of idempotency entirely to the downstream consumer |
| Exactly-once via Idempotency Keys | Guarantees safe retries, no duplicates | Requires persistent storage for execution history and unique constraints | **Selected** as it provides the safest abstraction for users |

## Consequences
- Positive: Developers writing jobs do not need to worry about duplicate executions if the worker crashes mid-job.
- Negative: Requires writing an execution record to Postgres before triggering the downstream API. Increased latency per job.
- Trade-offs accepted: We accept higher latency per job for the guarantee of correctness.

## References
- Stripe's Guide to Idempotency
