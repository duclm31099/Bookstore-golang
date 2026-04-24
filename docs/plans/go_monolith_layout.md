# Tài liệu Thiết kế Go Modular Monolith Layout - Bản Hoàn Chỉnh Theo SRD Revised

## 1. Mục đích tài liệu

Tài liệu này đặc tả **Go project layout** cho backend bookstore dạng **modular monolith** sử dụng Golang, Gin, PostgreSQL, Redis và Kafka.

Mục tiêu của tài liệu là biến SRD revised, ERD revised và module-boundary blueprint thành một cấu trúc mã nguồn có thể triển khai thật, giữ đúng tinh thần:

- correctness trước convenience;
- PostgreSQL là canonical source of truth;
- state ownership rõ ràng theo module;
- mutation quan trọng đi qua service layer có transaction boundary rõ;
- side effects async đi qua outbox;
- code được tổ chức theo **module nghiệp vụ** chứ không theo framework;
- implementation persistence dùng **pgx**, không dùng `sqlc`.

Tài liệu này không chỉ mô tả cây thư mục, mà còn chốt các quy tắc về:

- module ownership;
- dependency direction;
- transaction boundary;
- event publishing boundary;
- repository responsibilities;
- service orchestration;
- HTTP adapter responsibilities;
- worker responsibilities;
- test organization;
- shared platform packages;
- anti-patterns phải tránh;
- cách áp dụng `pgx` vào layout thực tế.

---

## 2. Nguyên tắc kiến trúc áp dụng vào layout

### 2.1 Nguyên tắc tổng quát

- Ứng dụng được deploy như một backend runtime dạng monolith, nhưng bên trong phải chia thành **internal modules** tách biệt về trách nhiệm.
- Không tổ chức code theo kiểu “controllers/services/repositories toàn cục”, vì mô hình đó nhanh bị rối ở các flow payment, order, refund, entitlement, reservation và audit.
- Mỗi module phải sở hữu rõ aggregate, service, repository interface, event contract và rule của chính nó.
- Module được phép giao tiếp với module khác qua service interface nội bộ, query contract hoặc event contract; không được import repository implementation của module khác.
- Shared platform code phải rất mỏng; business logic không được trôi vào `pkg/common`, `shared/service`, `utils` hoặc `helpers`.
- Tất cả business mutation quan trọng phải đi qua application/service layer có transaction boundary rõ ràng.
- Mọi event publish sang Kafka phải đi qua outbox persistence trong transaction owner thay vì publish trực tiếp từ handler hoặc provider callback.
- Redis không được là canonical source cho order/payment/refund/inventory reservation state.
- Layout phải phục vụ được cả public API, admin API, webhook, worker, scheduler-like jobs và migration.

### 2.2 Định hướng master backend

Một backend engineer mạnh không bắt đầu từ HTTP handler, mà bắt đầu từ các câu hỏi sau:

1. State nào là canonical và module nào sở hữu state đó?
2. Việc ghi dữ liệu nào phải nằm chung transaction?
3. Side effect nào synchronous, side effect nào async qua outbox?
4. Nếu retry, duplicate webhook, crash giữa chừng, dữ liệu có bị sai không?
5. Package boundary hiện tại có phản ánh đúng business boundary không?
6. Runtime nào cần tách riêng: API, worker, migrate?

### 2.3 Tiêu chí đánh giá layout tốt

Một layout tốt cho project này phải đáp ứng:

- Có thể tìm nhanh owner của một bảng hoặc một flow.
- Không tạo circular dependency giữa các business modules.
- Tách rõ platform concern với business concern.
- Dễ viết integration test theo module.
- Dễ thêm worker/outbox consumer mà không làm bẩn HTTP layer.
- Dễ mở rộng từ read-only logic sang flow giao dịch mà không phá kiến trúc.
- Dễ reason về transaction, idempotency, retries và failure recovery.
- Không ép team phải đi qua generic abstraction vô nghĩa.

---

## 3. Module decomposition theo SRD

### 3.1 Danh sách module chính

Các module nội bộ nên gồm:

- `identity`
- `catalog`
- `pricing`
- `membership`
- `entitlement`
- `reader`
- `cart`
- `order`
- `payment`
- `refund`
- `chargeback`
- `inventory`
- `shipment`
- `notification`
- `audit`
- `integration`
- `invoice`
- `reporting`
- `search`
- `admin`

### 3.2 Ghi chú về các module không nên hiểu sai

- `admin` là application/API composition layer cho backoffice, **không phải** nơi sở hữu mọi business state.
- `integration` sở hữu outbox, processed_events và event dispatch/consumer idempotency infra; không phải nơi nhét business logic của các module khác.
- `reporting` và `search` là projection/read-model oriented modules; không sở hữu canonical transactional state.
- `reader` sở hữu reader sessions và progress online reading.
- `downloads` trong phase 1 nên được đặt dưới `entitlement` thay vì tách thành module độc lập, vì flow issue signed-link phụ thuộc chặt vào entitlement, policy và quota.
- `checkout` không cần là owner module riêng; checkout orchestration nên nằm ở `order/app`.

### 3.3 Ownership theo module

