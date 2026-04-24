BINARY_API         := bookstore-api
BINARY_WORKER      := bookstore-worker
BINARY_SCHEDULER   := bookstore-scheduler
BINARY_MIGRATE     := bookstore-migrate
BUILD_DIR          := ./bin
MAIN_API           := ./cmd/api/main.go
MAIN_WORKER        := ./cmd/worker/main.go
MAIN_SCHEDULER     := ./cmd/scheduler/main.go
MAIN_MIGRATE       := ./cmd/migrate/main.go
MIGRATION_DIR      := ./migrations

include .env
export

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: build build-api build-worker build-scheduler build-migrate
build: build-api build-worker build-scheduler build-migrate ## Build all binaries

build-api: ## Build API binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_API) $(MAIN_API)

build-worker: ## Build worker binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_WORKER) $(MAIN_WORKER)

build-scheduler: ## Build scheduler binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_SCHEDULER) $(MAIN_SCHEDULER)

build-migrate: ## Build migrate binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_MIGRATE) $(MAIN_MIGRATE)

.PHONY: wire
wire: ## Generate Wire injectors
	wire ./internal/bootstrap

.PHONY: dev dev-worker dev-scheduler

dev: wire ## Run API with air
	air -c .air.api.toml

dev-worker: wire ## Run worker with air
	air -c .air.worker.toml

dev-scheduler: wire ## Run scheduler with air
	air -c .air.scheduler.toml

.PHONY: run-api run-worker run-scheduler
run-api: wire build-api ## Run API without hot reload
	$(BUILD_DIR)/$(BINARY_API)

run-worker: wire build-worker ## Run worker without hot reload
	$(BUILD_DIR)/$(BINARY_WORKER)

run-scheduler: wire build-scheduler ## Run scheduler without hot reload
	$(BUILD_DIR)/$(BINARY_SCHEDULER)

.PHONY: migrate-up migrate-down migrate-version migrate-force migrate-create
migrate-up: ## Apply all migrations
	migrate -path $(MIGRATION_DIR) -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" up

migrate-down: ## Rollback 1 migration
	migrate -path $(MIGRATION_DIR) -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" down 1

migrate-version: ## Show current migration version
	migrate -path $(MIGRATION_DIR) -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" version

migrate-force: ## Force migration version (usage: make migrate-force version=1)
	migrate -path $(MIGRATION_DIR) -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" force $(version)

migrate-create: ## Create migration (usage: make migrate-create name=create_users)
	migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(name)

.PHONY: infra-up infra-down infra-reset infra-ps infra-logs
infra-up: ## Start local infra
	docker compose up -d

infra-down: ## Stop local infra
	docker compose down

infra-reset: ## Reset local infra and delete volumes
	docker compose down -v && docker compose up -d

infra-ps: ## Show infra status
	docker compose ps

infra-logs: ## Tail infra logs
	docker compose logs -f

.PHONY: lint fmt vet tidy test test-coverage
lint: ## Run linter
	golangci-lint run ./...

fmt: ## Format code
	gofmt -w -s ./

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy modules
	go mod tidy

test: ## Run all tests
	go test ./... -race -count=1

test-coverage: ## Run tests with coverage
	go test ./... -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

.PHONY: tools
tools: ## Install tools
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest