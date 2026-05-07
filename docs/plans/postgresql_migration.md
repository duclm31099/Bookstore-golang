# Tài liệu PostgreSQL Migration Reference - Bản tiếng Việt

## 1. Mục đích tài liệu

Tài liệu này đặc tả chiến lược **PostgreSQL migration** cho backend bookstore modular monolith dựa trên SRD revised và ERD revised. Mục tiêu không chỉ là “tạo bảng”, mà là đảm bảo migration phản ánh đúng business invariants, transaction boundaries, data ownership, auditability, rollback strategy, seed strategy, backward compatibility và khả năng vận hành production.

Tài liệu này dành cho backend engineers, solution architects, reviewers và DevOps tham gia dựng schema, backfill, migration pipeline và data validation.

## 2. Tư duy migration của master backend

Một migration tốt không chỉ quan tâm đến DDL, mà quan tâm đến:
- dữ liệu cũ có an toàn không;
- invariants có bị phá khi rollout không;
- code cũ và code mới có chạy song song được không;
- rollback có thực tế không;
- index tạo khi nào để tránh khóa quá mức;
- backfill chạy ở đâu, theo batch nào, checkpoint nào;
- audit và monitoring cho migration có đủ hay không.

Với project này, migration phải phục vụ bốn mục tiêu tối thượng:
1. correctness của transactional state;
2. bảo toàn lịch sử giao dịch;
3. idempotency và chống duplicate side effects;
4. khả năng mở rộng schema mà không làm mơ hồ ownership.

## 3. Nguyên tắc migration bắt buộc

### 3.1 Nguyên tắc tổng quát
- PostgreSQL là source of truth cho state bền vững.
- Không dùng migration để encode business rule phức tạp thay cho service layer, trừ các invariants thật sự đáng khóa ở DB.
- Mỗi migration phải **atomic**, **re-runnable theo tool**, **có thứ tự rõ**, và **có down strategy hoặc documented no-downgrade reason**.
- Không gộp quá nhiều concern vào một migration khổng lồ nếu điều đó làm rollback/diagnosis khó khăn.
- Schema migration, data backfill migration và index optimization migration nên được phân lớp rõ.

### 3.2 Nguyên tắc naming
- Dùng `snake_case` cho bảng, cột, index, constraint.
- File migration dùng prefix tăng dần, ví dụ `000001_...`.
- Tên index/constraint nên mô tả được bảng và cột.

### 3.3 Phân loại migration
- **Schema migration**: tạo/sửa bảng, cột, FK, index, constraint.
- **Reference seed migration**: seed dữ liệu cấu hình tĩnh như membership plans mẫu, admin bootstrap tối thiểu nếu cần.
- **Backfill migration**: điền dữ liệu mới từ schema cũ sang schema mới.
- **Operational migration**: tạo view hỗ trợ, partial index, concurrent index, trigger hỗ trợ tạm thời.

## 4. Thứ tự migration chuẩn cho project này

## 4.1 Giai đoạn nền
1. Extensions và helper setup.
2. Identity tables.
3. Catalog tables.
4. Membership tables.
5. Sellable SKU + pricing tables.
6. Cart tables.
7. Order tables.
8. Payment / refund / chargeback tables.
9. Inventory / shipment tables.
10. Reader / download / entitlement tables.
11. Audit / notification / outbox / processed_events / invoice export tables.
12. Index tối ưu bổ sung.
13. Seed data tối thiểu.

## 4.2 Lý do thứ tự này
- Identity và catalog là nền cho hầu hết FK sau.
- `sellable_skus` phải xuất hiện trước `cart_items`, `order_items`, `inventory_items` nếu chọn canonical commerce reference mới.
- `orders` phải có trước `payments`, `inventory_reservations`, `shipments`, `e_invoice_exports`.
- `users`, `books`, `membership_plans` phải có trước `entitlements`, `downloads`.
- `outbox_events` nên được tạo sớm, nhưng thường tiện hơn khi tạo cùng nhóm operations sau khi business tables đã ổn định.

## 5. Cấu trúc thư mục migration đề xuất

```text
migrations/
├── 000001_init_extensions.up.sql
├── 000001_init_extensions.down.sql
├── 000010_identity_users.up.sql
├── 000010_identity_users.down.sql
├── 000011_identity_credentials_sessions_devices_addresses.up.sql
├── 000011_identity_credentials_sessions_devices_addresses.down.sql
├── 000020_catalog_books.up.sql
├── 000021_catalog_authors_categories_joins.up.sql
├── 000022_catalog_book_formats.up.sql
├── 000023_catalog_membership_plans.up.sql
├── 000024_catalog_sellable_skus.up.sql
├── 000025_catalog_prices_search_documents.up.sql
├── 000030_cart.up.sql
├── 000031_orders.up.sql
├── 000032_order_state_logs.up.sql
├── 000033_payments_refunds_chargebacks.up.sql
├── 000034_inventory_shipments.up.sql
├── 000035_entitlements_reader_downloads.up.sql
├── 000036_ops_outbox_audit_notifications_invoice.up.sql
├── 000090_indexes_extra.up.sql
├── 000100_seed_dev_reference.up.sql
└── backfill/
    ├── 001_backfill_sellable_skus.sql
    ├── 002_backfill_order_items_sellable_sku_id.sql
    └── ...
```

