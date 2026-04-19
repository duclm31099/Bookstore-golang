# Tài liệu ERD Logic và Physical Schema Specification

## 1. Mục đích tài liệu

Tài liệu này đặc tả **ERD logic** và **physical schema specification** cho hệ thống backend monolith bán sách sử dụng Golang/Gin, PostgreSQL, Redis và Kafka. Tài liệu được xây dựng để chuyển hóa trực tiếp từ URD/SRD sang thiết kế dữ liệu đủ chi tiết cho việc dựng migration, repository, query layer, transaction boundary, Redis invalidation và event publishing qua outbox. Thiết kế này phục vụ một nền tảng bán sách tại Việt Nam, hỗ trợ sách giấy, ebook PDF/EPUB, membership, thanh toán thật, COD cho đơn physical-only, fulfillment, entitlement và e-invoice export sang nhà cung cấp hóa đơn điện tử bên ngoài [web:48][web:54][web:62].

## 2. Nguyên tắc thiết kế dữ liệu

### 2.1 Nguyên tắc tổng quát

- PostgreSQL là **source of truth** cho mọi trạng thái nghiệp vụ bền vững như user, order, payment, entitlement, inventory, shipment, refund và audit.
- Redis chỉ được dùng cho cache, quota, session hot-path, idempotency, lock và state ngắn hạn; không được là nguồn sự thật duy nhất cho payment/order/refund.
- Mọi nghiệp vụ có side effect liên module phải được thiết kế sao cho tương thích với **transaction + outbox pattern** để publish Kafka event sau commit thành công [web:40][web:43].
- Schema phải ưu tiên tính **correctness**, **auditability**, **idempotency** và **khả năng mở rộng** hơn là tối ưu cực đoan sớm.
- Mọi bảng cốt lõi phải có `created_at`, `updated_at`; bảng nghiệp vụ nhạy cảm nên có `version` để hỗ trợ optimistic locking ở service layer.

### 2.2 Quy ước kỹ thuật

- Khóa chính: ưu tiên `BIGSERIAL` hoặc `UUID`, nhưng trong tài liệu này mặc định dùng `BIGINT` cho đơn giản triển khai monolith. Có thể chuyển sang UUID nếu muốn giảm lộ ID tuần tự ra public API.
- Tiền tệ: chỉ dùng VND; giá trị tiền nên dùng `NUMERIC(18,2)` hoặc integer minor unit. Với VND không có số lẻ, lựa chọn thực dụng là `BIGINT` lưu đơn vị đồng để tránh sai số.
- Trạng thái nghiệp vụ: dùng `VARCHAR` + check constraint hoặc PostgreSQL enum nếu team chấp nhận chi phí migration enum. Tài liệu này khuyến nghị `VARCHAR(50)` + check ở app/service để linh hoạt hơn.
- Dữ liệu JSON linh hoạt như payload webhook, audit metadata, provider response: dùng `JSONB`.
- Search giai đoạn đầu dùng PostgreSQL full-text search, phù hợp với lộ trình hiện tại [web:79][web:83].

### 2.3 Chuẩn hóa quan hệ

- Thiết kế ưu tiên chuẩn hóa ở mức phù hợp để tránh anomaly dữ liệu, nhưng vẫn cho phép snapshot tại các thực thể giao dịch như order, payment, invoice export để bảo toàn bối cảnh lịch sử [web:74][web:79].
- Các thông tin dễ thay đổi theo thời gian như giá, địa chỉ giao hàng, thông tin buyer trên order sẽ được **snapshot** tại thời điểm order thay vì join ngược hoàn toàn sang master data.

## 3. Phạm vi domain dữ liệu

### 3.1 Nhóm domain chính

- Identity & Access
- Catalog & Pricing
- Membership & Entitlement
- Cart & Checkout
- Order & Payment
- Refund & Chargeback
- Inventory & Shipment
- Reader & Download
- Notification
- Audit & Reporting Support
- Integration & Outbox
- E-invoice Export

### 3.2 Ranh giới dữ liệu

- Hệ thống chỉ phục vụ **một pháp nhân bán hàng** và **một quốc gia là Việt Nam**.
- Không có multi-seller, không có family account, không có native DRM engine; việc đã tải file local thì không thể thu hồi tuyệt đối chỉ bằng backend trong mô hình DRM-free [web:66].
- COD chỉ áp dụng cho physical-only order, phù hợp với rủi ro fulfillment và đặc thù thương mại điện tử tại Việt Nam [web:54][web:71].

## 4. ERD logic tổng thể

### 4.1 Danh sách thực thể logic

| Nhóm | Thực thể logic | Vai trò |
|---|---|---|
| Identity | User | Đại diện khách hàng hoặc admin |
| Identity | UserCredential | Dữ liệu xác thực |
| Identity | UserSession | Phiên đăng nhập |
| Identity | UserDevice | Thiết bị được ghi nhận |
| Identity | Address | Địa chỉ giao hàng |
| Catalog | Book | Đầu sách thương mại |
| Catalog | BookFormat | Format PDF/EPUB/physical |
| Catalog | Author | Tác giả |
| Catalog | Category | Danh mục |
| Catalog | BookAuthor | Liên kết N-N sách/tác giả |
| Catalog | BookCategory | Liên kết N-N sách/danh mục |
| Catalog | PriceRule / PriceSnapshot | Giá hiệu lực / snapshot giao dịch |
| Membership | MembershipPlan | Gói hội viên |
| Membership | MembershipSubscription | Đăng ký hội viên của user |
| Entitlement | Entitlement | Quyền đọc/tải sách số |
| Cart | Cart | Giỏ hàng |
| Cart | CartItem | Dòng hàng trong giỏ |
| Order | Order | Đơn hàng |
| Order | OrderItem | Dòng hàng đơn hàng |
| Payment | Payment | Đối tượng thanh toán chuẩn hóa |
| Payment | PaymentAttempt | Lần thử thanh toán với gateway |
| Refund | Refund | Hoàn tiền |
| Chargeback | ChargebackCase | Tranh chấp thanh toán |
| Reader | ReaderSession | Phiên đọc online |
| Reader | DownloadRecord | Lịch sử và token tải |
| Inventory | InventoryItem | Tồn kho tổng |
| Inventory | InventoryReservation | Giữ tồn |
| Shipment | Shipment | Vận đơn nội bộ |
| Shipment | ShipmentStatusLog | Lịch sử trạng thái vận chuyển |
| Notification | NotificationJob | Yêu cầu gửi email |
| Audit | AuditLog | Nhật ký audit |
| Integration | OutboxEvent | Sự kiện chờ publish Kafka |
| Integration | ProcessedEvent | Đánh dấu consumer đã xử lý event |
| Invoice | EInvoiceExport | Bản ghi export hóa đơn |

