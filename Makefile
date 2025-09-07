# Makefile for Modular Go Framework
.PHONY: help tasks-check lint test test-core test-modules test-examples test-cli fmt clean all

# Default target
all: fmt lint test

# Help target
help:
	@echo "Available targets:"
	@echo "  tasks-check     - Run lint and all tests (idempotent, for task validation)"
	@echo "  lint            - Run golangci-lint"
	@echo "  test            - Run all tests (core, modules, examples, CLI)"
	@echo "  test-core       - Run core framework tests"
	@echo "  test-modules    - Run tests for all modules"
	@echo "  test-examples   - Run tests for all examples"
	@echo "  test-cli        - Run CLI tool tests"
	@echo "  fmt             - Format Go code with gofmt"
	@echo "  clean           - Clean temporary files"
	@echo "  all             - Run fmt, lint, and test"

# Main task validation target as specified in T003
tasks-check: lint test

# Linting
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

# Core framework tests
test-core:
	@echo "Running core framework tests..."
	go test ./... -v

# Module tests
test-modules:
	@echo "Running module tests..."
	@for module in modules/*/; do \
		if [ -f "$$module/go.mod" ]; then \
			echo "Testing $$module"; \
			cd "$$module" && go test ./... -v && cd - > /dev/null; \
		fi; \
	done

# Example tests
test-examples:
	@echo "Running example tests..."
	@for example in examples/*/; do \
		if [ -f "$$example/go.mod" ]; then \
			echo "Testing $$example"; \
			cd "$$example" && go test ./... -v && cd - > /dev/null; \
		fi; \
	done

# CLI tests
test-cli:
	@echo "Running CLI tests..."
	@if [ -f "cmd/modcli/go.mod" ]; then \
		cd cmd/modcli && go test ./... -v; \
	else \
		echo "CLI module not found or has no go.mod"; \
	fi

# All tests
test: test-core test-modules test-examples test-cli

# Format code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

# Clean temporary files
clean:
	@echo "Cleaning temporary files..."
	go clean ./...
	@find . -name "*.tmp" -delete 2>/dev/null || true
	@find . -name "*.log" -delete 2>/dev/null || true