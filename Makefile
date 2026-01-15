.PHONY: all build run test clean deps init-storage docker-build docker-run docker-logs

# Build the project
build:
	go build -o vibecheck .

# Run the MCP server
run: build
	./vibecheck mcp-server

# Run tests
test:
	go test ./... -v

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

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run ./...

# Run the CLI
cli: build
	./vibecheck --help

# Docker: Build the image
docker-build:
	docker compose -f compose.dev.yaml build

# Docker: Run the MCP server
docker-run:
	docker compose -f compose.dev.yaml up

# Docker: Run in background
docker-up:
	docker compose -f compose.dev.yaml up -d

# Docker: Stop containers
docker-down:
	docker compose -f compose.dev.yaml down

# Docker: View logs
docker-logs:
	docker compose -f compose.dev.yaml logs -f