### 4.2 Quan hệ logic chính

#### Identity
- `User` 1 - N `UserSession`
- `User` 1 - N `UserDevice`
- `User` 1 - N `Address`
- `User` 1 - 1 `UserCredential`

#### Catalog
- `Book` N - N `Author` qua `BookAuthor`
- `Book` N - N `Category` qua `BookCategory`
- `Book` 1 - N `BookFormat`

#### Commerce
- `User` 1 - N `Cart`
- `Cart` 1 - N `CartItem`
- `User` 1 - N `Order`
- `Order` 1 - N `OrderItem`
- `Order` 1 - N `Payment`
- `Payment` 1 - N `PaymentAttempt`
- `Order` 1 - N `Refund`
- `Payment` 1 - N `ChargebackCase`

#### Membership & Entitlement
- `MembershipPlan` 1 - N `MembershipSubscription`
- `User` 1 - N `MembershipSubscription`
- `User` 1 - N `Entitlement`
- `Book` 1 - N `Entitlement`
- `OrderItem` 0..1 - N `Entitlement` theo nguồn cấp quyền

#### Reader
- `User` 1 - N `ReaderSession`
- `Book` 1 - N `ReaderSession`
- `User` 1 - N `DownloadRecord`
- `Book` 1 - N `DownloadRecord`

#### Fulfillment
- `Book` 1 - 1 `InventoryItem` cho SKU vật lý
- `Order` 1 - N `InventoryReservation`
- `Order` 1 - 1 `Shipment` theo rule 1 order = 1 shipment
- `Shipment` 1 - N `ShipmentStatusLog`

#### Operations
- `Order` 1 - N `EInvoiceExport`
- `OutboxEvent` độc lập nhưng tham chiếu aggregate logic qua `aggregate_type`, `aggregate_id`

## 5. ERD logic theo domain chi tiết

## 5.1 Identity & Access

### 5.1.1 User
User là thực thể gốc cho customer và admin. Không dùng bảng tách riêng cho admin; thay vào đó dùng role/permission hoặc `user_type` + RBAC ở tầng ứng dụng. Điều này đơn giản hơn cho monolith nhưng vẫn đủ khả năng mở rộng.

**Thuộc tính logic cốt lõi**
- thông tin nhận diện: email, full_name, phone
- trạng thái tài khoản: active, locked, pending_verification, disabled
- xác minh email
- role/permission scope

### 5.1.2 UserCredential
Tách password hash ra khỏi `users` để giảm rủi ro truy cập nhầm, dễ cô lập security concern và thuận lợi hơn khi audit thay đổi mật khẩu.

### 5.1.3 UserSession
Lưu refresh token chain và các metadata bảo mật như IP, user agent, device_id, revoked_at. Đây là bảng quan trọng để quản lý số phiên đăng nhập đồng thời.

### 5.1.4 UserDevice
Thiết bị được đăng ký để phục vụ giới hạn số thiết bị đọc/tải và chống lạm dụng chia sẻ tài khoản. `fingerprint_hash` phải được chuẩn hóa theo chiến lược fingerprint của backend/client.

### 5.1.5 Address
Snapshot địa chỉ giao hàng sẽ được copy sang order, nhưng bảng địa chỉ gốc vẫn cần để quản lý sổ địa chỉ người dùng.

## 5.2 Catalog & Pricing

### 5.2.1 Book
Book là aggregate trung tâm của catalog. Một `book` có thể thuộc một trong ba loại:
- physical-only
- digital-only
- hybrid

Các cờ nghiệp vụ quan trọng:
- `membership_eligible`
- `published`
- `preorder`
- `allow_individual_purchase`
- `allow_download`

### 5.2.2 BookFormat
Không nhét toàn bộ format vào bảng `books`, vì PDF/EPUB/physical có thể có metadata, asset path, trạng thái publish, và reader mode khác nhau.

### 5.2.3 Author / Category / BookAuthor / BookCategory
Thiết kế N-N chuẩn hóa, vì một sách có thể nhiều tác giả và nhiều danh mục.

### 5.2.4 Giá
Có hai hướng:
- bảng `prices` có hiệu lực theo thời gian
- snapshot giá vào `cart_items` và `order_items`

Cần cả hai: bảng giá hiện hành để tính toán hiện tại và snapshot giao dịch để bảo toàn lịch sử thương mại.

## 5.3 Membership & Entitlement

### 5.3.1 MembershipPlan
Định nghĩa gói hội viên tháng/năm, số thiết bị, số session đọc đồng thời, số lượt tải tối đa và các policy liên quan.

### 5.3.2 MembershipSubscription
Mỗi lần user mua plan tạo ra một bản ghi subscription. Bản ghi này được dùng làm nguồn của một phần quyền entitlement.

