.PHONY: build run test docker-up docker-down demo

build:
	go build -o scheduler ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down -v

# Demo: register a job and watch it execute
demo:
	@echo "Registering a job that runs every 10 seconds..."
	curl -s -X POST http://localhost:8080/api/v1/jobs \
	  -H "Content-Type: application/json" \
	  -d '{"name":"demo-job","cron_expr":"*/10 * * * * *","payload":{"task":"hello"}}' | python -m json.tool
	@echo ""
	@echo "Listing jobs:"
	curl -s http://localhost:8080/api/v1/jobs | python -m json.tool
	@echo ""
	@echo "Checking leader:"
	curl -s http://localhost:8080/api/v1/leader | python -m json.tool
