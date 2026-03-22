.PHONY: build test run clean lint format help

# Default target
all: build

# Build the application
build:
	@echo "🏗️ Building niceboy..."
	go build -ldflags="-X main.version=v1.5.0 -X main.commit=$$(git rev-parse --short HEAD)" -o niceboy cmd/niceboy/main.go

# Run tests
test:
	@echo "🧪 Running unit tests..."
	go test ./...

# Run the application
run:
	@echo "⚡ Starting niceboy..."
	go run cmd/niceboy/main.go

# Format the code
format:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Check for lint errors (requires golangci-lint)
lint:
	@echo "🔍 Linting..."
	golangci-lint run ./... || echo "Install golangci-lint for full checks"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -f niceboy
	rm -f *.log
	rm -f audit_log_*.txt

# Show help
help:
	@echo "niceboy Makefile targets:"
	@echo "  build   - Build the binary"
	@echo "  test    - Run tests"
	@echo "  run     - Run locally"
	@echo "  format  - Format code"
	@echo "  lint    - Run linter"
	@echo "  clean   - Remove binaries and logs"