### 5.3.3 Entitlement
Entitlement là **engine nghiệp vụ trung tâm** cho sách số. Không nên suy luận quyền bằng cách join ad-hoc từ orders + memberships ở mọi request, vì sẽ rất phức tạp khi có refund, chargeback, admin grant, revoke và expiry. Entitlement là lớp quyền đã được materialize hoặc tính toán bền vững.

Nguồn của entitlement:
- ebook purchase
- membership active
- admin grant
- bundle

Trạng thái:
- active
- suspended
- expired
- revoked

## 5.4 Cart, Order, Payment

### 5.4.1 Cart / CartItem
Cart phải cho phép item thuộc nhiều loại: physical book, ebook, membership plan. `cart_items` cần snapshot giá tại thời điểm add/update để hỗ trợ UX, nhưng checkout luôn phải re-price server-side.

### 5.4.2 Order / OrderItem
Order là aggregate thương mại. OrderItem cần chứa snapshot đủ mạnh để không phụ thuộc vào catalog thay đổi về sau.

Snapshot bắt buộc trong `order_items`:
- title
- sku_type
- sku_id
- unit_price_vnd
- quantity
- discount_vnd
- final_line_total_vnd
- metadata_snapshot (JSONB)

### 5.4.3 Payment / PaymentAttempt
Tách `payments` và `payment_attempts` để xử lý nhiều lần thử gateway cho cùng một order. `payments` là canonical business object; `payment_attempts` là log vận hành/gateway.

### 5.4.4 Refund / Chargeback
Refund và chargeback là aggregate riêng vì có vòng đời độc lập, nhiều lần cập nhật trạng thái và ảnh hưởng phức tạp tới entitlement/inventory/accounting.

## 5.5 Reader, Download

### 5.5.1 ReaderSession
Bảng này phục vụ limit concurrent sessions và lưu last read heartbeat. Có thể kết hợp Redis cho hot path và PostgreSQL cho persistence/history.

### 5.5.2 DownloadRecord
Mỗi lần yêu cầu tạo link tải phải có log bền vững. Không chỉ lưu success, mà cần lưu cả requested/issued/expired/revoked/consumed để audit.

## 5.6 Inventory & Shipment

### 5.6.1 InventoryItem
Một SKU vật lý có một inventory summary record. Nếu sau này có nhiều kho, bảng này sẽ phải tách thành warehouse-level inventory, nhưng phase hiện tại giữ đơn giản.

### 5.6.2 InventoryReservation
Reservation tách riêng để xử lý race condition, release timeout và audit stock movement.

### 5.6.3 Shipment / ShipmentStatusLog
Một order có đúng một shipment. ShipmentStatusLog là bảng append-only để ghi lịch sử thay đổi trạng thái và payload từ carrier webhook/polling.

## 5.7 Audit, Notification, Integration

### 5.7.1 NotificationJob
Dùng cho email phase 1. Trạng thái riêng giúp retry qua Kafka worker và audit được gửi thành công/thất bại.

### 5.7.2 AuditLog
Append-only. Đây là bảng cực kỳ quan trọng cho admin override, refund thủ công, revoke entitlement, stock adjustment và security review.

### 5.7.3 OutboxEvent / ProcessedEvent
Cặp bảng này là nền của outbox + idempotent consumer, giúp modular monolith tích hợp Kafka một cách an toàn [web:40][web:43].

## 6. Physical schema specification chi tiết

## 6.1 Nhóm bảng Identity

### 6.1.1 Bảng `users`

**Mục đích**: lưu hồ sơ tài khoản người dùng và trạng thái account ở mức business.

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| email | VARCHAR(255) | No |  | Email đăng nhập, unique |
| full_name | VARCHAR(255) | No |  | Tên hiển thị |
| phone | VARCHAR(20) | Yes |  | Số điện thoại |
| user_type | VARCHAR(30) | No | 'customer' | customer/admin_operator |
| account_status | VARCHAR(30) | No | 'pending_verification' | pending_verification/active/locked/disabled |
| email_verified_at | TIMESTAMPTZ | Yes |  | Thời điểm xác minh email |
| last_login_at | TIMESTAMPTZ | Yes |  | Lần đăng nhập gần nhất |
| locked_reason | VARCHAR(255) | Yes |  | Lý do khóa |
| metadata | JSONB | No | '{}' | Thông tin phụ trợ |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |
| version | BIGINT | No | 1 | Optimistic locking |

**Constraint / Index**
- PK: `id`
- UNIQUE: `email`
- INDEX: `(account_status)`
- INDEX: `(user_type)`
- INDEX: `(email_verified_at)`

### 6.1.2 Bảng `user_credentials`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| user_id | BIGINT | No |  | PK + FK -> users(id) |
| password_hash | TEXT | No |  | Hash mật khẩu |
| password_algo | VARCHAR(30) | No | 'argon2id' | Ghi nhận thuật toán |
| password_changed_at | TIMESTAMPTZ | No | now() |  |
| failed_login_count | INT | No | 0 | Chống brute force |
| last_failed_login_at | TIMESTAMPTZ | Yes |  |  |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Constraint / Index**
- PK: `user_id`
- FK: `user_id` -> `users(id)` ON DELETE CASCADE

### 6.1.3 Bảng `user_sessions`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| user_id | BIGINT | No |  | FK |
| device_id | BIGINT | Yes |  | FK -> user_devices |
| refresh_token_hash | TEXT | No |  | Hash refresh token |
| session_status | VARCHAR(20) | No | 'active' | active/revoked/expired |
| ip_address | INET | Yes |  | IP |
| user_agent | TEXT | Yes |  | UA |
| last_seen_at | TIMESTAMPTZ | No | now() |  |
| expires_at | TIMESTAMPTZ | No |  |  |
| revoked_at | TIMESTAMPTZ | Yes |  |  |
| revoke_reason | VARCHAR(255) | Yes |  |  |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Index**
- INDEX `(user_id, session_status)`
- INDEX `(expires_at)`
- UNIQUE `(refresh_token_hash)`

