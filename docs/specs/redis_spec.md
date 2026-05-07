# Tài liệu Redis Key & TTL Specification Chi Tiết

## 1. Mục đích tài liệu

Tài liệu này đặc tả chi tiết cách sử dụng Redis cho hệ thống backend monolith bán sách sử dụng Golang/Gin, PostgreSQL, Redis và Kafka. Tài liệu đóng vai trò là bản thiết kế kỹ thuật cấp triển khai cho toàn bộ lớp Redis trong project đang planning hiện tại, bám sát các quyết định đã chốt trong SRD/URD, ERD logic, physical schema và Kafka event contract. Redis trong hệ thống này không phải là nguồn sự thật chính cho trạng thái nghiệp vụ bền vững; thay vào đó, Redis được dùng có chủ đích cho cache, rate limiting, session/token/device registry, idempotency, cart cache, distributed lock, flash sale stock guard, job deduplication, quota hot-path và metadata ngắn hạn cho download link.

## 2. Vai trò Redis trong kiến trúc hiện tại

### 2.1 Mục tiêu sử dụng Redis

Redis được dùng để đạt các mục tiêu sau:
- giảm tải truy vấn lặp lên PostgreSQL;
- hỗ trợ hot-path có độ trễ thấp cho API công cộng;
- cung cấp cơ chế atomic cho quota, rate limit, lock và guard;
- hỗ trợ trạng thái ngắn hạn của session, cart và download token;
- cung cấp lớp coordination đơn giản giữa web handlers, workers và scheduler.

### 2.2 Redis không được dùng cho những gì

Redis **không** được xem là canonical source of truth cho:
- order state;
- payment state;
- refund/chargeback state;
- entitlement state canonical;
- inventory reservation canonical;
- shipment state canonical.

Nếu Redis mất dữ liệu, hệ thống phải vẫn phục hồi được correctness từ PostgreSQL, dù có thể mất hiệu năng tạm thời.

### 2.3 Nguyên tắc tổng quát

- Mọi key phải có naming convention thống nhất.
- Mọi key phải có owner module rõ ràng.
- Mọi key phải có TTL hoặc lý do chính đáng nếu không TTL.
- Không lưu object quá lớn hoặc graph phức tạp không cần thiết.
- Dữ liệu cache phải có chiến lược invalidation rõ ràng.
- Dữ liệu coordination phải ưu tiên atomic primitives và Lua script khi cần.
- Mọi usage phải phân biệt rõ: **cache**, **ephemeral state**, **coordination**, **counter**, **lock**, **dedup**.

## 3. Phân loại key Redis toàn hệ thống

| Nhóm | Mục đích | Ví dụ |
|---|---|---|
| Cache catalog | Cache dữ liệu đọc nhiều | `catalog:list:v1:{hash}` |
| Cache search | Cache kết quả search/filter | `search:books:v1:{hash}` |
| Cache chi tiết sách | Cache book detail + format info | `book:detail:v1:{book_id}` |
| Cart cache | Lưu cart nóng của user | `cart:v1:{user_id}` |
| Session registry | Session active và refresh chain hot-path | `session:v1:{session_id}` |
| User sessions index | Liệt kê session active của user | `user:sessions:v1:{user_id}` |
| Device registry | Thiết bị đã đăng ký / active | `device:v1:{user_id}:{device_id}` |
| Entitlement hot cache | Cache can-read/can-download | `entitlement:check:v1:{user_id}:{book_id}` |
| Reader quota | Concurrent reading counter | `reader:active:v1:{user_id}` |
| Download quota | Giới hạn lượt tải | `download:quota:v1:{scope}:{id}` |
| Rate limiting | Hạn mức request/action | `rate:v1:{scope}:{subject}` |
| Idempotency | Chống submit/callback lặp | `idem:v1:{scope}:{key}` |
| Lock | Distributed lock ngắn hạn | `lock:v1:{resource}` |
| Stock guard | Guard flash sale và reserve hot-path | `stock:guard:v1:{book_id}` |
| Job dedup | Chống scheduler/job xử lý lặp | `jobdedup:v1:{job_type}:{biz_key}` |
| Download token meta | Metadata link tải ngắn hạn | `download:token:v1:{token_id}` |
| Notification rate control | Chống spam gửi email | `notify:guard:v1:{template}:{target}` |
| Webhook dedup | Chống callback payment/shipping lặp | `webhook:dedup:v1:{provider}:{external_id}` |

## 4. Chuẩn đặt tên key

### 4.1 Format chuẩn

Format bắt buộc:

```text
{domain}:{subdomain}:{version}:{identifier...}
```

