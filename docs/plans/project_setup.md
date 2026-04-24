# Hướng dẫn Project Setup Hoàn Chỉnh - Bookstore Backend Go Modular Monolith

---

## Tổng quan

Tài liệu này là bản cập nhật theo hướng mới:

- Sử dụng **Google Wire** để làm compile-time DI container.
- Sử dụng **pgx thuần** để quản lý kết nối PostgreSQL và thao tác database.
- Sử dụng **golang-migrate** là công cụ migration duy nhất.
- Không dùng `sqlx`.
- Không dùng manual wiring dài trong `main.go`.

Thứ tự setup đúng theo hướng mới:

```text
1. Init module Go                  → go.mod
2. .gitignore                      → clean repo
3. .env + config                   → env design
4. docker-compose                  → local infrastructure
5. Makefile                        → automation
6. Air hot reload                  → dev experience
7. go get libraries                → dependencies
8. platform/config                 → config loader
9. platform/logger                 → structured logger
10. platform/db                    → pgx pool + healthcheck
11. platform/tx                    → transaction manager
12. platform/redis                 → redis client
13. platform/kafka                 → producer / consumer
14. platform/httpx                 → gin + middleware stack
15. internal/bootstrap/providers   → Wire provider sets
16. internal/bootstrap/wire.go     → DI graph definition
17. cmd/api/main.go                → bootstrap via generated injector
18. cmd/worker/main.go             → bootstrap worker via generated injector
19. cmd/scheduler/main.go          → bootstrap scheduler via generated injector
20. cmd/migrate/main.go            → migration runner bằng golang-migrate
```

---

## 1. Quyết định kiến trúc mới

### 1.1 Dependency Injection

Dự án này **dùng Google Wire** thay cho manual wiring.

Lý do chọn Wire:
- Wire là **compile-time DI**, không phải runtime reflection container.
- Dependency graph được generate thành Go code nên vẫn minh bạch và Go-idiomatic.
- Phù hợp với modular monolith khi số lượng module tăng và constructor chain dài.
- Giảm độ rối trong `cmd/api/main.go`, `cmd/worker/main.go`, `cmd/scheduler/main.go`.

### 1.2 Database access

Dự án này **dùng pgx thuần**, không dùng `sqlx`.

Lý do:
- `pgx/v5` là lựa chọn mạnh và phổ biến cho PostgreSQL trong Go.
- Tận dụng tốt `pgxpool`, transaction API, batch, row scanning, PostgreSQL-specific features.
- Giảm abstraction không cần thiết trong giai đoạn xây modular monolith nghiêm ngặt boundary.

### 1.3 Migration

Dự án này **chỉ dùng golang-migrate** cho schema migration.

Nguyên tắc:
- Migration files nằm trong `migrations/`.
- App runtime không tự chạy migration ở production.
- `cmd/migrate` hoặc CLI `migrate` chịu trách nhiệm `up/down/status/create`.

---

## 2. Init Go Module

Chạy tại root project:

```bash
go mod init github.com/yourorg/bookstore-backend
```

`go.mod` tối thiểu:

```go
module github.com/yourorg/bookstore-backend

go 1.22
```

Tất cả import nội bộ dùng module path thống nhất:

```go
import "github.com/yourorg/bookstore-backend/internal/platform/db"
import "github.com/yourorg/bookstore-backend/internal/modules/order/application/service"
```

---

## 3. `.gitignore`

Tạo `.gitignore` như sau:

```gitignore
# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
bin/
dist/
vendor/

# Air
tmp/

# Wire generated
after_gen/
wire_gen.tmp

# Env
.env
.env.*
!.env.example

# Secrets
*.pem
*.key
*.crt
secrets/

# IDE
.idea/
.vscode/
*.swp
*.swo
.DS_Store

# Coverage
coverage.out
coverage.html

# Build artifacts
/bookstore-api
/bookstore-worker
/bookstore-scheduler
/bookstore-migrate

# Docker
docker-compose.override.yml
```

**Lưu ý**:
- Không ignore `wire_gen.go`, vì đây là generated source cần được commit để build ổn định trên CI/CD.
- Chỉ ignore file tạm, không ignore output generate chính thức.

---

## 4. `.env` và config

Tạo:
- `.env.example` → commit.
- `.env` → local only.

### `.env.example`