### 6.1.4 Bảng `user_devices`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| user_id | BIGINT | No |  | FK |
| fingerprint_hash | TEXT | No |  | Fingerprint chuẩn hóa |
| device_label | VARCHAR(255) | Yes |  | Tên user-friendly |
| first_seen_at | TIMESTAMPTZ | No | now() |  |
| last_seen_at | TIMESTAMPTZ | No | now() |  |
| revoked_at | TIMESTAMPTZ | Yes |  |  |
| revoke_reason | VARCHAR(255) | Yes |  |  |
| metadata | JSONB | No | '{}' | OS/browser/device attrs |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Constraint / Index**
- INDEX `(user_id)`
- UNIQUE `(user_id, fingerprint_hash)`

### 6.1.5 Bảng `addresses`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| user_id | BIGINT | No |  | FK |
| full_name | VARCHAR(255) | No |  | Người nhận |
| phone | VARCHAR(20) | No |  |  |
| province_code | VARCHAR(20) | No |  | Mã tỉnh |
| district_code | VARCHAR(20) | No |  | Mã quận/huyện |
| ward_code | VARCHAR(20) | No |  | Mã phường/xã |
| line1 | VARCHAR(255) | No |  | Địa chỉ dòng 1 |
| line2 | VARCHAR(255) | Yes |  | Địa chỉ dòng 2 |
| postal_code | VARCHAR(20) | Yes |  | Có thể null ở VN |
| is_default | BOOLEAN | No | false |  |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Index**
- INDEX `(user_id)`
- INDEX `(user_id, is_default)`

## 6.2 Nhóm bảng Catalog & Pricing

### 6.2.1 Bảng `books`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| slug | VARCHAR(255) | No |  | URL slug, unique |
| title | VARCHAR(500) | No |  | Tên sách |
| subtitle | VARCHAR(500) | Yes |  | Phụ đề |
| short_description | TEXT | Yes |  | Mô tả ngắn |
| long_description | TEXT | Yes |  | Mô tả dài |
| product_type | VARCHAR(20) | No |  | physical/digital/hybrid |
| isbn | VARCHAR(50) | Yes |  |  |
| publisher_name | VARCHAR(255) | Yes |  |  |
| cover_image_url | TEXT | Yes |  |  |
| page_count | INT | Yes |  |  |
| language_code | VARCHAR(10) | No | 'vi' |  |
| membership_eligible | BOOLEAN | No | false | Thuộc chương trình hội viên |
| allow_individual_purchase | BOOLEAN | No | true |  |
| published | BOOLEAN | No | false |  |
| preorder | BOOLEAN | No | false |  |
| publish_at | TIMESTAMPTZ | Yes |  |  |
| tags | JSONB | No | '[]' |  |
| metadata | JSONB | No | '{}' | Mở rộng |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |
| version | BIGINT | No | 1 |  |

**Index / Constraint**
- UNIQUE `(slug)`
- INDEX `(published, membership_eligible)`
- INDEX `(product_type)`
- INDEX `(publish_at)`

### 6.2.2 Bảng `book_formats`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| book_id | BIGINT | No |  | FK |
| format_type | VARCHAR(20) | No |  | pdf/epub/physical |
| asset_path | TEXT | Yes |  | Với pdf/epub |
| file_size_bytes | BIGINT | Yes |  |  |
| mime_type | VARCHAR(100) | Yes |  |  |
| reader_mode | VARCHAR(30) | Yes |  | pdf_render/epub_reader/none |
| downloadable | BOOLEAN | No | false | Có cho tải hay không |
| online_readable | BOOLEAN | No | false | Có cho đọc online không |
| active | BOOLEAN | No | true |  |
| metadata | JSONB | No | '{}' |  |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Constraint / Index**
- FK `book_id` -> `books(id)` ON DELETE CASCADE
- UNIQUE `(book_id, format_type)`
- INDEX `(book_id, active)`

### 6.2.3 Bảng `authors`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| slug | VARCHAR(255) | No |  | Unique |
| name | VARCHAR(255) | No |  |  |
| bio | TEXT | Yes |  |  |
| avatar_url | TEXT | Yes |  |  |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Constraint**
- UNIQUE `(slug)`
- INDEX `(name)`

### 6.2.4 Bảng `categories`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| slug | VARCHAR(255) | No |  | Unique |
| name | VARCHAR(255) | No |  |  |
| parent_id | BIGINT | Yes |  | Self FK nếu cần cây danh mục |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Constraint / Index**
- UNIQUE `(slug)`
- INDEX `(parent_id)`

### 6.2.5 Bảng `book_authors`

| Cột | Kiểu dữ liệu |
|---|---|
| book_id | BIGINT |
| author_id | BIGINT |
| display_order | INT |

**PK**: `(book_id, author_id)`

### 6.2.6 Bảng `book_categories`

| Cột | Kiểu dữ liệu |
|---|---|
| book_id | BIGINT |
| category_id | BIGINT |

**PK**: `(book_id, category_id)`

### 6.2.7 Bảng `prices`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| subject_type | VARCHAR(30) | No |  | book/membership_plan |
| subject_id | BIGINT | No |  | ID đối tượng |
| list_price_vnd | BIGINT | No |  | Giá niêm yết |
| sale_price_vnd | BIGINT | No |  | Giá bán |
| starts_at | TIMESTAMPTZ | No |  |  |
| ends_at | TIMESTAMPTZ | Yes |  |  |
| active | BOOLEAN | No | true |  |
| metadata | JSONB | No | '{}' | Campaign info |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Index**
- INDEX `(subject_type, subject_id, active)`
- INDEX `(starts_at, ends_at)`