Ví dụ:
- `book:detail:v1:302`
- `search:books:v1:8e8fcb4f`
- `session:v1:7f2d2d63-3a1c-4fd4-bd8b-0f1122334455`
- `idem:v1:checkout:2be99bf9...`

### 4.2 Quy tắc naming

- tất cả lowercase;
- dùng dấu `:` làm delimiter;
- có version segment `v1` để hỗ trợ thay đổi cấu trúc value sau này;
- không embed thông tin quá dài nếu có thể hash query/params;
- không dùng key chứa PII thô như email đầy đủ nếu không thực sự cần;
- nếu subject là query phức tạp, phải hash canonical string thành digest ngắn.

### 4.3 Quy tắc hash query

Đối với cache list/search, key phải được tạo theo canonical serialization rồi hash SHA-256 hoặc xxhash:
- sort params theo thứ tự cố định;
- bỏ field mặc định không ảnh hưởng query plan nếu muốn chuẩn hóa;
- normalize boolean/date/range format;
- sau đó hash để tránh key quá dài.

## 5. Chiến lược serialize value

### 5.1 Định dạng value

- JSON là mặc định cho object nhỏ/vừa, dễ debug.
- String đơn giản cho flag, token, lock owner, idempotency marker.
- Hash Redis cho object có update field-level nhỏ, nhưng không lạm dụng.
- Set/Sorted Set cho registry/index/counter theo thời gian.
- Lua script khi cần thao tác nhiều key/counter atomically.

### 5.2 Nguyên tắc chọn data structure

| Use case | Data structure | Lý do |
|---|---|---|
| Cache object | String(JSON) | Dễ debug, dễ invalidate |
| Danh sách session active | Set / Sorted Set | Dễ thêm/xóa/liệt kê |
| Rate limit rolling window | Sorted Set | Xóa theo thời gian |
| Simple rate limit fixed window | String counter | Nhanh, đơn giản |
| Lock | String | SET NX PX chuẩn |
| Idempotency | String/Hash | Lưu state request |
| Device registry index | Set | Lấy toàn bộ device của user |
| Quota count | String counter | Atomic incr/decr |
| Download token meta | Hash hoặc JSON string | TTL ngắn, đọc nhanh |

## 6. Chiến lược TTL tổng thể

### 6.1 Nguyên tắc TTL

- TTL phải phản ánh tính chất dữ liệu: càng gần canonical state thay đổi nhanh, TTL càng ngắn.
- Dữ liệu derived từ catalog có thể TTL dài hơn dữ liệu liên quan checkout/payment.
- Key không TTL chỉ chấp nhận nếu là index/set phải tồn tại lâu và có cleanup policy riêng.
- Key coordination như lock/idempotency/webhook dedup phải luôn có TTL để tránh rác vĩnh viễn.

### 6.2 Ma trận TTL tổng quát

| Nhóm key | TTL khuyến nghị | Ghi chú |
|---|---|---|
| Book detail cache | 15 phút | Invalidate theo admin update |
| Catalog list cache | 5 phút | Query thay đổi tương đối thường |
| Search result cache | 2–5 phút | Dễ stale khi đổi publish/price |
| Cart cache | 7 ngày | Gia hạn theo hoạt động user |
| Session object | đến `refresh_token_expires_at` hoặc 30 ngày | Đồng bộ với auth policy |
| Session index | 30 ngày | Refresh khi session active |
| Device activity | 90 ngày | Có cleanup job |
| Entitlement check cache | 1–5 phút | Invalidate theo order/refund/membership |
| Reader concurrent marker | 5–15 phút | Gia hạn theo heartbeat |
| Download quota counters | theo policy window | 1 ngày / 30 ngày / vĩnh viễn logic |
| Rate limit | 1 phút / 5 phút / 1 giờ | Theo endpoint/action |
| Idempotency checkout | 24 giờ | Chống double submit |
| Webhook dedup | 7–30 ngày | Theo provider behavior |
| Lock | 5–30 giây | Tùy critical section |
| Flash sale guard | 3–10 giây | Gia hạn ngắn |
| Job dedup | 1–24 giờ | Theo loại job |
| Download token meta | 1–15 phút | Phải ngắn hạn |
| Notification guard | 5–30 phút | Chống spam cùng template |

## 7. Spec chi tiết theo từng domain

## 7.1 Catalog cache

### 7.1.1 Book detail cache

**Key pattern**
```text
book:detail:v1:{book_id}
```

**Value**
```json
{
  "book_id": 302,
  "slug": "ky-nang-doc-sau",
  "title": "Kỹ năng đọc sâu",
  "product_type": "hybrid",
  "membership_eligible": true,
  "published": true,
  "formats": [
    {"type": "pdf", "downloadable": true, "online_readable": true},
    {"type": "epub", "downloadable": true, "online_readable": true},
    {"type": "physical", "downloadable": false, "online_readable": false}
  ],
  "price": {
    "list_price_vnd": 120000,
    "sale_price_vnd": 99000
  },
  "updated_at": "2026-04-17T18:00:00Z"
}
```