```env
# ============================================================
# APPLICATION
# ============================================================
APP_ENV=development
APP_NAME=bookstore-backend
APP_PORT=8080
APP_VERSION=0.0.1
APP_DEBUG=true
APP_GRACEFUL_SHUTDOWN_TIMEOUT_SECONDS=10

# ============================================================
# DATABASE - POSTGRESQL / PGX
# ============================================================
DB_HOST=localhost
DB_PORT=5432
DB_NAME=bookstore
DB_USER=bookstore_user
DB_PASSWORD=bookstore_pass
DB_SSL_MODE=disable
DB_MAX_OPEN_CONNS=25
DB_MIN_IDLE_CONNS=5
DB_MAX_CONN_LIFETIME_MINUTES=30
DB_MAX_CONN_IDLE_TIME_MINUTES=5
DB_HEALTHCHECK_PERIOD_SECONDS=30
DB_MIGRATION_DIR=./migrations

# ============================================================
# REDIS
# ============================================================
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_MAX_RETRIES=3
REDIS_POOL_SIZE=10
REDIS_MIN_IDLE_CONNS=5
REDIS_DIAL_TIMEOUT_SECONDS=5
REDIS_READ_TIMEOUT_SECONDS=3
REDIS_WRITE_TIMEOUT_SECONDS=3

# ============================================================
# KAFKA
# ============================================================
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP_ID=bookstore-consumer-group
KAFKA_CLIENT_ID=bookstore-backend
KAFKA_MAX_RETRY=3
KAFKA_RETRY_BACKOFF_MS=500
KAFKA_PRODUCER_TIMEOUT_MS=5000
KAFKA_CONSUMER_TIMEOUT_MS=10000
KAFKA_TOPIC_PREFIX=bookstore.

# ============================================================
# JWT / AUTH
# ============================================================
JWT_SECRET=super-secret-jwt-key-change-in-production
JWT_ACCESS_TOKEN_TTL_MINUTES=15
JWT_REFRESH_TOKEN_TTL_DAYS=30
JWT_ISSUER=bookstore-backend
BCRYPT_COST=12

# ============================================================
# MINIO
# ============================================================
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET_BOOKS=book-assets
MINIO_BUCKET_AVATARS=user-avatars
MINIO_USE_SSL=false
MINIO_PRESIGNED_URL_EXPIRY_MINUTES=15
MINIO_REGION=us-east-1

# ============================================================
# VNPAY
# ============================================================
VNPAY_TMN_CODE=your_tmn_code
VNPAY_HASH_SECRET=your_hash_secret
VNPAY_PAYMENT_URL=https://sandbox.vnpayment.vn/paymentv2/vpcpay.html
VNPAY_RETURN_URL=http://localhost:8080/api/v1/payments/vnpay/return
VNPAY_NOTIFY_URL=http://localhost:8080/api/v1/payments/webhook/vnpay

# ============================================================
# MAIL / NOTIFICATION
# ============================================================
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM_EMAIL=noreply@bookstore.local
SMTP_FROM_NAME=Bookstore
SMTP_TLS=false

# ============================================================
# ASYNQ
# ============================================================
ASYNQ_REDIS_HOST=localhost
ASYNQ_REDIS_PORT=6379
ASYNQ_REDIS_PASSWORD=
ASYNQ_REDIS_DB=1
ASYNQ_CONCURRENCY=10
ASYNQ_QUEUES_CRITICAL=6
ASYNQ_QUEUES_DEFAULT=3
ASYNQ_QUEUES_LOW=1
ASYNQMON_PORT=8081

# ============================================================
# RATE LIMIT
# ============================================================
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=100
RATE_LIMIT_BURST_SIZE=20

# ============================================================
# CORS
# ============================================================
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
CORS_ALLOWED_METHODS=GET,POST,PUT,PATCH,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Content-Type,Authorization,X-Request-ID,X-Idempotency-Key
CORS_ALLOW_CREDENTIALS=true
CORS_MAX_AGE_HOURS=12

# ============================================================
# OBSERVABILITY
# ============================================================
LOG_LEVEL=debug
LOG_FORMAT=json
LOG_OUTPUT=stdout
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_SERVICE_NAME=bookstore-backend
```

---

## 5. Docker Compose

Tạo `deployments/compose/docker-compose.dev.yml`:

```yaml
version: '3.9'

networks:
  bookstore-net:
    driver: bridge

volumes:
  postgres_data:
  redis_data:
  minio_data:
  kafka_data:
  zookeeper_data:

services:
  postgres:
    image: postgres:16-alpine
    container_name: bookstore-postgres
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: bookstore
      POSTGRES_USER: bookstore_user
      POSTGRES_PASSWORD: bookstore_pass
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U bookstore_user -d bookstore"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: bookstore-redis
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    container_name: bookstore-zookeeper
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "2181:2181"
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    volumes:
      - zookeeper_data:/var/lib/zookeeper/data

  kafka:
    image: confluentinc/cp-kafka:7.5.0
    container_name: bookstore-kafka
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "9092:9092"
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: true
    volumes:
      - kafka_data:/var/lib/kafka/data

  kafka-ui:
    image: provectuslabs/kafka-ui:latest
    container_name: bookstore-kafka-ui
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "8085:8080"
    depends_on:
      - kafka
    environment:
      KAFKA_CLUSTERS_0_NAME: bookstore-local
      KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:9092

  minio:
    image: minio/minio:latest
    container_name: bookstore-minio
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - minio_data:/data
    command: server /data --console-address ":9001"

  mailhog:
    image: mailhog/mailhog:latest
    container_name: bookstore-mailhog
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "1025:1025"
      - "8025:8025"

  asynqmon:
    image: hibiken/asynqmon:latest
    container_name: bookstore-asynqmon
    restart: unless-stopped
    networks: [bookstore-net]
    ports:
      - "8081:8080"
    environment:
      REDIS_ADDR: redis:6379
      REDIS_DB: 1
    depends_on:
      - redis
```

Root shortcut:

```yaml
include:
  - deployments/compose/docker-compose.dev.yml
```

---

## 6. Makefile

```makefile
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
```

---

## 7. Air hot reload

### `.air.api.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "make wire && go build -o ./tmp/bookstore-api ./cmd/api/main.go"
  bin = "./tmp/bookstore-api"
  full_bin = "./tmp/bookstore-api"
  include_ext = ["go", "yaml", "toml"]
  include_dir = ["cmd/api", "internal"]
  exclude_dir = ["tmp", "vendor", ".git", "test", "migrations", "docs"]
  delay = 1200
  stop_on_error = true

[log]
  time = true
```

### `.air.worker.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "make wire && go build -o ./tmp/bookstore-worker ./cmd/worker/main.go"
  bin = "./tmp/bookstore-worker"
  full_bin = "./tmp/bookstore-worker"
  include_ext = ["go", "yaml", "toml"]
  include_dir = ["cmd/worker", "internal"]
  exclude_dir = ["tmp", "vendor", ".git", "test", "migrations", "docs"]
  delay = 1200
  stop_on_error = true
```

### `.air.scheduler.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "make wire && go build -o ./tmp/bookstore-scheduler ./cmd/scheduler/main.go"
  bin = "./tmp/bookstore-scheduler"
  full_bin = "./tmp/bookstore-scheduler"
  include_ext = ["go", "yaml", "toml"]
  include_dir = ["cmd/scheduler", "internal"]
  exclude_dir = ["tmp", "vendor", ".git", "test", "migrations", "docs"]
  delay = 1200
  stop_on_error = true
```

---

## 8. `go get` - danh sách thư viện mới

### Core web

```bash
go get github.com/gin-gonic/gin@latest
go get github.com/gin-contrib/cors@latest
go get github.com/gin-contrib/requestid@latest
go get github.com/gin-contrib/timeout@latest
```

### DI

```bash
go get github.com/google/wire@latest
```

### PostgreSQL - pgx only

```bash
go get github.com/jackc/pgx/v5@latest
go get github.com/jackc/pgx/v5/pgxpool@latest
go get github.com/jackc/pgx/v5/pgconn@latest
go get github.com/jackc/pgx/v5/pgtype@latest
```

### Migration - golang-migrate only

```bash
go get -tags 'postgres' github.com/golang-migrate/migrate/v4@latest
go get -tags 'postgres' github.com/golang-migrate/migrate/v4/database/postgres@latest
go get github.com/golang-migrate/migrate/v4/source/file@latest
```

### Redis

```bash
go get github.com/redis/go-redis/v9@latest
go get github.com/go-redis/redis_rate/v10@latest
```

### Kafka

```bash
go get github.com/segmentio/kafka-go@latest
```

### Asynq

```bash
go get github.com/hibiken/asynq@latest
```

### Auth

```bash
go get github.com/golang-jwt/jwt/v5@latest
go get golang.org/x/crypto@latest
```

### Config + validation

```bash
go get github.com/joho/godotenv@latest
go get github.com/kelseyhightower/envconfig@latest
go get github.com/go-playground/validator/v10@latest
```

### Storage + mail

```bash
go get github.com/minio/minio-go/v7@latest
go get github.com/wneessen/go-mail@latest
```

### Logging + tracing

```bash
go get go.uber.org/zap@latest
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/trace@latest
```

### Testing

```bash
go get github.com/stretchr/testify@latest
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
go get github.com/testcontainers/testcontainers-go/modules/redis@latest
```

### Utility

```bash
go get github.com/google/uuid@latest
go get github.com/shopspring/decimal@latest
go get golang.org/x/time/rate@latest
```

### Install tools

```bash
go install github.com/google/wire/cmd/wire@latest
go install github.com/air-verse/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

