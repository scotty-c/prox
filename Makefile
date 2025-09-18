# Variables
BIN_DIR=./bin
BINARY_NAME=prox
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=${VERSION}"

# Default target
.DEFAULT_GOAL := help

# Help target - displays available targets
.PHONY: help
help: ## Display this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_][a-zA-Z0-9_-]*:.*## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
.PHONY: build
build: ## Build the binary
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BIN_DIR}
	go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME} main.go
	@echo "Binary built: ${BIN_DIR}/${BINARY_NAME}"

.PHONY: build-all
build-all: ## Build binaries for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p ${BIN_DIR}
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME}-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME}-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME}-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME}-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME}-windows-amd64.exe main.go
	@echo "Multi-platform binaries built in ${BIN_DIR}/"

.PHONY: install
install: build ## Install the binary to GOPATH/bin
	@echo "Installing ${BINARY_NAME} to GOPATH/bin..."
	go install ${LDFLAGS} .
	@echo "Installed successfully"

# Development targets
.PHONY: dev
dev: ## Build and run in development mode
	@echo "Building and running in development mode..."
	go run main.go

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

.PHONY: test-e2e
test-e2e: build ## Run end-to-end tests
	@echo "Running E2E tests..."
	@cd tests/e2e && ./setup.sh run

.PHONY: test-e2e-setup
test-e2e-setup: ## Setup E2E test configuration
	@echo "Setting up E2E test configuration..."
	@cd tests/e2e && ./setup.sh setup

.PHONY: test-e2e-validate
test-e2e-validate: ## Validate E2E test configuration
	@echo "Validating E2E test configuration..."
	@cd tests/e2e && ./setup.sh validate

.PHONY: test-e2e-discover
test-e2e-discover: build ## Discover available resources for E2E tests
	@echo "Discovering available resources for E2E tests..."
	@cd tests/e2e && ./setup.sh discover

.PHONY: test-e2e-dry-run
test-e2e-dry-run: build ## Run E2E tests in dry-run mode
	@echo "Running E2E tests in dry-run mode..."
	@cd tests/e2e && DRY_RUN=true ./setup.sh run

.PHONY: test-all
test-all: test test-e2e ## Run all tests (unit and E2E)
	@echo "All tests completed"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Code quality targets
.PHONY: lint
lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: mod-tidy
mod-tidy: ## Clean up go modules
	@echo "Tidying go modules..."
	go mod tidy

.PHONY: mod-download
mod-download: ## Download go modules
	@echo "Downloading go modules..."
	go mod download

# Security targets
.PHONY: security-check
security-check: ## Run security checks (requires gosec)
	@echo "Running security checks..."
	gosec ./...

.PHONY: vuln-check
vuln-check: ## Check for vulnerabilities (requires govulncheck)
	@echo "Checking for vulnerabilities..."
	govulncheck ./...

# Clean targets
.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf ${BIN_DIR}
	rm -f coverage.out coverage.html
	@echo "Clean completed"

.PHONY: clean-all
clean-all: clean ## Remove all generated files including dependencies
	@echo "Cleaning all generated files..."
	go clean -modcache
	@echo "Deep clean completed"

# Release targets
.PHONY: release-check
release-check: fmt vet test test-e2e-validate ## Run pre-release checks
	@echo "Pre-release checks completed successfully"

.PHONY: version
version: ## Show version information
	@echo "Version: ${VERSION}"
	@echo "Binary: ${BINARY_NAME}"
	@echo "Build directory: ${BIN_DIR}"

# Docker targets (optional)
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t ${BINARY_NAME}:${VERSION} .
	docker tag ${BINARY_NAME}:${VERSION} ${BINARY_NAME}:latest

.PHONY: docker-run
docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -it ${BINARY_NAME}:latest

# Development tools installation
.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Development tools installed"