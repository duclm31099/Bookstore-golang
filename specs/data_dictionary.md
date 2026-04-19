# Tài liệu Data Dictionary Chi Tiết Toàn Bộ Schema

## 1. Mục đích tài liệu

Tài liệu này là **Data Dictionary** chi tiết cho toàn bộ schema PostgreSQL của dự án backend bookstore đang planning hiện tại. Mục tiêu của tài liệu là chuẩn hóa cách team hiểu, sử dụng, mở rộng và kiểm soát dữ liệu ở cấp bảng, cột, kiểu dữ liệu, ràng buộc, ý nghĩa nghiệp vụ, ownership, sensitivity, lifecycle và mối quan hệ giữa các thành phần dữ liệu.

Tài liệu này không chỉ là danh sách table/column. Theo tư duy của **master backend developer**, một data dictionary tốt phải trả lời được các câu hỏi sau:
- bảng này là canonical hay derived;
- module nào sở hữu bảng này;
- cột này có ý nghĩa nghiệp vụ gì;
- cột nào tham gia invariant quan trọng;
- cột nào là PII hoặc dữ liệu nhạy cảm;
- cột nào được dùng cho hot query hoặc reconciliation;
- enum này có state transition ràng buộc ra sao;
- cột nào không được update trực tiếp mà phải đi qua state machine;
- dữ liệu nào phải audit, dữ liệu nào chỉ là snapshot, dữ liệu nào có thể tái tạo.

Tài liệu này phải được dùng cùng với:
- SRD/URD;
- ERD logic và physical schema;
- Redis key & TTL spec;
- Kafka event contract;
- State Transition Specification;
- ADR pack;
- Golang Module Boundary & Implementation Blueprint;
- OpenAPI Specs.

## 2. Nguyên tắc thiết kế Data Dictionary

### 2.1 Chuẩn cấu trúc entry

Mỗi bảng trong tài liệu này được mô tả theo các trường chuẩn:
- tên bảng;
- module owner;
- mục đích;
- phân loại dữ liệu: canonical / derived / integration / audit / operational;
- mô tả các cột;
- primary key;
- foreign keys;
- unique constraints;
- indexes quan trọng;
- quy tắc update;
- retention / archival note;
- sensitivity note.

### 2.2 Quy ước naming

Theo thông lệ PostgreSQL và để giảm rủi ro quote identifier, toàn bộ object name nên dùng **lowercase + snake_case** thống nhất [web:147][web:150][web:153]. Tên cột phải rõ nghĩa, tránh từ viết tắt mơ hồ, và các object/column naming conventions phải đồng nhất để dễ đọc, dễ review schema và dễ maintain lâu dài [web:147][web:149][web:150].

### 2.3 Tư duy master backend áp dụng

Master backend developer không nhìn database như chỗ “lưu dữ liệu cho đủ”. Database là lớp mô hình hóa **business truth**. Vì vậy data dictionary phải gắn field với:
- business meaning;
- mutation authority;
- auditability;
- operational implications.

Nhiều team chỉ ghi kiểu dữ liệu và nullable/not-null. Điều đó chưa đủ. Một data dictionary trưởng thành phải gắn được **semantics**, **ownership** và **change discipline** cho từng field [web:142][web:148][web:152].

## 3. Quy ước chung toàn schema

### 3.1 Quy ước khóa chính

- Khóa chính nội bộ dùng `bigint` hoặc `uuid` tùy object; trong blueprint hiện tại ưu tiên numeric internal ID cho bảng nghiệp vụ chính.
- Tên cột PK thống nhất là `id` hoặc `{entity}_id` trong tài liệu business-facing; khi implement vật lý cần chọn một convention duy nhất và bám chặt.
- Trong tài liệu này, để rõ nghĩa nghiệp vụ, phần mô tả sẽ dùng tên field business như `order_id`, `payment_id`, `membership_id`; còn implementation vật lý có thể chuẩn hóa về `id` + alias trong query layer nếu team chọn.

### 3.2 Quy ước audit columns

Hầu hết bảng canonical nên có:
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Một số bảng immutable hoặc append-only có thể không cần `updated_at`.

### 3.3 Quy ước soft delete

- Không áp dụng soft delete đại trà.
- Bảng canonical quan trọng ưu tiên state-based lifecycle hơn là `deleted_at` tùy tiện.
- Chỉ dùng soft delete nếu thực sự cần về business hoặc compliance.

### 3.4 Quy ước tiền tệ

- Tất cả giá trị tiền dùng integer theo đơn vị **VND**.
- Không dùng float/double cho amount.
- Field tiền có hậu tố `_vnd` để không mơ hồ.

### 3.5 Quy ước thời gian

- Dùng `timestamptz` cho mọi cột thời gian nghiệp vụ và audit.
- UTC trong storage, convert timezone ở lớp presentation nếu cần.

### 3.6 Quy ước trạng thái

Các field state quan trọng:
- `order_state`
- `payment_state`
- `refund_state`
- `membership_state`
- `entitlement_state`
- `shipment_state`
- `notification_state`

Các cột state này **không được update ad-hoc**. Phải đi qua state machine / application use case tương ứng.

## 4. Phân loại bảng toàn hệ thống

| Nhóm | Mục đích | Ví dụ |
|---|---|---|
| Identity/Auth | user, session, device, permission | `users`, `user_sessions`, `admin_users` |
| Catalog | books, categories, formats, pricing | `books`, `book_formats`, `book_prices` |
| Commerce | cart, order, payment, refund | `carts`, `orders`, `payments`, `refunds` |
| Access Rights | membership, entitlement, reader, download | `memberships`, `entitlements`, `reader_sessions`, `downloads` |
| Fulfillment | inventory, reservation, shipment | `inventory_items`, `inventory_reservations`, `shipments` |
| Integration | invoice export, provider refs | `e_invoice_exports`, `payment_callbacks` |
| Eventing/Operational | outbox, processed events, jobs | `outbox_events`, `processed_events`, `job_runs` |
| Audit/Support | audit logs, notification logs | `audit_logs`, `notifications` |