## 6. Chiến lược migration theo nhóm bảng

## 6.1 Nhóm Identity
### Tạo trước
- `users`
- `user_credentials`
- `user_devices`
- `user_sessions`
- `addresses`

### Lưu ý migration
- `users.email` unique ngay từ đầu.
- `version` nên có default 1.
- FK `user_sessions.device_id` dùng `CASCADE` để tự động dọn dẹp session khi device bị xóa.
- `addresses` nên có index `(user_id, is_default)` ngay từ đầu.

## 6.2 Nhóm Catalog
### Tạo trước
- `books`
- `authors`
- `categories`
- `book_authors`
- `book_categories`
- `book_formats`
- `membership_plans`
- `sellable_skus`
- `prices`
- `search_documents`

### Lưu ý migration
- `sellable_skus` là migration rất quan trọng trong bản revised.
- Nên dùng CHECK constraint để ép chỉ một trong `book_id`, `membership_plan_id` có giá trị.
- `prices` chuyển sang `sellable_sku_id` để loại bỏ polymorphic reference.
- Index `(sku_type, active)` và `(sellable_sku_id, active)` nên có sớm vì được dùng nhiều.

## 6.3 Nhóm Cart & Order
### Tạo theo thứ tự
- `carts`
- `cart_items`
- `orders`
- `order_items`
- `order_state_logs`

### Lưu ý migration
- `cart_items` dùng `sellable_sku_id` unique theo `(cart_id, sellable_sku_id)`.
- `orders` phải có `billing_snapshot` ngay từ đầu ở bản revised.
- `order_items` phải có `sellable_sku_id`, `item_title_snapshot`, `final_unit_price_vnd`, `line_total_vnd`, cùng snapshot phụ `book_id`, `membership_plan_id`.
- Không tái tạo schema cũ dùng `sku_type + sku_id` làm canonical reference.

## 6.4 Nhóm Payment / Refund / Chargeback
### Tạo theo thứ tự
- `payments`
- `payment_attempts`
- `refunds`
- `chargebacks`

### Lưu ý migration
- `payments` có index `(provider, external_payment_ref)` ngay từ đầu để phục vụ webhook reconciliation.
- `payment_attempts` unique `(payment_id, attempt_no)` để đảm bảo thứ tự lần thử.
- `refunds` và `chargebacks` dùng FK `RESTRICT` để bảo toàn audit trail.

## 6.5 Nhóm Inventory & Shipment
### Tạo theo thứ tự
- `inventory_items`
- `inventory_reservations`
- `shipments`
- `shipment_status_logs`

### Lưu ý migration
- `inventory_items` dùng `sellable_sku_id` làm PK ở bản revised.
- `inventory_reservations` phải có `hold_type`, `expires_at`, `release_reason` để encode đúng policy COD và online payment pending hold.
- Index `(reservation_state, expires_at)` là bắt buộc vì worker sweep dùng liên tục.
- `shipments.order_id` unique để phản ánh rule một order một shipment.

## 6.6 Nhóm Membership / Entitlement / Reader / Download
### Tạo theo thứ tự
- `memberships`
- `entitlements`
- `reader_sessions`
- `downloads`

### Lưu ý migration
- `downloads.token_id` unique.
- `entitlements` cần index `(user_id, book_id, state)`.
- Không lưu signed URL secret lâu dài nếu policy bảo mật không cho phép; `metadata` chỉ giữ reference cần thiết.

## 6.7 Nhóm Operations & Integration
### Tạo theo thứ tự
- `coupons`
- `coupon_redemptions`
- `notifications`
- `audit_logs`
- `outbox_events`
- `processed_events`
- `e_invoice_exports`

### Lưu ý migration
- `outbox_events.state` cần index `(state, created_at)`.
- `processed_events` dùng composite PK `(consumer_name, event_id)`.
- `coupon_redemptions` giữ FK đầy đủ về coupon, user, order.

## 7. Mẫu nội dung migration nên có

