# ADR-003: Storage Backend

## Status: Accepted

## Context
We need a persistent storage system to maintain job definitions, execution history, and handle distributed locking.

## Decision
We chose **PostgreSQL**.

## Alternatives Considered
| Option | Pros | Cons | Why rejected |
|---|---|---|---|
| MongoDB | Flexible schema, easy to scale | Weak transactional guarantees historically, no native advisory locks | We require strict ACID guarantees for idempotency |
| Redis | Fast, natively supports locking | In-memory limits scale, persistence is eventual | Risk of data loss during crashes |
| PostgreSQL | Strong ACID, native advisory locks, robust constraints | Requires schema migrations, vertical scaling limits | **Selected** due to reliability and native lock support |

## Consequences
- Positive: We can use a single datastore for both state and locking, drastically simplifying the architecture.
- Negative: Scaling horizontally for massive write loads is difficult compared to NoSQL.
- Trade-offs accepted: We prioritize correctness and simplicity over infinite horizontal write scalability.