## 5. Identity & Access domain

## 5.1 `users`

### Mục đích
- lưu thực thể người dùng cuối của hệ thống.

### Module owner
- `auth`

### Phân loại
- canonical

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả | Ghi chú |
|---|---|---|---|---|
| `user_id` | bigint | không | định danh nội bộ của user | PK |
| `email` | varchar(255) | không | email đăng nhập duy nhất | unique, PII |
| `email_normalized` | varchar(255) | không | email chuẩn hóa lowercase/trim | unique để lookup nhanh |
| `password_hash` | varchar(255) | không | hash mật khẩu | nhạy cảm, không log |
| `full_name` | varchar(255) | không | họ tên hiển thị | PII |
| `phone_number` | varchar(32) | có | số điện thoại | PII |
| `phone_verified` | boolean | không | đã xác minh số điện thoại hay chưa | default false |
| `email_verified` | boolean | không | đã xác minh email hay chưa | default false |
| `account_state` | varchar(32) | không | trạng thái tài khoản | `active`, `suspended`, `blocked` |
| `last_login_at` | timestamptz | có | lần đăng nhập cuối | |
| `created_at` | timestamptz | không | thời điểm tạo | |
| `updated_at` | timestamptz | không | thời điểm cập nhật cuối | |

### Constraints và indexes
- PK: `user_id`
- UNIQUE: `email_normalized`
- INDEX: `account_state`, `created_at`

### Quy tắc update
- `password_hash` chỉ update qua use case đổi/reset mật khẩu.
- `account_state` là field nhạy cảm, không update trực tiếp từ CRUD admin cơ bản.

### Sensitivity
- PII: `email`, `full_name`, `phone_number`
- Sensitive secret-ish: `password_hash`

### Tư duy master backend

User table phải gọn, giữ identity core. Đừng nhồi mọi profile/cài đặt/tùy chọn vào cùng bảng, vì identity là vùng được đụng tới thường xuyên và có nhiều policy bảo mật riêng.

## 5.2 `user_sessions`

### Mục đích
- lưu session đăng nhập canonical.

### Module owner
- `auth`

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả | Ghi chú |
|---|---|---|---|---|
| `session_id` | uuid | không | định danh session | PK |
| `user_id` | bigint | không | FK tới `users` | |
| `device_id` | bigint | có | FK tới `user_devices` nếu đã đăng ký | |
| `refresh_token_hash` | varchar(255) | không | hash refresh token | không lưu plaintext |
| `session_state` | varchar(32) | không | `active`, `revoked`, `expired` | canonical |
| `issued_at` | timestamptz | không | thời điểm tạo session | |
| `expires_at` | timestamptz | không | thời điểm session hết hạn | |
| `last_seen_at` | timestamptz | có | lần hoạt động cuối | |
| `ip_address` | inet | có | IP tạo session | nhạy cảm vận hành |
| `user_agent` | text | có | user agent tại login | |
| `revoked_at` | timestamptz | có | thời điểm revoke | |
| `created_at` | timestamptz | không | audit create | |
| `updated_at` | timestamptz | không | audit update | |

### Indexes
- `idx_user_sessions_user_id_session_state`
- `idx_user_sessions_expires_at`

### Quy tắc update
- `refresh_token_hash` không rotate tùy tiện ngoài flow refresh.
- `session_state` chỉ đi qua auth state policy.

## 5.3 `user_devices`

### Mục đích
- lưu registry thiết bị của user phục vụ login/device policy, reader/download constraints.

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `device_id` | bigint | không | PK |
| `user_id` | bigint | không | owner user |
| `device_fingerprint_hash` | varchar(128) | không | hash fingerprint |
| `device_label` | varchar(255) | có | tên thiết bị hiển thị |
| `platform` | varchar(64) | có | web, ios, android, desktop |
| `last_seen_at` | timestamptz | có | lần thấy gần nhất |
| `device_state` | varchar(32) | không | `active`, `revoked` |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Constraints
- unique theo `user_id + device_fingerprint_hash`

## 5.4 `admin_users`

### Mục đích
- tài khoản vận hành/back-office.

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `admin_user_id` | bigint | không | PK |
| `email` | varchar(255) | không | email đăng nhập admin |
| `password_hash` | varchar(255) | không | hash password |
| `full_name` | varchar(255) | không | họ tên |
| `admin_state` | varchar(32) | không | `active`, `suspended` |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 5.5 `admin_roles`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `admin_role_id` | bigint | không | PK |
| `role_code` | varchar(64) | không | mã role |
| `role_name` | varchar(255) | không | tên role |
| `description` | text | có | mô tả |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 5.6 `admin_role_permissions`

### Mục đích
- ánh xạ role -> permission codes.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `admin_role_permission_id` | bigint | không | PK |
| `admin_role_id` | bigint | không | FK role |
| `permission_code` | varchar(128) | không | ví dụ `refund.approve` |
| `created_at` | timestamptz | không | |

## 5.7 `admin_user_roles`

### Mục đích
- ánh xạ user admin -> roles.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `admin_user_role_id` | bigint | không | PK |
| `admin_user_id` | bigint | không | FK admin |
| `admin_role_id` | bigint | không | FK role |
| `created_at` | timestamptz | không | |

## 6. Catalog domain

## 6.1 `authors`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `author_id` | bigint | không | PK |
| `author_name` | varchar(255) | không | tên tác giả |
| `author_slug` | varchar(255) | không | slug duy nhất |
| `bio` | text | có | tiểu sử ngắn |
| `avatar_url` | text | có | ảnh đại diện |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 6.2 `categories`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `category_id` | bigint | không | PK |
| `parent_category_id` | bigint | có | self FK nếu cần cây danh mục |
| `category_name` | varchar(255) | không | tên danh mục |
| `category_slug` | varchar(255) | không | slug |
| `sort_order` | integer | không | thứ tự hiển thị |
| `is_active` | boolean | không | trạng thái hoạt động |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 6.3 `books`

