.DEFAULT_GOAL := help

GO ?= go
AIR_VERSION ?= v1.67.4
COMPOSE := docker compose -f deployments/local/compose.yaml
BIN_DIR := bin
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.Version=$(VERSION) \
	-X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.Commit=$(COMMIT) \
	-X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.BuildTime=$(BUILD_TIME)

.PHONY: help fmt check-format check-migrations sql-list init-admin vet lint vulncheck test test-integration build ci clean dev-api dev-up dev-down openapi-check

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"}; /^[a-zA-Z0-9_-]+:.*##/ {printf "%-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go source
	@gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

check-format: ## Fail if Go source is not formatted
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"

check-migrations: ## Validate manual SQL versions and up/down pairs
	@sh scripts/check-migrations.sh

sql-list: ## List versioned SQL files for manual execution
	@find migrations -type f -name '*.up.sql' -print | sort

init-admin: ## Interactively create the first local ROOT administrator
	@./scripts/init-admin.sh

vet: ## Run go vet
	@$(GO) vet ./...

lint: ## Run pinned Staticcheck
	@$(GO) tool staticcheck ./...

vulncheck: ## Report reachable known vulnerabilities
	@$(GO) tool govulncheck ./...

test: ## Run fast tests with race detection
	@$(GO) test -race ./...

test-integration: ## Run tests that require real local dependencies
	@$(GO) test -count=1 -race -tags=integration ./...

build: build-api build-job build-collection-consumer build-transformation-consumer build-upload-consumer build-infrastructure-consumer ## Build every role
build-api:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-api ./cmd/api

run-api: ## Run the api role
	@started_at_ms="$$(perl -MTime::HiRes=time -e 'printf "%.0f", time() * 1000')"; \
		echo "Compiling and starting API..."; \
		GO_SERVICE_MAIN_STARTUP_STARTED_AT_MS="$$started_at_ms" $(GO) run ./cmd/api

dev-api: ## Run the api role with local hot reload
	@$(GO) run github.com/air-verse/air@$(AIR_VERSION) -c .air.api.toml

build-collection-consumer:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-collection-consumer ./cmd/collection-consumer
run-collection-consumer: ## Run the material collection consumer role
	@$(GO) run ./cmd/collection-consumer
build-transformation-consumer:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-transformation-consumer ./cmd/transformation-consumer
run-transformation-consumer: ## Run the material transformation consumer role
	@$(GO) run ./cmd/transformation-consumer
build-upload-consumer:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-upload-consumer ./cmd/upload-consumer
run-upload-consumer: ## Run the channel upload consumer role
	@$(GO) run ./cmd/upload-consumer
build-infrastructure-consumer:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-infrastructure-consumer ./cmd/infrastructure-consumer
run-infrastructure-consumer: ## Run the channel infrastructure consumer role
	@$(GO) run ./cmd/infrastructure-consumer
build-job:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-job ./cmd/job

run-job: ## Run the job role
	@$(GO) run ./cmd/job
openapi-check: ## Validate OpenAPI syntax and contracts
	@$(GO) test -count=1 ./internal/adapters/http -run '^TestOpenAPI'

ci: check-format check-migrations vet lint test test-integration openapi-check build ## Run the complete CI gate
dev-up: ## Start local infrastructure
	@$(COMPOSE) up -d --wait

dev-down: ## Stop local infrastructure
	@$(COMPOSE) down

clean: ## Remove build artifacts
	@rm -rf $(BIN_DIR) coverage.out
