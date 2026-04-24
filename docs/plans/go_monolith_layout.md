# Tài liệu Thiết kế Go Modular Monolith Layout Reference - Bản tiếng Việt

## 1. Mục đích tài liệu

Tài liệu này đặc tả **Go project layout** cho backend bookstore dạng **modular monolith** sử dụng Golang, PostgreSQL, Redis và Kafka. Mục tiêu của tài liệu là biến SRD revised và ERD revised thành một cấu trúc mã nguồn có thể triển khai thật, giữ đúng tinh thần: correctness trước convenience, database là canonical source of truth, state ownership rõ ràng, side effects đi qua outbox, và code được tổ chức theo module nghiệp vụ chứ không theo framework.

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
- anti-patterns phải tránh.

## 2. Nguyên tắc kiến trúc áp dụng vào layout

### 2.1 Nguyên tắc tổng quát

- Ứng dụng được deploy như **một binary/backend runtime duy nhất**, nhưng bên trong phải chia thành **internal modules** tách biệt về trách nhiệm.
- Không tổ chức code theo kiểu “controllers/services/repositories toàn cục” vì mô hình đó nhanh bị rối ở các flow payment, order, entitlement, reservation và audit.
- Mỗi module phải sở hữu rõ aggregate, service, repository interface, event contract và rule của chính nó.
- Module được phép giao tiếp với module khác qua service interface nội bộ hoặc domain event contract, không được truy cập xuyên module bằng cách import repository implementation của nhau.
- Shared platform code phải cực kỳ mỏng; business logic không được trôi vào `pkg/common` hoặc `utils`.
- Tất cả business mutation quan trọng phải đi qua service layer có transaction boundary rõ ràng.
- Mọi event publish sang Kafka phải đi qua outbox persistence trong transaction owner thay vì publish trực tiếp từ handler.

### 2.2 Định hướng master backend

Một kỹ sư backend mạnh không bắt đầu từ HTTP handler, mà bắt đầu từ các câu hỏi sau:

1. State nào là canonical và module nào sở hữu state đó?
2. Việc ghi dữ liệu nào phải nằm chung transaction?
3. Side effect nào synchronous, side effect nào async qua outbox?
4. Nếu retry, duplicate webhook, crash giữa chừng, dữ liệu có bị sai không?
5. Package boundary hiện tại có phản ánh đúng business boundary không?

### 2.3 Tiêu chí đánh giá layout tốt

Một layout tốt cho project này phải đáp ứng:

- Có thể tìm nhanh owner của một bảng hoặc một flow.
- Không tạo circular dependency giữa các domain modules.
- Tách rõ platform concern với domain concern.
- Dễ viết integration test theo module.
- Dễ thêm worker/outbox consumer mà không làm bẩn HTTP layer.
- Dễ mở rộng từ read-only logic sang flow giao dịch mà không phá kiến trúc.

## 3. Module decomposition đề xuất

### 3.1 Danh sách module chính

Các module nội bộ nên gồm:

- `identity`
- `catalog`
- `pricing`
- `membership`
- `entitlement`
- `cart`
- `order`
- `payment`
- `inventory`
- `shipment`
- `reader`
- `notification`
- `audit`
- `integration`
- `invoice`
- `admin` (chỉ là application/API composition layer, không phải nơi sở hữu tất cả business state)

### 3.2 Ownership theo module

- `identity` sở hữu: users, user_credentials, user_sessions, user_devices, addresses.
- `catalog` sở hữu: books, book_formats, authors, categories, book_authors, book_categories, search_documents.
- `pricing` sở hữu: prices và logic resolve effective price.
- `membership` sở hữu: membership_plans, memberships.
- `entitlement` sở hữu: entitlements, downloads và logic commercial rights cho digital access.
- `cart` sở hữu: carts, cart_items và cart repricing preparation.
- `order` sở hữu: orders, order_items, order_state_logs, checkout orchestration ở mức commerce order.
- `payment` sở hữu: payments, payment_attempts, provider callback/reconciliation logic.
- `inventory` sở hữu: inventory_items, inventory_reservations.
- `shipment` sở hữu: shipments, shipment_status_logs.
- `notification` sở hữu: notifications và email command execution.
- `audit` sở hữu: audit_logs.
- `integration` sở hữu: outbox_events, processed_events, event dispatching, consumer idempotency infra.
- `invoice` sở hữu: e_invoice_exports và provider export orchestration.

### 3.3 Dependency direction đề xuất

