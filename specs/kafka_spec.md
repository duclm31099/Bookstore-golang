# Tài liệu Kafka Event Contract Specification Chi Tiết

## 1. Mục đích tài liệu

Tài liệu này đặc tả chi tiết toàn bộ **Kafka Event Contract** cho hệ thống backend monolith bán sách sử dụng Golang/Gin, PostgreSQL, Redis và Kafka. Mục tiêu là chuẩn hóa tuyệt đối cách các module trong hệ thống phát, tiêu thụ, retry, deduplicate, versioning, quan sát và mở rộng event, để việc triển khai worker, scheduler, outbox publisher, reconciliation jobs, email jobs, shipment sync, invoice export, entitlement propagation và reporting feed không bị lệch semantic giữa các team hoặc giữa các giai đoạn phát triển.

Tài liệu này là lớp hợp đồng dữ liệu trung gian giữa:
- business state trong PostgreSQL;
- lớp Redis cache/coordination;
- worker và scheduler nền;
- kênh observability;
- các integration adapters như payment gateway, carrier adapter và nhà cung cấp e-invoice.

## 2. Vai trò Kafka trong kiến trúc hệ thống

### 2.1 Vai trò cốt lõi

Kafka trong project này là **backbone bất đồng bộ** của monolith, không phải queue phụ. Hệ thống vẫn là modular monolith ở lớp ứng dụng, nhưng dùng Kafka để tách execution path, giảm coupling runtime, tăng khả năng retry, tăng khả năng recover khi lỗi tạm thời và tạo audit trail bất đồng bộ.

### 2.2 Use cases bắt buộc của Kafka

Kafka phải bao phủ đầy đủ các use case sau trong phase hiện tại:
- phát domain events sau khi order/payment/refund/membership/entitlement/shipment đổi trạng thái;
- phát command events cho email worker, invoice export worker, scheduler jobs và reconciliation jobs;
- phát integration events để đồng bộ với các adapter ngoài;
- phát operational events phục vụ reporting, audit side-stream, metric enrichment hoặc cache invalidation;
- đảm bảo replay được ở mức hợp lý cho các consumer side-effects nếu cần rebuild projection.

### 2.3 Những gì Kafka không làm

Kafka **không** là source of truth cho business state chính. Không được dùng Kafka như nơi duy nhất lưu order/payment/refund/membership state. Không được suy diễn correctness chỉ từ việc event đã publish. Trạng thái đúng của hệ thống luôn phải quay về PostgreSQL.

## 3. Nguyên tắc thiết kế contract

### 3.1 Nguyên tắc chung

- Mọi event phải có ý nghĩa nghiệp vụ rõ ràng.
- Tên event phải ổn định, không phụ thuộc implementation detail.
- Producer owner phải là module sở hữu aggregate gốc.
- Consumer không được yêu cầu event “quá bé” đến mức phải query ngược nhiều bảng mới hiểu semantic.
- Consumer cũng không được nhận payload “quá to” làm tăng coupling, lộ dữ liệu và khó versioning.
- Mọi event phải idempotent ở phía consumer.
- Mọi side effect phải assume message có thể đến lặp, trễ hoặc lệch thứ tự.
- Mọi event phải có schema version rõ ràng.

### 3.2 Phân loại event

| Nhóm | Định nghĩa | Ví dụ |
|---|---|---|
| Domain Event | Một việc nghiệp vụ đã xảy ra | `order.paid`, `membership.expired` |
| Command Event | Yêu cầu module/worker khác thực hiện hành động | `notification.send_email`, `shipment.poll_status` |
| Integration Event | Event chuẩn hóa để phục vụ adapter hoặc boundary ngoài | `invoice.export_requested` |
| Operational Event | Event phục vụ report, audit, invalidation hoặc monitor | `catalog.cache_invalidate_requested` |

### 3.3 Quy tắc semantic theo nhóm

- Domain Event dùng thì quá khứ: `created`, `paid`, `captured`, `expired`, `revoked`, `completed`.
- Command Event dùng động từ mệnh lệnh: `send`, `reconcile`, `poll`, `release`, `rebuild`.
- Integration Event phải trung lập về nhà cung cấp ngoài.
- Operational Event không được trá hình business event.

## 4. Mô hình publish an toàn

### 4.1 Outbox pattern bắt buộc

Mọi domain event và command event phát từ transaction nghiệp vụ phải đi qua **outbox pattern**. Không được publish thẳng từ HTTP handler hoặc service transaction sang Kafka broker rồi mới commit DB. Flow chuẩn:
1. transaction business commit vào PostgreSQL;
2. cùng transaction đó ghi record vào `outbox_events`;
3. publisher worker đọc outbox và publish sang Kafka;
4. update trạng thái publish trong outbox.

### 4.2 Lý do bắt buộc dùng outbox

- tránh tình huống DB commit nhưng Kafka publish fail;
- tránh tình huống Kafka publish thành công nhưng DB rollback;
- hỗ trợ retry publish an toàn;
- hỗ trợ audit và replay nội bộ;
- dễ quan sát lag/outbox backlog.

### 4.3 Outbox state machine

| State | Ý nghĩa |
|---|---|
| `pending` | Đã ghi outbox, chưa publish |
| `publishing` | Worker đang xử lý |
| `published` | Đã publish thành công |
| `failed` | Publish lỗi, chờ retry |
| `parked` | Lỗi cần can thiệp tay |

## 5. Envelope chuẩn cho mọi message

## 5.1 Schema envelope chuẩn

```json
{
  "event_id": "d2a33114-bf8d-4af9-93ee-a8f95b5d8c67",
  "event_type": "order.paid",
  "event_version": 1,
  "topic": "order.events.v1",
  "event_key": "order:100245",
  "aggregate_type": "order",
  "aggregate_id": 100245,
  "occurred_at": "2026-04-17T11:05:00Z",
  "produced_at": "2026-04-17T11:05:02Z",
  "correlation_id": "9cbb5f10-2d19-47c1-bb78-f16d1d6a96b2",
  "causation_id": "b7d7c8dd-c1d4-40ad-a59d-09c0f4be9ec8",
  "trace_id": "d1eb7b4f41d9400ab90d3b7a61d7aa01",
  "actor_type": "system",
  "actor_id": 0,
  "environment": "prod",
  "schema_ref": "order.paid.v1",
  "payload": {}
}
```