## 7.1 Mẫu schema migration
Một migration schema tốt nên chứa:
- `CREATE TABLE` với column order rõ ràng;
- PK/UNIQUE/FK/CHECK gần definition hoặc tách cuối file nhất quán;
- comments nếu team dùng `COMMENT ON COLUMN`;
- index creation;
- default values hợp lý;
- không nhồi seed data nếu seed có vòng đời riêng.

## 7.2 Mẫu backfill migration
Backfill nên có:
- source query rõ ràng;
- batch strategy nếu volume lớn;
- checkpoint/logging nếu chạy qua app job thay vì SQL file;
- validation query sau backfill;
- kế hoạch cleanup cột cũ sau khi code đọc cột mới ổn định.

## 8. Chiến lược nâng cấp từ ERD cũ sang ERD revised

## 8.1 Mục tiêu nâng cấp
- Bỏ canonical `sku_type + sku_id` ở `cart_items`, `order_items`.
- Thêm `sellable_skus`.
- Chuẩn hóa `prices` theo `sellable_sku_id`.
- Thêm `billing_snapshot`, `order_state_logs`, `final_unit_price_vnd`, `line_total_vnd`.
- Chuyển inventory sang `sellable_sku_id`.

## 8.2 Kế hoạch nhiều bước an toàn
### Bước 1: Expand schema
- Tạo bảng `sellable_skus`.
- Thêm cột nullable `sellable_sku_id` vào `cart_items`, `order_items`, `inventory_items`, `inventory_reservations` nếu đang dùng cột cũ.
- Thêm cột mới `billing_snapshot`, `final_unit_price_vnd`, `line_total_vnd`, `hold_type`, `release_reason`.
- Tạo bảng `order_state_logs`.

### Bước 2: Backfill dữ liệu
- Tạo bản ghi `sellable_skus` cho physical books, ebook formats, membership plans.
- Map `sku_type + sku_id` cũ sang `sellable_sku_id`.
- Backfill `order_items.sellable_sku_id`, `cart_items.sellable_sku_id`.
- Backfill `inventory_items.sellable_sku_id` từ `book_id` theo mapping physical SKU.
- Tính `line_total_vnd` nếu cột này mới thêm.

### Bước 3: Read path dual-read hoặc controlled cutover
- Code mới ưu tiên đọc `sellable_sku_id`.
- Có thể tạm giữ cột cũ trong một giai đoạn ngắn để rollback logic dễ hơn.

### Bước 4: Enforce constraints
- Sau khi backfill xong và validate pass, mới set `NOT NULL`, thêm FK thật và unique/check cần thiết.

### Bước 5: Cleanup
- Bỏ cột cũ `sku_type`, `sku_id` khỏi line items nếu không còn dùng.
- Bỏ `book_id` ở inventory nếu đã chuyển hoàn toàn sang `sellable_sku_id`.

## 9. Zero-downtime / low-risk migration strategy

Project phase đầu có thể chưa cần zero-downtime enterprise-grade tuyệt đối, nhưng vẫn nên theo các nguyên tắc an toàn:
- Ưu tiên mô hình **expand -> backfill -> switch reads -> enforce -> cleanup**.
- Tránh rename/drop cột trực tiếp nếu có code cũ đang chạy.
- Với index lớn, dùng chiến lược phù hợp để giảm lock nếu production volume tăng.
- Tránh migration quá lâu trong startup path của app production.

## 10. Index migration strategy

## 10.1 Index bắt buộc tạo sớm
- `users(email)` unique
- `books(slug)` unique
- `sellable_skus(sku_code)` unique
- `orders(order_no)` unique
- `payments(provider, external_payment_ref)`
- `entitlements(user_id, book_id, state)`
- `downloads(token_id)` unique
- `inventory_reservations(reservation_state, expires_at)`
- `outbox_events(state, created_at)`

## 10.2 Index tối ưu bổ sung có thể tạo sau
- Các composite index dashboard/admin ít truy cập.
- Partial index cho active rows nếu profile truy vấn chứng minh cần.
- FTS indexes nếu search bắt đầu nặng.

## 10.3 Lưu ý
- Không tạo quá nhiều index ngay từ đầu chỉ vì “có thể cần”.
- Nhưng các index phục vụ integrity, webhook lookup, sweep jobs và access checks phải có ngay.

## 11. Seed data strategy

## 11.1 Seed được phép trong project này
- admin bootstrap tối thiểu cho dev/staging;
- sample categories/authors/books;
- sample membership plans;
- sample sellable_skus;
- sample prices;
- sample coupons dev-only.

## 11.2 Seed không nên nhét vào migration production cố định
- dữ liệu nghiệp vụ biến động do business vận hành;
- catalog thật;
- giá thật thay đổi thường xuyên;
- inventory thực tế.

## 12. Rollback strategy

