.PHONY: build build-all install clean test lint run

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X github.com/jeroenfrenken/worktree-manager/internal/cli.Version=$(VERSION) -X github.com/jeroenfrenken/worktree-manager/internal/cli.Commit=$(COMMIT)"

# Default binary output
BINARY := bin/wtm

# Build for current platform
build:
	@echo "Building wtm $(VERSION)..."
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BINARY) ./cmd/wtm

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/wtm-darwin-amd64 ./cmd/wtm
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/wtm-darwin-arm64 ./cmd/wtm
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/wtm-linux-amd64 ./cmd/wtm
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/wtm-linux-arm64 ./cmd/wtm

# Install to /usr/local/bin
install: build
	@echo "Installing wtm to /usr/local/bin..."
	@cp $(BINARY) /usr/local/bin/wtm
	@echo "Done! Run 'wtm --help' to get started."

# Uninstall
uninstall:
	@echo "Removing wtm from /usr/local/bin..."
	@rm -f /usr/local/bin/wtm

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run the application
run:
	go run ./cmd/wtm $(ARGS)

# Development: watch and rebuild
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

# Tidy dependencies
tidy:
	go mod tidy

# Download dependencies
deps:
	go mod download

# Generate (if needed in future)
generate:
	go generate ./...

# Help
help:
	@echo "worktree-manager Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build       Build for current platform"
	@echo "  make build-all   Build for all platforms"
	@echo "  make install     Install to /usr/local/bin"
	@echo "  make uninstall   Remove from /usr/local/bin"
	@echo "  make clean       Remove build artifacts"
	@echo "  make test        Run tests"
	@echo "  make lint        Run linter"
	@echo "  make fmt         Format code"
	@echo "  make run         Run the application"
	@echo "  make dev         Run with hot reload"
	@echo "  make help        Show this help"
