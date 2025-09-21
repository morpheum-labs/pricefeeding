# Chainlink Price Feed Monitor Makefile

# Variables
BINARY_NAME=chainlink-price-feed
BUILD_DIR=build
MAIN_FILE=main.go
MODULE_NAME=github.com/aelmanaa/chainlink-price-feed-golang

# Go build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"
BUILD_FLAGS=-v $(LDFLAGS)

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
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)
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
	go run $(MAIN_FILE)

# Run with timeout for testing
.PHONY: test-run
test-run: build
	@echo "Running $(BINARY_NAME) for 30 seconds..."
	timeout 30s ./$(BUILD_DIR)/$(BINARY_NAME) || true

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

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

# Install the binary to GOPATH/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	go install $(BUILD_FLAGS) $(MAIN_FILE)
	@echo "✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Create a test script for network configuration
.PHONY: test-networks
test-networks:
	@echo "Testing network configuration..."
	@echo "package main" > test_networks.go
	@echo "" >> test_networks.go
	@echo "import (" >> test_networks.go
	@echo "	\"fmt\"" >> test_networks.go
	@echo "	\"log\"" >> test_networks.go
	@echo "	\"$(MODULE_NAME)/rpcscan\"" >> test_networks.go
	@echo ")" >> test_networks.go
	@echo "" >> test_networks.go
	@echo "func main() {" >> test_networks.go
	@echo "	// Test loading extraRpcs.json" >> test_networks.go
	@echo "	extraRPCs, err := rpcscan.LoadExtraRPCs(\"conf/extraRpcs.json\")" >> test_networks.go
	@echo "	if err != nil {" >> test_networks.go
	@echo "		log.Fatalf(\"Failed to load extraRpcs.json: %v\", err)" >> test_networks.go
	@echo "	}" >> test_networks.go
	@echo "" >> test_networks.go
	@echo "	fmt.Printf(\"✅ Loaded %d networks from extraRpcs.json\\n\", len(*extraRPCs))" >> test_networks.go
	@echo "" >> test_networks.go
	@echo "	// Test creating network config" >> test_networks.go
	@echo "	config := &rpcscan.ExtendedConfig{}" >> test_networks.go
	@echo "	networkConfig := config.CreateNetworkConfig()" >> test_networks.go
	@echo "	" >> test_networks.go
	@echo "	fmt.Printf(\"✅ Created network configuration with %d networks\\n\", len(networkConfig.Networks))" >> test_networks.go
	@echo "	" >> test_networks.go
	@echo "	// Show sample networks" >> test_networks.go
	@echo "	count := 0" >> test_networks.go
	@echo "	for _, network := range networkConfig.Networks {" >> test_networks.go
	@echo "		if count >= 5 {" >> test_networks.go
	@echo "			break" >> test_networks.go
	@echo "		}" >> test_networks.go
	@echo "		fmt.Printf(\"  - %s (%s): %d endpoints\\n\", network.NameStd, network.NetworkID, len(network.Endpoints))" >> test_networks.go
	@echo "		count++" >> test_networks.go
	@echo "	}" >> test_networks.go
	@echo "	" >> test_networks.go
	@echo "	if len(networkConfig.Networks) > 5 {" >> test_networks.go
	@echo "		fmt.Printf(\"  ... and %d more networks\\n\", len(networkConfig.Networks)-5)" >> test_networks.go
	@echo "	}" >> test_networks.go
	@echo "}" >> test_networks.go
	go run test_networks.go
	rm -f test_networks.go

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

# Show help
.PHONY: help
help:
	@echo "Chainlink Price Feed Monitor - Available targets:"
	@echo ""
	@echo "  build         - Build the application"
	@echo "  build-all     - Build for multiple platforms (Linux, macOS, Windows)"
	@echo "  run           - Build and run the application"
	@echo "  run-dev       - Run in development mode (go run)"
	@echo "  test-run      - Run for 30 seconds to test functionality"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  benchmark     - Run benchmarks"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  deps          - Download dependencies"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  test-networks - Test network configuration loading"
	@echo "  dev-setup     - Setup development environment"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build && make run"
	@echo "  make test-networks"
	@echo "  make test-coverage"