## 12.1 Nguyên tắc
- Không migration nào được coi là “có rollback” chỉ vì có file down nếu data semantics đã đổi.
- Với migration phá schema hoặc drop cột, rollback thực tế có thể là restore từ backup hoặc forward-fix.
- Phải phân biệt rollback kỹ thuật với rollback nghiệp vụ.

## 12.2 Khi nào down migration hữu ích
- tạo bảng mới sai;
- thêm index sai;
- thêm cột nhưng chưa có code dùng;
- dev/staging reset.

## 12.3 Khi nào cần forward-fix hơn rollback
- đã backfill và code mới đã dùng cột mới;
- đã drop cột cũ;
- đã tạo record mới dựa trên semantics mới như `sellable_skus`.

## 13. Validation sau migration

## 13.1 Validation schema
- Tất cả bảng expected đã tồn tại.
- FK đúng direction.
- NOT NULL đúng như ERD revised.
- Index bắt buộc tồn tại.

## 13.2 Validation dữ liệu
- Không `cart_items` nào thiếu `sellable_sku_id`.
- Không `order_items` nào thiếu `sellable_sku_id`.
- Không `order_items` nào thiếu `item_title_snapshot` hoặc `final_unit_price_vnd`.
- Không `inventory_reservations` kiểu `online_payment_pending` nào thiếu `expires_at`.
- Không `downloads` nào trùng `token_id`.

## 13.3 Validation semantics
- COD order không chứa SKU digital/membership.
- `prices` map đúng sang SKU active.
- `inventory_items` chỉ tồn tại cho physical SKU.
- `orders` có snapshot đúng cho shipment/invoice flows.

## 14. Migration pipeline trong CI/CD

## 14.1 CI checks
- lint SQL nếu có tool phù hợp;
- spin up PostgreSQL trống;
- chạy full up migrations;
- chạy smoke seed;
- chạy test integration tối thiểu;
- optionally chạy down/up một số migration mới trong PR.

## 14.2 Deployment checks
- backup/restore readiness;
- migration window assessment;
- app version compatibility note;
- observability cho migration runtime.

## 14.3 Sau deploy
- chạy validation queries;
- check error logs;
- check webhook/payment/order critical flows;
- monitor slow queries nếu có index mới.

## 15. Mẫu query kiểm tra sau migration

```sql
-- order items chưa backfill SKU
SELECT COUNT(*)
FROM order_items
WHERE sellable_sku_id IS NULL;

-- pending online holds thiếu expires_at
SELECT COUNT(*)
FROM inventory_reservations
WHERE hold_type = 'online_payment_pending'
  AND expires_at IS NULL;

-- duplicate token
SELECT token_id, COUNT(*)
FROM downloads
GROUP BY token_id
HAVING COUNT(*) > 1;
```

## 16. Những migration nhạy cảm nhất trong project này

### 16.1 `sellable_skus`
Đây là migration quan trọng nhất vì nó sửa lỗi referential integrity cũ của commerce layer.

### 16.2 `order_items` snapshot upgrade
Nếu thiếu snapshot chuẩn hóa, lịch sử đơn hàng sẽ sai hoặc phụ thuộc vào master data.

### 16.3 `inventory_reservations`
Sai migration ở đây có thể dẫn tới overselling hoặc release hold sai logic.

### 16.4 `outbox_events` / `processed_events`
Sai ở đây sẽ gây duplicate side effects, hoặc event không được phát an toàn.

## 17. Anti-patterns migration phải tránh

- Tạo bảng commerce line item mới nhưng vẫn giữ app logic dùng `sku_type + sku_id` vô thời hạn.
- Chạy backfill trực tiếp trong app startup mà không có kiểm soát.
- Thêm FK/NOT NULL trước khi backfill dữ liệu xong.
- Gộp schema migration với business seed production quá nhiều.
- Drop cột cũ ngay trong release đầu tiên.
- Tạo index quá nặng trong giờ cao điểm mà không có plan.
- Không có query validation sau migration.

## 18. Checklist migration readiness

- Có mapping ERD -> migration plan rõ ràng.
- Có danh sách bảng theo thứ tự tạo.
- Có strategy expand/backfill/enforce/cleanup cho các thay đổi lớn.
- Có seed strategy tách dev/staging/prod.
- Có validation queries sau migration.
- Có CI chạy full migration từ DB rỗng.
- Có ghi chú rollback/forward-fix cho migration nhạy cảm.

## 19. Kết luận áp dụng

Tài liệu migration này không chỉ dùng để viết SQL, mà là để đảm bảo database evolution của project luôn đi đúng với SRD revised, ERD revised và tư duy của một system designer: schema phải phản ánh business truth, migration phải bảo toàn dữ liệu, và mọi thay đổi phải có đường nâng cấp an toàn chứ không chỉ “chạy được trên máy dev”.