---

## 9. `platform/config`

`internal/platform/config/config.go` nên load env tập trung và fail-fast khi thiếu config critical.

Ví dụ khung:

```go
package config

import (
    "fmt"
    "log"
    "os"
    "strconv"
    "time"

    "github.com/joho/godotenv"
)

type Config struct {
    App       AppConfig
    DB        DBConfig
    Redis     RedisConfig
    Kafka     KafkaConfig
    JWT       JWTConfig
    Bcrypt    BcryptConfig
    MinIO     MinIOConfig
    VNPay     VNPayConfig
    Mail      MailConfig
    Asynq     AsynqConfig
    Log       LogConfig
    CORS      CORSConfig
    RateLimit RateLimitConfig
}

type DBConfig struct {
    Host               string
    Port               string
    Name               string
    User               string
    Password           string
    SSLMode            string
    MaxOpenConns       int32
    MinIdleConns       int32
    MaxConnLifetime    time.Duration
    MaxConnIdleTime    time.Duration
    HealthcheckPeriod  time.Duration
    MigrationDir       string
}

func (c DBConfig) DSN() string {
    return fmt.Sprintf(
        "postgres://%s:%s@%s:%s/%s?sslmode=%s",
        c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
    )
}

func MustLoad() *Config {
    _ = godotenv.Load()
    return &Config{
        DB: loadDBConfig(),
        // ... load các config khác
    }
}

func mustGetEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("missing required env: %s", key)
    }
    return v
}

func getEnvAsInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return def
}
```

---

## 10. `platform/db` - pgx thuần

### `internal/platform/db/db.go`

```go
package db

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/yourorg/bookstore-backend/internal/platform/config"
)

func NewPool(cfg config.DBConfig) (*pgxpool.Pool, error) {
    poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
    if err != nil {
        return nil, err
    }

    poolCfg.MaxConns = cfg.MaxOpenConns
    poolCfg.MinConns = cfg.MinIdleConns
    poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
    poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
    poolCfg.HealthCheckPeriod = cfg.HealthcheckPeriod

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil {
        return nil, err
    }

    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, err
    }

    return pool, nil
}
```

### Quy ước repository với pgx

- Dùng `Query`, `QueryRow`, `Exec` trực tiếp từ `pgxpool.Pool` hoặc `pgx.Tx`.
- Scan bằng `row.Scan(...)`, không dùng `sqlx` tags.
- Nếu cần mapping lớn, viết mapper riêng trong `infrastructure/persistence`.

Ví dụ:

```go
func (r *OrderRepository) GetByID(ctx context.Context, id int64) (*entity.Order, error) {
    q := `SELECT id, user_id, status, total_amount, created_at FROM orders WHERE id = $1`

    var o entity.Order
    err := r.q(ctx).QueryRow(ctx, q, id).Scan(
        &o.ID,
        &o.UserID,
        &o.Status,
        &o.TotalAmount,
        &o.CreatedAt,
    )
    if err != nil {
        return nil, err
    }
    return &o, nil
}
```

---

## 11. `platform/tx` - transaction manager với pgx

```go
package tx

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type Manager struct {
    pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
    return &Manager{pool: pool}
}

func (m *Manager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
    txx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }

    ctx = context.WithValue(ctx, txKey{}, txx)

    if err := fn(ctx); err != nil {
        _ = txx.Rollback(ctx)
        return err
    }

    if err := txx.Commit(ctx); err != nil {
        return fmt.Errorf("commit tx: %w", err)
    }
    return nil
}

func Extract(ctx context.Context) (pgx.Tx, bool) {
    tx, ok := ctx.Value(txKey{}).(pgx.Tx)
    return tx, ok
}
```

Repository helper:

```go
package persistence

import (
    "context"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    platformtx "github.com/yourorg/bookstore-backend/internal/platform/tx"
)

type DBTX interface {
    Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
    Query(context.Context, string, ...any) (pgx.Rows, error)
    QueryRow(context.Context, string, ...any) pgx.Row
}

func dbtx(ctx context.Context, pool *pgxpool.Pool) DBTX {
    if tx, ok := platformtx.Extract(ctx); ok {
        return tx
    }
    return pool
}
```

---

## 12. Google Wire setup

### 12.1 Thư mục bootstrap

Khuyến nghị tạo:

```text
internal/bootstrap/
├── app.go
├── providers.go
├── wire.go
└── wire_gen.go
```

### 12.2 `providers.go`

Nơi gom provider sets theo nhóm:

```go
package bootstrap

import (
    "github.com/google/wire"
)

var PlatformSet = wire.NewSet(
    ProvideConfig,
    ProvideLogger,
    ProvideDBPool,
    ProvideTxManager,
    ProvideRedis,
    ProvideKafkaProducer,
)

var ModuleSet = wire.NewSet(
    ProvideAuthModule,
    ProvideCatalogModule,
    ProvideInventoryModule,
    ProvidePaymentModule,
    ProvideOrderModule,
)

var AppSet = wire.NewSet(
    PlatformSet,
    ModuleSet,
    ProvideHTTPServer,
    ProvideAPIApp,
)
```

### 12.3 `wire.go`

```go
//go:build wireinject
// +build wireinject

package bootstrap

import "github.com/google/wire"

func InitializeAPIApp() (*APIApp, func(), error) {
    wire.Build(AppSet)
    return nil, nil, nil
}

func InitializeWorkerApp() (*WorkerApp, func(), error) {
    wire.Build(WorkerSet)
    return nil, nil, nil
}

func InitializeSchedulerApp() (*SchedulerApp, func(), error) {
    wire.Build(SchedulerSet)
    return nil, nil, nil
}
```

### 12.4 Generate

```bash
wire ./internal/bootstrap
```

Wire sẽ tạo `wire_gen.go` chứa code injector thật.

### 12.5 Vì sao Wire hợp với bạn

Với modular monolith có nhiều module như `auth`, `catalog`, `order`, `payment`, `inventory`, `refund`, `notification`, `outbox`, `audit`, constructor graph sẽ dài rất nhanh. Wire giúp tách **bootstrap graph** khỏi business code nhưng vẫn giữ được type safety lúc compile. 

---

## 13. Provider design đúng cách với Wire

### Không nên

- Không cho Wire inject trực tiếp từ env rải rác.
- Không để provider vừa build dependency vừa chứa business logic.
- Không tạo một mega-provider trả về mọi thứ trong một hàm dài 500 dòng.

### Nên làm

- Mỗi platform service có provider riêng.
- Mỗi module có provider trả về đúng facade / service / handler mà binary cần.
- Dùng `func() cleanup` để đóng resource như DB pool, Kafka producer khi app shutdown.

Ví dụ:

```go
func ProvideDBPool(cfg *config.Config) (*pgxpool.Pool, func(), error) {
    pool, err := db.NewPool(cfg.DB)
    if err != nil {
        return nil, nil, err
    }
    cleanup := func() { pool.Close() }
    return pool, cleanup, nil
}
```

---

## 14. `cmd/api/main.go` theo hướng Wire

```go
package main

import (
    "log"

    "github.com/yourorg/bookstore-backend/internal/bootstrap"
)

func main() {
    app, cleanup, err := bootstrap.InitializeAPIApp()
    if err != nil {
        log.Fatalf("failed to initialize api app: %v", err)
    }
    defer cleanup()

    if err := app.Run(); err != nil {
        log.Fatalf("api app stopped with error: %v", err)
    }
}
```

### `cmd/worker/main.go`

```go
package main

import (
    "log"

    "github.com/yourorg/bookstore-backend/internal/bootstrap"
)

func main() {
    app, cleanup, err := bootstrap.InitializeWorkerApp()
    if err != nil {
        log.Fatalf("failed to initialize worker app: %v", err)
    }
    defer cleanup()

    if err := app.Run(); err != nil {
        log.Fatalf("worker app stopped with error: %v", err)
    }
}
```

### `cmd/scheduler/main.go`