**TTL**
- 15 phút mặc định.

**Nguồn dữ liệu**
- PostgreSQL `books`, `book_formats`, `prices`, join liên quan.

**Invalidation triggers**
- book updated;
- format updated;
- price changed;
- published flag changed;
- membership flag changed.

**Invalidation strategy**
- delete exact key `book:detail:v1:{book_id}` khi có event catalog update.

### 7.1.2 Catalog list cache

**Key pattern**
```text
catalog:list:v1:{hash}
```

**Hash input**
- page
- page_size
- category
- author
- membership_eligible
- product_type
- sort
- published=true fixed

**Value**
```json
{
  "items": [302, 303, 401],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 120
  },
  "facets": {
    "categories": [],
    "formats": []
  },
  "generated_at": "2026-04-17T18:00:00Z"
}
```

**TTL**
- 5 phút.

**Invalidation triggers**
- publish/unpublish sách;
- giá thay đổi;
- category mapping thay đổi;
- preorder status thay đổi.

**Ghi chú**
- chỉ cache ID list + facet nếu cần, chi tiết card nên hydrate từ `book:detail` để giảm duplication.

### 7.1.3 Search result cache

**Key pattern**
```text
search:books:v1:{hash}
```

**TTL**
- 2 phút cho query text có sort relevance.
- 5 phút cho query filter-only không có text.

**Invalidation**
- không cần xóa toàn bộ theo từng record nếu TTL ngắn; nhưng khi admin publish/unpublish hàng loạt, có thể bump `search namespace version`.

**Namespace version pattern tùy chọn**
```text
namespace:search:books:v1 -> integer
search:books:v1:{namespace}:{hash}
```

## 7.2 Cart cache

### 7.2.1 Cart object

**Key pattern**
```text
cart:v1:{user_id}
```

**Value**
```json
{
  "cart_id": 501,
  "user_id": 901,
  "items": [
    {
      "sku_type": "ebook",
      "sku_id": 302,
      "quantity": 1,
      "unit_price_vnd": 99000,
      "final_unit_price_vnd": 89000,
      "metadata_snapshot": {}
    }
  ],
  "coupon_code": "SALE10",
  "currency": "VND",
  "pricing_snapshot": {
    "subtotal_vnd": 99000,
    "discount_vnd": 10000,
    "total_vnd": 89000
  },
  "updated_at": "2026-04-17T18:00:00Z"
}
```

**TTL**
- 7 ngày kể từ lần cập nhật cuối.
- Gia hạn TTL mỗi khi user thao tác cart.

**Write policy**
- Write-through hoặc cache-aside hybrid.
- Khi sửa cart: cập nhật PostgreSQL trước, sau đó set Redis.

**Invalidation**
- checkout thành công -> xóa `cart:v1:{user_id}`;
- cart expired -> cleanup job xóa;
- user logout không bắt buộc xóa cart.

### 7.2.2 Cart lock ngắn hạn khi checkout

**Key pattern**
```text
lock:v1:cart-checkout:{user_id}
```

**Value**
- request_id / actor info

**TTL**
- 10 giây.

**Use case**
- chống double-submit checkout đồng thời.

## 7.3 Session & auth

### 7.3.1 Session object

**Key pattern**
```text
session:v1:{session_id}
```

**Value**
```json
{
  "id": 12345,
  "user_id": 901,
  "device_id": 2001,
  "session_status": "active",
  "refresh_token_hash": "...",
  "expires_at": "2026-05-17T18:00:00Z",
  "last_seen_at": "2026-04-17T18:10:00Z",
  "created_at": "2026-04-17T18:00:00Z",
  "updated_at": "2026-04-17T18:10:00Z"
}
```

**TTL**
- đến `expires_at` của refresh session.
- tối đa 30 ngày theo policy hiện tại.

### 7.3.2 User sessions index

**Key pattern**
```text
user:sessions:v1:{user_id}
```

**Data structure**
- Sorted Set: member = `session_id`, score = last_seen unix timestamp.

**TTL**
- 30 ngày, refresh khi có session hoạt động.

**Use case**
- list active sessions nhanh;
- enforce concurrent login limit;
- cleanup session cũ.

### 7.3.3 Access token denylist (nếu dùng revoke tức thời)

**Key pattern**
```text
token:blacklist:v1:{jti}
```

**Value**
- `1`

**TTL**
- bằng thời gian còn lại đến khi access token hết hạn.

**Chỉ dùng khi cần revoke access token ngay**.

## 7.4 Device registry

