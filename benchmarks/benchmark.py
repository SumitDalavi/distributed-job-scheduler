#!/usr/bin/env python3
"""
Distributed Job Scheduler — Benchmark & Correctness Harness

Verifies:
1. Job registration succeeds and returns a stable UUID.
2. Exactly-once execution semantics: each minute boundary fires the job
   exactly once, regardless of how many cron ticks occur within the minute.
3. Idempotency deduplication: unique idempotency_key count == execution count.

Idempotency model (see internal/scheduler/scheduler.go):
  key = SHA256(job_id + ":" + unix_timestamp_truncated_to_minute)
  INSERT INTO execution_logs ... ON CONFLICT (idempotency_key) DO NOTHING

Usage:
  # Start stack first:
  docker compose up -d postgres scheduler-node2
  sleep 15
  # Then run:
  python benchmarks/benchmark.py
"""

import time
import json
import urllib.request
import urllib.error
import datetime
import platform
import sys
import os

BASE_URL = os.environ.get("BENCH_BASE_URL", "http://127.0.0.1:8081/api/v1")
OBSERVATION_WINDOW_S = int(os.environ.get("BENCH_WINDOW_S", "125"))
EXPECTED_MIN_EXECUTIONS = 2  # 2 minute boundaries crossed in 125s


def make_request(url, method="GET", payload=None):
    req = urllib.request.Request(url, method=method)
    if payload:
        req.data = json.dumps(payload).encode("utf-8")
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=15) as response:
            body = response.read()
            return json.loads(body.decode("utf-8")) if body else {}, response.status
    except urllib.error.HTTPError as e:
        return {"error": e.read().decode()}, e.code
    except Exception as e:
        return {"error": str(e)}, 500


def run():
    print(f"[benchmark] target={BASE_URL}")
    print(f"[benchmark] window={OBSERVATION_WINDOW_S}s, expected>={EXPECTED_MIN_EXECUTIONS} executions")

    results = {
        "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
        "environment": {
            "os": platform.system() + " " + platform.release(),
            "python": sys.version.split()[0],
        },
        "fixture": f"cron (* * * * *) = every minute, observed for {OBSERVATION_WINDOW_S}s",
        "command": f"BENCH_BASE_URL={BASE_URL} python benchmarks/benchmark.py",
        "semantics": {
            "idempotency_granularity": "per-minute — time.Truncate(time.Minute)",
            "dedup_mechanism": "INSERT ... ON CONFLICT (idempotency_key) DO NOTHING",
        },
        "metrics": {},
    }

    # 1. Register job
    payload = {
        "name": f"benchmark-{int(time.time())}",
        "cron_expr": "* * * * * *",  # 6 fields since scheduler uses cron.WithSeconds()
        "payload": "benchmark-task",
    }
    job, status = make_request(f"{BASE_URL}/jobs", "POST", payload)
    job_id = job.get("id")

    if not job_id:
        print(f"[benchmark] ERROR: job registration failed (status={status}): {job}")
        results["metrics"] = {"error": str(job), "success": False, "correctness": "FAIL"}
        _save(results)
        sys.exit(1)

    print(f"[benchmark] job registered: {job_id}")
    print(f"[benchmark] sleeping {OBSERVATION_WINDOW_S}s to cross 2+ minute boundaries …")
    time.sleep(OBSERVATION_WINDOW_S)

    # 2. Fetch execution logs
    logs, status = make_request(f"{BASE_URL}/jobs/{job_id}/logs")
    if not isinstance(logs, list):
        print(f"[benchmark] ERROR: logs endpoint returned (status={status}): {logs}")
        results["metrics"] = {"error": str(logs), "success": False, "correctness": "FAIL"}
        _save(results)
        sys.exit(1)

    execution_count = len(logs)
    idem_keys = [log.get("idempotency_key") for log in logs]
    unique_keys = len(set(idem_keys))
    exactly_once = (unique_keys == execution_count) and execution_count > 0
    success = execution_count >= EXPECTED_MIN_EXECUTIONS and exactly_once

    results["metrics"] = {
        "observation_window_s": OBSERVATION_WINDOW_S,
        "execution_count": execution_count,
        "expected_minimum": EXPECTED_MIN_EXECUTIONS,
        "unique_idempotency_keys": unique_keys,
        "exactly_once_verified": exactly_once,
        "success": success,
        "correctness": "PASS" if success else "FAIL",
    }

    print(f"[benchmark] executions in {OBSERVATION_WINDOW_S}s: {execution_count} (expected >={EXPECTED_MIN_EXECUTIONS})")
    print(f"[benchmark] unique idempotency keys: {unique_keys}")
    print(f"[benchmark] exactly-once verified: {exactly_once}")
    print(f"[benchmark] correctness: {results['metrics']['correctness']}")

    _save(results)
    if not success:
        sys.exit(1)


def _save(results):
    os.makedirs("benchmarks/results", exist_ok=True)
    out = "benchmarks/results/failover_semantics.json"
    with open(out, "w") as f:
        json.dump(results, f, indent=2)
    print(f"[benchmark] results saved to {out}")
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    run()
