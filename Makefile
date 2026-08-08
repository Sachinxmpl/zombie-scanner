BINARY := zombie-scanner
PKGS := ./...

.DEFAULT_GOAL := help

.PHONY: help build test race lint fmt tidy cover clean check

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Compile the binary
	go build -o $(BINARY) .

test: ## Run tests with race detector
	go test -race -count=1 $(PKGS)

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format the code
	go fmt $(PKGS)

tidy: ## Sync go.mod/go.sum
	go mod tidy

cover: ## Report test coverage
	go test -coverprofile=coverage.out $(PKGS)
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out coverage.html

check: build race lint