### 6.2.8 Bảng `search_documents` (tùy chọn)

| Cột | Kiểu dữ liệu |
|---|---|
| subject_type | VARCHAR(30) |
| subject_id | BIGINT |
| document | TEXT |
| tsv | TSVECTOR |
| updated_at | TIMESTAMPTZ |

**PK**: `(subject_type, subject_id)`

**Index**
- GIN `(tsv)`

## 6.3 Nhóm bảng Membership & Entitlement

### 6.3.1 Bảng `membership_plans`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| code | VARCHAR(100) | No |  | Unique |
| name | VARCHAR(255) | No |  |  |
| duration_days | INT | No |  | 30/365... |
| price_vnd | BIGINT | No |  |  |
| list_price_vnd | BIGINT | Yes |  |  |
| max_devices | INT | No |  |  |
| max_concurrent_reader_sessions | INT | No |  |  |
| max_downloads_total | INT | Yes |  | Nếu policy global |
| renewable | BOOLEAN | No | true |  |
| stackable | BOOLEAN | No | true |  |
| active | BOOLEAN | No | true |  |
| metadata | JSONB | No | '{}' | Policy mở rộng |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |

**Constraint**
- UNIQUE `(code)`
- INDEX `(active)`

### 6.3.2 Bảng `memberships`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| user_id | BIGINT | No |  | FK |
| plan_id | BIGINT | No |  | FK |
| source_order_id | BIGINT | Yes |  | Order cấp membership |
| state | VARCHAR(30) | No | 'pending_activation' |  |
| starts_at | TIMESTAMPTZ | No |  |  |
| expires_at | TIMESTAMPTZ | No |  |  |
| revoked_at | TIMESTAMPTZ | Yes |  |  |
| revoke_reason | VARCHAR(255) | Yes |  |  |
| metadata | JSONB | No | '{}' |  |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |
| version | BIGINT | No | 1 |  |

**Index**
- INDEX `(user_id, state)`
- INDEX `(user_id, expires_at)`
- INDEX `(plan_id)`
- INDEX `(source_order_id)`

### 6.3.3 Bảng `entitlements`

| Cột | Kiểu dữ liệu | Null | Default | Mô tả |
|---|---|---|---|---|
| id | BIGSERIAL | No |  | PK |
| user_id | BIGINT | No |  | FK |
| book_id | BIGINT | No |  | FK |
| source_type | VARCHAR(30) | No |  | ebook_purchase/membership/admin_grant/bundle |
| source_id | BIGINT | Yes |  | ID nguồn |
| state | VARCHAR(20) | No | 'active' | active/suspended/expired/revoked |
| allow_read_online | BOOLEAN | No | true |  |
| allow_download | BOOLEAN | No | true |  |
| starts_at | TIMESTAMPTZ | No |  |  |
| expires_at | TIMESTAMPTZ | Yes |  | Null với quyền vĩnh viễn |
| revoked_at | TIMESTAMPTZ | Yes |  |  |
| revoke_reason | VARCHAR(255) | Yes |  |  |
| metadata | JSONB | No | '{}' | quota snapshot, plan snapshot |
| created_at | TIMESTAMPTZ | No | now() |  |
| updated_at | TIMESTAMPTZ | No | now() |  |
| version | BIGINT | No | 1 |  |

**Index / Constraint**
- INDEX `(user_id, book_id, state)`
- INDEX `(user_id, expires_at)`
- INDEX `(source_type, source_id)`
- INDEX `(book_id)`

> Ghi chú: không khuyến nghị UNIQUE `(user_id, book_id, source_type, source_id)` cứng nếu business muốn cho nhiều record lịch sử. Có thể dùng `active` uniqueness ở tầng service hoặc partial index sau.

## 6.4 Nhóm bảng Cart & Order

### 6.4.1 Bảng `carts`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| user_id | BIGINT |
| state | VARCHAR(20) |
| coupon_code | VARCHAR(100) |
| currency | VARCHAR(10) |
| expires_at | TIMESTAMPTZ |
| pricing_snapshot | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(user_id, state)`

### 6.4.2 Bảng `cart_items`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| cart_id | BIGINT |
| sku_type | VARCHAR(30) |
| sku_id | BIGINT |
| quantity | INT |
| unit_price_vnd | BIGINT |
| list_price_vnd | BIGINT |
| discount_vnd | BIGINT |
| final_unit_price_vnd | BIGINT |
| metadata_snapshot | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(cart_id)`
- INDEX `(cart_id, sku_type, sku_id)`

### 6.4.3 Bảng `orders`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| user_id | BIGINT |
| order_no | VARCHAR(50) |
| order_state | VARCHAR(30) |
| order_channel | VARCHAR(30) |
| payment_method | VARCHAR(30) |
| currency | VARCHAR(10) |
| subtotal_vnd | BIGINT |
| shipping_fee_vnd | BIGINT |
| discount_vnd | BIGINT |
| tax_vnd | BIGINT |
| total_vnd | BIGINT |
| buyer_snapshot | JSONB |
| shipping_address_snapshot | JSONB |
| pricing_snapshot | JSONB |
| notes | TEXT |
| placed_at | TIMESTAMPTZ |
| cancelled_at | TIMESTAMPTZ |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |
| version | BIGINT |

**Constraint / Index**
- UNIQUE `(order_no)`
- INDEX `(user_id, order_state)`
- INDEX `(placed_at)`
- INDEX `(payment_method, order_state)`

