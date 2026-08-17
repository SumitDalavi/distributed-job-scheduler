# distributed-job-scheduler Architecture

## System Diagram
The following Mermaid.js sequence diagram maps the core workflow and interactions:

```mermaid
sequenceDiagram
    Worker1->>Postgres: Acquire Advisory Lock
Worker1->>DB: Fetch pending jobs
Worker1->>Worker1: Execute Job
Worker1->>DB: Mark complete
```

## Component Breakdown
- **Core Technology**: Go, Postgres
- **Design Paradigm**: Emphasizes high availability, fault tolerance, and security.

## Security & Scaling Considerations
- Strict boundary validations.
- Horizontal scalability achieved via stateless workers.
- Encrypted data at rest and in transit.