## 5.2 Ý nghĩa field

| Field | Bắt buộc | Mô tả |
|---|---|---|
| `event_id` | Có | UUID duy nhất của event |
| `event_type` | Có | Tên semantic của event |
| `event_version` | Có | Version của payload event |
| `topic` | Có | Kafka topic thực tế |
| `event_key` | Có | Key partition |
| `aggregate_type` | Có | aggregate gốc, ví dụ `order`, `payment` |
| `aggregate_id` | Có | ID aggregate gốc |
| `occurred_at` | Có | lúc nghiệp vụ thực sự xảy ra |
| `produced_at` | Có | lúc event được publish ra broker |
| `correlation_id` | Có | ID xâu chuỗi nghiệp vụ xuyên module |
| `causation_id` | Có | ID request hoặc event gần nhất gây ra event này |
| `trace_id` | Có | ID observability |
| `actor_type` | Có | `user`, `admin`, `system`, `scheduler`, `gateway`, `carrier` |
| `actor_id` | Có | ID actor, hoặc 0 nếu hệ thống |
| `environment` | Có | `local`, `dev`, `staging`, `prod` |
| `schema_ref` | Có | logical schema name để validate |
| `payload` | Có | dữ liệu nghiệp vụ |

### 5.3 Quy tắc field

- `event_id` luôn là UUID v4.
- `event_type` dùng chữ thường và dấu chấm, ví dụ `order.paid`.
- `aggregate_type` dùng lower snake hoặc lower singular ổn định, ví dụ `order`, `membership`, `entitlement`.
- `occurred_at` không được lấy bừa theo thời điểm worker publish nếu state business đã đổi trước đó.
- `produced_at` có thể muộn hơn `occurred_at` vài giây hoặc vài phút nếu outbox backlog.
- `payload` không được là `null`; nếu rỗng phải là `{}`.

## 6. Quy tắc topic

### 6.1 Topic chuẩn của hệ thống

| Topic | Mục đích |
|---|---|
| `catalog.events.v1` | Event catalog, price, publish state, search invalidation |
| `order.events.v1` | Event vòng đời order |
| `payment.events.v1` | Event payment và reconciliation |
| `membership.events.v1` | Event vòng đời membership |
| `entitlement.events.v1` | Event grant/revoke/expire quyền |
| `reader.events.v1` | Event phiên đọc và vị trí đọc |
| `download.events.v1` | Event cấp/revoke link tải và audit tải |
| `inventory.events.v1` | Event reserve/release/adjust stock |
| `shipment.events.v1` | Event shipment và sync carrier |
| `refund.events.v1` | Event refund |
| `chargeback.events.v1` | Event dispute/chargeback |
| `notification.commands.v1` | Command gửi email |
| `scheduler.commands.v1` | Command do scheduler phát |
| `invoice.events.v1` | Event export hóa đơn điện tử |
| `audit.events.v1` | Optional audit side-stream |
| `reporting.events.v1` | Feed chuẩn hóa cho reporting |
| `dlq.general.v1` | Dead-letter chung giai đoạn đầu |

### 6.2 Topic ownership

| Topic | Producer owner chính |
|---|---|
| `catalog.events.v1` | Catalog module |
| `order.events.v1` | Order / Checkout module |
| `payment.events.v1` | Payment module |
| `membership.events.v1` | Membership module |
| `entitlement.events.v1` | Entitlement module |
| `reader.events.v1` | Reader module |
| `download.events.v1` | Download module |
| `inventory.events.v1` | Inventory module |
| `shipment.events.v1` | Shipment module |
| `refund.events.v1` | Refund module |
| `chargeback.events.v1` | Chargeback module |
| `notification.commands.v1` | Notification bridge |
| `scheduler.commands.v1` | Scheduler |
| `invoice.events.v1` | Invoice integration module |
| `reporting.events.v1` | Domain bridge / projector |

### 6.3 Topic partition khuyến nghị

| Topic | Partition gợi ý ban đầu | Ghi chú |
|---|---:|---|
| catalog.events.v1 | 3 | tải vừa |
| order.events.v1 | 6 | nhiều event thương mại |
| payment.events.v1 | 6 | payment callback/reconcile |
| membership.events.v1 | 3 | ít hơn order/payment |
| entitlement.events.v1 | 6 | library/download/read hot-path |
| reader.events.v1 | 6 | heartbeat nếu cần publish chọn lọc |
| download.events.v1 | 3 | audit/tải |
| inventory.events.v1 | 3 | reserve/release |
| shipment.events.v1 | 3 | shipment lifecycle |
| refund.events.v1 | 3 | khối lượng vừa |
| chargeback.events.v1 | 1 | volume thấp |
| notification.commands.v1 | 3 | email worker fan-out |
| scheduler.commands.v1 | 3 | batch job |
| invoice.events.v1 | 1 | volume thấp |
| reporting.events.v1 | 3 | feed downstream |
| dlq.general.v1 | 1 | tập trung xử lý |

## 7. Quy tắc event key và ordering

### 7.1 Event key chuẩn

| Aggregate | Key format | Ví dụ |
|---|---|---|
| Order | `order:{order_id}` | `order:100245` |
| Payment | `payment:{payment_id}` | `payment:77891` |
| Membership | `membership:{membership_id}` | `membership:5002` |
| Entitlement | `entitlement:{entitlement_id}` | `entitlement:88001` |
| Reader session | `reader:{reader_session_id}` | `reader:99001` |
| Download | `download:{download_id}` | `download:12001` |
| Inventory by book | `book:{book_id}` | `book:302` |
| Shipment | `shipment:{shipment_id}` | `shipment:1212` |
| Notification | `notification:{notification_id}` | `notification:9001` |
| Invoice export | `invoice:{invoice_export_id}` | `invoice:6101` |
| Scheduler job | `job:{job_type}:{biz_key}` | `job:payment_reconcile:77891` |

### 7.2 Quy tắc ordering