Luồng dependency nên theo nguyên tắc:

- HTTP adapters -> application services -> repositories/interfaces/domain rules.
- Workers -> application services của module owner.
- Module A không được import repository implementation của module B.
- Nếu A cần dữ liệu từ B, A gọi interface do B public hoặc dùng query service/read model chuyên biệt.
- `platform` là tầng dùng chung thấp nhất, không được import ngược business module.

## 4. Cây thư mục Go đề xuất

```text
bookstore-backend/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── configs/
│   ├── app.example.yaml
│   ├── app.dev.yaml
│   └── app.staging.yaml
├── deployments/
│   ├── docker/
│   ├── compose/
│   └── k8s/                  # tùy chọn giai đoạn sau
├── docs/
│   ├── srd_revised.md
│   ├── erd_revised.md
│   ├── module-boundary.md
│   ├── state-transition-spec.md
│   ├── openapi.yaml
│   ├── adr/
│   └── runbooks/
├── migrations/
│   ├── 000001_init_extensions.up.sql
│   ├── 000001_init_extensions.down.sql
│   ├── ...
│   └── seeds/
├── internal/
│   ├── platform/
│   │   ├── config/
│   │   ├── db/
│   │   ├── tx/
│   │   ├── log/
│   │   ├── httpx/
│   │   ├── auth/
│   │   ├── redisx/
│   │   ├── kafkax/
│   │   ├── timeutil/
│   │   ├── idempotency/
│   │   ├── errors/
│   │   ├── observability/
│   │   └── validation/
│   ├── modules/
│   │   ├── identity/
│   │   │   ├── domain/
│   │   │   ├── app/
│   │   │   ├── infra/
│   │   │   ├── http/
│   │   │   ├── events/
│   │   │   └── tests/
│   │   ├── catalog/
│   │   ├── pricing/
│   │   ├── membership/
│   │   ├── entitlement/
│   │   ├── cart/
│   │   ├── order/
│   │   ├── payment/
│   │   ├── inventory/
│   │   ├── shipment/
│   │   ├── reader/
│   │   ├── notification/
│   │   ├── audit/
│   │   ├── integration/
│   │   └── invoice/
│   └── bootstrap/
│       ├── app.go
│       ├── http.go
│       ├── worker.go
│       └── modules.go
├── test/
│   ├── integration/
│   ├── contract/
│   ├── fixtures/
│   └── e2e/
├── scripts/
├── Makefile
├── go.mod
└── README.md
```

## 5. Giải thích chi tiết từng tầng

## 5.1 `cmd/`

- `cmd/api/main.go`: entrypoint cho HTTP API runtime.
- `cmd/worker/main.go`: entrypoint cho background workers, outbox dispatcher, scheduled jobs, notification consumers, reconciliation jobs.
- Không đặt business logic ở `cmd`.
- `cmd` chỉ làm bootstrap, đọc config, init dependency graph và start runtime.

## 5.2 `internal/platform/`

Đây là tầng hạ tầng kỹ thuật dùng chung, không chứa nghiệp vụ bán sách.

### 5.2.1 `config/`

- Load config từ env và file YAML.
- Validate config ngay khi startup.
- Tách cấu hình theo domain: db, redis, kafka, auth, email, provider payment, object storage, rate limiting.

### 5.2.2 `db/`

- Khởi tạo pool PostgreSQL.
- Cung cấp query runner / pgx wrapper theo lựa chọn.
- Không đặt query business cụ thể ở đây.

### 5.2.3 `tx/`

- Transaction manager dùng chung.
- Cung cấp API kiểu `WithinTransaction(ctx, fn)` để service layer của module owner thực hiện atomic mutation.
- Đảm bảo nested transaction không tạo chaos; nếu cần thì dùng same transaction context propagation.

### 5.2.4 `log/`

- Structured JSON logging.
- Gắn request_id, trace_id, user_id, order_id, payment_id khi có thể.

### 5.2.5 `httpx/`

- Router bootstrap.
- Middleware dùng chung: recovery, request-id, tracing, auth context, rate limit adapter, idempotency adapter, error rendering.
- Không chứa handler business cụ thể.

### 5.2.6 `auth/`

- JWT signing/verification hoặc opaque token helper.
- Password hashing adapter.
- Claims/permission parsing helper.

### 5.2.7 `redisx/`

- Redis client factory, namespaced key builder, atomic helper, Lua script wrapper nếu cần.
- Không chứa business policy cụ thể như “download quota” ở đây; policy nằm ở module owner.