```go
package main

import (
    "log"

    "github.com/yourorg/bookstore-backend/internal/bootstrap"
)

func main() {
    app, cleanup, err := bootstrap.InitializeSchedulerApp()
    if err != nil {
        log.Fatalf("failed to initialize scheduler app: %v", err)
    }
    defer cleanup()

    if err := app.Run(); err != nil {
        log.Fatalf("scheduler app stopped with error: %v", err)
    }
}
```

---

## 15. `cmd/migrate/main.go`

Nếu muốn có binary migration riêng ngoài CLI, dùng `golang-migrate` trực tiếp:

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/yourorg/bookstore-backend/internal/platform/config"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("usage: migrate [up|down|version]")
    }

    cfg := config.MustLoad()
    m, err := migrate.New(
        "file://"+cfg.DB.MigrationDir,
        cfg.DB.DSN(),
    )
    if err != nil {
        log.Fatal(err)
    }

    switch os.Args[1] {
    case "up":
        if err := m.Up(); err != nil && err != migrate.ErrNoChange {
            log.Fatal(err)
        }
    case "down":
        if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
            log.Fatal(err)
        }
    case "version":
        v, dirty, err := m.Version()
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("version=%d dirty=%v\n", v, dirty)
    default:
        log.Fatalf("unknown command: %s", os.Args[1])
    }
}
```

---

## 16. Cấu trúc file đề xuất theo hướng mới

```text
bookstore-backend/
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   ├── scheduler/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── bootstrap/
│   │   ├── app.go
│   │   ├── providers.go
│   │   ├── wire.go
│   │   └── wire_gen.go
│   ├── modules/
│   │   ├── auth/
│   │   ├── catalog/
│   │   ├── cart/
│   │   ├── order/
│   │   ├── payment/
│   │   ├── refund/
│   │   ├── inventory/
│   │   ├── notification/
│   │   └── outbox/
│   ├── platform/
│   │   ├── config/
│   │   ├── db/
│   │   ├── tx/
│   │   ├── redis/
│   │   ├── kafka/
│   │   ├── logger/
│   │   ├── httpx/
│   │   ├── errors/
│   │   └── validation/
│   └── shared/
├── migrations/
├── contracts/
├── docs/
└── test/
```

---

## 17. Trình tự setup thực tế nên làm

### Giai đoạn 1: nền project

```bash
go mod init github.com/yourorg/bookstore-backend
make tools
make infra-up
```

### Giai đoạn 2: config và platform

1. Viết `internal/platform/config`.
2. Viết `internal/platform/logger`.
3. Viết `internal/platform/db` bằng `pgxpool`.
4. Viết `internal/platform/tx` bằng `pgx.Tx`.
5. Viết `internal/platform/redis`.
6. Viết `internal/platform/httpx`.

### Giai đoạn 3: Wire bootstrap

1. Tạo `internal/bootstrap/providers.go`.
2. Tạo `internal/bootstrap/wire.go`.
3. Chạy `make wire`.
4. Đảm bảo `cmd/api/main.go` chỉ gọi injector.

### Giai đoạn 4: migration

```bash
make migrate-create name=init_extensions
make migrate-up
make migrate-version
```

### Giai đoạn 5: module đầu tiên

Bắt đầu với:
- `auth`
- `outbox`
- `audit`
- `catalog`

Vì đây là các module nền hoặc dependency sớm của các flow còn lại.

---

## 18. Checklist “project ready”

Bạn chỉ nên bắt đầu viết business module khi pass toàn bộ checklist sau:

- `go.mod` đúng module path.
- `.env.example` đã đủ biến cần thiết.
- `docker compose up -d` chạy ổn.
- PostgreSQL, Redis, Kafka, MinIO, MailHog, Asynqmon đều healthy.
- `make wire` generate thành công.
- `make dev` boot được API.
- `/health` trả 200.
- `make migrate-up` chạy được.
- `pgxpool.Ping()` pass.
- `make test` không panic.

---

## 19. Kết luận setup mới

Với hướng mới của bạn, setup đúng chuẩn là:

- **Google Wire** cho compile-time DI và bootstrap sạch.
- **pgx thuần** cho PostgreSQL layer và transaction boundary.
- **golang-migrate duy nhất** cho migration lifecycle.
- `main.go` cực mỏng, business code không lẫn bootstrap logic.
- module boundary vẫn giữ đúng tinh thần modular monolith master backend.

Nếu đi theo hướng này, project của bạn sẽ sạch hơn bản manual wiring cũ ở phần composition root, đồng thời vẫn giữ được độ explicit và type safety mà một backend Go lớn cần có.
