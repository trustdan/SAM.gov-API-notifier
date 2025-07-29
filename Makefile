.PHONY: build test run lint docker clean help

# Default target
help:
	@echo "SAM.gov Monitor - Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  build           - Build the monitor binary"
	@echo "  dev             - Quick development cycle (build + test + lint)"
	@echo "  run             - Run the monitor locally"
	@echo "  run-dry         - Run in dry-run mode (safe testing)"
	@echo "  validate-env    - Validate environment variables"
	@echo ""
	@echo "Testing:"
	@echo "  test            - Run all unit tests"
	@echo "  test-short      - Run quick tests only"
	@echo "  test-integration- Run integration tests (requires SAM_API_KEY)"
	@echo "  benchmark       - Run performance benchmarks"
	@echo "  coverage        - Generate test coverage report"
	@echo "  lint            - Run code linter"
	@echo ""  
	@echo "Docker:"
	@echo "  docker          - Build Docker image"
	@echo "  docker-up       - Build and run with docker-compose"
	@echo "  docker-dev      - Run development version with docker-compose"
	@echo "  docker-down     - Stop docker-compose services"
	@echo "  docker-logs     - View docker-compose logs"
	@echo "  docker-clean    - Clean up Docker artifacts"
	@echo ""
	@echo "Utilities:"
	@echo "  clean           - Clean build artifacts"
	@echo "  deps            - Install/update dependencies"
	@echo "  release         - Build release binaries for multiple platforms"
	@echo "  pre-push        - Run all pre-push checks"
	@echo "  maintenance     - Run maintenance tasks"

# Build the monitor binary with version info
build:
	@mkdir -p bin
	go build -ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/monitor ./cmd/monitor
	go build -ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/maintenance ./cmd/maintenance

# Run all tests
test:
	go test -v ./...

# Run quick tests only
test-short:
	go test -short -v ./...

# Run integration tests (requires SAM_API_KEY)
test-integration:
	@if [ -z "$(SAM_API_KEY)" ]; then \
		echo "Warning: SAM_API_KEY not set. Integration tests will be skipped."; \
	fi
	go test -v -tags=integration ./test/...

# Run benchmarks
benchmark:
	go test -bench=. -benchmem ./...

# Run the monitor locally
run:
	go run ./cmd/monitor -config config/queries.yaml

# Run with dry-run mode
run-dry:
	go run ./cmd/monitor -config config/queries.yaml -dry-run -v

# Lint the code
lint:
	golangci-lint run

# Build Docker image
docker:
	docker build -t sam-gov-monitor:latest .

# Build and run with docker-compose
docker-up:
	docker-compose up --build -d

# Run development version with docker-compose
docker-dev:
	docker-compose --profile dev up --build sam-monitor-dev

# Stop docker-compose services
docker-down:
	docker-compose down

# View docker-compose logs
docker-logs:
	docker-compose logs -f sam-monitor

# Clean up Docker artifacts
docker-clean:
	docker-compose down -v
	docker rmi sam-gov-monitor:latest 2>/dev/null || true
	docker system prune -f

# Generate test coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	@echo "Coverage report generated: coverage.out"

# Generate coverage for CI
coverage-ci:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out *.log state/*.json

# Install dependencies
deps:
	go mod download
	go mod tidy
	@echo "Dependencies updated"

# Validate environment variables
validate-env:
	go run ./cmd/monitor -validate-env

# Build release binaries for multiple platforms
release:
	@mkdir -p bin/release
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/release/monitor-linux-amd64 ./cmd/monitor
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/release/monitor-darwin-amd64 ./cmd/monitor
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/release/monitor-windows-amd64.exe ./cmd/monitor
	@echo "Release binaries built in bin/release/"

# Quick development cycle - build and test
dev:
	@$(MAKE) build
	@$(MAKE) test-short
	@$(MAKE) lint || true
	@echo "Development cycle complete"

# Pre-push checks
pre-push:
	@$(MAKE) test
	@$(MAKE) test-integration
	@$(MAKE) lint
	@$(MAKE) build
	@echo "Pre-push checks passed"

# Maintenance tasks
maintenance:
	@$(MAKE) build
	@echo "Running system maintenance..."
	./bin/maintenance -task health-check -v
	./bin/maintenance -task cleanup-state -v
	./bin/maintenance -task optimize-cache -v
	@echo "Maintenance completed"

# Individual maintenance tasks
maintenance-health:
	@$(MAKE) build
	./bin/maintenance -task health-check -v

maintenance-cleanup:
	@$(MAKE) build
	./bin/maintenance -task cleanup-state -age-days 30 -v

maintenance-report:
	@$(MAKE) build
	@mkdir -p reports
	./bin/maintenance -task generate-report -output reports/maintenance-$(shell date +%Y%m%d).md -v