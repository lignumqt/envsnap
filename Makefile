BINARY     := envsnap
MODULE     := github.com/lignumqt/envsnap
CMD        := ./cmd/envsnap

# Version info (injected via ldflags)
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE       ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS    := -s -w \
              -X '$(MODULE)/internal/version.VersionNumber=$(VERSION)' \
              -X '$(MODULE)/internal/version.GitCommit=$(COMMIT)' \
              -X '$(MODULE)/internal/version.BuildDate=$(DATE)'

GOFLAGS    := -trimpath

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Format source code with gofmt
	gofmt -w -s .

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: test
test: ## Run tests
	go test -race -timeout 60s ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	go test -race -timeout 60s -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: vet
vet: ## Run go vet
	go vet ./...

##@ Build

.PHONY: build
build: ## Build binary to ./envsnap
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

.PHONY: install
install: ## Install binary to /usr/local/bin
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(CMD)

##@ Dependencies

.PHONY: deps
deps: ## Install/update dependencies
	go mod tidy
	go mod download

.PHONY: update-deps
update-deps: ## Update all dependencies
	go get -u ./...
	go mod tidy

##@ Cleanup

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.txt coverage.html
	go clean -testcache