- Kafka chỉ đảm bảo order trong cùng partition.
- Vì vậy event cùng aggregate phải luôn đi cùng key ổn định.
- Consumer không được giả định event của các aggregate khác nhau sẽ đến theo thứ tự thời gian toàn cục.
- Mọi state machine phải tự bảo vệ trước event đến trễ hoặc lặp.

## 8. Quy tắc payload

### 8.1 Payload tối thiểu cần có

Mỗi payload phải đủ để consumer hiểu:
- ai / cái gì vừa thay đổi;
- trạng thái mới là gì;
- khi nào xảy ra;
- nguồn nào gây ra;
- các thuộc tính nghiệp vụ thiết yếu ảnh hưởng tới side effect;
- metadata truy vết nếu cần.

### 8.2 Payload không được chứa

- password hash;
- raw refresh token;
- signed download URL đầy đủ;
- thông tin thẻ thanh toán thô;
- secret gateway / signature secret;
- dữ liệu nhạy cảm không cần cho consumer.

### 8.3 Metadata khuyến nghị trong payload

```json
{
  "meta": {
    "source_module": "payment",
    "initiated_by": "gateway_webhook",
    "idempotency_key": "optional-key",
    "replay": false
  }
}
```

## 9. Contract chi tiết theo domain

## 9.1 Catalog events

### 9.1.1 `catalog.book_created`

**Topic**: `catalog.events.v1`

**Trigger**
- admin tạo mới sách và lưu thành công.

**Payload**
```json
{
  "book_id": 302,
  "slug": "ky-nang-doc-sau",
  "title": "Kỹ năng đọc sâu",
  "product_type": "hybrid",
  "membership_eligible": false,
  "published": false,
  "created_at": "2026-04-17T10:00:00Z",
  "meta": {
    "source_module": "catalog",
    "initiated_by": "admin"
  }
}
```

**Consumers**
- reporting bridge
- audit enrichment

### 9.1.2 `catalog.book_published`

**Payload**
```json
{
  "book_id": 302,
  "slug": "ky-nang-doc-sau",
  "title": "Kỹ năng đọc sâu",
  "product_type": "hybrid",
  "membership_eligible": true,
  "published": true,
  "published_at": "2026-04-17T10:15:00Z",
  "search_reindex_required": true,
  "cache_invalidate_required": true,
  "meta": {
    "source_module": "catalog",
    "initiated_by": "admin"
  }
}
```

### 9.1.3 `catalog.book_updated`

**Payload**
```json
{
  "book_id": 302,
  "changed_fields": ["title", "membership_eligible", "cover_image_url"],
  "search_reindex_required": true,
  "cache_invalidate_keys": [
    "book:detail:v1:302"
  ],
  "updated_at": "2026-04-17T10:20:00Z",
  "meta": {
    "source_module": "catalog",
    "initiated_by": "admin"
  }
}
```

### 9.1.4 `catalog.price_changed`

**Payload**
```json
{
  "subject_type": "book",
  "subject_id": 302,
  "old_list_price_vnd": 120000,
  "old_sale_price_vnd": 99000,
  "new_list_price_vnd": 120000,
  "new_sale_price_vnd": 89000,
  "effective_at": "2026-04-17T10:30:00Z",
  "cache_invalidate_required": true,
  "meta": {
    "source_module": "pricing",
    "initiated_by": "admin"
  }
}
```

## 9.2 Order events

### 9.2.1 `order.created`

**Topic**: `order.events.v1`

**Trigger**
- checkout tạo order thành công.

**Payload**
```json
{
  "order_id": 100245,
  "order_no": "ORD202604170001",
  "user_id": 901,
  "order_state": "pending_payment",
  "payment_method": "vnpay",
  "currency": "VND",
  "subtotal_vnd": 250000,
  "shipping_fee_vnd": 0,
  "discount_vnd": 20000,
  "tax_vnd": 0,
  "total_vnd": 230000,
  "contains_digital_items": true,
  "contains_physical_items": false,
  "items": [
    {
      "order_item_id": 1,
      "sku_type": "ebook",
      "sku_id": 302,
      "quantity": 1,
      "final_line_total_vnd": 90000
    },
    {
      "order_item_id": 2,
      "sku_type": "membership_plan",
      "sku_id": 2,
      "quantity": 1,
      "final_line_total_vnd": 140000
    }
  ],
  "placed_at": "2026-04-17T11:00:00Z",
  "meta": {
    "source_module": "checkout",
    "initiated_by": "user"
  }
}
```

### 9.2.2 `order.payment_processing`

**Payload**
```json
{
  "order_id": 100245,
  "order_no": "ORD202604170001",
  "user_id": 901,
  "order_state": "payment_processing",
  "payment_id": 77891,
  "payment_method": "vnpay",
  "changed_at": "2026-04-17T11:01:00Z",
  "meta": {
    "source_module": "payment",
    "initiated_by": "system"
  }
}
```

### 9.2.3 `order.paid`

**Payload**
```json
{
  "order_id": 100245,
  "order_no": "ORD202604170001",
  "user_id": 901,
  "order_state": "paid",
  "payment_id": 77891,
  "payment_provider": "vnpay",
  "paid_amount_vnd": 230000,
  "paid_at": "2026-04-17T11:05:00Z",
  "contains_digital_items": true,
  "contains_physical_items": false,
  "items": [
    {
      "order_item_id": 1,
      "sku_type": "ebook",
      "sku_id": 302,
      "item_state": "paid"
    },
    {
      "order_item_id": 2,
      "sku_type": "membership_plan",
      "sku_id": 2,
      "item_state": "paid"
    }
  ],
  "meta": {
    "source_module": "payment",
    "initiated_by": "gateway_webhook"
  }
}
```

**Consumers**
- membership activation bridge
- entitlement grant bridge
- notification bridge
- invoice export request bridge
- reporting feed
- audit side-stream

### 9.2.4 `order.cancelled`

```json
{
  "order_id": 100245,
  "order_no": "ORD202604170001",
  "user_id": 901,
  "previous_state": "pending_payment",
  "order_state": "cancelled",
  "cancel_reason_code": "user_requested",
  "cancel_reason_note": "Người dùng hủy đơn trước khi thanh toán",
  "cancelled_at": "2026-04-17T11:10:00Z",
  "meta": {
    "source_module": "order",
    "initiated_by": "user"
  }
}
```

