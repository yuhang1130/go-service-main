.DEFAULT_GOAL := help

GO ?= go
AIR_VERSION ?= v1.67.4
COMPOSE := docker compose -f deployments/local/compose.yaml
ROCKETMQ_TOPIC ?= go-service-main-local-events
BIN_DIR := bin
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.Version=$(VERSION) \
	-X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.Commit=$(COMMIT) \
	-X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.BuildTime=$(BUILD_TIME)

.PHONY: help fmt check-format check-migrations sql-list init-admin vet lint vulncheck test test-integration build ci clean dev-api dev-up dev-down mq-init openapi-check

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
	@$(GO) test -race -tags=integration ./...

build: build-api build-consumer build-job ## Build every role
build-api:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-api ./cmd/api

run-api: ## Run the api role
	@$(GO) run ./cmd/api

dev-api: ## Run the api role with local hot reload
	@$(GO) run github.com/air-verse/air@$(AIR_VERSION) -c .air.api.toml

build-consumer:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-consumer ./cmd/consumer

run-consumer: ## Run the consumer role
	@$(GO) run ./cmd/consumer
build-job:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/go-service-main-job ./cmd/job

run-job: ## Run the job role
	@$(GO) run ./cmd/job
openapi-check: ## Validate OpenAPI syntax and contracts
	@$(GO) test ./internal/adapters/http -run TestOpenAPIContractIsValid

ci: check-format check-migrations vet lint test test-integration openapi-check build ## Run the complete CI gate
dev-up: ## Start local infrastructure
	@$(COMPOSE) up -d --wait
	@$(MAKE) mq-init

mq-init: ## Create the local RocketMQ topic
	@for attempt in 1 2 3 4 5 6 7 8 9 10; do \
		if $(COMPOSE) exec -T rocketmq-broker sh mqadmin updateTopic \
			-n rocketmq-namesrv:9876 -c DefaultCluster -t $(ROCKETMQ_TOPIC) >/dev/null 2>&1; then \
			echo "RocketMQ topic ready: $(ROCKETMQ_TOPIC)"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	$(COMPOSE) exec -T rocketmq-broker sh mqadmin updateTopic \
		-n rocketmq-namesrv:9876 -c DefaultCluster -t $(ROCKETMQ_TOPIC)

dev-down: ## Stop local infrastructure
	@$(COMPOSE) down

clean: ## Remove build artifacts
	@rm -rf $(BIN_DIR) coverage.out
