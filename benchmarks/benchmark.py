import time
import json
import urllib.request
import datetime

def make_request(url, method='GET', payload=None):
    req = urllib.request.Request(url, method=method)
    if payload:
        req.data = json.dumps(payload).encode('utf-8')
        req.add_header('Content-Type', 'application/json')
    try:
        with urllib.request.urlopen(req) as response:
            body = response.read()
            return json.loads(body.decode('utf-8')) if body else {}, response.status
    except Exception as e:
        return {"error": str(e)}, 500

print("Starting Distributed Job Scheduler Benchmark...")

results = {
    "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
    "metrics": {}
}

base_url = "http://localhost:8080/api/v1"

# 1. Register a job to run every second
payload = {
    "name": "benchmark-job",
    "cron_expr": "* * * * * *",
    "payload": {"task": "benchmark"}
}
job, status = make_request(f"{base_url}/jobs", 'POST', payload)
job_id = job.get('id')

if job_id:
    print(f"Registered job {job_id}. Waiting 15 seconds for executions...")
    time.sleep(15)

    # 2. Collect logs
    logs, status = make_request(f"{base_url}/jobs/{job_id}/logs")
    execution_count = len(logs) if isinstance(logs, list) else 0

    results["metrics"]["execution_count"] = execution_count
    results["metrics"]["expected_minimum"] = 10
    results["metrics"]["success"] = execution_count >= 10
else:
    results["metrics"]["error"] = "Failed to register job"

output_file = f"benchmarks/results_{int(time.time())}.json"
with open(output_file, "w") as f:
    json.dump(results, f, indent=2)

print(f"Benchmark completed successfully. Results saved to {output_file}")