### 7.4.1 Device object

**Key pattern**
```text
device:v1:{user_id}:{device_id}
```

**Value**
```json
{
  "device_id": 2001,
  "user_id": 901,
  "fingerprint_hash": "abcxyz",
  "label": "Chrome on Windows",
  "last_seen_at": "2026-04-17T18:00:00Z",
  "revoked": false
}
```

**TTL**
- 90 ngày kể từ lần seen cuối.
- refresh khi đọc online, tải file, login.

### 7.4.2 User devices index

**Key pattern**
```text
user:devices:v1:{user_id}
```

**Data structure**
- Set chứa `device_id`

**TTL**
- 90 ngày, refresh theo activity.

**Use case**
- check nhanh số thiết bị đang tồn tại;
- hỗ trợ enforce `max_devices`.

## 7.5 Entitlement hot cache

### 7.5.1 Quyền đọc/tải một sách

**Key pattern**
```text
entitlement:check:v1:{user_id}:{book_id}
```

**Value**
```json
{
  "user_id": 901,
  "book_id": 302,
  "can_read": true,
  "can_download": true,
  "source_type": "ebook_purchase",
  "source_id": 1,
  "expires_at": null,
  "checked_at": "2026-04-17T18:00:00Z"
}
```

**TTL**
- 1 phút cho sách membership hoặc quyền dễ đổi.
- 5 phút cho ebook purchase vĩnh viễn.

**Invalidation triggers**
- order paid;
- refund completed;
- chargeback resolved;
- membership activated/expired/revoked;
- admin manual grant/revoke.

**Lưu ý**
- Redis chỉ là accelerate path. Khi miss hoặc nghi ngờ stale, service phải đọc PostgreSQL `entitlements` canonical.

### 7.5.2 Thư viện user

**Key pattern**
```text
library:list:v1:{user_id}:{hash}
```

**TTL**
- 2 phút.

**Value**
- danh sách `book_id` + cursor/pagination summary.

## 7.6 Reader session & concurrent limit

### 7.6.1 Bộ đếm session đọc active theo user

**Key pattern**
```text
reader:active:count:v1:{user_id}
```

**Value**
- integer

**TTL**
- 15 phút.
- refresh theo heartbeat.

### 7.6.2 Registry session đọc active

**Key pattern**
```text
reader:active:sessions:v1:{user_id}
```

**Data structure**
- Sorted Set, member = `reader_session_id`, score = `last_heartbeat_unix`

**TTL**
- 15 phút.

**Use case**
- đếm concurrent sessions;
- cleanup stale session entries;
- liệt kê session đang đọc.

### 7.6.3 Per-session heartbeat

**Key pattern**
```text
reader:session:v1:{reader_session_id}
```

**Value**
```json
{
  "reader_session_id": 99001,
  "user_id": 901,
  "book_id": 302,
  "device_id": 2001,
  "format_type": "epub",
  "last_position": {
    "chapter": 5,
    "progress_percent": 43.5
  },
  "last_heartbeat_at": "2026-04-17T18:00:00Z"
}
```

**TTL**
- 10 phút.

**Flow**
- start reading: set session key + add zset member + incr counter;
- heartbeat: extend TTL + update zset score;
- end reading: del session key + remove zset + decr counter;
- stale cleanup worker: quét zset score cũ và sửa counter nếu lệch.

## 7.7 Download token & quota

### 7.7.1 Download token metadata

**Key pattern**
```text
download:token:v1:{token_id}
```

**Value**
```json
{
  "download_id": 12001,
  "user_id": 901,
  "book_id": 302,
  "format_type": "pdf",
  "device_id": 2001,
  "source_type": "ebook_purchase",
  "source_id": 1,
  "single_use": true,
  "issued_at": "2026-04-17T18:00:00Z",
  "expires_at": "2026-04-17T18:05:00Z"
}
```

**TTL**
- 5 phút mặc định.
- tối đa 15 phút nếu có yêu cầu mạng yếu, nhưng phase hiện tại khuyến nghị 5 phút.

**Use case**
- xác thực metadata cho Signed Download URL tại CDN/file-serving layer;
- tránh query DB liên tục mỗi lần mở link, tự rụng khi hết TTL ngắn.

### 7.7.2 Download single-use marker

**Key pattern**
```text
download:token:used:v1:{token_id}
```

**Value**
- timestamp hoặc `1`

**TTL**
- bằng thời gian còn lại của token + 1 giờ buffer.

**Use case**
- enforce one-time use nếu policy bật.

### 7.7.3 Download quota theo user-book

**Key pattern**
```text
download:quota:v1:user-book:{user_id}:{book_id}:{window}
```

Ví dụ:
```text
download:quota:v1:user-book:901:302:2026-04
```