### Mục đích
- bảng canonical mô tả sản phẩm sách.

### Module owner
- `catalog`

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả | Ghi chú |
|---|---|---|---|---|
| `book_id` | bigint | không | PK | |
| `book_slug` | varchar(255) | không | slug public | unique |
| `title` | varchar(500) | không | tên sách | |
| `subtitle` | varchar(500) | có | phụ đề | |
| `description` | text | có | mô tả dài | |
| `short_description` | text | có | mô tả ngắn | |
| `cover_image_url` | text | có | ảnh bìa | |
| `product_type` | varchar(32) | không | `ebook`, `physical`, `hybrid`, `membership_only` | |
| `membership_eligible` | boolean | không | có thuộc quyền membership hay không | |
| `publish_state` | varchar(32) | không | `draft`, `published`, `unpublished`, `archived` | canonical |
| `isbn_print` | varchar(64) | có | ISBN bản in | |
| `isbn_ebook` | varchar(64) | có | ISBN bản số | |
| `language_code` | varchar(16) | có | ngôn ngữ | |
| `page_count` | integer | có | số trang | |
| `publisher_name` | varchar(255) | có | tên NXB/đơn vị phát hành | |
| `published_at` | timestamptz | có | thời điểm public publish | |
| `search_vector` | tsvector | có | hỗ trợ full-text search | tùy chọn triển khai |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Indexes
- unique `book_slug`
- index `publish_state`
- index `product_type`
- index `membership_eligible`
- full-text index trên `search_vector` nếu dùng

### Quy tắc update
- `publish_state` phải đi qua publish/unpublish flow.
- `membership_eligible` ảnh hưởng entitlement từ membership, cần event invalidation.

## 6.4 `book_authors`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `book_author_id` | bigint | không | PK |
| `book_id` | bigint | không | FK sách |
| `author_id` | bigint | không | FK tác giả |
| `author_order` | integer | không | thứ tự hiển thị |
| `created_at` | timestamptz | không | |

## 6.5 `book_categories`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `book_category_id` | bigint | không | PK |
| `book_id` | bigint | không | FK sách |
| `category_id` | bigint | không | FK danh mục |
| `created_at` | timestamptz | không | |

## 6.6 `book_formats`

### Mục đích
- mô tả các format và khả năng đọc/tải của mỗi sách.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `book_format_id` | bigint | không | PK |
| `book_id` | bigint | không | FK sách |
| `format_type` | varchar(32) | không | `pdf`, `epub`, `physical` |
| `is_enabled` | boolean | không | format có đang bật bán/phục vụ không |
| `is_downloadable` | boolean | không | có cho tải file không |
| `is_online_readable` | boolean | không | có hỗ trợ reader online không |
| `file_asset_id` | bigint | có | FK sang asset nếu là digital |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Unique
- `book_id + format_type`

## 6.7 `file_assets`

### Mục đích
- metadata asset file cho ebook/ảnh/tài nguyên số.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `file_asset_id` | bigint | không | PK |
| `storage_provider` | varchar(64) | không | s3, gcs, local, ... |
| `storage_bucket` | varchar(255) | có | bucket/container |
| `storage_key` | text | không | path/key trong storage |
| `mime_type` | varchar(255) | có | content type |
| `file_size_bytes` | bigint | có | kích thước |
| `checksum_sha256` | varchar(64) | có | checksum file |
| `asset_type` | varchar(64) | không | `book_pdf`, `book_epub`, `cover_image`, ... |
| `asset_state` | varchar(32) | không | `active`, `disabled` |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Sensitivity
- `storage_key` có thể xem là nhạy cảm vận hành, không nên lộ trực tiếp qua public API.

## 6.8 `book_prices`

### Mục đích
- lưu giá hiện hành hoặc lịch sử giá theo sách/sku.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `book_price_id` | bigint | không | PK |
| `book_id` | bigint | không | FK sách |
| `format_type` | varchar(32) | có | nếu giá theo format; null nếu giá mức sách |
| `currency` | varchar(8) | không | hiện tại luôn VND |
| `list_price_vnd` | bigint | không | giá niêm yết |
| `sale_price_vnd` | bigint | có | giá bán hiện tại |
| `effective_from` | timestamptz | không | bắt đầu hiệu lực |
| `effective_to` | timestamptz | có | kết thúc hiệu lực |
| `price_state` | varchar(32) | không | `active`, `inactive`, `scheduled` |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Quy tắc
- Tại một thời điểm, không được có nhiều record active cùng scope `book_id + format_type`.

## 7. Cart domain

## 7.1 `carts`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `cart_id` | bigint | không | PK |
| `user_id` | bigint | không | owner user |
| `cart_state` | varchar(32) | không | `active`, `checked_out`, `abandoned` |
| `coupon_code` | varchar(64) | có | coupon hiện đang apply |
| `currency` | varchar(8) | không | VND |
| `last_priced_at` | timestamptz | có | lần preview pricing gần nhất |
| `checked_out_at` | timestamptz | có | thời điểm checkout |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 7.2 `cart_items`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `cart_item_id` | bigint | không | PK |
| `cart_id` | bigint | không | FK cart |
| `sku_type` | varchar(32) | không | `ebook`, `physical`, `membership_plan` |
| `sku_id` | bigint | không | id SKU/business item |
| `quantity` | integer | không | số lượng |
| `unit_price_vnd_preview` | bigint | có | giá preview gần nhất |
| `final_unit_price_vnd_preview` | bigint | có | giá sau discount preview |
| `metadata_snapshot` | jsonb | có | snapshot bổ sung nếu cần |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 8. Order domain

## 8.1 `orders`

### Mục đích
- aggregate canonical cho đơn hàng.