### 9.2.5 `order.fulfilled`

```json
{
  "order_id": 100300,
  "order_no": "ORD202604170100",
  "user_id": 901,
  "order_state": "fulfilled",
  "fulfilled_at": "2026-04-19T16:30:00Z",
  "shipment_id": 1212,
  "meta": {
    "source_module": "shipment",
    "initiated_by": "carrier_delivery_confirmation"
  }
}
```

## 9.3 Payment events

### 9.3.1 `payment.initiated`

**Topic**: `payment.events.v1`

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "vnpay",
  "payment_state": "initiated",
  "amount_vnd": 230000,
  "attempt_no": 1,
  "created_at": "2026-04-17T11:00:30Z",
  "meta": {
    "source_module": "payment",
    "initiated_by": "user"
  }
}
```

### 9.3.2 `payment.authorized`

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "stripe",
  "payment_state": "authorized",
  "amount_vnd": 230000,
  "external_payment_ref": "pi_12345",
  "authorized_at": "2026-04-17T11:02:00Z",
  "meta": {
    "source_module": "payment",
    "initiated_by": "gateway_webhook"
  }
}
```

### 9.3.3 `payment.captured`

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "vnpay",
  "payment_state": "captured",
  "amount_vnd": 230000,
  "external_payment_ref": "VNP123456",
  "attempt_no": 1,
  "captured_at": "2026-04-17T11:05:00Z",
  "meta": {
    "source_module": "payment",
    "initiated_by": "gateway_webhook"
  }
}
```

### 9.3.4 `payment.failed`

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "momo",
  "payment_state": "failed",
  "amount_vnd": 230000,
  "attempt_no": 1,
  "error_code": "PAYMENT_DECLINED",
  "error_message": "Giao dịch bị từ chối",
  "failed_at": "2026-04-17T11:05:00Z",
  "meta": {
    "source_module": "payment",
    "initiated_by": "gateway_webhook"
  }
}
```