**Value**
- integer counter

**TTL**
- hết kỳ quota + 7 ngày buffer.

### 7.7.4 Download quota toàn membership

**Key pattern**
```text
download:quota:v1:membership:{membership_id}:{window}
```

**TTL**
- đến cuối quota window + 7 ngày buffer.

### 7.7.5 Download history precomputed short cache

**Key pattern**
```text
download:history:v1:{user_id}:{page}:{page_size}
```

**TTL**
- 1 phút.

## 7.8 Rate limiting

### 7.8.1 Rate limit login

**Key pattern**
```text
rate:v1:login:ip:{ip}
rate:v1:login:user:{user_id_or_email_hash}
```

**TTL**
- 15 phút cho failed-attempt window.

**Policy ví dụ**
- max 10 attempts / 15 phút / IP
- max 5 attempts / 15 phút / account

### 7.8.2 Rate limit API search

**Key pattern**
```text
rate:v1:search:user:{user_id}
rate:v1:search:ip:{ip}
```

**TTL**
- 1 phút.

### 7.8.3 Rate limit download request

**Key pattern**
```text
rate:v1:download:user:{user_id}
rate:v1:download:book:{user_id}:{book_id}
```

**TTL**
- 1 phút cho chống spam request tạo link;
- 1 giờ cho quota mềm nếu cần.

### 7.8.4 Rate limit checkout / payment initiation

**Key pattern**
```text
rate:v1:checkout:user:{user_id}
rate:v1:payment:init:user:{user_id}
```

**TTL**
- 5 phút.

### 7.8.5 Thuật toán khuyến nghị

- fixed window cho endpoint đơn giản;
- sliding log / token bucket cho action nhạy cảm hơn;
- Lua script cho atomics đa key.

## 7.9 Idempotency

### 7.9.1 Checkout idempotency

**Key pattern**
```text
idem:v1:checkout:{idempotency_key}
```

**Value**
```json
{
  "scope": "checkout",
  "user_id": 901,
  "request_hash": "abc123",
  "status": "completed",
  "order_id": 100245,
  "created_at": "2026-04-17T18:00:00Z"
}
```

**TTL**
- 24 giờ.

### 7.9.2 Payment callback idempotency

**Key pattern**
```text
idem:v1:payment-callback:{provider}:{external_ref}
```

**TTL**
- 30 ngày.

### 7.9.3 Refund request idempotency

**Key pattern**
```text
idem:v1:refund:{request_key}
```

**TTL**
- 7 ngày.

### 7.9.4 Admin action idempotency cho operation nhạy cảm

**Key pattern**
```text
idem:v1:admin:{action}:{idempotency_key}
```

**TTL**
- 24 giờ.

## 7.10 Webhook deduplication

### 7.10.1 Payment webhook

**Key pattern**
```text
webhook:dedup:v1:payment:{provider}:{external_event_id}
```

**TTL**
- 30 ngày.

### 7.10.2 Shipment webhook

**Key pattern**
```text
webhook:dedup:v1:shipment:{carrier}:{external_event_id}
```

**TTL**
- 14 ngày.

### 7.10.3 Invoice provider callback

**Key pattern**
```text
webhook:dedup:v1:invoice:{provider}:{external_event_id}
```

**TTL**
- 30 ngày.

## 7.11 Distributed lock

### 7.11.1 Quy tắc lock

- dùng `SET key value NX PX ttl_ms`;
- value phải là lock token ngẫu nhiên;
- release lock phải check token bằng Lua script;
- không `DEL` lock mù;
- không dùng lock có TTL quá dài trừ khi có heartbeat/renewal rõ ràng.

### 7.11.2 Lock patterns

| Key | TTL | Use case |
|---|---|---|
| `lock:v1:checkout:{user_id}` | 10 giây | chống double checkout |
| `lock:v1:payment-finalize:{payment_id}` | 15 giây | tránh finalize song song từ redirect + webhook |
| `lock:v1:inventory-reserve:{book_id}` | 5 giây | guard reserve cùng SKU nóng |
| `lock:v1:membership-activate:{user_id}` | 10 giây | tránh cộng dồn membership song song |
| `lock:v1:entitlement-grant:{user_id}:{book_id}` | 10 giây | tránh grant trùng |
| `lock:v1:invoice-export:{order_id}` | 30 giây | tránh export hóa đơn trùng |

## 7.12 Flash sale stock guard

### 7.12.1 Guard counter

**Key pattern**
```text
stock:guard:v1:{sellable_sku_id}
```

**Value**
- integer available guard count, sync định kỳ từ DB

**TTL**
- 30 giây đến 2 phút, tùy campaign.