### Module owner
- `order`

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả | Ghi chú |
|---|---|---|---|---|
| `order_id` | bigint | không | PK | |
| `order_no` | varchar(64) | không | mã đơn public | unique |
| `user_id` | bigint | không | owner user | |
| `order_state` | varchar(32) | không | trạng thái đơn hàng | state machine-managed |
| `payment_method` | varchar(32) | không | `vnpay`, `momo`, `cod`, ... | |
| `currency` | varchar(8) | không | VND | |
| `subtotal_vnd` | bigint | không | tổng trước giảm giá/phí | |
| `discount_vnd` | bigint | không | tổng giảm giá | |
| `shipping_fee_vnd` | bigint | không | phí ship | |
| `tax_vnd` | bigint | không | thuế | hiện tại có thể 0 |
| `total_vnd` | bigint | không | tổng phải trả | |
| `contains_digital_items` | boolean | không | cờ hỗ trợ logic fulfillment | |
| `contains_physical_items` | boolean | không | cờ hỗ trợ logic fulfillment/COD | |
| `buyer_email` | varchar(255) | không | snapshot email lúc đặt | PII snapshot |
| `buyer_full_name` | varchar(255) | không | snapshot tên lúc đặt | PII snapshot |
| `buyer_phone_number` | varchar(32) | có | snapshot số điện thoại | PII snapshot |
| `billing_snapshot` | jsonb | có | snapshot thông tin hóa đơn | |
| `shipping_snapshot` | jsonb | có | snapshot địa chỉ giao hàng | |
| `cancel_reason_code` | varchar(64) | có | mã lý do hủy | |
| `cancel_reason_note` | text | có | ghi chú hủy | |
| `placed_at` | timestamptz | không | lúc đặt order | |
| `paid_at` | timestamptz | có | lúc paid | |
| `fulfilled_at` | timestamptz | có | lúc hoàn tất | |
| `cancelled_at` | timestamptz | có | lúc hủy | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Indexes
- unique `order_no`
- index `user_id, created_at desc`
- index `order_state`
- index `payment_method`
- index `placed_at`

### Quy tắc update
- `order_state` đi qua state machine.
- pricing snapshot là immutable sau khi order được tạo, trừ corrective flow có audit.

## 8.2 `order_items`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `order_item_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `sku_type` | varchar(32) | không | loại item |
| `sku_id` | bigint | không | id item |
| `book_id` | bigint | có | nếu là book-related item |
| `membership_plan_id` | bigint | có | nếu là membership line |
| `item_title_snapshot` | varchar(500) | không | snapshot title |
| `format_type` | varchar(32) | có | nếu line gắn format cụ thể |
| `quantity` | integer | không | số lượng |
| `unit_price_vnd` | bigint | không | giá gốc |
| `final_unit_price_vnd` | bigint | không | giá cuối |
| `line_discount_vnd` | bigint | không | giảm giá line |
| `line_total_vnd` | bigint | không | tổng line |
| `item_state` | varchar(32) | không | state line |
| `metadata_snapshot` | jsonb | có | snapshot thêm |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Tư duy master backend

Order items phải snapshot đủ dữ liệu để sau này giá/catalog thay đổi vẫn tái hiện được lịch sử giao dịch. Đây là tư duy transaction history rất quan trọng mà nhiều dev mới bỏ quên.

## 8.3 `order_state_logs`

### Mục đích
- log append-only các transition order.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `order_state_log_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `from_state` | varchar(32) | có | state cũ |
| `to_state` | varchar(32) | không | state mới |
| `trigger_code` | varchar(64) | không | trigger business |
| `actor_type` | varchar(32) | không | user/admin/system/gateway |
| `actor_id` | bigint | có | id actor |
| `reason_code` | varchar(64) | có | mã lý do |
| `reason_note` | text | có | ghi chú |
| `correlation_id` | varchar(128) | có | trace business |
| `created_at` | timestamptz | không | thời điểm log |

## 9. Payment domain

## 9.1 `payments`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `payment_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `provider_code` | varchar(32) | không | `vnpay`, `momo`, `stripe`, ... |
| `payment_method` | varchar(32) | không | online method |
| `payment_state` | varchar(32) | không | canonical state |
| `amount_vnd` | bigint | không | số tiền thanh toán |
| `captured_amount_vnd` | bigint | có | số tiền captured |
| `refunded_amount_vnd` | bigint | không | tổng đã refund |
| `external_payment_ref` | varchar(255) | có | mã giao dịch ngoài |
| `provider_order_ref` | varchar(255) | có | ref request provider |
| `attempt_count` | integer | không | số attempt |
| `initiated_at` | timestamptz | có | |
| `authorized_at` | timestamptz | có | |
| `captured_at` | timestamptz | có | |
| `failed_at` | timestamptz | có | |
| `expired_at` | timestamptz | có | |
| `last_error_code` | varchar(128) | có | lỗi gần nhất |
| `last_error_message` | text | có | lỗi gần nhất |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 9.2 `payment_attempts`

### Mục đích
- lịch sử các lần thử thanh toán.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `payment_attempt_id` | bigint | không | PK |
| `payment_id` | bigint | không | FK payment |
| `attempt_no` | integer | không | lần thứ mấy |
| `provider_request_ref` | varchar(255) | có | ref gửi provider |
| `provider_response_ref` | varchar(255) | có | ref phản hồi |
| `attempt_state` | varchar(32) | không | `initiated`, `redirected`, `callback_received`, `captured`, `failed` |
| `request_payload_snapshot` | jsonb | có | payload rút gọn |
| `response_payload_snapshot` | jsonb | có | payload rút gọn |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 9.3 `payment_callbacks`

