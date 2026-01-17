.PHONY: all build run test lint format clean deps init-storage docker-build docker-build-dev docker-run docker-run-dev docker-up docker-down docker-logs test-cover cli

# Build the project
build:
	go build -o vibecheck .

# Run the MCP server
run: build
	./vibecheck mcp-server

# Run tests (CI-friendly, quiet mode)
test:
	go test ./... -v

# Run tests with coverage
test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

# Lint code
lint:
	golangci-lint run ./...

# Format code (check only, CI-friendly)
format:
	go fmt ./...
	go mod tidy
	@git diff --exit-code || echo "Run 'make format' to fix formatting issues"

# Clean build artifacts
clean:
	rm -f vibecheck
	rm -rf tmp/
	go clean

# Download dependencies
deps:
	go mod tidy
	go mod download

# Initialize storage directories
init-storage:
	mkdir -p storage/cv storage/jd
	@echo "Storage directories created: storage/cv, storage/jd"

# Install development tools
install-tools:
	go get -tool github.com/air-verse/air@latest

# Run with hot reload (requires air)
dev: install-tools
	go tool air mcp-server

# Run the CLI
cli: build
	./vibecheck --help

# Docker: Build production image (CI-friendly)
docker-build:
	docker compose -f compose.yaml build

# Docker: Build development image
docker-build-dev:
	docker compose -f compose.yaml -f compose.dev.yaml build

# Docker: Run the MCP server (production)
docker-run:
	docker compose -f compose.yaml up

# Docker: Run development server
docker-run-dev:
	docker compose -f compose.yaml -f compose.dev.yaml up

# Docker: Run in background
docker-up:
	docker compose -f compose.dev.yaml up -d

# Docker: Stop containers
docker-down:
	docker compose -f compose.dev.yaml down

# Docker: View logs
docker-logs:
	docker compose -f compose.dev.yaml logs -f
