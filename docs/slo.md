# Service Level Objectives

## Availability SLO
- Target: 99.9% over 30-day rolling window
- Error budget: 43.8 minutes/month
- Measurement: health endpoint response rate measured externally

## Latency SLO  
- P50 < 50ms for job execution triggering
- P99 < 150ms for job execution triggering
- Measurement: histogram from internal telemetry

## Correctness SLO
- Zero duplicate job executions per scheduled tick
- Measurement: verification against the unique idempotency keys in the database execution log

## How to Measure
```bash
make slo-report  # generates 30-day SLO report from local metrics (future implementation)
```