### Mục đích
- lưu callback/webhook raw đã qua normalization để audit và dedup.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `payment_callback_id` | bigint | không | PK |
| `provider_code` | varchar(32) | không | provider |
| `external_event_id` | varchar(255) | có | event ref bên ngoài |
| `external_payment_ref` | varchar(255) | có | ref transaction |
| `signature_valid` | boolean | không | verify chữ ký hay chưa |
| `payload_raw` | jsonb | không | payload callback |
| `processed` | boolean | không | đã xử lý canonical chưa |
| `received_at` | timestamptz | không | lúc nhận |
| `processed_at` | timestamptz | có | lúc xử lý |

### Tư duy master backend

Raw callback storage cực quan trọng cho forensic/debug. Nhiều team bỏ qua rồi sau này không chứng minh được payment issue nằm ở đâu.

## 10. Refund & Chargeback domain

## 10.1 `refunds`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `refund_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `payment_id` | bigint | không | FK payment |
| `refund_state` | varchar(32) | không | canonical state |
| `amount_vnd` | bigint | không | số tiền refund |
| `reason_code` | varchar(64) | không | mã lý do |
| `reason_note` | text | có | ghi chú |
| `requested_by_type` | varchar(32) | không | user/admin/system |
| `requested_by_id` | bigint | có | actor id |
| `approved_by_admin_id` | bigint | có | approver |
| `provider_refund_ref` | varchar(255) | có | ref ngoài |
| `requested_at` | timestamptz | không | |
| `approved_at` | timestamptz | có | |
| `completed_at` | timestamptz | có | |
| `failed_at` | timestamptz | có | |
| `last_error_code` | varchar(128) | có | |
| `last_error_message` | text | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 10.2 `refund_items`

### Mục đích
- chi tiết refund theo line item.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `refund_item_id` | bigint | không | PK |
| `refund_id` | bigint | không | FK refund |
| `order_item_id` | bigint | không | FK order item |
| `refund_amount_vnd` | bigint | không | số tiền refund line |
| `revoke_entitlement_required` | boolean | không | có cần revoke quyền số không |
| `created_at` | timestamptz | không | |

## 10.3 `chargebacks`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `chargeback_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `payment_id` | bigint | không | FK payment |
| `chargeback_state` | varchar(32) | không | `opened`, `under_review`, `submitted`, `won`, `lost`, `cancelled` |
| `amount_vnd` | bigint | không | disputed amount |
| `reason_code` | varchar(64) | có | lý do tranh chấp |
| `provider_case_ref` | varchar(255) | có | ref case ngoài |
| `opened_at` | timestamptz | không | |
| `resolved_at` | timestamptz | có | |
| `resolution` | varchar(32) | có | `won`, `lost`, ... |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 11. Membership & Entitlement domain

## 11.1 `membership_plans`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `membership_plan_id` | bigint | không | PK |
| `plan_code` | varchar(64) | không | unique code |
| `plan_name` | varchar(255) | không | tên gói |
| `duration_days` | integer | không | thời hạn |
| `price_vnd` | bigint | không | giá bán |
| `max_devices` | integer | không | quota thiết bị |
| `max_concurrent_reader_sessions` | integer | không | quota đọc đồng thời |
| `max_downloads_total` | integer | có | quota tải tổng nếu áp dụng |
| `plan_state` | varchar(32) | không | `active`, `inactive` |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 11.2 `memberships`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `membership_id` | bigint | không | PK |
| `user_id` | bigint | không | owner |
| `membership_plan_id` | bigint | không | FK plan |
| `source_order_id` | bigint | có | order phát sinh |
| `membership_state` | varchar(32) | không | canonical state |
| `starts_at` | timestamptz | không | bắt đầu hiệu lực |
| `expires_at` | timestamptz | không | hết hạn |
| `revoked_at` | timestamptz | có | nếu revoke |
| `suspended_at` | timestamptz | có | nếu suspend |
| `reason_code` | varchar(64) | có | lý do revoke/suspend |
| `reason_note` | text | có | ghi chú |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 11.3 `entitlements`

### Mục đích
- canonical business-right per user-book-source.

### Module owner
- `entitlement`

### Columns

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `entitlement_id` | bigint | không | PK |
| `user_id` | bigint | không | owner user |
| `book_id` | bigint | không | book được cấp quyền |
| `source_type` | varchar(32) | không | `ebook_purchase`, `membership`, `admin_grant`, ... |
| `source_id` | bigint | không | ID nguồn cấp quyền |
| `entitlement_state` | varchar(32) | không | canonical state |
| `allow_read_online` | boolean | không | quyền đọc online |
| `allow_download` | boolean | không | quyền tải |
| `starts_at` | timestamptz | không | bắt đầu |
| `expires_at` | timestamptz | có | hết hạn nếu có |
| `revoked_at` | timestamptz | có | |
| `reason_code` | varchar(64) | có | lý do revoke/expire manual |
| `reason_note` | text | có | ghi chú |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Constraints
- unique logic tùy source có thể là `user_id + book_id + source_type + source_id`

### Tư duy master backend

Entitlement là lớp canonical cho quyền hiện tại. Nếu không có bảng này, logic quyền sẽ vỡ ra ở order, membership, download, reader và trở nên cực khó reasoning.

## 11.4 `entitlement_state_logs`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `entitlement_state_log_id` | bigint | không | PK |
| `entitlement_id` | bigint | không | FK entitlement |
| `from_state` | varchar(32) | có | |
| `to_state` | varchar(32) | không | |
| `trigger_code` | varchar(64) | không | |
| `actor_type` | varchar(32) | không | |
| `actor_id` | bigint | có | |
| `reason_code` | varchar(64) | có | |
| `reason_note` | text | có | |
| `created_at` | timestamptz | không | |

## 12. Reader & Download domain