### 6.4.4 Bảng `order_items`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| sku_type | VARCHAR(30) |
| sku_id | BIGINT |
| item_state | VARCHAR(30) |
| item_title_snapshot | VARCHAR(500) |
| quantity | INT |
| unit_price_vnd | BIGINT |
| discount_vnd | BIGINT |
| final_line_total_vnd | BIGINT |
| metadata_snapshot | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(order_id)`
- INDEX `(sku_type, sku_id)`
- INDEX `(item_state)`

## 6.5 Nhóm bảng Payment, Refund, Chargeback

### 6.5.1 Bảng `payments`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| provider | VARCHAR(30) |
| payment_state | VARCHAR(30) |
| amount_vnd | BIGINT |
| external_payment_ref | VARCHAR(255) |
| provider_customer_ref | VARCHAR(255) |
| latest_error_code | VARCHAR(100) |
| latest_error_message | TEXT |
| captured_at | TIMESTAMPTZ |
| expired_at | TIMESTAMPTZ |
| metadata | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |
| version | BIGINT |

**Index**
- INDEX `(order_id)`
- INDEX `(provider, external_payment_ref)`
- INDEX `(payment_state)`

### 6.5.2 Bảng `payment_attempts`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| payment_id | BIGINT |
| attempt_no | INT |
| attempt_state | VARCHAR(30) |
| request_payload | JSONB |
| response_payload | JSONB |
| provider_request_id | VARCHAR(255) |
| redirect_url | TEXT |
| webhook_verified | BOOLEAN |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Constraint**
- UNIQUE `(payment_id, attempt_no)`
- INDEX `(provider_request_id)`

### 6.5.3 Bảng `refunds`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| payment_id | BIGINT |
| refund_state | VARCHAR(30) |
| amount_vnd | BIGINT |
| reason_code | VARCHAR(100) |
| reason_note | TEXT |
| requested_by_type | VARCHAR(30) |
| requested_by_id | BIGINT |
| external_refund_ref | VARCHAR(255) |
| requested_at | TIMESTAMPTZ |
| completed_at | TIMESTAMPTZ |
| metadata | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(order_id)`
- INDEX `(payment_id)`
- INDEX `(refund_state)`

### 6.5.4 Bảng `chargebacks`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| payment_id | BIGINT |
| chargeback_state | VARCHAR(30) |
| amount_vnd | BIGINT |
| provider_case_ref | VARCHAR(255) |
| reason_code | VARCHAR(100) |
| evidence_payload | JSONB |
| opened_at | TIMESTAMPTZ |
| resolved_at | TIMESTAMPTZ |
| resolution | VARCHAR(30) |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(payment_id)`
- INDEX `(chargeback_state)`

## 6.6 Nhóm bảng Reader & Download

### 6.6.1 Bảng `reader_sessions`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| user_id | BIGINT |
| book_id | BIGINT |
| device_id | BIGINT |
| format_type | VARCHAR(20) |
| session_state | VARCHAR(20) |
| started_at | TIMESTAMPTZ |
| last_heartbeat_at | TIMESTAMPTZ |
| ended_at | TIMESTAMPTZ |
| last_position | JSONB |
| metadata | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(user_id, session_state)`
- INDEX `(user_id, book_id)`
- INDEX `(last_heartbeat_at)`

### 6.6.2 Bảng `downloads`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| user_id | BIGINT |
| book_id | BIGINT |
| format_type | VARCHAR(20) |
| device_id | BIGINT |
| source_type | VARCHAR(30) |
| source_id | BIGINT |
| token_id | VARCHAR(255) |
| download_state | VARCHAR(30) |
| requested_at | TIMESTAMPTZ |
| link_expires_at | TIMESTAMPTZ |
| consumed_at | TIMESTAMPTZ |
| revoked_at | TIMESTAMPTZ |
| revoke_reason | VARCHAR(255) |
| metadata | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Constraint / Index**
- UNIQUE `(token_id)`
- INDEX `(user_id, book_id)`
- INDEX `(user_id, requested_at)`
- INDEX `(download_state)`

## 6.7 Nhóm bảng Inventory & Shipment

### 6.7.1 Bảng `inventory_items`

| Cột | Kiểu dữ liệu |
|---|---|
| book_id | BIGINT |
| on_hand | INT |
| reserved | INT |
| available | INT |
| reorder_threshold | INT |
| metadata | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |
| version | BIGINT |

**PK**: `book_id`

### 6.7.2 Bảng `inventory_reservations`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| book_id | BIGINT |
| qty | INT |
| reservation_state | VARCHAR(20) |
| reserved_at | TIMESTAMPTZ |
| expires_at | TIMESTAMPTZ |
| released_at | TIMESTAMPTZ |
| metadata | JSONB |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(order_id)`
- INDEX `(book_id)`
- INDEX `(reservation_state, expires_at)`

### 6.7.3 Bảng `shipments`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| carrier_code | VARCHAR(50) |
| carrier_name | VARCHAR(255) |
| tracking_no | VARCHAR(255) |
| shipment_state | VARCHAR(30) |
| shipping_fee_vnd | BIGINT |
| cod_amount_vnd | BIGINT |
| payload_snapshot | JSONB |
| shipped_at | TIMESTAMPTZ |
| delivered_at | TIMESTAMPTZ |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Constraint / Index**
- UNIQUE `(order_id)`
- INDEX `(tracking_no)`
- INDEX `(shipment_state)`

### 6.7.4 Bảng `shipment_status_logs`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| shipment_id | BIGINT |
| shipment_state | VARCHAR(30) |
| source | VARCHAR(30) |
| raw_payload | JSONB |
| note | TEXT |
| created_at | TIMESTAMPTZ |

**Index**
- INDEX `(shipment_id, created_at)`

## 6.8 Nhóm bảng Notification, Audit, Integration

### 6.8.1 Bảng `notifications`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| channel | VARCHAR(20) |
| template_code | VARCHAR(100) |
| recipient | VARCHAR(255) |
| subject_snapshot | VARCHAR(500) |
| payload | JSONB |
| notification_state | VARCHAR(30) |
| attempt_count | INT |
| next_retry_at | TIMESTAMPTZ |
| sent_at | TIMESTAMPTZ |
| last_error | TEXT |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(notification_state, next_retry_at)`
- INDEX `(recipient)`