- `identity` sở hữu: `users`, `user_credentials`, `user_sessions`, `user_devices`, `addresses`.
- `catalog` sở hữu: `books`, `book_formats`, `authors`, `categories`, `book_authors`, `book_categories`, `sellable_skus`, `search_documents` source metadata.
- `pricing` sở hữu: `prices`, effective price resolution, campaign/coupon calculation helpers ở mức pricing.
- `membership` sở hữu: `membership_plans`, `memberships`.
- `entitlement` sở hữu: `entitlements`, `downloads`, digital access policy, download-link issuance policy.
- `reader` sở hữu: `reader_sessions`, reading progress và session concurrency logic.
- `cart` sở hữu: `carts`, `cart_items`, cart repricing preparation.
- `order` sở hữu: `orders`, `order_items`, `order_state_logs`, checkout/order creation orchestration.
- `payment` sở hữu: `payments`, `payment_attempts`, provider callback/reconciliation logic.
- `refund` sở hữu: `refunds`, refund workflow và refund state machine.
- `chargeback` sở hữu: `chargebacks`, dispute lifecycle.
- `inventory` sở hữu: `inventory_items`, `inventory_reservations`.
- `shipment` sở hữu: `shipments`, `shipment_status_logs`.
- `notification` sở hữu: `notifications` và email command execution.
- `audit` sở hữu: `audit_logs`.
- `integration` sở hữu: `outbox_events`, `processed_events`, dispatching và consumer dedup infra.
- `invoice` sở hữu: `e_invoice_exports` và provider export orchestration.
- `reporting` sở hữu: reporting projections, aggregate reporting feeds nếu có.
- `search` sở hữu: search projection/cache orchestration nếu tách riêng khỏi catalog query side.
- `admin` sở hữu: admin route composition, authorization entry checks, admin-facing orchestration; không sở hữu canonical table nghiệp vụ lớn.

### 3.4 Ownership matrix rút gọn

| Aggregate / Table                                     | Owner module   |
| ----------------------------------------------------- | -------------- |
| `users`, `user_sessions`, `user_devices`, `addresses` | `identity`     |
| `books`, `book_formats`, `sellable_skus`              | `catalog`      |
| `prices`                                              | `pricing`      |
| `membership_plans`, `memberships`                     | `membership`   |
| `entitlements`, `downloads`                           | `entitlement`  |
| `reader_sessions`                                     | `reader`       |
| `carts`, `cart_items`                                 | `cart`         |
| `orders`, `order_items`, `order_state_logs`           | `order`        |
| `payments`, `payment_attempts`                        | `payment`      |
| `refunds`                                             | `refund`       |
| `chargebacks`                                         | `chargeback`   |
| `inventory_items`, `inventory_reservations`           | `inventory`    |
| `shipments`, `shipment_status_logs`                   | `shipment`     |
| `notifications`                                       | `notification` |
| `audit_logs`                                          | `audit`        |
| `outbox_events`, `processed_events`                   | `integration`  |
| `e_invoice_exports`                                   | `invoice`      |

---

## 4. Dependency direction và boundary rules

### 4.1 Hướng dependency chuẩn

Dependency chỉ được chảy theo hướng:

```text
domain <- app <- interfaces
       <- infra
```

Giải thích:

- `domain`: business rules, state transitions, invariants, domain errors, value objects.
- `app`: orchestration use case, transaction boundary, phối hợp repository và ports.
- `infra`: PostgreSQL, Redis, Kafka, provider adapters, storage adapters.
- `interfaces`: HTTP handlers, webhooks, consumers, jobs, CLI entrypoints.

### 4.2 Quy tắc giao tiếp giữa các module

Được phép:

- gọi application port nội bộ đã được publish rõ ràng;
- gọi query service/read contract nếu chỉ cần đọc;
- publish/consume Kafka event;
- dùng shared primitive thật sự generic.

Không được phép:

- import repository implementation của module khác;
- gọi trực tiếp vào `infra/postgres` của module khác;
- mutate canonical table của module khác bằng raw SQL;
- lôi DTO HTTP của module này sang module khác làm internal contract;
- nhét business logic cross-module vào `shared` hoặc `platform`.

### 4.3 Nguyên tắc owner module

- Module nào sở hữu canonical state thì module đó sở hữu mutation logic của state đó.
- Module khác muốn đổi state phải gọi port/service của owner hoặc publish event để owner xử lý.
- Worker và consumer cũng phải tuân thủ nguyên tắc này; không có ngoại lệ chỉ vì “đang ở background”.

---

## 5. Cây thư mục Go đề xuất

```text
bookstore-backend/
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── configs/
│   ├── app.example.yaml
│   ├── app.dev.yaml
│   ├── app.staging.yaml
│   └── app.prod.yaml
├── contracts/
│   ├── openapi/
│   │   ├── public.yaml
│   │   ├── admin.yaml
│   │   └── webhooks.yaml
│   └── events/
│       ├── order.events.v1.yaml
│       ├── payment.events.v1.yaml
│       ├── membership.events.v1.yaml
│       ├── entitlement.events.v1.yaml
│       ├── inventory.events.v1.yaml
│       ├── shipment.events.v1.yaml
│       └── notification.commands.v1.yaml
├── deployments/
│   ├── docker/
│   ├── compose/
│   └── k8s/
├── docs/
│   ├── specs/
│   │   ├── srd_revised.md
│   │   ├── erd_revised.md
│   │   ├── module_boundary.md
│   │   ├── state_transition_spec.md
│   │   └── redis_kafka_specs.md
│   ├── adr/
│   └── runbooks/
├── migrations/
│   ├── 000001_init_extensions.up.sql
│   ├── 000001_init_extensions.down.sql
│   ├── 000002_init_tables.up.sql
│   ├── 000002_init_tables.down.sql
│   └── seeds/
├── internal/
│   ├── bootstrap/
│   │   ├── app.go
│   │   ├── config.go
│   │   ├── platform.go
│   │   ├── modules.go
│   │   ├── http.go
│   │   └── worker.go
│   ├── platform/
│   │   ├── config/
│   │   ├── db/
│   │   ├── tx/
│   │   ├── logger/
│   │   ├── tracing/
│   │   ├── httpx/
│   │   ├── auth/
│   │   ├── redisx/
│   │   ├── kafkax/
│   │   ├── clock/
│   │   ├── idgen/
│   │   ├── idempotency/
│   │   ├── errors/
│   │   ├── validation/
│   │   └── observability/
│   ├── shared/
│   │   ├── money/
│   │   ├── pagination/
│   │   ├── ptr/
│   │   ├── slices/
│   │   └── nullable/
│   └── modules/
│       ├── identity/
│       ├── catalog/
│       ├── pricing/
│       ├── membership/
│       ├── entitlement/
│       ├── reader/
│       ├── cart/
│       ├── order/
│       ├── payment/
│       ├── refund/
│       ├── chargeback/
│       ├── inventory/
│       ├── shipment/
│       ├── notification/
│       ├── audit/
│       ├── integration/
│       ├── invoice/
│       ├── reporting/
│       ├── search/
│       └── admin/
├── test/
│   ├── integration/
│   ├── contract/
│   ├── fixtures/
│   ├── e2e/
│   └── concurrency/
├── scripts/
├── Makefile
├── go.mod
└── README.md
```