## 12.1 `reader_sessions`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `reader_session_id` | bigint | không | PK |
| `user_id` | bigint | không | user |
| `book_id` | bigint | không | sách đang đọc |
| `device_id` | bigint | có | thiết bị |
| `format_type` | varchar(32) | không | pdf/epub |
| `reader_session_state` | varchar(32) | không | `created`, `active`, `expired`, `ended`, `kicked` |
| `last_position_snapshot` | jsonb | có | vị trí đọc cuối |
| `started_at` | timestamptz | không | |
| `last_heartbeat_at` | timestamptz | có | |
| `ended_at` | timestamptz | có | |
| `end_reason_code` | varchar(64) | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 12.2 `reader_progresses`

### Mục đích
- lưu tiến độ đọc lâu dài theo user-book-format.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `reader_progress_id` | bigint | không | PK |
| `user_id` | bigint | không | |
| `book_id` | bigint | không | |
| `format_type` | varchar(32) | không | |
| `progress_percent` | numeric(5,2) | có | phần trăm tiến độ |
| `position_snapshot` | jsonb | có | chapter/locator/... |
| `last_read_at` | timestamptz | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 12.3 `downloads`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `download_id` | bigint | không | PK |
| `user_id` | bigint | không | owner |
| `book_id` | bigint | không | book |
| `format_type` | varchar(32) | không | pdf/epub |
| `device_id` | bigint | có | thiết bị |
| `source_type` | varchar(32) | không | nguồn entitlement |
| `source_id` | bigint | không | id nguồn |
| `download_state` | varchar(32) | không | `requested`, `issued`, `consumed`, `expired`, `revoked`, `failed` |
| `token_id` | varchar(128) | có | token định danh |
| `single_use` | boolean | không | token dùng một lần |
| `issued_at` | timestamptz | có | |
| `expires_at` | timestamptz | có | |
| `consumed_at` | timestamptz | có | |
| `revoked_at` | timestamptz | có | |
| `failure_code` | varchar(64) | có | nếu failed |
| `failure_note` | text | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 13. Inventory & Shipment domain

## 13.1 `inventory_items`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `inventory_item_id` | bigint | không | PK |
| `book_id` | bigint | không | FK book |
| `sku_code` | varchar(128) | không | mã SKU nội bộ |
| `on_hand_qty` | integer | không | tồn vật lý |
| `reserved_qty` | integer | không | đang giữ |
| `available_qty` | integer | không | có thể bán |
| `inventory_state` | varchar(32) | không | `active`, `inactive` |
| `updated_reason_code` | varchar(64) | có | lý do cập nhật gần nhất |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Quy tắc
- `available_qty` phải nhất quán với policy tính toán và không âm.

## 13.2 `inventory_reservations`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `inventory_reservation_id` | bigint | không | PK |
| `order_id` | bigint | có | order liên quan |
| `order_item_id` | bigint | có | line liên quan |
| `book_id` | bigint | không | item |
| `quantity` | integer | không | số lượng |
| `reservation_state` | varchar(32) | không | `pending`, `reserved`, `released`, `consumed`, `returned` |
| `reserved_at` | timestamptz | có | |
| `expires_at` | timestamptz | có | timeout reservation |
| `released_at` | timestamptz | có | |
| `release_reason_code` | varchar(64) | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 13.3 `inventory_movements`

### Mục đích
- append-only log biến động tồn kho.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `inventory_movement_id` | bigint | không | PK |
| `inventory_item_id` | bigint | không | FK inventory item |
| `movement_type` | varchar(32) | không | `reserve`, `release`, `consume`, `return`, `adjust` |
| `quantity_delta` | integer | không | biến động dương/âm |
| `before_on_hand_qty` | integer | có | |
| `after_on_hand_qty` | integer | có | |
| `before_reserved_qty` | integer | có | |
| `after_reserved_qty` | integer | có | |
| `reason_code` | varchar(64) | có | |
| `reference_type` | varchar(32) | có | `order`, `reservation`, `admin_adjustment` |
| `reference_id` | bigint | có | id tham chiếu |
| `actor_type` | varchar(32) | không | admin/system |
| `actor_id` | bigint | có | |
| `created_at` | timestamptz | không | |

## 13.4 `shipments`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `shipment_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `shipment_state` | varchar(32) | không | canonical state |
| `carrier_code` | varchar(32) | có | mã carrier |
| `tracking_no` | varchar(255) | có | tracking |
| `cod_amount_vnd` | bigint | có | số tiền COD nếu có |
| `recipient_name` | varchar(255) | không | snapshot người nhận |
| `recipient_phone` | varchar(32) | có | snapshot điện thoại |
| `shipping_address_snapshot` | jsonb | không | snapshot địa chỉ |
| `packed_at` | timestamptz | có | |
| `shipped_at` | timestamptz | có | |
| `delivered_at` | timestamptz | có | |
| `failed_at` | timestamptz | có | |
| `returned_at` | timestamptz | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 13.5 `shipment_status_logs`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `shipment_status_log_id` | bigint | không | PK |
| `shipment_id` | bigint | không | FK shipment |
| `from_state` | varchar(32) | có | |
| `to_state` | varchar(32) | không | |
| `external_event_id` | varchar(255) | có | event từ carrier |
| `source_type` | varchar(32) | không | `carrier_webhook`, `carrier_poll`, `admin`, `system` |
| `payload_snapshot` | jsonb | có | dữ liệu status raw/normalized |
| `created_at` | timestamptz | không | |

## 14. Invoice & Notification domain

