# Kalyx Makefile 🌸

.PHONY: all build test test-integration bench test-e2e test-coverage lint vulncheck run clean setup-hooks

BINARY_DIR ?= bin
GO ?= go

all: test lint build

build:
	@mkdir -p $(BINARY_DIR)
	$(GO) build -v -o $(BINARY_DIR)/kalyx ./cmd/kalyx
	$(GO) build -v -o $(BINARY_DIR)/kalyxctl ./cmd/kalyxctl

# Testing Pyramid Commands
test:
	$(GO) test -v -race ./internal/...

test-integration:
	$(GO) test -v -race ./tests/integration/...

bench:
	$(GO) test -v -bench=. -benchmem -run=^# ./internal/...

test-e2e:
	$(GO) test -v ./tests/e2e/...

test-coverage:
	$(GO) test -coverprofile=coverage.out ./internal/...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✔ Coverage report generated in coverage.html"

# Quality & Security Linters
lint:
	golangci-lint run ./...

vulncheck:
	govulncheck ./...

run:
	$(GO) run ./cmd/kalyx

clean:
	rm -rf $(BINARY_DIR) coverage.out coverage.html

setup-hooks:
	@mkdir -p .git/hooks
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'set -e' >> .git/hooks/pre-commit
	@echo 'echo "🌸 Running Kalyx pre-commit checks..."' >> .git/hooks/pre-commit
	@echo 'go test ./internal/...' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit || true
	@echo "✔ Git pre-commit hook installed successfully!"
