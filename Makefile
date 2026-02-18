# ═══════════════════════════════════════════════════════════════════════════
# Vigil Makefile
# ═══════════════════════════════════════════════════════════════════════════

# Configuration
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)
GO := go
GOFLAGS := -v

# Output directories
DIST_DIR := dist
COVERAGE_DIR := coverage

# Binary names
SERVER_BIN := vigil-server
AGENT_BIN := vigil-agent

# Colors for output
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: all build build-server build-agent test test-coverage test-race lint vet clean help

# ═══════════════════════════════════════════════════════════════════════════
# Default target
# ═══════════════════════════════════════════════════════════════════════════
all: lint test build

# ═══════════════════════════════════════════════════════════════════════════
# Build targets
# ═══════════════════════════════════════════════════════════════════════════

## build: Build both server and agent binaries
build: build-server build-agent
	@echo "$(GREEN)✓ Build complete$(NC)"

## build-server: Build the server binary
build-server:
	@echo "$(CYAN)Building server...$(NC)"
	@mkdir -p $(DIST_DIR)
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(SERVER_BIN) ./cmd/server

## build-agent: Build the agent binary
build-agent:
	@echo "$(CYAN)Building agent...$(NC)"
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BIN) ./cmd/agent

## build-all: Build binaries for all platforms
build-all: clean
	@echo "$(CYAN)Building for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	
	@echo "  → linux/amd64 agent"
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BIN)-linux-amd64 ./cmd/agent
	
	@echo "  → linux/arm64 agent"
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BIN)-linux-arm64 ./cmd/agent
	
	@echo "  → linux/amd64 server"
	@GOOS=linux GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(SERVER_BIN)-linux-amd64 ./cmd/server
	
	@cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "$(GREEN)✓ All binaries built in $(DIST_DIR)/$(NC)"

# ═══════════════════════════════════════════════════════════════════════════
# Test targets
# ═══════════════════════════════════════════════════════════════════════════

## test: Run all tests
test:
	@echo "$(CYAN)Running tests...$(NC)"
	$(GO) test ./...
	@echo "$(GREEN)✓ All tests passed$(NC)"

## test-v: Run tests with verbose output
test-v:
	@echo "$(CYAN)Running tests (verbose)...$(NC)"
	$(GO) test -v ./...

## test-race: Run tests with race detector
test-race:
	@echo "$(CYAN)Running tests with race detector...$(NC)"
	$(GO) test -race ./...
	@echo "$(GREEN)✓ No race conditions detected$(NC)"

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "$(CYAN)Running tests with coverage...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo ""
	@echo "$(CYAN)Generating HTML coverage report...$(NC)"
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)✓ Coverage report: $(COVERAGE_DIR)/coverage.html$(NC)"

## test-pkg: Run tests for a specific package (usage: make test-pkg PKG=./internal/version)
test-pkg:
	@echo "$(CYAN)Running tests for $(PKG)...$(NC)"
	$(GO) test -v $(PKG)

# ═══════════════════════════════════════════════════════════════════════════
# Code quality targets
# ═══════════════════════════════════════════════════════════════════════════

## lint: Run all linters
lint: vet
	@echo "$(GREEN)✓ Linting passed$(NC)"

## vet: Run go vet
vet:
	@echo "$(CYAN)Running go vet...$(NC)"
	$(GO) vet ./...

## fmt: Format all Go files
fmt:
	@echo "$(CYAN)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

## tidy: Tidy go.mod
tidy:
	@echo "$(CYAN)Tidying modules...$(NC)"
	$(GO) mod tidy
	@echo "$(GREEN)✓ Modules tidied$(NC)"

# ═══════════════════════════════════════════════════════════════════════════
# Development targets
# ═══════════════════════════════════════════════════════════════════════════

## run-server: Build and run the server
run-server: build-server
	@echo "$(CYAN)Starting server...$(NC)"
	./$(DIST_DIR)/$(SERVER_BIN)

## run-agent: Build and run the agent
run-agent: build-agent
	@echo "$(CYAN)Starting agent...$(NC)"
	./$(DIST_DIR)/$(AGENT_BIN)

## dev: Run tests, lint, and build (quick dev cycle)
dev: fmt vet test build
	@echo "$(GREEN)✓ Dev build complete$(NC)"

# ═══════════════════════════════════════════════════════════════════════════
# Docker targets
# ═══════════════════════════════════════════════════════════════════════════

## docker-server: Build server Docker image
docker-server:
	@echo "$(CYAN)Building server Docker image...$(NC)"
	docker build -t vigil:$(VERSION) --build-arg VERSION=$(VERSION) -f Dockerfile .

## docker-agent: Build agent Docker image (Alpine)
docker-agent:
	@echo "$(CYAN)Building agent Docker image...$(NC)"
	docker build -t vigil-agent:$(VERSION) --build-arg VERSION=$(VERSION) -f Dockerfile.agent .

## docker-agent-debian: Build agent Docker image (Debian)
docker-agent-debian:
	@echo "$(CYAN)Building agent Docker image (Debian)...$(NC)"
	docker build -t vigil-agent:$(VERSION)-debian --build-arg VERSION=$(VERSION) -f Dockerfile.agent.debian .

## docker-all: Build all Docker images
docker-all: docker-server docker-agent docker-agent-debian
	@echo "$(GREEN)✓ All Docker images built$(NC)"

# ═══════════════════════════════════════════════════════════════════════════
# Utility targets
# ═══════════════════════════════════════════════════════════════════════════

## clean: Remove build artifacts
clean:
	@echo "$(CYAN)Cleaning...$(NC)"
	@rm -rf $(DIST_DIR) $(COVERAGE_DIR)
	@$(GO) clean
	@echo "$(GREEN)✓ Clean$(NC)"

## version: Show version
version:
	@echo "$(VERSION)"

## help: Show this help
help:
	@echo ""
	@echo "$(CYAN)Vigil Makefile$(NC)"
	@echo ""
	@echo "Usage: make $(YELLOW)<target>$(NC)"
	@echo ""
	@echo "$(CYAN)Targets:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | sed 's/: /\t/'
	@echo ""
	@echo "$(CYAN)Examples:$(NC)"
	@echo "  make                    # Lint, test, and build"
	@echo "  make test               # Run all tests"
	@echo "  make test-v             # Run tests with verbose output"
	@echo "  make test-coverage      # Run tests with coverage report"
	@echo "  make test-pkg PKG=./internal/version  # Test specific package"
	@echo "  make build              # Build server and agent"
	@echo "  make dev                # Quick dev cycle (fmt, vet, test, build)"
	@echo ""