**Use case**
- absorb spike đọc stock trong flash sale;
- pre-check trước khi vào transaction tạo Pending Hold TTL ở PostgreSQL. Mọi Canonical State vẫn nằm ở DB.

### 7.12.2 Reservation pending short-lived

**Key pattern**
```text
stock:pending:v1:{sellable_sku_id}:{request_id}
```

**TTL**
- 10 giây.

**Use case**
- đánh dấu reserve đang diễn ra để giảm oversubscription trong khoảng cực ngắn.

## 7.13 Job deduplication & scheduler coordination

### 7.13.1 Job dedup key

**Key pattern**
```text
jobdedup:v1:{job_type}:{biz_key}
```

**Ví dụ**
- `jobdedup:v1:payment_reconcile:77891`
- `jobdedup:v1:membership_expiry_check:2026-04-17`
- `jobdedup:v1:shipment_poll:1212:2026-04-17T18`

**TTL**
- 1 giờ đến 24 giờ tùy job type.

### 7.13.2 Scheduler lease

**Key pattern**
```text
lock:v1:scheduler:{job_type}
```

**TTL**
- 30 giây đến 2 phút.

**Use case**
- tránh hai worker cùng quét một nhóm job batch nếu scale ngang.

## 7.14 Notification guard

### 7.14.1 Anti-spam email cùng template

**Key pattern**
```text
notify:guard:v1:{template_code}:{recipient_hash}
```

**TTL**
- 5 phút đến 1 giờ tùy loại email.

**Use case**
- tránh gửi lặp do retry logic sai hoặc event duplicated.

### 7.14.2 Notification batch dedup

**Key pattern**
```text
notify:batch:v1:{template_code}:{biz_key}
```

**TTL**
- 24 giờ.

## 8. Chi tiết TTL theo module

## 8.1 Bảng TTL chuẩn tổng hợp

| Module | Key pattern | TTL | Gia hạn TTL | Xóa chủ động |
|---|---|---:|---|---|
| Catalog | `book:detail:v1:{book_id}` | 15 phút | Không | Có |
| Catalog | `catalog:list:v1:{hash}` | 5 phút | Không | Có |
| Search | `search:books:v1:{hash}` | 2–5 phút | Không | Có / namespace bump |
| Cart | `cart:v1:{user_id}` | 7 ngày | Có | Có |
| Auth | `session:v1:{session_id}` | đến expiry | Có | Có |
| Auth | `user:sessions:v1:{user_id}` | 30 ngày | Có | Có |
| Device | `device:v1:{user_id}:{device_id}` | 90 ngày | Có | Có |
| Device | `user:devices:v1:{user_id}` | 90 ngày | Có | Có |
| Entitlement | `entitlement:check:v1:{user_id}:{book_id}` | 1–5 phút | Không | Có |
| Reader | `reader:session:v1:{id}` | 10 phút | Có | Có |
| Reader | `reader:active:sessions:v1:{user_id}` | 15 phút | Có | Có |
| Download | `download:token:v1:{token_id}` | 5 phút | Không | Có |
| Download | `download:token:used:v1:{token_id}` | token remaining + 1h | Không | Không |
| Rate limit | `rate:v1:*` | 1 phút–1 giờ | Không | Không |
| Idempotency | `idem:v1:*` | 24h / 7d / 30d | Không | Tùy use case |
| Webhook dedup | `webhook:dedup:v1:*` | 14–30 ngày | Không | Không |
| Lock | `lock:v1:*` | 5–30 giây | Có chọn lọc | Có |
| Job dedup | `jobdedup:v1:*` | 1–24 giờ | Không | Không |
| Notification guard | `notify:guard:v1:*` | 5 phút–1 giờ | Không | Không |

## 9. Invalidation strategy chi tiết

### 9.1 Cache-aside chuẩn

Flow chuẩn:
1. đọc Redis;
2. miss -> đọc PostgreSQL;
3. serialize set vào Redis với TTL;
4. trả dữ liệu.

### 9.2 Write-through áp dụng hạn chế

Dùng cho:
- cart;
- session hot object;
- device activity;
- reader session ephemeral.

### 9.3 Event-driven invalidation

Nguồn invalidation chính:
- Kafka event từ catalog/payment/membership/entitlement/refund/shipment.

Ví dụ mapping:
- `catalog.book_updated` -> xóa `book:detail` liên quan, bump namespace search nếu cần;
- `order.paid` -> xóa cart user, invalidate entitlement/library cache;
- `membership.expired` -> xóa entitlement check/library cache, revoke download token liên quan theo logic service;
- `refund.completed` -> invalidate entitlement/order summary cache.

### 9.4 Namespace versioning

Đề xuất dùng cho cache khó xóa theo từng key hàng loạt:
- search namespace;
- catalog list namespace.

