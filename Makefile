# Variables
BINARY_NAME=aws-slurm-burst
VERSION=$(shell cat VERSION)
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commitHash=$(COMMIT_HASH)"

# Directories
BUILD_DIR=build
COVERAGE_DIR=coverage
DOCS_DIR=docs

# Colors
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: all build test clean install lint fmt vet security coverage help
.DEFAULT_GOAL := help

## Build commands
all: clean lint test build ## Run all checks and build

build: ## Build all binaries
	@echo "$(GREEN)Building binaries...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/resume ./cmd/resume
	@go build $(LDFLAGS) -o $(BUILD_DIR)/suspend ./cmd/suspend
	@go build $(LDFLAGS) -o $(BUILD_DIR)/state-manager ./cmd/state-manager
	@go build $(LDFLAGS) -o $(BUILD_DIR)/validate ./cmd/validate
	@go build $(LDFLAGS) -o $(BUILD_DIR)/export-performance ./cmd/export-performance
	@echo "$(GREEN)Build completed successfully$(NC)"

build-all-platforms: ## Build for multiple platforms
	@echo "$(GREEN)Building for multiple platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/resume-linux-amd64 ./cmd/resume
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/resume-linux-arm64 ./cmd/resume
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/resume-darwin-amd64 ./cmd/resume
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/resume-darwin-arm64 ./cmd/resume
	@echo "$(GREEN)Multi-platform build completed$(NC)"

install: build ## Install binaries to system
	@echo "$(GREEN)Installing binaries...$(NC)"
	@sudo cp $(BUILD_DIR)/resume /usr/local/bin/$(BINARY_NAME)-resume
	@sudo cp $(BUILD_DIR)/suspend /usr/local/bin/$(BINARY_NAME)-suspend
	@sudo cp $(BUILD_DIR)/state-manager /usr/local/bin/$(BINARY_NAME)-state-manager
	@sudo cp $(BUILD_DIR)/validate /usr/local/bin/$(BINARY_NAME)-validate
	@sudo cp $(BUILD_DIR)/export-performance /usr/local/bin/$(BINARY_NAME)-export-performance
	@sudo cp scripts/slurm-epilog-aws.sh /usr/local/bin/slurm-epilog-aws-burst.sh
	@sudo chmod +x /usr/local/bin/slurm-epilog-aws-burst.sh
	@echo "$(GREEN)Installation completed$(NC)"

## Development commands
dev: ## Run in development mode with hot reload
	@echo "$(YELLOW)Starting development mode...$(NC)"
	@air

## Test commands
test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v -race -timeout 30s ./...

test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	@go test -v -race -timeout 300s -tags=integration ./test/integration/...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	@go test -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1
	@echo "$(GREEN)Coverage report generated: $(COVERAGE_DIR)/coverage.html$(NC)"

test-coverage-ci: ## Generate coverage for CI
	@mkdir -p $(COVERAGE_DIR)
	@go test -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out

benchmark: ## Run benchmarks
	@echo "$(GREEN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...

## Quality commands (Go Report Card standards)
reportcard: ## Run Go Report Card checks for A grade
	@echo "$(GREEN)Running Go Report Card checks...$(NC)"
	@echo "Checking gofmt..."
	@test -z "$$(gofmt -l .)" || (echo "$(RED)gofmt issues found:$(NC)" && gofmt -l . && exit 1)
	@echo "Checking go vet..."
	@go vet ./...
	@echo "Checking gocyclo..."
	@test -z "$$(gocyclo -over 15 .)" || (echo "$(RED)Cyclomatic complexity > 15:$(NC)" && gocyclo -over 15 . && exit 1)
	@echo "Checking misspell..."
	@misspell -error .
	@echo "Checking ineffassign..."
	@ineffassign ./...
	@echo "$(GREEN)All Go Report Card checks passed!$(NC)"

install-reportcard-tools: ## Install Go Report Card tools
	@echo "$(GREEN)Installing Go Report Card tools...$(NC)"
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@go install github.com/client9/misspell/cmd/misspell@latest
	@go install github.com/gordonklaus/ineffassign@latest

lint: ## Run linter
	@echo "$(GREEN)Running linters...$(NC)"
	@golangci-lint run

lint-fix: ## Fix linting issues
	@echo "$(GREEN)Fixing linting issues...$(NC)"
	@golangci-lint run --fix

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	@go fmt ./...
	@goimports -w .

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...

security: ## Run security checks
	@echo "$(GREEN)Running security checks...$(NC)"
	@gosec ./...

deps: ## Download dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	@go mod download
	@go mod tidy

deps-update: ## Update dependencies
	@echo "$(GREEN)Updating dependencies...$(NC)"
	@go get -u ./...
	@go mod tidy

deps-check: ## Check for dependency vulnerabilities
	@echo "$(GREEN)Checking dependencies for vulnerabilities...$(NC)"
	@govulncheck ./...

## Documentation commands
docs: ## Generate documentation
	@echo "$(GREEN)Generating documentation...$(NC)"
	@mkdir -p $(DOCS_DIR)/api
	@go doc -all ./pkg/... > $(DOCS_DIR)/api/pkg.md
	@go doc -all ./internal/... > $(DOCS_DIR)/api/internal.md

docs-serve: ## Serve documentation locally
	@echo "$(GREEN)Serving documentation...$(NC)"
	@pkgsite -http :6060

## Cleanup commands
clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR) $(COVERAGE_DIR)
	@go clean -cache -testcache -modcache

clean-all: clean ## Clean everything including dependencies
	@echo "$(GREEN)Cleaning everything...$(NC)"
	@go clean -cache -testcache -modcache -fuzzcache

## Git and release commands
git-hooks: ## Install git hooks
	@echo "$(GREEN)Installing git hooks...$(NC)"
	@pre-commit install
	@pre-commit install --hook-type commit-msg

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit: $(COMMIT_HASH)"

release-check: ## Check if ready for release
	@echo "$(GREEN)Checking release readiness...$(NC)"
	@make lint
	@make test-coverage
	@make security
	@echo "$(GREEN)Release checks passed!$(NC)"

tag-release: ## Tag a new release (VERSION=x.y.z make tag-release)
	@if [ -z "$(VERSION)" ]; then echo "$(RED)VERSION is required. Usage: VERSION=1.0.0 make tag-release$(NC)"; exit 1; fi
	@echo "$(GREEN)Tagging release v$(VERSION)...$(NC)"
	@echo "$(VERSION)" > VERSION
	@git add VERSION CHANGELOG.md
	@git commit -m "chore: bump version to $(VERSION)"
	@git tag -a v$(VERSION) -m "Release version $(VERSION)"
	@echo "$(GREEN)Tagged v$(VERSION). Push with: git push origin main --tags$(NC)"

## CI/CD commands
ci: ## Run CI pipeline locally
	@echo "$(GREEN)Running CI pipeline...$(NC)"
	@make deps
	@make install-reportcard-tools
	@make reportcard
	@make test-coverage-ci
	@make security
	@make build

## Help
help: ## Display this help message
	@echo "$(GREEN)AWS Slurm Burst - Makefile Commands$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Examples:$(NC)"
	@echo "  make build                    # Build all binaries"
	@echo "  make test-coverage           # Run tests with coverage report"
	@echo "  make lint-fix               # Auto-fix linting issues"
	@echo "  VERSION=1.0.0 make tag-release  # Tag a new release"