### 5.2.8 `kafkax/`

- Producer/consumer factory.
- Common consumer middleware: tracing, retry wrapper, metrics, dedup hook.
- Không chứa domain event payload cố định.

### 5.2.9 `idempotency/`

- Shared helper cho idempotency middleware / store abstraction.
- Có thể dùng Redis hot path nhưng result/canonical effect vẫn do DB quyết định.

### 5.2.10 `errors/`

- Chuẩn hóa application errors.
- Mapping domain error -> HTTP status -> machine-readable error code.

### 5.2.11 `observability/`

- Metrics registry.
- OpenTelemetry tracing hooks.
- Health/readiness checks.

## 5.3 `internal/modules/`

Mỗi module là một mini-application boundary bên trong monolith.

Mỗi module nên có cấu trúc chuẩn sau:

```text
internal/modules/order/
├── domain/
│   ├── model.go
│   ├── states.go
│   ├── errors.go
│   ├── rules.go
│   └── repository.go
├── app/
│   ├── service.go
│   ├── commands.go
│   ├── queries.go
│   ├── dto.go
│   └── orchestrator.go
├── infra/
│   ├── postgres/
│   │   ├── repository.go
│   │   ├── queries.sql        # nếu dùng sqlc hoặc tương tự
│   │   └── mapper.go
│   ├── redis/
│   └── kafka/
├── http/
│   ├── handler.go
│   ├── request.go
│   ├── response.go
│   └── routes.go
├── events/
│   ├── publisher.go
│   ├── consumer.go
│   └── payloads.go
└── tests/
```

### 5.3.1 `domain/`

- Chứa model nghiệp vụ, state constants, domain errors, interfaces và business rules thuần.
- Không import HTTP, Redis client, Kafka client.
- Có thể import chuẩn Go và very-stable primitives.

### 5.3.2 `app/`

- Chứa use cases thật sự.
- Đây là nơi orchestration giữa repository, transaction manager, outbox append, audit append và module collaboration diễn ra.
- App service là nơi quyết định synchronous flow.

### 5.3.3 `infra/`

- Chứa implementation của repository interfaces, provider adapters, storage adapters, Redis helpers của module đó.
- SQL truy vấn module-specific nằm tại đây.

### 5.3.4 `http/`

- Parse request, validate input, gọi app service, render response.
- Không chứa business decision trừ input shaping và authorization entry check.

### 5.3.5 `events/`

- Định nghĩa event payload của module.
- Chứa event handler nội bộ cho worker runtime.
- Không cho phép business handler cập nhật state cross-module bằng cách bỏ qua owner service.

## 6. Thiết kế chi tiết theo module trọng yếu

## 6.1 Module `order`

### Ownership

- `orders`
- `order_items`
- `order_state_logs`

### Public app services đề xuất

- `CreateOrderFromCheckout`
- `GetOrderDetail`
- `ListOrdersByUser`
- `CancelOrder`
- `TransitionOrderState`
- `RecordOrderStateLog`

### Không nên làm

- Không để payment module tự cập nhật `orders` bằng repository trực tiếp.
- Payment module phải gọi order service hoặc dùng transition API nội bộ có kiểm soát.

## 6.2 Module `payment`

### Ownership

- `payments`
- `payment_attempts`

### Public app services đề xuất

- `InitiatePayment`
- `HandleGatewayWebhook`
- `ReconcilePayment`
- `MarkPaymentCaptured`
- `MarkPaymentFailed`
- `CreateRefundRequest`

### Rule

- Payment là owner của payment state, nhưng order commercial state transition phải phối hợp với order owner trong cùng transaction boundary hợp lý hoặc orchestration đã chuẩn hóa.

## 6.3 Module `inventory`

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

- Chỉ module này được quyết định stock mutation.
- Order không được tự giảm `inventory_items` bằng SQL riêng.

## 6.4 Module `entitlement`

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

### Rule

- Entitlement không được suy luận ad-hoc mỗi request từ orders/memberships.
- Module này materialize commercial rights theo đúng ERD revised.

## 6.5 Module `integration`

### Ownership

- `outbox_events`
- `processed_events`

### Public app services đề xuất

- `AppendOutboxEvent`
- `DispatchPendingOutbox`
- `MarkProcessedEvent`
- `HasProcessedEvent`

### Rule

- Không module nào được publish Kafka trực tiếp từ handler path nếu event thuộc business transaction bắt buộc.

## 7. Bootstrapping và wiring