**Pattern**
```text
namespace:catalog:list:v1 -> integer
catalog:list:v1:{ns}:{hash}
```

Khi có thay đổi hàng loạt:
- INCR namespace key;
- key cũ tự chết theo TTL.

## 10. Atomicity và Lua script cần có

### 10.1 Download token consume

Yêu cầu atomic:
- kiểm tra token tồn tại;
- kiểm tra chưa used;
- set used marker;
- trả metadata token.

Nên dùng Lua để tránh race click đôi.

### 10.2 Reader session start

Yêu cầu atomic:
- check current active count;
- nếu dưới quota -> incr count + add zset + set session key;
- nếu vượt -> reject.

### 10.3 Device registration guard

Yêu cầu atomic:
- check số device hiện có;
- nếu chưa vượt quota -> add device index;
- set device object.

### 10.4 Checkout idempotency claim

Yêu cầu atomic:
- nếu key chưa tồn tại -> set processing;
- nếu đã completed -> trả order cũ;
- nếu processing -> trả xung đột/retry.

## 11. Memory management và max size policy

### 11.1 Chính sách bộ nhớ

- Redis dùng cho dữ liệu nóng, không giữ vô hạn.
- Object cache nên compact, không nhét HTML, binary, full content sách.
- Không lưu full EPUB/PDF bytes trong Redis.

### 11.2 Max payload khuyến nghị

| Nhóm value | Kích thước khuyến nghị |
|---|---:|
| Book detail cache | < 20 KB |
| Cart cache | < 50 KB |
| Session/device object | < 5 KB |
| Entitlement check | < 2 KB |
| Download token meta | < 2 KB |
| Notification guard / rate / lock | < 512 B |

### 11.3 Eviction policy

Khuyến nghị Redis instance cho project này:
- `volatile-ttl` hoặc `allkeys-lru` tùy profile;
- nếu Redis trộn cả ephemeral coordination và cache, nên cân nhắc tách logical DB/instance hoặc namespace rõ ràng;
- coordination keys quan trọng cần có monitor để tránh eviction bất ngờ.

## 12. Bảo mật dữ liệu Redis

### 12.1 Không lưu gì trong Redis

- password hash;
- raw access token nếu không cần;
- raw refresh token plaintext;
- signed download URL đầy đủ nếu có thể tránh;
- dữ liệu thanh toán nhạy cảm;
- payload webhook chứa secret.

### 12.2 Dữ liệu cần hash/mask

- email trong notification guard nên hash;
- fingerprint nếu nhạy cảm có thể chỉ lưu hash rút gọn;
- IP trong rate limit có thể dùng raw nếu nội bộ an toàn, nhưng cần policy rõ.

### 12.3 Transport & access

- Redis chỉ mở trong network nội bộ;
- bắt buộc auth/password/ACL;
- TLS nếu môi trường yêu cầu;
- production không cho truy cập console rộng rãi.

## 13. Observability cho Redis

### 13.1 Metrics bắt buộc

- cache hit rate / miss rate theo domain;
- latency per command nhóm chính;
- used memory / fragmentation;
- evicted keys;
- expired keys;
- keyspace hit/miss;
- Lua error count;
- lock contention count;
- rate-limited request count;
- idempotency hit count.

### 13.2 Logging

- log miss bất thường cho key nóng;
- log fallback DB khi Redis lỗi;
- log contention ở checkout, payment finalize, inventory reserve;
- không log full payload chứa PII.

### 13.3 Dashboard vận hành

Dashboard nên có các panel:
- catalog cache hit ratio;
- search cache hit ratio;
- session count active;
- reader active count;
- rate limit rejects;
- idempotency duplicates prevented;
- download token issued vs consumed vs expired;
- lock timeout/concurrency conflicts.

## 14. Failure mode và degradation strategy

### 14.1 Redis unavailable

Ứng xử hệ thống:
- catalog/search: fallback PostgreSQL, có thể chậm hơn;
- cart: fallback PostgreSQL;
- session validation: fallback DB nếu thiết kế cho phép; nếu không thì degrade theo policy auth;
- rate limiting: fail-open với endpoint ít nhạy cảm, fail-closed hoặc soft-block với login/payment/webhook tùy risk;
- download token: có thể fallback DB validate nếu token meta có log bền vững tương ứng;
- lock/idempotency: với nghiệp vụ nhạy cảm phải có fallback DB transaction/unique constraint hỗ trợ.

### 14.2 Stale cache

- catalog stale chấp nhận ngắn hạn vài phút;
- entitlement stale chấp nhận kém hơn, nên TTL ngắn và invalidate tích cực;
- checkout không được tin cache giá hoàn toàn, luôn re-price server-side từ source đúng.

### 14.3 Counter drift

