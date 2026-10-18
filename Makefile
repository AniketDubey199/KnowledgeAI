.PHONY: all build test run-api run-worker docker-up docker-down docker-logs migrate-up migrate-down clean pdf

all: build test

build:
	@mkdir -p bin
	@echo "Building API server..."
	go build -o bin/api ./cmd/api
	@echo "Building Worker process..."
	go build -o bin/worker ./cmd/worker
	@echo "Building Migration runner..."
	go build -o bin/migrate ./cmd/migrate
	@echo "Build complete."

test:
	@echo "Running test suite..."
	go test -v -race ./tests/...

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

migrate-up: build
	./bin/migrate -path ./migrations up

migrate-down: build
	./bin/migrate -path ./migrations down

pdf:
	python3 docs/generate_guide_pdf.py

clean:
	rm -rf bin/ tmp/ coverage.out