### 6.8.2 Bảng `audit_logs`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| actor_type | VARCHAR(30) |
| actor_id | BIGINT |
| action | VARCHAR(100) |
| resource_type | VARCHAR(50) |
| resource_id | BIGINT |
| before_data | JSONB |
| after_data | JSONB |
| metadata | JSONB |
| ip_address | INET |
| created_at | TIMESTAMPTZ |

**Index**
- INDEX `(actor_type, actor_id)`
- INDEX `(resource_type, resource_id)`
- INDEX `(action)`
- INDEX `(created_at)`

### 6.8.3 Bảng `outbox_events`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| event_id | UUID |
| topic | VARCHAR(255) |
| event_key | VARCHAR(255) |
| aggregate_type | VARCHAR(50) |
| aggregate_id | BIGINT |
| event_type | VARCHAR(100) |
| payload | JSONB |
| headers | JSONB |
| publish_state | VARCHAR(20) |
| created_at | TIMESTAMPTZ |
| published_at | TIMESTAMPTZ |
| retry_count | INT |
| last_error | TEXT |

**Constraint / Index**
- UNIQUE `(event_id)`
- INDEX `(publish_state, created_at)`
- INDEX `(topic, publish_state)`

### 6.8.4 Bảng `processed_events`

| Cột | Kiểu dữ liệu |
|---|---|
| consumer_name | VARCHAR(100) |
| event_id | UUID |
| processed_at | TIMESTAMPTZ |
| metadata | JSONB |

**PK**: `(consumer_name, event_id)`

## 6.9 Nhóm bảng E-invoice export

### 6.9.1 Bảng `e_invoice_exports`

| Cột | Kiểu dữ liệu |
|---|---|
| id | BIGSERIAL |
| order_id | BIGINT |
| export_state | VARCHAR(30) |
| buyer_payload | JSONB |
| invoice_payload | JSONB |
| provider_name | VARCHAR(100) |
| provider_ref | VARCHAR(255) |
| last_error | TEXT |
| requested_at | TIMESTAMPTZ |
| exported_at | TIMESTAMPTZ |
| created_at | TIMESTAMPTZ |
| updated_at | TIMESTAMPTZ |

**Index**
- INDEX `(order_id)`
- INDEX `(export_state, requested_at)`

## 7. Ràng buộc khóa ngoại đề xuất

| Bảng con | Cột FK | Bảng cha | Chính sách xóa |
|---|---|---|---|
| user_credentials | user_id | users | CASCADE |
| user_sessions | user_id | users | RESTRICT/CASCADE theo policy |
| user_devices | user_id | users | RESTRICT/CASCADE theo policy |
| addresses | user_id | users | CASCADE |
| book_formats | book_id | books | CASCADE |
| book_authors | book_id | books | CASCADE |
| book_authors | author_id | authors | CASCADE |
| book_categories | book_id | books | CASCADE |
| book_categories | category_id | categories | CASCADE |
| memberships | user_id | users | RESTRICT |
| memberships | plan_id | membership_plans | RESTRICT |
| entitlements | user_id | users | RESTRICT |
| entitlements | book_id | books | RESTRICT |
| carts | user_id | users | CASCADE |
| cart_items | cart_id | carts | CASCADE |
| orders | user_id | users | RESTRICT |
| order_items | order_id | orders | CASCADE |
| payments | order_id | orders | RESTRICT |
| payment_attempts | payment_id | payments | CASCADE |
| refunds | order_id | orders | RESTRICT |
| refunds | payment_id | payments | RESTRICT |
| chargebacks | order_id | orders | RESTRICT |
| chargebacks | payment_id | payments | RESTRICT |
| reader_sessions | user_id | users | RESTRICT |
| reader_sessions | book_id | books | RESTRICT |
| downloads | user_id | users | RESTRICT |
| downloads | book_id | books | RESTRICT |
| inventory_items | book_id | books | RESTRICT |
| inventory_reservations | order_id | orders | RESTRICT |
| inventory_reservations | book_id | books | RESTRICT |
| shipments | order_id | orders | RESTRICT |
| shipment_status_logs | shipment_id | shipments | CASCADE |
| e_invoice_exports | order_id | orders | RESTRICT |

## 8. Chiến lược index và truy vấn nóng

### 8.1 Truy vấn nóng dự kiến

- Liệt kê catalog theo filter published, category, membership eligible, product type.
- Search full-text theo title/author/category.
- Lấy `my library` theo user + entitlement active.
- Lấy session/device active theo user.
- Lấy order history theo user và admin filter theo state/date/payment method.
- Lấy shipment theo order hoặc tracking number.
- Reconciliation payment theo `provider + external_ref`.
- Worker scan outbox chưa publish.
- Worker scan notifications retry đến hạn.

### 8.2 Index bắt buộc

- `users(email)` unique
- `books(slug)` unique
- `search_documents(tsv)` GIN
- `orders(order_no)` unique
- `orders(user_id, order_state)`
- `payments(provider, external_payment_ref)`
- `memberships(user_id, state, expires_at)`
- `entitlements(user_id, book_id, state)`
- `downloads(user_id, requested_at desc)`
- `inventory_reservations(reservation_state, expires_at)`
- `outbox_events(publish_state, created_at)`
- `notifications(notification_state, next_retry_at)`