## 7.1 Bootstrap application

`internal/bootstrap/modules.go` nên là nơi wiring dependency graph theo hướng explicit.

Pseudo responsibilities:

- init DB, Redis, Kafka clients;
- init tx manager;
- init repositories cho từng module;
- init app services;
- init HTTP routes;
- init worker consumers;
- init schedulers.

Không dùng service locator động tùy tiện nếu nó che mất dependency graph.

## 7.2 Constructor style

Khuyến nghị constructor rõ ràng:

- `NewOrderService(orderRepo, itemRepo, stateLogRepo, txManager, outboxAppender, auditAppender, inventoryGateway, paymentGateway)`
- Không truyền “god container” chứa mọi dependency.

## 8. HTTP adapter design

## 8.1 Route grouping đề xuất

- `/api/v1/auth/*`
- `/api/v1/me/*`
- `/api/v1/books/*`
- `/api/v1/cart/*`
- `/api/v1/checkout/*`
- `/api/v1/orders/*`
- `/api/v1/payments/*`
- `/api/v1/admin/*`

## 8.2 Handler rules

- Handler không mở transaction thủ công bằng SQL trừ khi được service gọi qua tx manager.
- Handler không emit event trực tiếp.
- Handler không quyết định state transition ngoài validation sơ cấp.
- Handler chỉ map request -> command, command -> service, service result -> response.

## 8.3 Error response

Chuẩn hóa dạng:

```json
{
  "error": {
    "code": "ORDER_INVALID_STATE",
    "message": "Đơn hàng không ở trạng thái cho phép thanh toán lại",
    "details": {}
  },
  "request_id": "..."
}
```

## 9. Worker runtime design

## 9.1 Worker categories

- Outbox dispatcher worker.
- Notification email worker.
- Reservation expiry sweeper.
- Payment reconciliation worker.
- Membership expiry worker.
- Invoice export worker.

## 9.2 Worker rules

- Worker không tự ý mutate cross-domain bằng SQL thẳng.
- Worker gọi app service của module owner.
- Consumer phải idempotent bằng `processed_events` hoặc dedup policy.
- Duplicate delivery phải an toàn.

## 10. Transaction boundary design trong code layout

## 10.1 Các flow phải có transaction mạnh

- Create order + order items + payment initial record + order_state_log + outbox append.
- Payment captured -> update payment + transition order + grant entitlement hoặc trigger reliable action + audit + outbox.
- Reserve inventory / release inventory.
- Refund create/update canonical state ban đầu.
- Admin override nhạy cảm + audit.

## 10.2 Cách encode trong code

- Transaction boundary nằm ở app service của module owner.
- Repository methods nhận `DBTX` / executor interface để chạy trong cùng transaction.
- Outbox append được gọi trong cùng transaction context.

## 11. Query design và read/write separation trong monolith

Không cần CQRS đầy đủ ở phase 1, nhưng cần tách tư duy:

- write services chịu trách nhiệm invariant;
- read queries tối ưu cho API response;
- query service có thể join nhiều bảng miễn không phá ownership ghi.

Ví dụ:

- `order/app/query_service.go` có thể join orders + items + payments + shipments cho order detail response.
- Nhưng mutation payment state vẫn phải ở payment owner service.

## 12. Testing layout

## 12.1 Unit tests

Mỗi module có unit tests cho:

- state transition rules;
- validation logic;
- pricing calculation;
- reservation release conditions;
- entitlement policy logic.

## 12.2 Integration tests

`test/integration/` nên có:

- bootstrap DB từ migrations;
- seed fixtures;
- test repository;
- test transaction flows create order / payment capture / inventory reserve / entitlement grant.

## 12.3 Contract tests

`test/contract/` dùng để kiểm tra OpenAPI response shape và webhook contracts.

## 12.4 E2E tests

`test/e2e/` cho happy path chính:

- register -> verify -> login;
- browse -> cart -> checkout -> pay -> library/download;
- physical COD order -> shipment delivered -> paid.

## 13. Naming conventions và coding conventions

### 13.1 File naming

- `service.go` cho entry service file.
- `repository.go` cho interface hoặc infra implementation theo folder.
- `states.go` cho constants + transition helpers.
- `mapper.go` cho mapping DB row <-> domain model.
- `routes.go` cho route registration.

### 13.2 Interface placement

- Interface đặt ở nơi **tiêu thụ** interface, không phải nơi triển khai.
- Ví dụ order service cần inventory gateway thì interface `InventoryGateway` có thể đặt ở `order/app/ports.go`.

