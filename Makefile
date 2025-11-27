.PHONY: all build test bench run clean fmt lint help validate profile

# Default target
all: fmt lint test build

# Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	@go build -o bin/collector ./cmd/collector
	@echo "✓ Built bin/collector"

# Run all tests
test:
	@echo "Running tests..."
	@go test -v -race ./...
	@echo "✓ All tests passed"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "✓ Linting passed"; \
	else \
		echo "⚠ golangci-lint not installed, skipping..."; \
	fi

# Run the collector service
run:
	@echo "Starting collector..."
	@go run ./cmd/collector

# Run the collector with custom config
run-dev:
	@echo "Starting collector (development mode)..."
	@LOG_LEVEL=debug WORKERS=5 go run ./cmd/collector

# Test the collector manually (requires collector to be running)
test-collector:
	@echo "Running manual collector tests..."
	@./scripts/test-collector.sh

# Run comprehensive validation (100 spans, all features)
validate:
	@echo "Running comprehensive validation..."
	@./scripts/validate-all.sh

# Profile memory usage
profile:
	@echo "Profiling memory usage..."
	@./scripts/profile-memory.sh

# Run load tests (requires collector to be running)
# Installs 'hey' tool automatically, falls back to curl if installation fails
load-test:
	@echo "Running load tests..."
	@./scripts/load-test.sh

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

# Run tests quickly (no race detector)
test-quick:
	@go test ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies installed"

# Verify everything is working
verify: fmt lint test build
	@echo "✓ All checks passed!"

# Help
help:
	@echo "TraceFlow Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build all binaries"
	@echo "  make test           Run all tests with race detector"
	@echo "  make test-coverage  Run tests and generate coverage report"
	@echo "  make bench          Run benchmarks"
	@echo "  make run            Start the collector service"
	@echo "  make clean          Remove build artifacts"
	@echo "  make fmt            Format all Go code"
	@echo "  make lint           Run golangci-lint"
	@echo "  make verify         Run all checks (fmt, lint, test, build)"
	@echo "  make deps           Download and tidy dependencies"
	@echo ""