### 8.3 Index cần cân nhắc sau khi benchmark

- Partial index cho `orders where order_state in (...)`
- Partial index cho `entitlements where state='active'`
- Partial index cho `reader_sessions where session_state='active'`
- BRIN index cho bảng append-only rất lớn như `audit_logs`, `shipment_status_logs`

## 9. Quy tắc transaction và consistency boundary

### 9.1 Transaction bắt buộc mạnh

- Tạo order + order_items + payment record ban đầu
- Capture payment thành công + transition order + grant entitlement hoặc create downstream reliable action
- Reserve inventory / release inventory
- Tạo refund record + update financial state ban đầu
- Admin override nhạy cảm có audit log cùng transaction nếu có thể

### 9.2 Eventually consistent qua Kafka

- Gửi email
- Rebuild cache/search projection
- Reporting aggregate
- Payment reconciliation follow-up
- Membership expiry reminders
- Shipment polling
- Invoice export

### 9.3 Chiến lược chống race condition

- Inventory reserve: dùng `SELECT ... FOR UPDATE` hoặc optimistic locking trên `inventory_items.version`
- Download quota / session limit hot path: Redis atomic ops + PostgreSQL audit trail
- Payment callback duplicate: idempotency qua `provider_request_id`, `external_payment_ref`, `event_id`
- Outbox publish duplicate: dedup ở consumer qua `processed_events`

## 10. Mapping dữ liệu sang Redis và Kafka

### 10.1 Thực thể cần cache mạnh ở Redis

- `books`, `book_formats`, search result
- hot cart
- session active
- device active
- quota counters
- idempotency keys

### 10.2 Thực thể không cache làm source of truth

- `orders`
- `payments`
- `refunds`
- `chargebacks`
- `inventory_reservations`
- `entitlements` canonical state

### 10.3 Thực thể phải phát event qua outbox

- orders
- payments
- refunds
- memberships
- entitlements
- inventory reservations
- shipments
- notifications command
- invoice export request

## 11. Khuyến nghị triển khai migration

### 11.1 Thứ tự tạo bảng

1. users
2. user_credentials / user_devices / user_sessions / addresses
3. authors / categories / books / book_formats / joins / prices / search_documents
4. membership_plans / memberships / entitlements
5. carts / cart_items
6. orders / order_items
7. payments / payment_attempts / refunds / chargebacks
8. reader_sessions / downloads
9. inventory_items / inventory_reservations
10. shipments / shipment_status_logs
11. notifications / audit_logs / outbox_events / processed_events / e_invoice_exports

### 11.2 Seed data nên có

- admin operator mặc định cho môi trường local/dev
- membership plan monthly/yearly
- category seed
- author seed demo
- sample books physical/digital/hybrid
- trạng thái chuẩn cho test data

## 12. Những quyết định thiết kế quan trọng

### 12.1 Vì sao cần bảng `entitlements`

Nếu không có bảng entitlement, mọi request đọc/tải sẽ phải suy diễn từ orders, order_items, memberships, refunds, chargebacks và admin overrides. Điều này làm query phức tạp, khó tối ưu và dễ sai nghiệp vụ. Bảng entitlement là lớp canonical business right để backend trả lời nhanh và đúng cho câu hỏi: “user này hiện tại có quyền đọc/tải cuốn sách này hay không?”.

### 12.2 Vì sao tách `payments` và `payment_attempts`

Một order có thể bị retry payment nhiều lần hoặc nhiều callback từ gateway. Nếu dồn tất cả vào một bảng sẽ khó giữ canonical state và khó audit. Tách hai bảng giúp payment domain rõ ràng hơn.

### 12.3 Vì sao `downloads` phải lưu cả token lifecycle

Vì hệ thống cần giới hạn số lượt tải, vô hiệu hóa link sau expiry membership, audit abuse, và điều tra chargeback/fraud. Chỉ lưu log thành công là không đủ.

### 12.4 Vì sao `outbox_events` là bắt buộc

Vì hệ thống đã xác định Kafka là core requirement. Publish thẳng sau khi commit hoặc trong transaction network call đều dễ gây mất đồng bộ. Outbox là cách đáng tin cậy hơn trong modular monolith [web:40][web:43].

## 13. Danh sách bảng tổng hợp cuối cùng

| Nhóm | Bảng |
|---|---|
| Identity | users, user_credentials, user_sessions, user_devices, addresses |
| Catalog | books, book_formats, authors, categories, book_authors, book_categories, prices, search_documents |
| Membership | membership_plans, memberships, entitlements |
| Cart | carts, cart_items |
| Order & Payment | orders, order_items, payments, payment_attempts, refunds, chargebacks, coupons, coupon_redemptions |
| Reader | reader_sessions, downloads |
| Inventory & Shipment | inventory_items, inventory_reservations, shipments, shipment_status_logs |
| Ops & Integration | notifications, audit_logs, outbox_events, processed_events, e_invoice_exports |

## 14. Hạng mục nên làm tiếp sau tài liệu này

Sau khi chốt tài liệu ERD logic và physical schema này, các deliverable tiếp theo nên là:
- data dictionary mở rộng theo từng cột với rule validate chi tiết
- migration plan SQL theo thứ tự
- OpenAPI spec cho public/admin domains
- state transition spec chi tiết cho order/payment/membership/shipment
- Kafka event contract chi tiết
- Redis key & TTL spec chi tiết

Tài liệu này là nền đủ mạnh để bắt đầu dựng migration, repository interfaces, query strategy và package boundary cho monolith backend hiện tại [web:74][web:79][web:81].