### 13.3 DTO policy

- Không expose trực tiếp DB row struct ra API.
- Request DTO, Response DTO, Domain Model là ba tầng khác nhau.

## 14. Anti-patterns phải tránh

- `internal/common` thành nơi chứa business logic của mọi module.
- Một service dài 2000 dòng xử lý cả order, payment, inventory, entitlement.
- Handler gọi repository trực tiếp rồi publish Kafka.
- Module A import SQL repo của module B.
- Dùng Redis như canonical source cho reservation/order state.
- Cho worker sửa trạng thái trực tiếp bằng SQL để “đỡ viết service”.
- Nhét mọi interface vào package `ports` toàn cục vô định.
- Tách package quá vụn vặt trước khi có nhu cầu thật.

## 15. Layout đề xuất chi tiết cho các module quan trọng

### 15.1 `order` module

```text
internal/modules/order/
├── domain/
│   ├── order.go
│   ├── order_item.go
│   ├── state.go
│   ├── errors.go
│   └── repository.go
├── app/
│   ├── create_order.go
│   ├── cancel_order.go
│   ├── get_order_detail.go
│   ├── list_orders.go
│   ├── transition_order_state.go
│   ├── dto.go
│   └── ports.go
├── infra/postgres/
│   ├── order_repository.go
│   ├── order_item_repository.go
│   ├── order_state_log_repository.go
│   └── mapper.go
├── http/
│   ├── handler.go
│   ├── routes.go
│   └── presenter.go
└── events/
    └── payloads.go
```

### 15.2 `payment` module

```text
internal/modules/payment/
├── domain/
├── app/
│   ├── initiate_payment.go
│   ├── handle_webhook.go
│   ├── reconcile_payment.go
│   ├── create_refund_request.go
│   └── ports.go
├── infra/
│   ├── postgres/
│   └── providers/
│       ├── momo/
│       ├── vnpay/
│       ├── stripe/
│       └── paypal/
├── http/
└── events/
```

### 15.3 `inventory` module

```text
internal/modules/inventory/
├── domain/
├── app/
│   ├── reserve_stock.go
│   ├── release_reservation.go
│   ├── consume_reservation.go
│   ├── sweep_expired.go
│   └── adjust_inventory.go
├── infra/postgres/
├── http/
└── events/
```

## 16. Phân chia package shared và package domain

### Shared package hợp lệ

- logger
- config
- tx manager
- error mapping infra
- tracing
- redis client factory
- kafka client factory
- validation helpers generic

### Shared package không nên tồn tại

- `businessutils`
- `servicehelpers` chứa logic nghiệp vụ dùng chung mơ hồ
- `models` toàn cục cho mọi module
- `constants` toàn cục chứa state của mọi domain

## 17. Ánh xạ layout với SRD revised và ERD revised

Layout này sinh ra để phục vụ các quyết định đã chốt trong SRD/ERD revised:

- `sellable_skus` yêu cầu catalog/pricing/cart/order tách ownership rõ.
- `order_state_logs` yêu cầu order module có state transition service riêng.
- `inventory_reservations` TTL yêu cầu inventory module có worker sweep riêng.
- `downloads` signed-link model yêu cầu entitlement/reader phối hợp nhưng không để handler stream file trực tiếp.
- `outbox_events` và `processed_events` yêu cầu integration module tách biệt với business modules.
- `billing_snapshot` và order snapshots yêu cầu order module giữ snapshot mapping rõ ràng tại app service.

## 18. Checklist triển khai layout

- Có `cmd/api` và `cmd/worker` riêng.
- Có `internal/platform` và `internal/modules` tách biệt.
- Mỗi module có ít nhất `domain`, `app`, `infra`, `http`.
- Không có circular imports giữa modules.
- Không module nào ghi vào bảng của module khác bằng repo implementation trực tiếp.
- Outbox append nằm trong transaction owner.
- Integration tests chạy được theo module.
- Docs và code cấu trúc phản ánh cùng một ownership map.

## 19. Kết luận áp dụng

Đây là layout được thiết kế cho một **modular monolith có độ phức tạp giao dịch thực sự**, không phải CRUD app đơn giản. Nếu tuân thủ tài liệu này, project sẽ giữ được tính dễ hiểu, auditability, khả năng scale codebase, và giảm mạnh nguy cơ “mọi thứ dính vào order service” khi bước vào các phase payment, inventory, entitlement và refunds.