- reader active count có thể drift nếu crash giữa chừng;
- cần periodic reconciler dựa trên zset/session timestamps và PostgreSQL session logs.

## 15. Mapping Redis với PostgreSQL/Kafka theo domain

| Domain | Redis key | PostgreSQL nguồn đúng | Kafka event invalidation |
|---|---|---|---|
| Catalog | `book:detail:*`, `catalog:list:*` | books, book_formats, prices | catalog events |
| Search | `search:books:*` | books + search projection | catalog events |
| Cart | `cart:*` | carts, cart_items | order created / cart updated |
| Session | `session:*`, `user:sessions:*` | user_sessions | auth events nếu có |
| Device | `device:*`, `user:devices:*` | user_devices | device/admin events |
| Entitlement | `entitlement:check:*`, `library:list:*` | entitlements, memberships | order/payment/refund/membership events |
| Reader | `reader:*` | reader_sessions | entitlement/membership events |
| Download | `download:token:*`, `download:quota:*` | downloads | entitlement/membership/refund events |
| Rate/idem/lock | `rate:*`, `idem:*`, `lock:*` | app logic / unique constraints backup | request-driven |

## 16. Danh mục key chuẩn đề xuất cuối cùng

## 16.1 Cache & read model
- `book:detail:v1:{book_id}`
- `catalog:list:v1:{hash}`
- `search:books:v1:{hash}`
- `library:list:v1:{user_id}:{hash}`
- `download:history:v1:{user_id}:{page}:{page_size}`

## 16.2 Session / device / reader
- `session:v1:{session_id}`
- `user:sessions:v1:{user_id}`
- `token:blacklist:v1:{jti}`
- `device:v1:{user_id}:{device_id}`
- `user:devices:v1:{user_id}`
- `reader:session:v1:{reader_session_id}`
- `reader:active:count:v1:{user_id}`
- `reader:active:sessions:v1:{user_id}`

## 16.3 Commerce / quota / anti-duplicate
- `cart:v1:{user_id}`
- `entitlement:check:v1:{user_id}:{book_id}`
- `download:token:v1:{token_id}`
- `download:token:used:v1:{token_id}`
- `download:quota:v1:user-book:{user_id}:{book_id}:{window}`
- `download:quota:v1:membership:{membership_id}:{window}`
- `rate:v1:{scope}:{subject}`
- `idem:v1:{scope}:{key}`
- `webhook:dedup:v1:{source}:{external_id}`
- `lock:v1:{resource}`
- `stock:guard:v1:{book_id}`
- `stock:pending:v1:{book_id}:{request_id}`
- `jobdedup:v1:{job_type}:{biz_key}`
- `notify:guard:v1:{template_code}:{recipient_hash}`
- `notify:batch:v1:{template_code}:{biz_key}`

## 17. Quy tắc coding bắt buộc cho team backend

- Mọi key phải được định nghĩa qua package constants/builder, không hardcode rải rác.
- Mọi TTL phải được định nghĩa bằng named constants theo domain.
- Mọi thao tác multi-key nhạy cảm phải có helper hoặc Lua script riêng.
- Mọi key mới phải khai báo owner module, data structure, TTL, invalidation trigger.
- PR thêm Redis key phải cập nhật tài liệu này.
- Phải có integration test cho idempotency, lock, quota và rate limit.

## 18. Acceptance criteria

### 18.1 Thiết kế
- Mọi use case Redis đã có key pattern, TTL, owner, data structure.
- Không có key “tạm” không TTL mà không có lý do rõ ràng.
- Không có use case nào dùng Redis như canonical source cho order/payment/refund.

### 18.2 Vận hành
- Có dashboard hit/miss, memory, evictions, contention.
- Có cleanup/reconciliation cho reader/device/session stale entries.
- Có degrade strategy khi Redis lỗi.

### 18.3 Độ đúng nghiệp vụ
- Checkout duplicate không tạo double order.
- Payment webhook duplicate không double finalize.
- Membership expiry invalidate được download/link/quyền hot cache.
- Reader concurrent limit không bị bypass do race condition phổ biến.
- Download token single-use không bị race click đôi.

## 19. Khuyến nghị triển khai tiếp theo

Sau tài liệu này, các phần nên làm tiếp là:
- package `rediskeys` và `redisttl` cho Golang;
- Lua scripts cho checkout idempotency, download consume, reader session claim;
- integration test matrix cho Redis coordination;
- operational runbook cho flushall, failover, stale counter reconciliation.

Tài liệu này đủ mức chi tiết để team backend bắt đầu implement lớp Redis theo chuẩn thống nhất cho project đang planning hiện tại mà không bị mơ hồ về key naming, TTL, ownership, invalidation và failure handling.