### 5.1 Giải thích chọn 3 binary

- `cmd/api`: HTTP runtime cho public/admin API và webhook endpoints.
- `cmd/worker`: outbox dispatcher, Kafka consumers, periodic jobs, reconciliation jobs, sweepers.
- `cmd/migrate`: chạy migrations độc lập với app startup.

### 5.2 Vì sao không cần `cmd/scheduler` riêng ở phase 1

- Với phase 1, scheduler-like jobs có thể chạy trong `worker` runtime bằng ticker/cron registry nội bộ.
- Chỉ tách `cmd/scheduler` khi operational isolation, permission scope hoặc resource profile thực sự đòi hỏi.
- Cách này giúp đơn giản hóa deployment ban đầu nhưng vẫn không phá kiến trúc.

---

## 6. Thiết kế `internal/platform/`

`internal/platform/` là tầng hạ tầng kỹ thuật dùng chung, không chứa nghiệp vụ bookstore.

### 6.1 `platform/config/`

Chứa:

- load config từ env và YAML;
- validate config ngay khi startup;
- tách cấu hình theo domain kỹ thuật: DB, Redis, Kafka, auth, email, payment provider, object storage, rate limiting.

Nguyên tắc:

- fail fast nếu thiếu config critical;
- không đọc env trực tiếp rải rác trong code.

### 6.2 `platform/db/`

Chứa:

- khởi tạo `pgxpool.Pool`;
- helper healthcheck DB;
- executor abstractions cần thiết cho repository;
- không đặt query business cụ thể tại đây.

`platform/db` phải được thiết kế theo hướng `pgx-first`, không sinh code SQL tự động.

Ví dụ abstraction:

```go
type DBTX interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

### 6.3 `platform/tx/`

Chứa transaction manager dùng chung.

Pseudo interface:

```go
type Manager interface {
    WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Nguyên tắc:

- app service gọi `WithinTransaction`;
- repository implementation lấy executor hiện hành từ context hoặc unit-of-work scope;
- tránh nested transaction tùy tiện; nếu cần savepoint thì phải explicit.

### 6.4 `platform/logger/`

- structured JSON logging;
- field chuẩn: `request_id`, `trace_id`, `correlation_id`, `user_id`, `order_id`, `payment_id`, `job_name`, `event_id` khi có.

### 6.5 `platform/tracing/`

- OpenTelemetry hooks;
- trace propagation qua HTTP, Kafka, worker jobs;
- hỗ trợ trace xuyên suốt từ API -> transaction -> outbox -> consumer.

### 6.6 `platform/httpx/`

- router bootstrap;
- middleware chung: recovery, request-id, logging, tracing, auth context, error rendering, rate limit, idempotency adapter;
- không chứa handler business cụ thể.

### 6.7 `platform/auth/`

- JWT signing/verification hoặc session token helpers;
- password hash/verify adapter;
- claims / RBAC parsing helper;
- không sở hữu user/session business state, vì phần đó thuộc `identity`.

### 6.8 `platform/redisx/`

- Redis client factory;
- namespaced key helper;
- Lua script wrapper nếu cần;
- metrics/tracing wrapper cho Redis ops.

Không chứa:

- key business-specific của từng module;
- quota policy cụ thể;
- reservation policy.

### 6.9 `platform/kafkax/`

- producer wrapper;
- consumer runner;
- retry / DLQ base;
- envelope validation;
- tracing propagation.

### 6.10 `platform/idempotency/`

- helper middleware/store abstraction cho idempotency key;
- có thể dùng Redis cho hot path;
- canonical effect vẫn do DB + owner module quyết định.

### 6.11 `platform/errors/`

- chuẩn hóa machine-readable error codes;
- mapping technical errors và business errors;
- mapping ra HTTP status và worker retry policy.

### 6.12 `platform/validation/`

- generic input validation helpers;
- không chứa business rules đặc thù domain.

### 6.13 `platform/observability/`

- metrics registry;
- health/readiness checks;
- probes cho DB, Redis, Kafka producer/consumer health.

---

## 7. Thiết kế `internal/shared/`

`internal/shared/` chỉ nên chứa primitive generic, nhỏ và ổn định.

### 7.1 Shared package hợp lệ

- `money`
- `pagination`
- `ptr`
- `slices`
- `nullable`

### 7.2 Shared package không nên tồn tại

- `businessutils`
- `servicehelpers`
- `models` toàn cục
- `constants` toàn cục chứa state của mọi domain
- `repositories` dùng chung cho mọi module

Nguyên tắc:

- nếu package có business meaning rõ thì nó nên ở module owner, không nên ở `shared`.

---

## 8. Cấu trúc chuẩn bên trong mỗi module

Mỗi module nên có cấu trúc nhất quán nhưng không bị giáo điều quá mức.

```text
internal/modules/order/
├── domain/
│   ├── entity/
│   ├── valueobject/
│   ├── policy/
│   ├── event/
│   ├── error/
│   └── repository.go
├── app/
│   ├── command/
│   ├── query/
│   ├── service/
│   ├── dto/
│   └── ports/
├── infra/
│   ├── postgres/
│   ├── redis/
│   ├── providers/
│   └── mapper/
├── interfaces/
│   ├── http/
│   ├── consumer/
│   ├── jobs/
│   └── webhook/
└── tests/
```

### 8.1 Ý nghĩa từng tầng

#### `domain/`

Chứa:

- entity, value object, state enum;
- invariant và transition policy;
- domain errors;
- repository interfaces hoặc aggregate persistence contract ở mức module.

Không được:

- import Gin, pgx client cụ thể, Redis client, Kafka client;
- parse HTTP request;
- gọi network ra ngoài.

#### `app/`

Chứa:

- use cases mutation và query;
- orchestration;
- transaction boundary;
- gọi repository và module ports;
- DTO nội bộ.

Không được:

- nhúng SQL raw trực tiếp trong service;
- encode state machine rải rác thay domain policy;
- tự sửa canonical state của module khác.

#### `infra/`

Chứa:

- Postgres repositories viết tay bằng `pgx`;
- Redis cache/quota implementation nếu module cần;
- provider adapters;
- mapper giữa DB rows và domain model.

#### `interfaces/`

Chứa:

- HTTP handlers;
- webhook handlers;
- Kafka consumers;
- scheduler-like jobs;
- request/response mapping.

### 8.2 Biến thể gọn cho module nhỏ

Không bắt buộc module nào cũng phải có đủ `redis/`, `providers/`, `consumer/`, `webhook/`.  
Chỉ tạo subfolder khi module thực sự có trách nhiệm đó.

---

## 9. Chuẩn triển khai `pgx` thay cho `sqlc`

### 9.1 Nguyên tắc chính

Project này dùng `pgx`, không dùng `sqlc`.

Điều đó có nghĩa:

- SQL được viết tay theo từng module owner;
- repository/query repository nằm trong `infra/postgres`;
- mapping row -> domain model được viết rõ ràng;
- không generate query layer từ `.sql` files;
- tránh generic repository abstraction quá mức.

### 9.2 Layout gợi ý cho `infra/postgres`

```text
internal/modules/order/infra/postgres/
├── repository.go
├── query_repository.go
├── scan.go
├── mapper.go
├── sql.go
└── filters.go
```

Ý nghĩa:

- `repository.go`: write-side / aggregate persistence.
- `query_repository.go`: read-side optimized queries.
- `scan.go`: scan helpers từ `pgx.Row` / `pgx.Rows`.
- `mapper.go`: mapping DB model -> domain / DTO.
- `sql.go`: khai báo SQL constants theo nhóm use case nếu cần.
- `filters.go`: build dynamic filtering có kiểm soát.

### 9.3 Interface executor cho pgx

Repository nên nhận abstraction kiểu `DBTX` để chạy được cả trên `pgxpool.Pool` và `pgx.Tx`.

Ví dụ:

```go
type DBTX interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

### 9.4 Query repository và write repository

Mỗi module có thể có:

- `Repository` cho aggregate canonical và write flows;
- `QueryRepository` cho list/detail/report optimized.

Ví dụ:

- `order.domain.Repository` để load/save aggregate;
- `order.app.query.Repository` để list orders với joins.

### 9.5 Nguyên tắc SQL

- SQL nằm gần module owner;
- query cho admin list/report có thể join nhiều bảng;
- query read được join cross-module nếu chỉ phục vụ response;
- mutation vẫn phải đi qua owner service.

---

## 10. Thiết kế `cmd/` và composition root

### 10.1 `cmd/api/main.go`

Trách nhiệm:

- load config;
- init logger/tracing;
- init DB/Redis/Kafka producer;
- build module graph;
- register routes/middlewares;
- start HTTP server.

Không nên:

- chứa raw SQL;
- chứa business logic;
- branching theo provider business flow.

### 10.2 `cmd/worker/main.go`

Trách nhiệm:

- init consumer runner;
- register outbox dispatcher;
- register notification worker;
- register reservation sweeper;
- register payment reconcile jobs;
- register membership expiry jobs;
- register invoice export jobs;
- run graceful shutdown.

### 10.3 `cmd/migrate/main.go`

Trách nhiệm:

- chạy migrations;
- validate DB connectivity;
- không share startup path với API runtime trong production.

### 10.4 `internal/bootstrap/`

`internal/bootstrap/` là composition root thực sự của repo.

Nên có:

- `config.go`: load/validate config.
- `platform.go`: init DB, Redis, Kafka, logger, tracer, tx manager.
- `modules.go`: wiring repositories, services, ports.
- `http.go`: register HTTP routes.
- `worker.go`: register consumers và periodic jobs.
- `app.go`: app container cấp cao nếu cần.

Nguyên tắc:

- wiring explicit;
- không dùng service locator mơ hồ;
- không truyền “god container” xuống module business.

---

## 11. Thiết kế các module trọng yếu

## 11.1 Module `identity`

### Ownership

- `users`
- `user_credentials`
- `user_sessions`
- `user_devices`
- `addresses`

### Public app services đề xuất

- `RegisterUser`
- `VerifyEmail`
- `Login`
- `RefreshSession`
- `Logout`
- `ListUserSessions`
- `RevokeSession`
- `ListUserDevices`
- `RevokeDevice`
- `CreateAddress`
- `UpdateAddress`
- `DeleteAddress`

### Rule

- `identity` là owner của session/device/account state.
- `platform/auth` chỉ hỗ trợ token/password primitives, không sở hữu lifecycle business.

## 11.2 Module `catalog`

### Ownership

- `books`
- `book_formats`
- `authors`
- `categories`
- `book_authors`
- `book_categories`
- `sellable_skus`
- `search_documents` source metadata

### Public app services đề xuất

- `CreateBook`
- `UpdateBook`
- `PublishBook`
- `UnpublishBook`
- `GetBookDetail`
- `ListBooks`
- `ListBookFormats`
- `GetSellableSKU`

### Rule

- catalog không grant entitlement;
- catalog không tính commercial final rights;
- catalog chỉ định nghĩa sản phẩm và metadata bán được.

## 11.3 Module `pricing`

### Ownership

- `prices`
- effective price resolution logic

### Public app services đề xuất

- `ResolveEffectivePrice`
- `PreviewCartPricing`
- `ValidateCoupon`
- `BuildPricingSnapshot`

### Rule

- giá canonical phải resolve ở server;
- cart chỉ giữ preview snapshot;
- final totals vẫn phải được tính lại tại checkout/order create.

## 11.4 Module `membership`

### Ownership

- `membership_plans`
- `memberships`

### Public app services đề xuất

- `ListPlans`
- `CreatePlan`
- `UpdatePlan`
- `ActivateMembership`
- `ExtendMembership`
- `ExpireMembership`
- `RevokeMembership`

### Rule

- membership là owner của lifecycle plan/subscription;
- entitlement consume membership outcome nhưng không tự sửa `memberships`.

## 11.5 Module `entitlement`

### Ownership

- `entitlements`
- `downloads`

### Public app services đề xuất

- `GrantPurchaseEntitlement`
- `GrantMembershipEntitlements`
- `RevokeEntitlement`
- `IssueDownloadLink`
- `MarkDownloadConsumed`
- `ListLibrary`
- `CheckCommercialRight`

### Rule

- entitlement phải materialize rights canonical;
- không suy luận ad-hoc từ order/membership ở mỗi request;
- signed download URL chỉ được issue qua module này.

## 11.6 Module `reader`

### Ownership

- `reader_sessions`

### Public app services đề xuất

- `StartReaderSession`
- `HeartbeatReaderSession`
- `EndReaderSession`
- `GetReadAccess`

### Rule

- `reader` kiểm tra entitlement qua port/query contract;
- `reader` không tự cấp download token.

## 11.7 Module `cart`

### Ownership

- `carts`
- `cart_items`

### Public app services đề xuất

- `GetActiveCart`
- `AddItem`
- `UpdateItemQty`
- `RemoveItem`
- `ApplyCoupon`
- `RemoveCoupon`
- `PreviewCheckout`

### Rule

- cart là mutable convenience state;
- cart không phải bằng chứng giao dịch;
- cart không được tạo order bằng SQL trực tiếp.

## 11.8 Module `order`

### Ownership

- `orders`
- `order_items`
- `order_state_logs`

### Public app services đề xuất

- `CreateOrderFromCheckout`
- `CreateCODOrder`
- `CreateOnlinePaymentOrder`
- `GetOrderDetail`
- `ListOrdersByUser`
- `CancelOrder`
- `TransitionOrderState`
- `RecordOrderStateLog`

### Rule

- order là owner của `order_state`;
- payment/shipment/refund không được tự update `orders` bằng repository trực tiếp;
- checkout orchestration nên nằm ở đây thay vì tách module `checkout`.

## 11.9 Module `payment`

### Ownership

- `payments`
- `payment_attempts`

### Public app services đề xuất

- `InitiatePayment`
- `HandleGatewayWebhook`
- `ReconcilePayment`
- `MarkPaymentCaptured`
- `MarkPaymentFailed`
- `MarkPaymentExpired`

### Rule

- payment là owner của `payment_state`;
- order commercial state phải phối hợp qua contract/transaction orchestration phù hợp;
- callback/webhook phải idempotent.

## 11.10 Module `refund`

### Ownership

- `refunds`

### Public app services đề xuất

- `CreateRefundRequest`
- `ApproveRefund`
- `MarkRefundProcessing`
- `MarkRefundSucceeded`
- `MarkRefundFailed`

### Rule

- refund không nên nhét vào payment module;
- refund lifecycle độc lập và có tác động tới order, entitlement, inventory.

## 11.11 Module `chargeback`

### Ownership

- `chargebacks`

### Public app services đề xuất

- `OpenChargebackCase`
- `UpdateChargebackState`
- `ResolveChargeback`

### Rule

- chargeback là dispute lifecycle riêng;
- entitlement/order/payment consume outcome nhưng không tự sửa canonical dispute state.

## 11.12 Module `inventory`

### Ownership

- `inventory_items`
- `inventory_reservations`

### Public app services đề xuất

- `ReserveStock`
- `ReleaseReservation`
- `ConsumeReservation`
- `SweepExpiredReservations`
- `AdjustInventory`

### Rule

- inventory canonical reference phải bám theo `sellable_sku_id`;
- chỉ module này được quyết định stock mutation;
- online payment physical order phải có reservation record với TTL trong PostgreSQL.

## 11.13 Module `shipment`

### Ownership

- `shipments`
- `shipment_status_logs`

### Public app services đề xuất

- `CreateShipment`
- `UpdateShipmentStatus`
- `SyncCarrierStatus`
- `GetShipmentByOrder`

### Rule

- shipment là owner của `shipment_state`;
- order chỉ consume outcome để derive order transition phù hợp.

## 11.14 Module `notification`

### Ownership

- `notifications`

### Public app services đề xuất

- `EnqueueEmail`
- `ProcessEmailJob`
- `RetryEmailJob`
- `CancelNotification`

### Rule

- module khác chỉ enqueue command hoặc append outbox event;
- không gửi email trực tiếp trong transaction business.

## 11.15 Module `audit`

### Ownership

- `audit_logs`

### Public app services đề xuất

- `AppendAuditLog`

### Rule

- audit append nên là capability được gọi trong owner transaction khi cần;
- admin override nhạy cảm phải có audit.

## 11.16 Module `integration`

### Ownership

- `outbox_events`
- `processed_events`

### Public app services đề xuất

- `AppendOutboxEvent`
- `DispatchPendingOutbox`
- `MarkProcessedEvent`
- `HasProcessedEvent`

### Rule

- không module nào publish Kafka trực tiếp từ handler path cho business event bắt buộc;
- consumer idempotency phải đi qua `processed_events` hoặc cơ chế tương đương.

## 11.17 Module `invoice`

### Ownership

- `e_invoice_exports`

### Public app services đề xuất

- `RequestInvoiceExport`
- `RetryInvoiceExport`
- `MarkInvoiceExported`
- `MarkInvoiceExportFailed`

### Rule

- export state canonical thuộc invoice module;
- tên trạng thái canonical nên là `export_state`.

## 11.18 Module `reporting`

### Ownership

- reporting projections / feed tables nếu có

### Rule

- không sở hữu canonical order/payment state;
- consume event để build projections.

## 11.19 Module `search`

### Ownership

- search projection/cache orchestration nếu cần

### Rule

- phase 1 có thể đơn giản, dùng PostgreSQL FTS;
- module này chỉ tồn tại nếu cần query/search projection tách khỏi catalog.

## 11.20 Module `admin`

### Ownership

- không sở hữu canonical business aggregate lớn

### Rule

- admin là route/composition/orchestration layer cho backoffice;
- admin phải gọi owner module tương ứng;
- tuyệt đối không biến admin thành “god service”.

---

## 12. HTTP adapter design

### 12.1 Route grouping đề xuất

- `/api/v1/auth/*`
- `/api/v1/me/*`
- `/api/v1/books/*`
- `/api/v1/cart/*`
- `/api/v1/checkout/*`
- `/api/v1/orders/*`
- `/api/v1/payments/*`
- `/api/v1/reader/*`
- `/api/v1/admin/*`

### 12.2 Handler rules

- Handler không mở transaction business thủ công.
- Handler không emit Kafka event trực tiếp.
- Handler không encode state machine.
- Handler chỉ map request -> command -> service -> response.
- Authorization entry check có thể nằm ở handler/middleware, nhưng business permission sâu vẫn phải được service kiểm tra nếu cần.

### 12.3 Webhook rules

Webhook của provider phải nằm trong module owner:

- payment webhook ở `payment/interfaces/webhook`;
- shipment carrier webhook ở `shipment/interfaces/webhook`;
- invoice provider callback ở `invoice/interfaces/webhook`.

Luồng chuẩn:

1. verify signature/authenticity;
2. parse payload;
3. normalize thành command nội bộ;
4. gọi app service;
5. trả response phù hợp.

---

## 13. Worker runtime design

### 13.1 Worker categories

- outbox dispatcher;
- notification email worker;
- reservation expiry sweeper;
- payment reconciliation worker;
- membership expiry worker;
- invoice export worker;
- shipment polling/sync worker;
- stale reader session cleanup.

### 13.2 Worker rules

- Worker không tự mutate cross-domain bằng SQL thẳng.
- Worker gọi app service của module owner.
- Consumer phải idempotent bằng `processed_events` hoặc dedup policy tương đương.
- Duplicate delivery phải an toàn.
- Kafka unavailable không được làm sai canonical business state; outbox backlog được chấp nhận.

### 13.3 Scheduled jobs trong worker

Periodic jobs có thể đăng ký tại `worker` runtime:

- payment reconcile pending;
- membership expiry sweep;
- inventory reservation timeout release;
- stale reader session cleanup;
- shipment poll batch;
- outbox republish retry sweep.

---

## 14. Transaction boundary design

### 14.1 Các flow phải có transaction mạnh

- Create order + order items + order_state_log + initial payment record nếu cần + outbox append.
- Payment captured -> update payment + transition order + grant entitlement hoặc append reliable action + audit + outbox.
- Reserve inventory / release inventory / consume reservation.
- Refund create/update canonical state ban đầu.
- Membership activation/extension.
- Admin override nhạy cảm + audit append.

### 14.2 Cách encode trong code

- Transaction boundary nằm ở app service của module owner.
- Repository methods nhận `DBTX` / executor abstraction.
- Outbox append được gọi trong cùng transaction context.
- Không gọi external network bên trong transaction dài nếu không cần thiết.

---

## 15. Query design và read/write separation

### 15.1 Nguyên tắc

Không cần CQRS đầy đủ ở phase 1, nhưng cần tách tư duy:

- write services chịu trách nhiệm invariant;
- read queries tối ưu cho response shape;
- query service có thể join nhiều bảng miễn không phá ownership ghi.

### 15.2 Query repository

Mỗi module có thể có:

- `Repository` cho aggregate canonical;
- `QueryRepository` cho list/detail/report optimized.

Ví dụ:

- `order/domain.Repository`
- `order/app/query.Repository`

### 15.3 Read side cross-module

- order detail query có thể join orders + items + payments + shipments;
- admin list query có thể join nhiều bảng;
- nhưng mutation vẫn phải quay về owner service của từng module.

---

## 16. Testing layout

### 16.1 Unit tests

Mỗi module nên có unit tests cho:

- state transition rules;
- validation logic;
- pricing calculation;
- reservation release conditions;
- entitlement policy logic;
- refund/chargeback outcome rules.

### 16.2 Integration tests

`test/integration/` nên có:

- bootstrap DB từ migrations;
- seed fixtures;
- repository tests;
- transaction flow tests: create order, payment capture, inventory reserve, entitlement grant, refund flow.

### 16.3 Contract tests

`test/contract/` dùng để kiểm tra:

- OpenAPI response shape;
- webhook contracts;
- Kafka event envelope/payload contract.

### 16.4 E2E tests

`test/e2e/` cho happy path chính:

- register -> verify -> login;
- browse -> cart -> checkout -> pay -> library/download;
- COD physical order -> shipment delivered -> paid;
- refund / entitlement revoke path.

### 16.5 Concurrency tests

`test/concurrency/` nên có:

- inventory reservation race;
- duplicate webhook delivery;
- repeated download issue request;
- reader session concurrency cap.

---

## 17. Naming conventions và coding conventions

### 17.1 File naming

- `service.go` cho entry service file.
- `repository.go` cho interface hoặc main repo implementation file.
- `query_repository.go` cho read model repository.
- `state.go` hoặc `states.go` cho constants + transition helpers.
- `mapper.go` cho mapping DB row <-> domain / DTO.
- `routes.go` cho route registration.
- `handler.go` cho HTTP/webhook handler.
- `consumer.go` cho Kafka consumer entrypoints.

### 17.2 Interface placement

- Interface đặt ở nơi **tiêu thụ** interface, không phải nơi triển khai.
- Ví dụ order cần inventory capability thì `InventoryPort` nên đặt ở `order/app/ports/`.

### 17.3 DTO policy

- Không expose trực tiếp DB row struct ra API.
- Request DTO, Response DTO, Domain Model là các tầng khác nhau.
- Provider payload models không được leak vào domain model.

### 17.4 Tên package nên tránh

- `common`
- `base`
- `utils`
- `helpers`
- `manager`
- `business`

Nếu phải dùng những tên này, gần như chắc chắn boundary đang bị mờ.

---

## 18. Error handling blueprint

### 18.1 Phân loại lỗi

| Nhóm lỗi                    | Ví dụ                                  | Hướng xử lý                      |
| --------------------------- | -------------------------------------- | -------------------------------- |
| Business rule violation     | COD không hợp lệ, entitlement expired  | 4xx domain error                 |
| Idempotency replay/conflict | duplicate checkout, duplicate callback | 200 replay hoặc 409 tùy use case |
| Infra transient             | Redis timeout, Kafka timeout           | retry / 5xx                      |
| External permanent          | payload invalid, signature sai         | 4xx hoặc park/DLQ                |
| Internal bug                | invariant broken, nil pointer          | 5xx + alert                      |

### 18.2 Ví dụ domain errors

- `ErrIllegalTransition`
- `ErrOrderNotCancelable`
- `ErrPaymentAlreadyCaptured`
- `ErrMembershipExpired`
- `ErrConcurrentReaderLimitExceeded`
- `ErrInsufficientStock`
- `ErrDownloadPolicyViolated`

---

## 19. Observability blueprint

### 19.1 Logging

Structured logs bắt buộc có thể chứa:

- `trace_id`
- `correlation_id`
- `request_id`
- `user_id`
- `order_id`
- `payment_id`
- `membership_id`
- `book_id`
- `event_id`
- `job_name`

### 19.2 Metrics

Theo module cần có:

- HTTP latency/error rate;
- DB query latency;
- Redis hit/miss/latency;
- Kafka consumer lag;
- outbox backlog;
- payment callback success/fail/dedup;
- entitlement grant/revoke counts;
- download issue/consume counts;
- inventory reserve conflict rate.

### 19.3 Tracing

Trace xuyên:

- HTTP request -> transaction -> outbox append -> consumer -> notification/invoice/payment side effect.

---

## 20. Anti-patterns phải tránh

- `internal/common` thành nơi chứa business logic của mọi module.
- Một service dài hàng nghìn dòng xử lý cả order, payment, inventory, entitlement, refund.
- Handler gọi repository trực tiếp rồi publish Kafka.
- Module A import SQL repo của module B.
- Consumer sửa state chéo module bằng raw SQL.
- Dùng Redis như canonical store cho reservation/order/payment state.
- Nhét mọi interface vào package `ports` toàn cục.
- Dùng generic CRUD repository cho mọi aggregate.
- Dùng `sqlc` generated layer rồi để query ownership bị mờ, nếu team không thực sự muốn workflow đó.
- Để `admin` thành nơi mutate thẳng tất cả bảng.

---

## 21. Ví dụ layout chi tiết cho một số module

### 21.1 `order`

```text
internal/modules/order/
├── domain/
│   ├── entity/
│   │   ├── order.go
│   │   └── order_item.go
│   ├── valueobject/
│   │   ├── order_state.go
│   │   └── order_no.go
│   ├── policy/
│   │   ├── transition_policy.go
│   │   └── checkout_policy.go
│   ├── event/
│   │   ├── order_created.go
│   │   ├── order_paid.go
│   │   └── order_cancelled.go
│   ├── error/
│   │   └── errors.go
│   └── repository.go
├── app/
│   ├── command/
│   │   ├── create_order.go
│   │   ├── cancel_order.go
│   │   └── transition_order_state.go
│   ├── query/
│   │   ├── get_order_detail.go
│   │   └── list_orders.go
│   ├── service/
│   │   └── service.go
│   ├── dto/
│   └── ports/
│       ├── payment_port.go
│       ├── inventory_port.go
│       ├── pricing_port.go
│       ├── audit_port.go
│       └── outbox_port.go
├── infra/
│   └── postgres/
│       ├── repository.go
│       ├── query_repository.go
│       ├── mapper.go
│       ├── scan.go
│       └── sql.go
├── interfaces/
│   ├── http/
│   └── consumer/
└── tests/
```

### 21.2 `payment`

```text
internal/modules/payment/
├── domain/
├── app/
│   ├── command/
│   │   ├── initiate_payment.go
│   │   ├── handle_webhook.go
│   │   ├── reconcile_payment.go
│   │   ├── mark_captured.go
│   │   └── mark_failed.go
│   ├── query/
│   ├── service/
│   ├── dto/
│   └── ports/
│       ├── order_port.go
│       ├── audit_port.go
│       └── outbox_port.go
├── infra/
│   ├── postgres/
│   └── providers/
│       ├── stripe/
│       ├── momo/
│       ├── vnpay/
│       └── paypal/
├── interfaces/
│   ├── http/
│   ├── webhook/
│   └── consumer/
└── tests/
```

### 21.3 `inventory`

```text
internal/modules/inventory/
├── domain/
├── app/
│   ├── command/
│   │   ├── reserve_stock.go
│   │   ├── release_reservation.go
│   │   ├── consume_reservation.go
│   │   ├── adjust_inventory.go
│   │   └── sweep_expired_reservations.go
│   ├── query/
│   ├── service/
│   └── dto/
├── infra/
│   ├── postgres/
│   └── redis/
├── interfaces/
│   ├── http/
│   └── jobs/
└── tests/
```

### 21.4 `entitlement`

```text
internal/modules/entitlement/
├── domain/
├── app/
│   ├── command/
│   │   ├── grant_purchase_entitlement.go
│   │   ├── grant_membership_entitlements.go
│   │   ├── revoke_entitlement.go
│   │   ├── issue_download_link.go
│   │   └── mark_download_consumed.go
│   ├── query/
│   │   ├── list_library.go
│   │   └── check_commercial_right.go
│   ├── service/
│   └── ports/
│       ├── membership_port.go
│       ├── catalog_port.go
│       └── audit_port.go
├── infra/
│   ├── postgres/
│   ├── redis/
│   └── storage/
├── interfaces/
│   ├── http/
│   └── consumer/
└── tests/
```

---

## 22. Ánh xạ layout với SRD revised

Layout này phục vụ trực tiếp các quyết định trong SRD:

- modular monolith có bounded modules rõ;
- PostgreSQL là source of truth;
- Redis chỉ là hot-path/cache/coordination;
- Kafka + outbox cho async side effects;
- order/payment/refund/chargeback/inventory/shipment có owner module riêng;
- entitlement là commercial right canonical;
- download chỉ cấp signed URL, không stream file từ app server;
- COD và online payment có state machine riêng;
- inventory reservation TTL tồn tại trong PostgreSQL;
- admin là backoffice surface, không phá ownership;
- reporting và search là projection concern, không chiếm ownership của transactional state.

---

## 23. Checklist áp dụng

- Có `cmd/api`, `cmd/worker`, `cmd/migrate`.
- Có `internal/platform`, `internal/shared`, `internal/modules`, `internal/bootstrap`.
- Mỗi module có ít nhất `domain`, `app`, `infra`, `interfaces`.
- `refund` và `chargeback` tồn tại như module riêng.
- `entitlement` sở hữu `downloads`.
- `identity` là owner của account/session/device/address.
- `inventory` dùng canonical key theo `sellable_sku_id`.
- Không module nào sửa bảng owner của module khác bằng repo implementation trực tiếp.
- Transaction boundary nằm ở app service.
- Outbox append nằm trong owner transaction.
- Persistence dùng `pgx` viết tay, không phụ thuộc `sqlc`.
- Contract artifacts sống ở `contracts/openapi` và `contracts/events`.
- Integration tests và concurrency tests có chỗ đặt rõ ràng.

---

## 24. Kết luận áp dụng

Đây là layout được tối ưu cho một **modular monolith có độ phức tạp giao dịch thực sự**, không phải CRUD app đơn giản.

Nếu tuân thủ tài liệu này, project sẽ:

- giữ được ownership rõ theo business module;
- tránh “god service” và “god admin layer”;
- dễ reason về transaction, retries, webhook idempotency và outbox;
- phù hợp với SRD revised;
- phù hợp với cách triển khai `pgx-first`;
- sẵn sàng mở rộng sang reporting/search/provider integrations mà không phá nền kiến trúc.
