.PHONY: build test clean lint coverage

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_NAME=memory
BINARY_UNIX=$(BINARY_NAME)_unix

# Main package
MAIN_PACKAGE=./cmd/memory

# Build directory
BUILD_DIR=./bin

# Test parameters
TEST_PARAMS=-v -race -coverprofile=coverage.out
TEST_DIR=./...

# Coverage parameters
COVERAGE_DIR=./coverage
COVERAGE_FILE=$(COVERAGE_DIR)/coverage.html

all: test build

build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

test:
	@echo "Running tests..."
	$(GOTEST) $(TEST_PARAMS) $(TEST_DIR)

coverage: test
	@echo "Generating coverage report..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOCMD) tool cover -html=coverage.out -o $(COVERAGE_FILE)
	@echo "Coverage report generated at $(COVERAGE_FILE)"

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out
	rm -rf $(COVERAGE_DIR)

lint:
	@echo "Running linters..."
	golangci-lint run ./...

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Cross-compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_UNIX) $(MAIN_PACKAGE)

# Docker
docker-build:
	docker build -t go-memory:latest .

docker-run:
	docker run -it --rm go-memory:latest

# Development
dev:
	@echo "Running in development mode..."
	$(GOCMD) run $(MAIN_PACKAGE)/main.go

watch:
	@echo "Watching for changes..."
	reflex -r '\.go$$' -s -- sh -c 'make test'

# Install
install: build
	@echo "Installing binary..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Help
help:
	@echo "Available targets:"
	@echo "  make build        - Build the binary"
	@echo "  make test         - Run tests"
	@echo "  make coverage     - Generate coverage report"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make lint         - Run linters"
	@echo "  make deps         - Download dependencies"
	@echo "  make dev          - Run in development mode"
	@echo "  make install      - Install binary to GOPATH/bin"