BINARY := zombie-scanner
PKGS := ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.DEFAULT_GOAL := help

.PHONY: help build test lint fmt tidy cover clean check verify iam-policy release-check snapshot

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Compile the binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: ## Run tests with race detector
	go test -race -count=1 ./test

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format the code
	go fmt $(PKGS)

tidy: ## Sync go.mod/go.sum
	go mod tidy

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out coverage.html

verify: ## Very the read-only and architecture gates
	@./scripts/check-readonly.sh
	@./scripts/check-arch.sh

check: build test lint verify ## Everything CI runs

iam-policy: build ## Regenerate docs/iam-policy.json
	./$(BINARY) iam-policy > docs/iam-policy.json

release-check: ## Validate .goreleaser.yaml
	goreleaser check

snapshot: ## Build a local release into dist/ without publishing
	goreleaser release --snapshot --clean --skip=sbom