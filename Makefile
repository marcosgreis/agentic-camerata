.PHONY: build install clean test test-unit test-integration test-cover lint fmt vet tidy

# Binary name
BINARY := cmt

# Build directory
BUILD_DIR := ./build

# Version from git tag or commit
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build flags
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

# Default target
all: build

# Build the binary
build: tidy
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/cmt

# Install to GOPATH/bin
install: tidy
	go install $(LDFLAGS) ./cmd/cmt

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean

# Run all tests
test:
	go test -v ./...

# Run unit tests only (skip tests requiring tmux)
test-unit:
	go test -v -short ./...

# Run integration tests (requires tmux)
test-integration:
	@if [ -z "$$TMUX" ]; then \
		echo "Warning: Not running in tmux. Some tests will be skipped."; \
	fi
	go test -v -run "Integration" ./...

# Run tests with coverage
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests with race detector
test-race:
	go test -v -race ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy

# Build for multiple platforms
release: tidy
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/cmt
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/cmt
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/cmt
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/cmt

# Development: build and run
run: build
	$(BUILD_DIR)/$(BINARY)

# Show help
help:
	@echo "Available targets:"
	@echo "  build            - Build the binary"
	@echo "  install          - Install to GOPATH/bin"
	@echo "  clean            - Remove build artifacts"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests (requires tmux)"
	@echo "  test-cover       - Run tests with coverage report"
	@echo "  test-race        - Run tests with race detector"
	@echo "  lint             - Run golangci-lint"
	@echo "  fmt              - Format code"
	@echo "  vet              - Run go vet"
	@echo "  tidy             - Tidy dependencies"
	@echo "  release          - Build for multiple platforms"
	@echo "  run              - Build and run"
