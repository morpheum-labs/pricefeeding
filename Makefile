# Chainlink Price Feed Monitor Makefile

# Variables
BINARY_NAME=oracle-price-feed
BUILD_DIR=build
MAIN_FILE=main.go
MODULE_NAME=github.com/morpheum-labs/pricefeeding

# Go build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S') -s -w"
BUILD_FLAGS=-v $(LDFLAGS)

# CGO settings for cross-platform builds
CGO_ENABLED=1

# Default target
.PHONY: all
all: clean build

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	# Linux AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	# macOS AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	# macOS ARM64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	# Windows AMD64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)
	@echo "✅ Multi-platform build completed"

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run without building (using go run)
.PHONY: run-dev
run-dev:
	@echo "Running in development mode..."
	@echo "⚠️  Please specify --chainlink or --pyth flag"
	@echo "Example: make run-chainlink or make run-pyth"

# Run Chainlink mode
.PHONY: run-chainlink
run-chainlink: build
	@echo "Running $(BINARY_NAME) in Chainlink mode..."
	./$(BUILD_DIR)/$(BINARY_NAME) --chainlink

# Run Pyth mode
.PHONY: run-pyth
run-pyth: build
	@echo "Running $(BINARY_NAME) in Pyth mode..."
	./$(BUILD_DIR)/$(BINARY_NAME) --pyth

# Run Chainlink mode in development (go run)
.PHONY: run-dev-chainlink
run-dev-chainlink:
	@echo "Running in development mode (Chainlink)..."
	go run $(MAIN_FILE) --chainlink

# Run Pyth mode in development (go run)
.PHONY: run-dev-pyth
run-dev-pyth:
	@echo "Running in development mode (Pyth)..."
	go run $(MAIN_FILE) --pyth

# Run with timeout for testing (Chainlink mode)
.PHONY: test-run
test-run: build
	@echo "Running $(BINARY_NAME) for 30 seconds (Chainlink mode)..."
	timeout 30s ./$(BUILD_DIR)/$(BINARY_NAME) --chainlink || true

# Run with timeout for testing (Pyth mode)
.PHONY: test-run-pyth
test-run-pyth: build
	@echo "Running $(BINARY_NAME) for 30 seconds (Pyth mode)..."
	timeout 30s ./$(BUILD_DIR)/$(BINARY_NAME) --pyth || true

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@echo "⚠️  Some tests may fail due to missing chain registry files"
	go test -v ./...

# Run tests excluding problematic packages
.PHONY: test-safe
test-safe:
	@echo "Running safe tests (excluding rpcscan)..."
	go test -v ./pricefeed ./pyth

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Lint the code
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "Running basic go vet instead..."; \
		go vet ./...; \
	fi

# Format the code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	go mod tidy
	@echo "✅ Dependencies tidied"

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	@echo "✅ Dependencies downloaded"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "✅ Clean completed"

# Clean everything including Docker
.PHONY: clean-all
clean-all: clean docker-clean
	@echo "✅ Full clean completed"

# Install the binary to GOPATH/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	go install $(BUILD_FLAGS) $(MAIN_FILE)
	@echo "✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Test network configuration loading
.PHONY: test-networks
test-networks:
	@echo "Testing network configuration..."
	@echo "⚠️  test-networks target requires manual implementation"

# Create a development environment setup
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	@if [ ! -f "conf/extraRpcs.json" ]; then \
		echo "⚠️  conf/extraRpcs.json not found. Please ensure it exists."; \
	fi
	@if [ ! -f "conf/crytos.yaml" ]; then \
		echo "⚠️  conf/crytos.yaml not found. Please ensure it exists."; \
	fi
	@if [ ! -f "conf/stocks.yaml" ]; then \
		echo "⚠️  conf/stocks.yaml not found. Please ensure it exists."; \
	fi
	go mod download
	@echo "✅ Development environment ready"

# Docker targets
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

.PHONY: docker-run-chainlink
docker-run-chainlink: docker-build
	@echo "Running Docker container in Chainlink mode..."
	docker run --rm -it $(BINARY_NAME):latest --chainlink

.PHONY: docker-run-pyth
docker-run-pyth: docker-build
	@echo "Running Docker container in Pyth mode..."
	docker run --rm -it $(BINARY_NAME):latest --pyth

.PHONY: docker-clean
docker-clean:
	@echo "Cleaning Docker images..."
	docker rmi $(BINARY_NAME):latest 2>/dev/null || true

# Show help
.PHONY: help
help:
	@echo "Oracle Price Feed Monitor - Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build         - Build the application"
	@echo "  build-all     - Build for multiple platforms (Linux, macOS, Windows)"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo ""
	@echo "Run targets:"
	@echo "  run-chainlink - Build and run in Chainlink mode"
	@echo "  run-pyth      - Build and run in Pyth mode"
	@echo "  run-dev-chainlink - Run in development mode (Chainlink)"
	@echo "  run-dev-pyth  - Run in development mode (Pyth)"
	@echo "  test-run      - Run for 30 seconds (Chainlink mode)"
	@echo "  test-run-pyth - Run for 30 seconds (Pyth mode)"
	@echo ""
	@echo "Test targets:"
	@echo "  test          - Run tests (may have some failures)"
	@echo "  test-safe     - Run tests excluding problematic packages"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  benchmark     - Run benchmarks"
	@echo ""
	@echo "Development targets:"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  deps          - Download dependencies"
	@echo "  dev-setup     - Setup development environment"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run-chainlink - Run Docker container (Chainlink mode)"
	@echo "  docker-run-pyth - Run Docker container (Pyth mode)"
	@echo "  docker-clean  - Clean Docker images"
	@echo ""
	@echo "Utility targets:"
	@echo "  clean         - Clean build artifacts"
	@echo "  clean-all     - Clean build artifacts and Docker images"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make run-chainlink"
	@echo "  make run-dev-pyth"
	@echo "  make test-coverage"
	@echo "  make docker-run-chainlink"