### 9.3.5 `payment.expired`

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "vnpay",
  "payment_state": "expired",
  "expired_at": "2026-04-17T11:30:00Z",
  "meta": {
    "source_module": "payment",
    "initiated_by": "scheduler"
  }
}
```

### 9.3.6 `payment.reconcile_requested`

**Loại**: Command Event

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "vnpay",
  "reason": "timeout_without_final_status",
  "requested_at": "2026-04-17T11:20:00Z",
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

### 9.3.7 `payment.reconciled`

```json
{
  "payment_id": 77891,
  "order_id": 100245,
  "provider": "vnpay",
  "old_state": "pending",
  "new_state": "captured",
  "reconciled_at": "2026-04-17T11:22:00Z",
  "provider_payload_ref": "reconcile-77891-1",
  "meta": {
    "source_module": "payment_reconcile_worker",
    "initiated_by": "system"
  }
}
```

## 9.4 Membership events

### 9.4.1 `membership.activated`

**Topic**: `membership.events.v1`

```json
{
  "membership_id": 5002,
  "user_id": 901,
  "plan_id": 2,
  "plan_code": "annual",
  "state": "active",
  "starts_at": "2026-04-17T11:05:00Z",
  "expires_at": "2027-04-17T11:04:59Z",
  "source_order_id": 100245,
  "quota": {
    "max_devices": 5,
    "max_concurrent_reader_sessions": 3,
    "max_downloads_total": null
  },
  "meta": {
    "source_module": "membership",
    "initiated_by": "order_paid"
  }
}
```

### 9.4.2 `membership.renewed`

```json
{
  "membership_id": 5002,
  "user_id": 901,
  "plan_id": 2,
  "previous_expires_at": "2027-04-17T11:04:59Z",
  "new_expires_at": "2028-04-17T11:04:59Z",
  "source_order_id": 100678,
  "meta": {
    "source_module": "membership",
    "initiated_by": "order_paid"
  }
}
```

### 9.4.3 `membership.expiring_soon`

```json
{
  "membership_id": 5002,
  "user_id": 901,
  "plan_id": 2,
  "expires_at": "2027-04-17T11:04:59Z",
  "days_remaining": 7,
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

### 9.4.4 `membership.expired`

```json
{
  "membership_id": 5002,
  "user_id": 901,
  "plan_id": 2,
  "expired_at": "2027-04-17T11:05:00Z",
  "state": "expired",
  "meta": {
    "source_module": "membership",
    "initiated_by": "scheduler"
  }
}
```

### 9.4.5 `membership.revoked`

```json
{
  "membership_id": 5002,
  "user_id": 901,
  "plan_id": 2,
  "revoked_at": "2026-04-20T10:00:00Z",
  "reason_code": "refund_full",
  "reason_note": "Hoàn tiền đơn hàng membership",
  "meta": {
    "source_module": "membership",
    "initiated_by": "refund_or_admin"
  }
}
```

## 9.5 Entitlement events

### 9.5.1 `entitlement.granted`

**Topic**: `entitlement.events.v1`

```json
{
  "entitlement_id": 88001,
  "user_id": 901,
  "book_id": 302,
  "source_type": "ebook_purchase",
  "source_id": 1,
  "state": "active",
  "allow_read_online": true,
  "allow_download": true,
  "starts_at": "2026-04-17T11:05:00Z",
  "expires_at": null,
  "formats": ["pdf", "epub"],
  "meta": {
    "source_module": "entitlement",
    "initiated_by": "order_paid"
  }
}
```

### 9.5.2 `entitlement.granted_from_membership`

```json
{
  "entitlement_id": 88002,
  "user_id": 901,
  "book_id": 450,
  "source_type": "membership",
  "source_id": 5002,
  "membership_id": 5002,
  "state": "active",
  "allow_read_online": true,
  "allow_download": true,
  "starts_at": "2026-04-17T11:05:00Z",
  "expires_at": "2027-04-17T11:04:59Z",
  "formats": ["pdf", "epub"],
  "meta": {
    "source_module": "entitlement",
    "initiated_by": "membership_activation"
  }
}
```

### 9.5.3 `entitlement.revoked`

```json
{
  "entitlement_id": 88002,
  "user_id": 901,
  "book_id": 450,
  "source_type": "membership",
  "source_id": 5002,
  "state": "revoked",
  "revoked_at": "2026-04-20T10:00:00Z",
  "reason_code": "membership_revoked",
  "reason_note": "Membership bị thu hồi",
  "meta": {
    "source_module": "entitlement",
    "initiated_by": "membership_or_refund"
  }
}
```

### 9.5.4 `entitlement.expired`

```json
{
  "entitlement_id": 88002,
  "user_id": 901,
  "book_id": 450,
  "source_type": "membership",
  "source_id": 5002,
  "state": "expired",
  "expired_at": "2027-04-17T11:05:00Z",
  "disable_new_download_links": true,
  "keep_download_history_visible": true,
  "meta": {
    "source_module": "entitlement",
    "initiated_by": "scheduler"
  }
}
```

## 9.6 Reader events

### 9.6.1 `reader.session_started`

**Topic**: `reader.events.v1`

```json
{
  "reader_session_id": 99001,
  "user_id": 901,
  "book_id": 302,
  "device_id": 2001,
  "format_type": "epub",
  "started_at": "2026-04-17T11:20:00Z",
  "concurrent_session_count": 2,
  "meta": {
    "source_module": "reader",
    "initiated_by": "user"
  }
}
```

### 9.6.2 `reader.position_updated`

```json
{
  "reader_session_id": 99001,
  "user_id": 901,
  "book_id": 302,
  "format_type": "epub",
  "position": {
    "chapter": 5,
    "progress_percent": 43.5,
    "locator": "epubcfi(/6/10!/4/2/8)"
  },
  "updated_at": "2026-04-17T11:35:00Z",
  "meta": {
    "source_module": "reader",
    "initiated_by": "heartbeat"
  }
}
```

### 9.6.3 `reader.session_ended`

```json
{
  "reader_session_id": 99001,
  "user_id": 901,
  "book_id": 302,
  "format_type": "epub",
  "ended_at": "2026-04-17T12:00:00Z",
  "reason": "user_closed",
  "meta": {
    "source_module": "reader",
    "initiated_by": "user"
  }
}
```

## 9.7 Download events

### 9.7.1 `download.link_issued`

**Topic**: `download.events.v1`

```json
{
  "download_id": 12001,
  "user_id": 901,
  "book_id": 302,
  "format_type": "pdf",
  "device_id": 2001,
  "source_type": "ebook_purchase",
  "source_id": 1,
  "token_id": "dl_abc123",
  "link_expires_at": "2026-04-17T11:25:00Z",
  "download_state": "issued",
  "meta": {
    "source_module": "download",
    "initiated_by": "user"
  }
}
```

### 9.7.2 `download.consumed`

```json
{
  "download_id": 12001,
  "user_id": 901,
  "book_id": 302,
  "format_type": "pdf",
  "token_id": "dl_abc123",
  "consumed_at": "2026-04-17T11:22:00Z",
  "download_state": "consumed",
  "meta": {
    "source_module": "download",
    "initiated_by": "file_gateway"
  }
}
```

### 9.7.3 `download.link_revoked`

```json
{
  "download_id": 12005,
  "user_id": 901,
  "book_id": 450,
  "token_id": "dl_xyz999",
  "download_state": "revoked",
  "revoked_at": "2027-04-17T11:05:00Z",
  "reason_code": "membership_expired",
  "meta": {
    "source_module": "download",
    "initiated_by": "scheduler"
  }
}
```

## 9.8 Inventory events

### 9.8.1 `inventory.reserved`

**Topic**: `inventory.events.v1`

```json
{
  "reservation_id": 7001,
  "order_id": 100300,
  "book_id": 888,
  "qty": 2,
  "reservation_state": "reserved",
  "reserved_at": "2026-04-17T11:05:00Z",
  "expires_at": "2026-04-17T11:20:00Z",
  "meta": {
    "source_module": "inventory",
    "initiated_by": "checkout_or_cod_confirm"
  }
}
```

### 9.8.2 `inventory.released`

```json
{
  "reservation_id": 7001,
  "order_id": 100300,
  "book_id": 888,
  "qty": 2,
  "reservation_state": "released",
  "released_at": "2026-04-17T11:22:00Z",
  "reason_code": "payment_expired",
  "meta": {
    "source_module": "inventory",
    "initiated_by": "scheduler"
  }
}
```

### 9.8.3 `inventory.adjusted`

```json
{
  "book_id": 888,
  "old_on_hand": 200,
  "new_on_hand": 180,
  "delta": -20,
  "reason_code": "manual_adjustment",
  "adjusted_by_admin_id": 12,
  "adjusted_at": "2026-04-17T14:00:00Z",
  "meta": {
    "source_module": "inventory",
    "initiated_by": "admin"
  }
}
```

## 9.9 Shipment events

### 9.9.1 `shipment.created`

**Topic**: `shipment.events.v1`

```json
{
  "shipment_id": 1212,
  "order_id": 100300,
  "shipment_state": "pending_pack",
  "carrier_code": null,
  "tracking_no": null,
  "cod_amount_vnd": 180000,
  "meta": {
    "source_module": "shipment",
    "initiated_by": "order_confirmed"
  }
}
```

### 9.9.2 `shipment.status_changed`

```json
{
  "shipment_id": 1212,
  "order_id": 100300,
  "previous_state": "in_transit",
  "shipment_state": "delivered",
  "tracking_no": "GHN1234567",
  "changed_at": "2026-04-19T14:30:00Z",
  "source": "carrier_webhook",
  "meta": {
    "source_module": "shipment",
    "initiated_by": "carrier"
  }
}
```

### 9.9.3 `shipment.poll_requested`

**Loại**: Command Event

```json
{
  "shipment_id": 1212,
  "order_id": 100300,
  "carrier_code": "ghn",
  "tracking_no": "GHN1234567",
  "requested_at": "2026-04-18T08:00:00Z",
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

## 9.10 Refund events

### 9.10.1 `refund.requested`

**Topic**: `refund.events.v1`

```json
{
  "refund_id": 3301,
  "order_id": 100245,
  "payment_id": 77891,
  "refund_state": "requested",
  "amount_vnd": 90000,
  "reason_code": "customer_request",
  "requested_by_type": "admin",
  "requested_by_id": 12,
  "requested_at": "2026-04-20T09:00:00Z",
  "meta": {
    "source_module": "refund",
    "initiated_by": "admin"
  }
}
```

### 9.10.2 `refund.completed`

```json
{
  "refund_id": 3301,
  "order_id": 100245,
  "payment_id": 77891,
  "refund_state": "completed",
  "amount_vnd": 90000,
  "completed_at": "2026-04-20T09:05:00Z",
  "revoke_entitlement_required": true,
  "meta": {
    "source_module": "refund",
    "initiated_by": "gateway_callback_or_manual"
  }
}
```

### 9.10.3 `refund.failed`

```json
{
  "refund_id": 3301,
  "order_id": 100245,
  "payment_id": 77891,
  "refund_state": "failed",
  "amount_vnd": 90000,
  "failed_at": "2026-04-20T09:06:00Z",
  "error_code": "GATEWAY_REFUND_ERROR",
  "meta": {
    "source_module": "refund",
    "initiated_by": "gateway"
  }
}
```

## 9.11 Chargeback events

### 9.11.1 `chargeback.opened`

**Topic**: `chargeback.events.v1`

```json
{
  "chargeback_id": 4101,
  "order_id": 100245,
  "payment_id": 77891,
  "chargeback_state": "open",
  "amount_vnd": 230000,
  "reason_code": "fraud_reported",
  "opened_at": "2026-05-01T10:00:00Z",
  "meta": {
    "source_module": "chargeback",
    "initiated_by": "gateway"
  }
}
```

### 9.11.2 `chargeback.resolved`

```json
{
  "chargeback_id": 4101,
  "order_id": 100245,
  "payment_id": 77891,
  "chargeback_state": "resolved",
  "resolution": "lost",
  "resolved_at": "2026-05-14T16:00:00Z",
  "revoke_entitlement_required": true,
  "meta": {
    "source_module": "chargeback",
    "initiated_by": "gateway"
  }
}
```

## 9.12 Notification command events

### 9.12.1 `notification.send_email`

**Topic**: `notification.commands.v1`

```json
{
  "notification_id": 9001,
  "template_code": "order_paid",
  "recipient_email": "user@example.com",
  "subject_params": {
    "order_no": "ORD202604170001"
  },
  "template_params": {
    "full_name": "Nguyễn Văn A",
    "order_no": "ORD202604170001",
    "total_vnd": 230000
  },
  "priority": "normal",
  "retry_policy": {
    "max_attempts": 5,
    "backoff_seconds": [30, 120, 300, 900, 1800]
  },
  "meta": {
    "source_module": "notification_bridge",
    "initiated_by": "domain_event"
  }
}
```

### 9.12.2 Mapping domain -> email command

| Domain event | Template email |
|---|---|
| `order.created` | `order_created` |
| `order.paid` | `order_paid` |
| `payment.failed` | `payment_failed` |
| `membership.activated` | `membership_activated` |
| `membership.expiring_soon` | `membership_expiring_soon` |
| `shipment.status_changed` | `shipment_status_changed` |
| `refund.completed` | `refund_success` |

## 9.13 Scheduler commands

### 9.13.1 `scheduler.membership_expiry_check`

**Topic**: `scheduler.commands.v1`

```json
{
  "job_type": "membership_expiry_check",
  "window_start": "2026-04-17T00:00:00Z",
  "window_end": "2026-04-17T23:59:59Z",
  "batch_no": 1,
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

### 9.13.2 `scheduler.payment_reconcile_pending`

```json
{
  "job_type": "payment_reconcile_pending",
  "provider": "vnpay",
  "payment_ids": [77891, 77892, 77893],
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

### 9.13.3 `scheduler.inventory_release_timeout`

```json
{
  "job_type": "inventory_release_timeout",
  "reservation_ids": [7001, 7002],
  "reason": "expired_reservation",
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

### 9.13.4 `scheduler.shipment_poll_batch`

```json
{
  "job_type": "shipment_poll_batch",
  "shipment_ids": [1212, 1213, 1214],
  "carrier_code": "ghn",
  "meta": {
    "source_module": "scheduler",
    "initiated_by": "system"
  }
}
```

## 9.14 Invoice events

### 9.14.1 `invoice.export_requested`

**Topic**: `invoice.events.v1`

```json
{
  "invoice_export_id": 6101,
  "order_id": 100245,
  "buyer_type": "individual",
  "amount_vnd": 230000,
  "tax_vnd": 0,
  "currency": "VND",
  "requested_at": "2026-04-17T11:06:00Z",
  "meta": {
    "source_module": "invoice",
    "initiated_by": "order_paid"
  }
}
```

### 9.14.2 `invoice.export_completed`

```json
{
  "invoice_export_id": 6101,
  "order_id": 100245,
  "provider_name": "external_einvoice_provider",
  "provider_ref": "INV-2026-0001",
  "exported_at": "2026-04-17T11:07:00Z",
  "meta": {
    "source_module": "invoice",
    "initiated_by": "provider_callback"
  }
}
```

### 9.14.3 `invoice.export_failed`

```json
{
  "invoice_export_id": 6101,
  "order_id": 100245,
  "failed_at": "2026-04-17T11:08:00Z",
  "error_code": "PROVIDER_TIMEOUT",
  "retryable": true,
  "meta": {
    "source_module": "invoice",
    "initiated_by": "worker"
  }
}
```

## 10. Producer contract chi tiết

### 10.1 Trách nhiệm producer

Producer phải đảm bảo:
- business transaction đã commit;
- payload hợp lệ theo schema version tương ứng;
- event key đúng theo aggregate;
- correlation/causation/trace IDs được gắn đầy đủ;
- không publish duplicate ngoài ý muốn nếu có thể phát hiện sớm;
- mọi lỗi publish đều được retry qua outbox worker, không nuốt lỗi thầm lặng.

### 10.2 Retry publish

| Tham số | Giá trị khuyến nghị |
|---|---|
| Số retry nhanh | 3 |
| Backoff | exponential + jitter |
| Max retry trước khi mark failed | 10 |
| Retry loop | worker định kỳ quét `failed` |
| Park message | khi vượt ngưỡng hoặc lỗi schema/config |

### 10.3 Khi nào không được phát event

- transaction business chưa commit;
- payload thiếu field bắt buộc;
- event version chưa được đăng ký trong repo contract;
- aggregate state chưa hợp lệ theo state machine.

## 11. Consumer contract chi tiết

### 11.1 Trách nhiệm consumer

Mỗi consumer phải khai báo rõ:
- `consumer_name`;
- topic subscribe;
- event types hỗ trợ;
- event versions hỗ trợ;
- idempotency strategy;
- retry strategy;
- DLQ strategy;
- side effects;
- timeout xử lý;
- owner team/module.

### 11.2 Quy trình xử lý chuẩn

1. nhận message;
2. parse envelope;
3. validate topic/event_type/version;
4. validate payload schema;
5. check idempotency;
6. lock hoặc claim nếu side effect nhạy cảm;
7. xử lý business;
8. ghi nhận processed event;
9. commit offset.

### 11.3 Idempotency bắt buộc

Consumer phải dùng `processed_events` hoặc cơ chế tương đương. Khuyến nghị chuẩn cho project này:
- `processed_events(consumer_name, event_id)` là canonical dedup persistence;
- có thể bổ sung Redis short dedup để giảm load DB nhưng không thay thế persistence chính.

### 11.4 Retry strategy

| Loại lỗi | Chiến lược |
|---|---|
| network timeout | retry |
| dependency unavailable | retry |
| lock contention ngắn hạn | retry |
| schema invalid | DLQ ngay |
| unsupported event version | DLQ hoặc parked queue |
| business invariant fail không recoverable | DLQ + alert |

### 11.5 Timeout xử lý khuyến nghị

| Consumer type | Timeout |
|---|---:|
| cache invalidation | 2 giây |
| email bridge | 5 giây |
| invoice export worker | 15 giây |
| payment reconcile | 15 giây |
| reporting projector | 5 giây |
| shipment poll worker | 20 giây |

## 12. DLQ specification

### 12.1 Topic DLQ

Phase hiện tại dùng chung:
- `dlq.general.v1`

Về sau có thể tách theo domain:
- `dlq.payment.v1`
- `dlq.notification.v1`
- `dlq.invoice.v1`

### 12.2 Payload DLQ chuẩn

```json
{
  "original_topic": "payment.events.v1",
  "consumer_name": "entitlement-grant-consumer",
  "event_id": "d2a33114-bf8d-4af9-93ee-a8f95b5d8c67",
  "event_type": "payment.captured",
  "failed_at": "2026-04-17T11:07:00Z",
  "failure_code": "SCHEMA_VALIDATION_FAILED",
  "failure_message": "missing payload.order_id",
  "retryable": false,
  "original_message": {}
}
```

### 12.3 Chính sách xử lý DLQ

- DLQ phải có dashboard và alert.
- Không được để DLQ “chết lâm sàng” không ai theo dõi.
- Phải có công cụ re-drive message sau khi sửa lỗi dữ liệu/code.
- Re-drive phải gắn cờ replay trong metadata nếu cần.

## 13. Versioning policy

### 13.1 Hai lớp version

- **Topic version**: ví dụ `order.events.v1`.
- **Event payload version**: `event_version = 1`, `2`, ...

### 13.2 Thay đổi không breaking

- thêm field optional;
- thêm metadata;
- thêm enum value mà consumer có thể ignore an toàn.

### 13.3 Thay đổi breaking

- đổi tên field bắt buộc;
- đổi kiểu dữ liệu field;
- xóa field bắt buộc;
- đổi semantic field hiện có.

### 13.4 Chính sách hỗ trợ version

- Event quan trọng như `order.paid`, `payment.captured`, `membership.activated`, `refund.completed` nên hỗ trợ ít nhất 2 version liên tiếp trong thời gian migrate.
- Producer không được nâng version breaking mà không có kế hoạch backward compatibility hoặc migration window.

## 14. Schema registry nội bộ

### 14.1 Hình thức lưu contract

Khuyến nghị lưu contract trong repo code dạng:
- `contracts/events/{topic}/{event_type}.v{n}.json`
hoặc
- Go struct + JSON schema generated.

### 14.2 Yêu cầu CI

- validate sample payload cho mọi event;
- validate backward compatibility nếu event upgrade non-breaking;
- contract test producer;
- contract test consumer.

### 14.3 Ví dụ file schema

`contracts/events/order.events.v1/order.paid.v1.json`

```json
{
  "type": "object",
  "required": [
    "order_id",
    "order_no",
    "user_id",
    "order_state",
    "payment_id",
    "payment_provider",
    "paid_amount_vnd",
    "paid_at"
  ],
  "properties": {
    "order_id": { "type": "integer" },
    "order_no": { "type": "string" },
    "user_id": { "type": "integer" },
    "order_state": { "type": "string", "enum": ["paid"] },
    "payment_id": { "type": "integer" },
    "payment_provider": { "type": "string" },
    "paid_amount_vnd": { "type": "integer" },
    "paid_at": { "type": "string", "format": "date-time" }
  }
}
```

## 15. Security và dữ liệu nhạy cảm

### 15.1 Nguyên tắc bảo mật

- chỉ đưa vào payload dữ liệu consumer thực sự cần;
- ưu tiên `user_id` thay vì email;
- email chỉ đưa vào `notification.send_email` hoặc event nào thực sự cần gửi ra ngoài;
- không log toàn bộ payload cho event có PII nhạy cảm;
- có policy mask trong logger.

### 15.2 Danh sách field cấm trong event

- password hash;
- plaintext token;
- secret gateway;
- raw provider signature secret;
- full signed download URL;
- raw card PAN/CVV;
- tài liệu nhị phân.

## 16. Observability

### 16.1 Metrics producer

- outbox pending count;
- outbox failed count;
- publish success count;
- publish failure count;
- publish latency;
- publish retry count.

### 16.2 Metrics consumer

- consume success/fail count;
- processing duration;
- retry count;
- DLQ count;
- schema validation fail count;
- idempotency hit count;
- lag per topic/group.

### 16.3 Traceability

- `trace_id`, `correlation_id`, `causation_id` là bắt buộc.
- API log, audit log, worker log phải in các ID này.
- Có thể truy từ HTTP request ban đầu đến event cuối cùng như email, invoice hoặc shipment sync.

## 17. Consumer group đề xuất

| Consumer group | Topic | Chức năng |
|---|---|---|
| `cg-catalog-cache` | `catalog.events.v1` | invalidate cache/search |
| `cg-order-reporting` | `order.events.v1` | reporting |
| `cg-payment-reconcile` | `payment.events.v1` | reconcile follow-up |
| `cg-membership-bridge` | `order.events.v1` | kích hoạt membership từ order.paid |
| `cg-entitlement-bridge` | `order.events.v1`, `membership.events.v1`, `refund.events.v1`, `chargeback.events.v1` | grant/revoke quyền |
| `cg-reader-sidefx` | `entitlement.events.v1`, `membership.events.v1` | đóng session/link khi quyền hết |
| `cg-download-audit` | `download.events.v1` | reporting/audit |
| `cg-email-worker` | `notification.commands.v1` | gửi email |
| `cg-shipment-sync` | `shipment.events.v1`, `scheduler.commands.v1` | poll và cập nhật shipment |
| `cg-invoice-export` | `invoice.events.v1` | gửi provider hóa đơn |
| `cg-reporting-feed` | nhiều topic | gom event cho reporting |

## 18. Mapping event -> side effect

| Event | Side effect chính |
|---|---|
| `order.paid` | kích hoạt membership, grant entitlement, gửi email, yêu cầu export invoice |
| `payment.failed` | gửi email thất bại thanh toán |
| `membership.expiring_soon` | gửi email nhắc gia hạn |
| `membership.expired` | expire entitlement membership, vô hiệu link tải mới |
| `refund.completed` | revoke entitlement nếu policy yêu cầu, gửi email |
| `chargeback.resolved` | revoke quyền hoặc khôi phục theo kết quả |
| `shipment.status_changed` | cập nhật order/shipment, gửi email |
| `catalog.price_changed` | invalidate cache/search, sync reporting |

## 19. Governance process

### 19.1 Khi thêm event mới

Bắt buộc phải có:
- tên event;
- owner module;
- topic;
- event key;
- sample payload;
- schema file;
- producer flow;
- consumer list;
- idempotency strategy;
- retry/DLQ policy;
- migration/versioning note.

### 19.2 Review checklist

- tên event có đúng semantic không;
- topic có đúng owner không;
- payload có thiếu field nghiệp vụ quan trọng không;
- payload có dư dữ liệu nhạy cảm không;
- key có đúng aggregate không;
- event có replay-safe không;
- consumer có idempotency không;
- có test contract không.

## 20. Acceptance criteria

### 20.1 Producer

- mọi domain event quan trọng đều publish qua outbox;
- event key đúng chuẩn aggregate;
- payload pass schema validation trước publish;
- retry publish hoạt động;
- duplicate publish không gây side effect sai ở downstream.

### 20.2 Consumer

- mọi consumer có idempotency;
- mọi consumer có retry strategy và DLQ policy;
- duplicate event không gây double email/double entitlement/double invoice export;
- unsupported version được phát hiện sớm và đẩy DLQ hoặc parked flow rõ ràng.

### 20.3 Operations

- có dashboard lag/outbox backlog/DLQ;
- có alert cho publish fail và consumer fail;
- có quy trình re-drive DLQ;
- có contract test trong CI.

## 21. Danh sách event tối thiểu bắt buộc cho project hiện tại

### 21.1 Catalog
- `catalog.book_created`
- `catalog.book_published`
- `catalog.book_updated`
- `catalog.price_changed`

### 21.2 Order
- `order.created`
- `order.payment_processing`
- `order.paid`
- `order.cancelled`
- `order.fulfilled`

### 21.3 Payment
- `payment.initiated`
- `payment.authorized`
- `payment.captured`
- `payment.failed`
- `payment.expired`
- `payment.reconcile_requested`
- `payment.reconciled`

### 21.4 Membership
- `membership.activated`
- `membership.renewed`
- `membership.expiring_soon`
- `membership.expired`
- `membership.revoked`

### 21.5 Entitlement
- `entitlement.granted`
- `entitlement.granted_from_membership`
- `entitlement.revoked`
- `entitlement.expired`

### 21.6 Reader/Download
- `reader.session_started`
- `reader.position_updated`
- `reader.session_ended`
- `download.link_issued`
- `download.consumed`
- `download.link_revoked`

### 21.7 Inventory/Shipment
- `inventory.reserved`
- `inventory.released`
- `inventory.adjusted`
- `shipment.created`
- `shipment.status_changed`
- `shipment.poll_requested`

### 21.8 Refund/Chargeback
- `refund.requested`
- `refund.completed`
- `refund.failed`
- `chargeback.opened`
- `chargeback.resolved`

### 21.9 Notification/Scheduler/Invoice
- `notification.send_email`
- `scheduler.membership_expiry_check`
- `scheduler.payment_reconcile_pending`
- `scheduler.inventory_release_timeout`
- `scheduler.shipment_poll_batch`
- `invoice.export_requested`
- `invoice.export_completed`
- `invoice.export_failed`

## 22. Kết luận triển khai

Tài liệu này đủ mức chi tiết để team backend bắt đầu dựng:
- package contract/schema cho event;
- outbox publisher worker;
- consumer base framework;
- retry/DLQ pipeline;
- mapping event-to-side-effect cho các domain chính;
- contract test và observability chuẩn cho Kafka.

Sau tài liệu này, bước hợp lý tiếp theo là viết:
- **Kafka Event Catalog dạng bảng chuẩn doanh nghiệp**;
- **State Transition Spec** cho order/payment/refund/membership/entitlement/shipment;
- **OpenAPI SPECS** cho public/admin APIs;
- **Implementation blueprint** cho package `events`, `outbox`, `consumers`, `workers` trong Golang monolith.