## 14.1 `e_invoice_exports`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `e_invoice_export_id` | bigint | không | PK |
| `order_id` | bigint | không | FK order |
| `invoice_state` | varchar(32) | không | `pending_export`, `exporting`, `exported`, `failed`, `cancelled` |
| `provider_code` | varchar(64) | không | nhà cung cấp hóa đơn |
| `provider_invoice_ref` | varchar(255) | có | mã hóa đơn/provider ref |
| `buyer_type` | varchar(32) | không | `individual`, `company` |
| `invoice_payload_snapshot` | jsonb | không | payload gửi provider |
| `requested_at` | timestamptz | không | |
| `exported_at` | timestamptz | có | |
| `failed_at` | timestamptz | có | |
| `last_error_code` | varchar(128) | có | |
| `last_error_message` | text | có | |
| `retry_count` | integer | không | số lần retry |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 14.2 `notifications`

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `notification_id` | bigint | không | PK |
| `recipient_user_id` | bigint | có | user nếu có |
| `channel` | varchar(32) | không | `email` |
| `template_code` | varchar(128) | không | template |
| `recipient_email` | varchar(255) | có | email nhận | PII |
| `payload_snapshot` | jsonb | có | params render template |
| `notification_state` | varchar(32) | không | `queued`, `processing`, `sent`, `retry_waiting`, `failed`, `cancelled` |
| `attempt_count` | integer | không | số lần gửi |
| `next_retry_at` | timestamptz | có | |
| `sent_at` | timestamptz | có | |
| `failed_at` | timestamptz | có | |
| `last_error_code` | varchar(128) | có | |
| `last_error_message` | text | có | |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 15. Eventing & Operational domain

## 15.1 `outbox_events`

### Mục đích
- canonical transactional outbox để publish Kafka an toàn.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `outbox_event_id` | bigint | không | PK |
| `event_id` | uuid | không | logical event id |
| `aggregate_type` | varchar(64) | không | order/payment/... |
| `aggregate_id` | bigint | không | id aggregate |
| `event_type` | varchar(128) | không | `order.paid`, ... |
| `event_version` | integer | không | version payload |
| `topic_name` | varchar(255) | không | topic Kafka |
| `event_key` | varchar(255) | không | partition key |
| `payload_json` | jsonb | không | payload đầy đủ |
| `headers_json` | jsonb | có | headers metadata |
| `outbox_state` | varchar(32) | không | `pending`, `publishing`, `published`, `failed`, `parked` |
| `retry_count` | integer | không | số lần retry publish |
| `last_error_message` | text | có | lỗi gần nhất |
| `occurred_at` | timestamptz | không | business occurred time |
| `published_at` | timestamptz | có | lúc publish thành công |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

### Indexes
- unique `event_id`
- index `outbox_state, created_at`
- index `aggregate_type, aggregate_id`

## 15.2 `processed_events`

### Mục đích
- dedup canonical cho consumer xử lý event.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `processed_event_id` | bigint | không | PK |
| `consumer_name` | varchar(255) | không | tên consumer |
| `event_id` | uuid | không | logical event id |
| `event_type` | varchar(128) | không | loại event |
| `processed_at` | timestamptz | không | thời điểm xử lý |
| `result_state` | varchar(32) | không | `success`, `skipped`, `failed` |
| `notes` | text | có | ghi chú |
| `created_at` | timestamptz | không | |

### Unique
- `consumer_name + event_id`

## 15.3 `job_runs`

### Mục đích
- log thực thi scheduler/background jobs.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `job_run_id` | bigint | không | PK |
| `job_type` | varchar(128) | không | loại job |
| `job_key` | varchar(255) | có | dedup key nếu có |
| `job_state` | varchar(32) | không | `queued`, `running`, `success`, `failed`, `cancelled` |
| `triggered_by` | varchar(32) | không | `scheduler`, `admin`, `system` |
| `started_at` | timestamptz | có | |
| `finished_at` | timestamptz | có | |
| `payload_json` | jsonb | có | input payload |
| `result_json` | jsonb | có | output summary |
| `error_message` | text | có | lỗi nếu fail |
| `created_at` | timestamptz | không | |
| `updated_at` | timestamptz | không | |

## 15.4 `audit_logs`

### Mục đích
- audit business/admin operations trọng yếu.

| Cột | Kiểu dữ liệu | Null | Mô tả |
|---|---|---|---|
| `audit_log_id` | bigint | không | PK |
| `actor_type` | varchar(32) | không | user/admin/system/gateway |
| `actor_id` | bigint | có | id actor |
| `action_code` | varchar(128) | không | hành động |
| `resource_type` | varchar(64) | không | order/payment/... |
| `resource_id` | bigint | không | id resource |
| `before_snapshot` | jsonb | có | state trước |
| `after_snapshot` | jsonb | có | state sau |
| `reason_code` | varchar(64) | có | |
| `reason_note` | text | có | |
| `correlation_id` | varchar(128) | có | |
| `ip_address` | inet | có | nếu có |
| `created_at` | timestamptz | không | |

### Sensitivity
- snapshot có thể chứa PII; cần policy truy cập và masking khi hiển thị.

## 16. Ràng buộc liên bảng quan trọng

### 16.1 FK quan trọng
- `orders.user_id -> users.user_id`
- `order_items.order_id -> orders.order_id`
- `payments.order_id -> orders.order_id`
- `refunds.payment_id -> payments.payment_id`
- `memberships.user_id -> users.user_id`
- `entitlements.user_id -> users.user_id`
- `entitlements.book_id -> books.book_id`
- `downloads.book_id -> books.book_id`
- `shipments.order_id -> orders.order_id`
- `inventory_reservations.order_item_id -> order_items.order_item_id`

### 16.2 Quy tắc không nên cascade delete mạnh

Với bảng nghiệp vụ lịch sử như `orders`, `payments`, `refunds`, `downloads`, `audit_logs`, **không nên dùng cascade delete**. Lịch sử giao dịch và audit phải được giữ để forensic, reconciliation và compliance.

### Tư duy master backend

Cascade delete nghe rất tiện nhưng thường phá forensic value của hệ thống. Trong thương mại điện tử và thanh toán, mất lịch sử còn nguy hiểm hơn giữ “thừa” dữ liệu.

## 17. Phân loại sensitivity dữ liệu

| Mức độ | Ví dụ cột |
|---|---|
| Public-ish | `books.title`, `book_slug`, `category_name` |
| Internal business | `order_state`, `payment_state`, `inventory quantities` |
| PII | `users.email`, `orders.buyer_full_name`, `shipments.recipient_phone` |
| Sensitive secret-ish | `password_hash`, `refresh_token_hash` |
| Operational sensitive | `file_assets.storage_key`, `payment_callbacks.payload_raw` |

### Quy tắc
- PII không log raw trong application logs.
- Bảng audit hiển thị qua admin cần masking theo permission.
- Password/token hash tuyệt đối không expose qua API.

## 18. Data lifecycle và retention notes

| Bảng | Retention note |
|---|---|
| `users` | giữ lâu dài |
| `user_sessions` | có thể archive/xóa sau khi hết giá trị vận hành và theo policy bảo mật |
| `orders`, `payments`, `refunds` | giữ lâu dài cho accounting/audit |
| `downloads` | giữ lịch sử dài hạn để forensic/support |
| `reader_sessions` | có thể archive/cleanup theo retention window |
| `payment_callbacks` | giữ đủ lâu để support dispute/reconcile |
| `outbox_events` | có thể archive sau khi published lâu và không cần forensic nóng |
| `processed_events` | giữ theo retry/replay policy |
| `audit_logs` | giữ lâu dài |

## 19. Danh mục enum gợi ý

### 19.1 `order_state`
- `draft`
- `pending_payment`
- `payment_processing`
- `confirmed_cod`
- `paid`
- `partially_fulfilled`
- `fulfilled`
- `cancel_requested`
- `cancelled`
- `refund_pending`
- `partially_refunded`
- `refunded`
- `failed`
- `cod_in_delivery`
- `failed_cod`
- `chargeback_open`
- `chargeback_resolved`

### 19.2 `payment_state`
- `initiated`
- `pending`
- `authorized`
- `captured`
- `failed`
- `expired`
- `cancelled`
- `refund_pending`
- `refunded_partial`
- `refunded_full`
- `chargeback_open`
- `chargeback_resolved`

### 19.3 `refund_state`
- `requested`
- `under_review`
- `approved`
- `processing_gateway`
- `completed`
- `failed`
- `rejected`
- `cancelled`

### 19.4 `membership_state`
- `pending_activation`
- `active`
- `suspended`
- `expired`
- `revoked`

### 19.5 `entitlement_state`
- `pending_grant`
- `active`
- `suspended`
- `expired`
- `revoked`

### 19.6 `shipment_state`
- `pending_pack`
- `packed`
- `awaiting_pickup`
- `in_transit`
- `delivered`
- `delivery_failed`
- `returning`
- `returned`
- `cancelled`

## 20. Index strategy ghi chú tổng quát

### 20.1 Hot indexes cần chú ý
- `orders(user_id, created_at desc)` cho order history user;
- `orders(order_state, placed_at desc)` cho admin queue;
- `payments(order_id)` và `payments(payment_state)`;
- `memberships(user_id, membership_state)`;
- `entitlements(user_id, book_id, entitlement_state)` cho access check;
- `downloads(user_id, created_at desc)`;
- `inventory_items(book_id)` unique;
- `shipments(order_id)` unique hoặc indexed mạnh;
- `outbox_events(outbox_state, created_at)`;
- `processed_events(consumer_name, event_id)` unique lookup.

### 20.2 Tư duy master backend

Index không chỉ phục vụ nhanh query. Index còn phản ánh những **lookup path có ý nghĩa nghiệp vụ và vận hành**. Nếu bạn biết đội support, worker, reconciliation và APIs sẽ truy gì nhiều, bạn sẽ thiết kế index chủ động thay vì chữa cháy sau.

## 21. Checklist quản trị Data Dictionary

Theo thực hành data dictionary tốt, mỗi data element nên có owner rõ ràng, cấu trúc mô tả nhất quán và được review định kỳ để tránh tài liệu lỗi thời [web:142][web:144][web:151][web:154]. Data dictionary cũng nên được cập nhật cùng lifecycle phát triển schema, thay vì để tách rời và trôi dần khỏi thực tế hệ thống [web:148][web:149][web:152].

Checklist bắt buộc cho team:
- mọi bảng mới phải được thêm vào data dictionary khi tạo migration;
- mọi cột state mới phải map với state machine và ADR tương ứng;
- mọi cột chứa PII phải được đánh dấu sensitivity;
- mọi unique/index/fk mới phải có lý do nghiệp vụ hoặc vận hành;
- data dictionary phải được review định kỳ theo release cadence.

## 22. Acceptance criteria

### 22.1 Thiết kế
- Toàn bộ bảng canonical chính đã được mô tả.
- Mọi cột trạng thái, tiền, snapshot, callback, audit, eventing đều có semantic rõ ràng.
- Có ownership, sensitivity và lifecycle notes cho bảng quan trọng.

### 22.2 Triển khai
- Có thể dùng tài liệu này để viết migration và comments cho schema.
- Có thể map từ bảng/cột sang modules trong Golang blueprint.
- Có thể map trực tiếp sang OpenAPI fields, Kafka payloads và Redis hot-path.

### 22.3 Vận hành
- Hỗ trợ audit, forensic, reconciliation và support troubleshooting.
- Tránh được lỗi hiểu sai ý nghĩa cột hoặc update sai ownership.
- Là nền cho schema review và code review về sau.

## 23. Hạng mục nên làm tiếp

Sau Data Dictionary, các tài liệu nên làm tiếp là:
- Test Strategy & QA Matrix chi tiết;
- Failure Recovery Runbook;
- Deployment & Environment Blueprint;
- Coding Standards & Code Review Checklist cho Golang team;
- Migration Review Checklist và Release Checklist.

Tài liệu này là lớp định nghĩa dữ liệu chuẩn doanh nghiệp cho dự án bookstore, giúp toàn team dùng cùng một ngôn ngữ khi nói về schema, trạng thái, quan hệ, audit, quyền truy cập và lifecycle dữ liệu, thay vì chỉ nhìn database như tập hợp cột và bảng rời rạc.
