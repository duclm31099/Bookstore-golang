# SRD Review — Phân tích & Hiệu chỉnh

> **Reviewer:** Senior Backend Engineer / System Architect perspective
> **Ngày review:** 2026-05-15
> **Tài liệu gốc:** `1.srd_specs.md`
> **Mục tiêu:** Phát hiện lỗi thiết kế, lỗ hổng bảo mật, anti-pattern, inconsistency và các điểm thiếu specification — giải thích theo tư duy của senior engineer để phát triển khả năng thiết kế hệ thống cho frontend developer muốn trở thành fullstack.

---

## Cách đọc tài liệu này

Mỗi vấn đề được trình bày theo cấu trúc:
- **Vấn đề:** Điều gì sai hoặc thiếu trong SRD.
- **Tại sao nguy hiểm:** Hậu quả thực tế nếu triển khai theo SRD hiện tại.
- **Thiết kế đúng:** Cách senior engineer xử lý.
- **Bài học tư duy:** Nguyên tắc tổng quát rút ra được.

Mức độ nghiêm trọng:
- 🔴 **CRITICAL** — Gây ra security breach hoặc data corruption
- 🟠 **HIGH** — Logic sai, có thể gây inconsistent state hoặc production incident
- 🟡 **MEDIUM** — Design anti-pattern, thiếu spec quan trọng
- 🟢 **MINOR** — Inconsistency, style issue, improvement suggestion

---

## Mục lục

1. [Lỗi bảo mật](#1-lỗi-bảo-mật)
2. [Lỗi State Machine](#2-lỗi-state-machine)
3. [Lỗi thiết kế Database Schema](#3-lỗi-thiết-kế-database-schema)
4. [Lỗi API Design](#4-lỗi-api-design)
5. [Thiếu Specification](#5-thiếu-specification)
6. [Inconsistency giữa SRD và Implementation](#6-inconsistency-giữa-srd-và-implementation)

---

## 1. Lỗi bảo mật

### 🔴 1.1 Webhook endpoint không có yêu cầu signature verification ở Security Requirements

**Vấn đề:**

SRD phần `11.4` định nghĩa endpoint:
```
POST /api/v1/payments/webhooks/{provider}
```

Đây là endpoint public (không cần authentication) nhận callback từ payment gateway. Phần `17.3` Security Requirements chỉ nói:
> "webhook verification secrets phải rotate an toàn"

Không nơi nào trong Security Requirements yêu cầu server **phải verify chữ ký (HMAC signature)** của webhook trước khi xử lý. Chỉ có trong flow description `9.3 Step 8` có nhắc "Hệ thống verify authenticity" — nhưng bị chôn giữa 12 bước, không phải security requirement cứng.

**Tại sao nguy hiểm:**

Không có HMAC verification, bất kỳ ai biết URL này đều có thể gửi HTTP request giả mạo:
```json
POST /api/v1/payments/webhooks/vnpay
{
  "order_id": "ORD-001",
  "status": "captured",
  "amount": 500000
}
```

Server xử lý → Order được mark là `paid` → Entitlement được grant → User có sách mà không trả tiền.

Đây là một trong những lỗ hổng phổ biến và nguy hiểm nhất trong e-commerce backend.

**Thiết kế đúng:**

Thêm vào **Security Requirements (Section 17)** một mục riêng:

```
17.6 Payment Webhook Security

- Tất cả payment provider webhooks phải được verify HMAC signature trước khi 
  đọc bất kỳ field nào trong payload.
- Signature verification phải là bước đầu tiên trong handler, trước validation, 
  trước logging payload.
- Secret key dùng để verify phải lưu trong environment variable, không trong code.
- Request phải bị reject ngay (HTTP 401) nếu signature không hợp lệ.
- Raw payload (chưa parse) phải được dùng để verify — sau khi parse JSON 
  thứ tự key có thể thay đổi và làm sai hash.
- Mỗi provider có cơ chế verify khác nhau:
  - Stripe: X-Stripe-Signature header + Stripe-Signature library
  - VNPay: hash bằng HMAC-SHA512 với secret key
  - MoMo: signature field trong payload + HMAC-SHA256
  - PayPal: Webhook-ID header + PayPal SDK verify
```

**Bài học tư duy:**

> **Quy tắc webhook:** Treat webhook như bạn treat user input — không tin tưởng cho đến khi verify. Một endpoint public không có auth phải bù đắp bằng signature verification. Security requirement quan trọng không được để ở phần flow description — phải là requirement cứng trong Security section.

---

### 🔴 1.2 CSRF vulnerability không được address

**Vấn đề:**

SRD quyết định dùng **HTTP-only cookie** cho refresh token (đây là design đúng để chống XSS). Tuy nhiên, toàn bộ tài liệu không đề cập đến **CSRF (Cross-Site Request Forgery)**.

Cookie được browser tự động gửi kèm theo mọi request, kể cả request từ trang web của kẻ tấn công:

```
1. User đang đăng nhập bookstore.com (có refresh token cookie)
2. User vô tình truy cập evil.com
3. evil.com có form tự submit:
   <form action="https://bookstore.com/api/v1/auth/logout" method="POST">
4. Browser tự gửi POST kèm cookie → User bị logout mà không hay biết
```

Với các endpoint nguy hiểm hơn như `POST /api/v1/orders/{orderId}/cancel` hoặc `DELETE /api/v1/me/sessions/{sessionId}`, hậu quả nghiêm trọng hơn.

**Tại sao nguy hiểm:**

HTTP-only cookie chống được XSS (JS không đọc được cookie) nhưng **không** chống được CSRF (browser vẫn tự gửi cookie). Đây là 2 attack vector độc lập cần biện pháp riêng.

**Thiết kế đúng:**

Thêm vào Security Requirements:

```
17.7 CSRF Protection

Option A (Recommended cho SPA): SameSite=Strict hoặc SameSite=Lax trên cookie
  - SameSite=Strict: Cookie chỉ gửi khi navigate từ cùng domain. 
    Tốt nhất nhưng có thể break legitimate cross-site redirects 
    (ví dụ: sau khi thanh toán xong gateway redirect về bookstore).
  - SameSite=Lax: Cookie gửi khi top-level navigation (GET), 
    không gửi khi cross-site POST/PUT/DELETE. Cân bằng tốt.

Option B (nếu cần hỗ trợ cross-origin): Double Submit Cookie Pattern
  - Server generate CSRF token, set trong cookie thứ 2 (không HttpOnly)
  - Frontend đọc và gửi kèm trong custom header X-CSRF-Token
  - Server verify header == cookie value

Recommendation cho hệ thống này:
  refresh_token cookie: HttpOnly; Secure; SameSite=Lax
  Với MoMo/VNPay redirect về sau thanh toán: dùng SameSite=Lax để 
  browser vẫn gửi cookie khi redirect từ gateway.
```

**Bài học tư duy:**

> **Cookie security có 3 thuộc tính độc lập:** `HttpOnly` (chống XSS đọc cookie), `Secure` (chỉ gửi qua HTTPS), `SameSite` (chống CSRF). Dùng cookie phải set cả 3. Thiếu bất kỳ thuộc tính nào là thiếu một lớp bảo vệ.

---

### 🟠 1.3 Admin RBAC được định nghĩa quá yếu

**Vấn đề:**

Section 17.2:
> "admin endpoints yêu cầu RBAC **và có thể có** finer-grained permissions"

"Có thể có" là tùy chọn — không phải yêu cầu. Hơn nữa, không nơi nào trong SRD định nghĩa:
- Có bao nhiêu admin role?
- Mỗi role có quyền làm gì?
- Ai có thể tạo/xóa admin user?

**Tại sao nguy hiểm:**

Không có role definition thì developer sẽ implement theo cách đơn giản nhất: một flag `is_admin = true/false`. Kết quả là mọi admin có toàn quyền — kể cả nhân viên chỉ cần xem báo cáo bán hàng cũng có thể vô tình (hoặc cố ý) xóa entitlement của user hoặc override payment state.

Principle of Least Privilege (nguyên tắc quyền tối thiểu) bị vi phạm hoàn toàn.

**Thiết kế đúng:**

Thêm section rõ ràng:

```
17.8 Admin Role Matrix

Roles:
- super_admin: Toàn quyền, bao gồm quản lý admin users khác
- catalog_manager: CRUD books, authors, categories, pricing
- order_manager: Xem orders, trigger refund, cập nhật shipment
- finance_manager: Xem payments, reconcile, xem reports
- customer_support: Xem user info, xem orders, có thể lock/unlock user
- report_viewer: Read-only access tới reports và audit logs

Rule:
- Mọi role phải được định nghĩa explicit bằng permission set
- Role assignment phải được audit
- Super admin account phải dùng MFA
- Admin API phải check role trước khi thực thi, không chỉ check "is admin"
```

**Bài học tư duy:**

> **Tư duy RBAC:** "Có admin không?" là câu hỏi quá thô. Câu đúng là: "Actor này có permission X trên resource Y không?" Hệ thống production không bao giờ thiết kế admin là một khối duy nhất.

---

### 🟠 1.4 Payment attempts lưu raw payload — nguy cơ PCI DSS

**Vấn đề:**

Section 13.4:
```
payment_attempts: id, payment_id, attempt_no, state, request_payload, response_payload
```

`request_payload` và `response_payload` là JSONB lưu raw request/response với payment gateway. Một số gateway (đặc biệt các gateway quốc tế) có thể include thông tin nhạy cảm trong response:
- Card BIN (Bank Identification Number)
- Partial card number (ví dụ: `****1234`)
- Cardholder name
- Bank account number fragment

**Tại sao nguy hiểm:**

PCI DSS (Payment Card Industry Data Security Standard) cấm lưu trữ sensitive authentication data. Dù hệ thống có thể không xử lý số thẻ đầy đủ, việc lưu raw payload mà không mask/filter trước là rủi ro compliance.

**Thiết kế đúng:**

Thêm note vào schema:

```sql
-- payment_attempts
request_payload   JSONB  -- PHẢI mask sensitive fields trước khi lưu
response_payload  JSONB  -- PHẢI mask sensitive fields trước khi lưu
                         -- Các field cần filter: card_number, cvv, 
                         --   cardholder_name, account_number, auth_code
```

Và thêm vào Security Requirements:
```
17.9 Payment Data Handling

- Raw gateway payload phải được sanitize (mask sensitive fields) 
  TRƯỚC khi persist vào database.
- Log hệ thống không được chứa payment credentials.
- Chỉ lưu những field cần thiết cho reconciliation và debugging.
- Reference đến PCI DSS SAQ A-EP requirements cho hệ thống redirect-based payment.
```

---

## 2. Lỗi State Machine

### 🔴 2.1 Order state: `chargeback_won → paid` là state regression

**Vấn đề:**

Section 10.1 — Allowed transitions của `chargeback_won`:
```
chargeback_won → paid, fulfilled, partially_refunded
```

Order chỉ có thể vào `chargeback_open` từ `paid` hoặc `fulfilled`. Điều này có nghĩa là:

```
paid → chargeback_open → chargeback_won → paid  ← LOOP!
```

Order đang ở `paid`, có chargeback, merchant thắng → quay về `paid`? Order đã là `paid` từ trước rồi — đây là **state regression** (đi ngược về trạng thái cũ).

Hơn nữa, transition `chargeback_won → paid` tạo ra vòng lặp trạng thái, vi phạm nguyên tắc state machine không có chu trình (acyclic).

**Tại sao nguy hiểm:**

- Audit trail bị rối: Cùng một order lần đầu là `paid`, sau là `chargeback_open`, rồi lại `paid` — không thể biết "đây là lần paid thứ mấy?"
- Code xử lý side effects (grant entitlement, send email confirmation) bị trigger 2 lần
- Reporting financial bị sai: revenue bị tính 2 lần?

**Thiết kế đúng:**

`chargeback_won` nên là **terminal state** (hoặc transition về `fulfilled` nếu đã fulfill rồi):

```
Trước khi chargeback: paid → chargeback_open
Sau khi thắng:        chargeback_open → chargeback_won (TERMINAL)

Chargeback won ý nghĩa: "Merchant thắng tranh chấp, payment được giữ lại"
Không cần quay lại paid vì order vẫn đang được coi là paid trong suốt thời gian tranh chấp.
```

Cập nhật allowed transitions:
```
chargeback_won → none (terminal)
chargeback_lost → refunded (merchant thua → phải hoàn tiền)
```

**Bài học tư duy:**

> **Nguyên tắc state machine:** State machine phải là **DAG (Directed Acyclic Graph)** — không có vòng lặp. Nếu thiết kế state machine có thể đi theo vòng (A→B→C→A), đó là dấu hiệu thiết kế sai. Terminal state là state không có outgoing transition. Chargeback_won là "việc đã xong" — không cần đi tiếp đâu cả.

---

### 🔴 2.2 Order item state machine hoàn toàn bị thiếu

**Vấn đề:**

Section 13.4 định nghĩa:
```sql
order_items: id, order_id, sellable_sku_id, item_state, qty, unit_price_vnd, ...
```

Có column `item_state` nhưng SRD **không có bất kỳ section nào** định nghĩa state machine cho order item. Toàn bộ section 10 chỉ định nghĩa state machine của order header, payment, membership, entitlement, shipment.

**Tại sao nguy hiểm:**

Partial refund — tình huống rất thực tế: User mua 1 ebook + 1 sách vật lý. Sách vật lý giao thành công nhưng ebook bị lỗi → refund chỉ ebook. Không có `item_state`, làm sao hệ thống biết item nào đã refunded, item nào chưa? Làm sao tính `partially_refunded` cho order header?

```
Không có item_state → partial refund logic phải tự suy diễn → race condition → 
user có thể request refund cùng 1 item 2 lần → double refund.
```

**Thiết kế đúng:**

Thêm state machine cho `order_items`:

```
order_item_state:
- pending        → item mới tạo, chờ payment
- fulfilled      → physical: đã giao / digital: entitlement đã grant
- refund_pending → đã submit refund request
- refunded       → đã hoàn tiền hoàn toàn (terminal)
- cancelled      → bị cancel trước fulfill (terminal)

Allowed transitions:
pending       → fulfilled, cancelled, refund_pending
fulfilled     → refund_pending
refund_pending → refunded
```

Order header state `partially_refunded` được **derived** từ item states: nếu có ít nhất 1 item `refunded` và ít nhất 1 item `fulfilled` → order là `partially_refunded`.

**Bài học tư duy:**

> **Quy tắc aggregate:** Trong e-commerce, order là một **aggregate phức tạp** gồm header + items. State của header và state của items phải được định nghĩa độc lập và có quan hệ rõ ràng. Header state có thể là **derived** từ item states, hoặc **authoritative** với items theo sau. Phải chọn 1 trong 2 và nhất quán.

---

### 🟠 2.3 Membership `renewed` state là unnecessary transient

**Vấn đề:**

Section 10.4 Membership states:
```
expired → renewed → active
```

State `renewed` là transient (chuyển tiếp ngay lập tức sang `active`) không mang ý nghĩa nghiệp vụ nào độc lập. Nó không có:
- Hành động cụ thể nào được trigger ở `renewed`
- Timeout hay condition để ở lại `renewed`
- Sự khác biệt về quyền của user ở state `renewed` so với `active`

**Tại sao nguy hiểm:**

State machine với transient state vô nghĩa làm phức tạp code:
```go
// Developer phải xử lý thêm 1 case:
switch membership.State {
case "expired":   // ...
case "renewed":   // Làm gì ở đây? Không rõ
case "active":    // ...
}
```

Nếu scheduler chạy từng bước: `expired → renewed → active`, có một khoảng thời gian ngắn membership ở `renewed` — user không thể đọc sách trong khoảng đó không?

**Thiết kế đúng:**

Bỏ `renewed` state. Khi renew membership:
- **Nếu cùng subscription**: Update `expires_at` và state về `active` trong 1 transaction
- **Nếu subscription mới**: Tạo subscription mới với state `active` từ đầu

```
expired → active  (direct, via renew payment)
```

Nếu muốn audit trail về lần renew, dùng `membership_state_logs` thay vì dùng state trung gian.

**Bài học tư duy:**

> **Quy tắc state machine:** Mỗi state phải có ít nhất 1 trong 3: (1) quyền/hạn chế khác với state khác, (2) hành động cụ thể trigger khi enter/exit, (3) timeout condition. Nếu không có cái nào → state đó không cần tồn tại.

---

### 🟠 2.4 Membership `grace` period được define nhưng không có business rules

**Vấn đề:**

Section 10.4 liệt kê state `grace` với mô tả "tùy chọn grace period" nhưng **không có business rule nào** định nghĩa:
- Grace period kéo dài bao lâu? (7 ngày? 3 ngày?)
- User được làm gì trong grace period? (đọc sách được không? tải được không?)
- Ai trigger `active → grace`? Scheduler?
- Nếu user gia hạn trong grace period → `grace → active`? Hay `grace → renewed → active`?
- Nếu membership đang `grace`, stacking rule trong 8.2.7 áp dụng như thế nào?

**Thiết kế đúng:**

Thêm vào Business Rules (Section 8.2):

```
8.2.8 Grace Period Rules

- Grace period kéo dài X ngày sau expires_at (quyết định business: 3-7 ngày)
- Trong grace period: user giữ nguyên quyền đọc nhưng KHÔNG được cấp link tải mới
- Scheduler trigger active → grace khi expires_at <= NOW() và grace_period chưa hết
- Scheduler trigger grace → expired khi expires_at + grace_period <= NOW()
- Nếu user renew trong grace: grace → active (direct, không qua renewed)
- Membership stacking khi đang grace: end date mới tính từ expires_at gốc (không phải từ NOW())
```

**Bài học tư duy:**

> **Quy tắc documentation:** Mọi state trong state machine phải có đủ thông tin để implement mà không cần hỏi lại. Nếu developer đọc spec mà phải đoán behavior, spec đó chưa đủ. "Tùy chọn grace period" là product decision, không phải kỹ thuật — phải được product owner quyết định và ghi rõ.

---

### 🟠 2.5 Shipment `returned` → không có flow restocking inventory

**Vấn đề:**

Section 10.6, sau khi shipment đạt state `returned` (hàng về lại kho), allowed transitions chỉ là:
```
returned → cancelled, refund_pending
```

Không có bất kỳ mention nào về việc **cộng lại tồn kho** khi hàng được return về.

**Tại sao nguy hiểm:**

```
Kịch bản thực tế:
1. User đặt mua 5 quyển sách, inventory: on_hand=10, reserved=5, available=5
2. Shipment giao thất bại → returning → returned
3. Hàng về kho vật lý
4. Nhưng DB vẫn: on_hand=10, reserved=0 (đã release), available=5

Câu hỏi: 5 quyển sách đó ở đâu trong DB? on_hand vẫn là 10 hay phải là 10 không đổi vì chưa giao?
```

Thực ra vấn đề phức tạp hơn: cần phân biệt "hàng đã xuất kho" (on_hand giảm) hay "hàng chưa xuất kho" (on_hand không đổi, chỉ reserved).

**Thiết kế đúng:**

Thêm transition `returned` + inventory flow:

```
8.7.6 Inventory và Return Flow

Khi shipment chuyển về `returned`:
- Nếu hàng đã được xuất kho (on_hand đã giảm tại thời điểm packed): 
  inventory_items.on_hand += qty_returned
  Tạo inventory_adjustment record với reason="return_to_stock"
- Nếu hàng chưa xuất kho: không cần điều chỉnh on_hand
- Trong cả 2 trường hợp: reservation đã được release trước đó (tại payment_failed/cancelled)

Trigger: Admin confirm "hàng về kho và còn tình trạng tốt" 
  → POST /api/v1/admin/shipments/{id}/confirm-return
  → Inventory adjustment
  → Shipment state: returned (đã ở đây rồi, không đổi)
  → Tạo AuditLog
```

---

### 🟡 2.6 Shipment `delivery_failed → in_transit`: ai trigger, điều kiện gì?

**Vấn đề:**

Section 10.6 cho phép:
```
delivery_failed → in_transit
```

Nhưng không có bất kỳ specification nào về:
- Ai trigger transition này? Carrier webhook? Admin?
- Điều kiện gì? (ví dụ: carrier phải xác nhận re-delivery attempt)
- Giới hạn bao nhiêu lần? (không thể delivery_failed → in_transit → delivery_failed vô hạn)

**Thiết kế đúng:**

```
Transition delivery_failed → in_transit:
- Trigger: Carrier webhook xác nhận re-delivery attempt, hoặc Admin action
- Điều kiện: Chỉ cho phép nếu shipment chưa quá max_delivery_attempts (ví dụ: 3)
- Nếu vượt quá max_attempts: bắt buộc chuyển sang returning hoặc cancelled
- Mỗi re-delivery attempt phải ghi log vào shipment_status_logs
```

---

## 3. Lỗi thiết kế Database Schema

### 🔴 3.1 `inventory_items.available` là computed field nhưng được lưu như regular column

**Vấn đề:**

Section 13.5:
```
inventory_items: id, sellable_sku_id, on_hand, reserved, available, version
```

`available = on_hand - reserved` là phép tính đơn giản. Nhưng SRD lưu cả 3 column — nghĩa là khi tăng `reserved`, code phải update cả `reserved` lẫn `available`. Nếu update một trong hai mà quên cái kia (hoặc transaction rollback ở giữa):

```
on_hand = 100, reserved = 60, available = 45  ← INCONSISTENT!
```

**Tại sao nguy hiểm:**

Data inconsistency trong inventory là thảm họa e-commerce: overselling (bán nhiều hơn có), underselling (từ chối đơn dù còn hàng), financial audit sai.

**Thiết kế đúng:**

**Option A (Recommended):** Dùng PostgreSQL `GENERATED ALWAYS AS` computed column:
```sql
CREATE TABLE inventory_items (
    id              BIGSERIAL PRIMARY KEY,
    sellable_sku_id BIGINT NOT NULL,
    on_hand         BIGINT NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
    reserved        BIGINT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    available       BIGINT GENERATED ALWAYS AS (on_hand - reserved) STORED,
    version         BIGINT NOT NULL DEFAULT 1,
    CHECK (reserved <= on_hand)  -- invariant: không thể reserve nhiều hơn có
);
```

DB tự tính `available` — không bao giờ inconsistent.

**Option B:** Không lưu `available`, query lúc cần:
```sql
SELECT on_hand - reserved AS available FROM inventory_items WHERE id = $1;
```

**Bài học tư duy:**

> **Derived data là nguồn gốc của inconsistency.** Mọi khi bạn thấy column C = f(column A, column B), hỏi ngay: "Ai chịu trách nhiệm giữ C đồng bộ?" Nếu câu trả lời là "application code", thì đó là bug đang chờ xảy ra. Database là nơi tốt nhất để enforce invariant này.

---

### 🔴 3.2 `inventory_reservations` thiếu `order_item_id`

**Vấn đề:**

Section 13.5:
```
inventory_reservations: id, order_id, sellable_sku_id, qty, reservation_state, hold_type, expires_at
```

Chỉ có `order_id` không đủ. Một order có thể có nhiều order_item với cùng `sellable_sku_id` (dù hiếm), hoặc quan trọng hơn: cần biết reservation nào đang giữ cho item nào để:
- Release đúng reservation khi cancel từng item
- Partial fulfillment: release reservation của item đã giao, giữ reservation của item chưa giao
- Audit: "Tại sao item này bị reserve nhưng chưa giải phóng?"

**Thiết kế đúng:**

```sql
inventory_reservations (
    id                BIGSERIAL PRIMARY KEY,
    order_id          BIGINT NOT NULL,
    order_item_id     BIGINT NOT NULL,  -- ← THÊM VÀO
    sellable_sku_id   BIGINT NOT NULL,
    qty               INT    NOT NULL CHECK (qty > 0),
    reservation_state VARCHAR(30) NOT NULL,
    hold_type         VARCHAR(30),  -- 'online_payment_hold' | 'cod_hold'
    expires_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

### 🔴 3.3 `sellable_skus` vẫn dùng polymorphic nullable FK — cùng anti-pattern mà SRD nói đã fix

**Vấn đề:**

SRD tự hào rằng đã fix anti-pattern polymorphic reference bằng cách thêm `sellable_skus`:

> Section 13.4.1: "cart_items và order_items không còn dùng cặp tham chiếu polymorphic sku_type + sku_id"

Nhưng nhìn vào chính bảng `sellable_skus` (Section 13.2):
```
sellable_skus: id, sku_code, sku_type, book_id, membership_plan_id, format_type, active
```

`book_id` và `membership_plan_id` là hai nullable FK — chính xác là polymorphic nullable FK pattern:
- Nếu `sku_type = 'physical'` → `book_id` có giá trị, `membership_plan_id` NULL
- Nếu `sku_type = 'ebook'` → `book_id` có giá trị, `membership_plan_id` NULL
- Nếu `sku_type = 'membership'` → `book_id` NULL, `membership_plan_id` có giá trị

**Tại sao nguy hiểm:**

```sql
-- Có thể tạo record không hợp lệ:
INSERT INTO sellable_skus (sku_type, book_id, membership_plan_id) 
VALUES ('physical', NULL, 42);  -- physical SKU không có book_id???
-- DB không reject điều này vì không có constraint
```

Không có DB-level enforcement nào ngăn data không hợp lệ.

**Thiết kế đúng:**

**Option A:** Tách thành bảng riêng:
```sql
book_skus       (id, book_id NOT NULL, format_type NOT NULL, sku_code, active)
membership_skus (id, membership_plan_id NOT NULL, sku_code, active)
```

Thêm table `sellable_skus` như view/union hoặc bảng bridge.

**Option B:** Dùng CHECK constraint nếu giữ 1 bảng:
```sql
ALTER TABLE sellable_skus ADD CONSTRAINT chk_sku_type_fk CHECK (
    (sku_type IN ('physical', 'ebook') AND book_id IS NOT NULL AND membership_plan_id IS NULL)
    OR
    (sku_type = 'membership' AND membership_plan_id IS NOT NULL AND book_id IS NULL)
);
```

**Bài học tư duy:**

> **Polymorphic association** là design smell phổ biến trong các hệ thống ORM-first. Nó hy sinh **referential integrity** để đổi lấy sự tiện lợi. Database không thể tạo foreign key constraint trên nullable polymorphic FK. Giải pháp đúng: hoặc tách bảng, hoặc dùng CHECK constraint. Đừng để data integrity phụ thuộc vào application code — DB enforcement là bền vững hơn.

---

### 🟠 3.4 `entitlements` vẫn dùng `source_type + source_id` polymorphic association

**Vấn đề:**

Section 13.3:
```
entitlements: id, user_id, book_id, source_type, source_id, state, starts_at, expires_at
```

`source_type` có thể là `'order_item'`, `'membership'`, hoặc `'admin_grant'`. `source_id` trỏ đến ID của bảng tương ứng. Đây là polymorphic association mà SRD đã tuyên bố fix ở cart_items/order_items.

**Thiết kế đúng:**

```sql
entitlements (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    book_id          BIGINT NOT NULL,
    -- Explicit nullable FKs thay vì polymorphic:
    order_item_id    BIGINT REFERENCES order_items(id),     -- NULL nếu từ membership hoặc admin
    membership_id    BIGINT REFERENCES memberships(id),     -- NULL nếu từ purchase hoặc admin
    admin_grant_id   BIGINT REFERENCES audit_logs(id),      -- NULL nếu từ purchase hoặc membership
    -- Chỉ một trong ba ở trên được phép NOT NULL:
    state            VARCHAR(30) NOT NULL,
    starts_at        TIMESTAMPTZ NOT NULL,
    expires_at       TIMESTAMPTZ,  -- NULL = permanent (purchase-based)
    CONSTRAINT chk_entitlement_source CHECK (
        (order_item_id IS NOT NULL AND membership_id IS NULL AND admin_grant_id IS NULL) OR
        (order_item_id IS NULL AND membership_id IS NOT NULL AND admin_grant_id IS NULL) OR
        (order_item_id IS NULL AND membership_id IS NULL AND admin_grant_id IS NOT NULL)
    )
);
```

---

### 🟠 3.5 `prices` table không enforce "chỉ 1 active price per SKU"

**Vấn đề:**

Section 13.2:
```
prices: id, sellable_sku_id, list_price_vnd, sale_price_vnd, starts_at, ends_at
```

Không có constraint nào đảm bảo tại một thời điểm, 1 SKU chỉ có 1 giá active. Có thể tạo 2 records với overlapping time range cho cùng SKU:

```
Record 1: sku_id=1, starts_at=2026-01-01, ends_at=2026-12-31, price=100k
Record 2: sku_id=1, starts_at=2026-06-01, ends_at=2027-06-30, price=80k
```

Tháng 7/2026, query hiện tại của cái nào? Hệ thống phải tự quyết định bằng ORDER BY và LIMIT 1 — nhưng nếu developer quên thì sao?

**Thiết kế đúng:**

**Option A:** Partial unique index (Postgres-specific):
```sql
-- Chỉ 1 price với ends_at IS NULL (permanent price) per SKU
CREATE UNIQUE INDEX idx_prices_active_permanent 
ON prices(sellable_sku_id) WHERE ends_at IS NULL;
```

**Option B:** Trigger kiểm tra overlap trước INSERT:
```sql
-- Khi insert price mới, check không có overlap với record cũ
```

**Option C (Simple):** Thay vì time-bounded pricing table, dùng `current_price_vnd` trên `sellable_skus` và dùng `price_history` table chỉ để audit.

---

### 🟡 3.6 `refunds` table thiếu timestamps quan trọng

**Vấn đề:**

Section 13.4:
```
refunds: id, order_id, payment_id, amount_vnd, state, reason_code
```

Để track refund SLA (Service Level Agreement) và reconciliation với gateway, cần thêm:
- `requested_at` — Khi nào refund request được tạo?
- `submitted_to_gateway_at` — Khi nào gửi cho payment provider?
- `gateway_ref` — Provider reference cho refund transaction
- `completed_at` — Khi nào gateway confirm hoàn tất?
- `requested_by_type` / `requested_by_id` — Admin hay automated rule?

Không có timestamps → không thể track "đã request refund 3 ngày rồi mà gateway chưa confirm" → customer support không có dữ liệu để resolve ticket.

---

### 🟡 3.7 `coupons` thiếu `per_user_limit` và `min_order_amount_vnd`

**Vấn đề:**

Section 13.4:
```
coupons: id, code, type, value_vnd, usage_limit, starts_at, ends_at
```

`usage_limit` là tổng số lần coupon được dùng. Nhưng thiếu:

1. **`per_user_limit`**: User A có thể sử dụng cùng 1 coupon 50 lần nếu không có giới hạn per-user, exhaust toàn bộ quota của coupon và không ai khác dùng được.

2. **`min_order_amount_vnd`**: Coupon giảm 50k nhưng không có điều kiện tối thiểu → user đặt đơn 1k để lấy discount 50k. Hầu hết e-commerce đều có minimum order value.

**Thiết kế đúng:**

```sql
coupons (
    id                    BIGSERIAL PRIMARY KEY,
    code                  VARCHAR(50) NOT NULL UNIQUE,
    type                  VARCHAR(20) NOT NULL,  -- 'fixed' | 'percent'
    value                 BIGINT NOT NULL,        -- VND hoặc percentage * 100
    max_discount_vnd      BIGINT,                 -- Cap cho percent coupon
    usage_limit           INT,                    -- NULL = unlimited
    per_user_limit        INT NOT NULL DEFAULT 1, -- ← THÊM VÀO
    min_order_amount_vnd  BIGINT NOT NULL DEFAULT 0,  -- ← THÊM VÀO
    starts_at             TIMESTAMPTZ NOT NULL,
    ends_at               TIMESTAMPTZ,
    active                BOOLEAN NOT NULL DEFAULT true
);
```

---

## 4. Lỗi API Design

### 🔴 4.1 `GET /api/v1/books/{bookId}/read` dùng GET để tạo side effects

**Vấn đề:**

Section 11.6:
```
GET /api/v1/books/{bookId}/read
```

Theo HTTP specification (RFC 9110):
- `GET` phải là **safe** (không có side effects) và **idempotent**
- Gọi GET 10 lần phải có kết quả như gọi 1 lần và không thay đổi server state

Nhưng "đọc sách" cần tạo `reader_session` record, enforce concurrent session limit, track reading position — tất cả đều là **side effects**. Đây là thao tác tạo/mở phiên đọc, không phải GET resource.

**Tại sao nguy hiểm:**

Browser và CDN có thể cache GET request. Nếu request này bị cache → reader session không được tạo → entitlement check bị bỏ qua → user đọc sách mà không có quyền.

**Thiết kế đúng:**

Tách thành 2 endpoint rõ ràng:
```
POST /api/v1/reader/sessions              ← Tạo reader session, trả về session token
                                             Body: { book_id, format, device_id }
                                             Response: { session_id, content_url, token }

GET /api/v1/reader/sessions/{sessionId}/content  ← Lấy content URL (idempotent)
```

---

### 🟠 4.2 `POST /api/v1/cart/coupon` nên là `PUT`

**Vấn đề:**

Cart chỉ có 1 coupon tại một thời điểm. "Đặt coupon cho cart" là thao tác idempotent replacement:
- Gọi lần 1: coupon = "SALE10"
- Gọi lần 2 với cùng body: coupon vẫn là "SALE10" (không tạo thêm)
- Gọi lần 3 với coupon khác: coupon = "HOLIDAY20" (replace)

Đây là ngữ nghĩa của `PUT`, không phải `POST`:
- `POST`: tạo resource mới (tạo thêm coupon?)
- `PUT`: replace/upsert resource tại URI đó

**Thiết kế đúng:**
```
PUT  /api/v1/cart/coupon     ← Set (replace) coupon code
DELETE /api/v1/cart/coupon   ← Remove coupon
GET /api/v1/cart             ← Cart đã có coupon info rồi
```

---

### 🟠 4.3 `POST /api/v1/orders/{orderId}/pay` thiếu request body specification

**Vấn đề:**

Section 11.4 chỉ list endpoint:
```
POST /api/v1/orders/{orderId}/pay
```

Nhưng không specify request body. Endpoint này cần biết:
- Payment method nào? (stripe, vnpay, momo, paypal)
- Return URL sau khi hoàn thành (để redirect về)
- Với Stripe: payment_intent hay payment_method?

**Thiết kế đúng:**

Phải rename và specify body:
```
POST /api/v1/orders/{orderId}/payment

Request Body:
{
  "payment_method": "vnpay",    // stripe | vnpay | momo | paypal
  "return_url": "https://...",  // URL redirect sau khi xong
  "device_id": "optional"       // Để track payment device
}

Response:
{
  "payment_id": 123,
  "redirect_url": "https://gateway.vnpay.vn/...",  // Redirect user đến đây
  "expires_at": "2026-05-15T08:30:00Z"              // Deadline cho payment
}
```

---

### 🟠 4.4 Admin inventory endpoint quá permissive

**Vấn đề:**

Section 12.4:
```
PATCH /api/v1/admin/inventory/{skuId}
```

Không rõ field nào được patch. Nếu admin có thể trực tiếp SET `on_hand`, `reserved`, `available` thì:
- Bypass toàn bộ reservation system
- Không có audit trail về "ai thay đổi, tại sao"
- Có thể set `available = 1000` khi `on_hand = 0`

**Thiết kế đúng:**

Tách thành operations có ý nghĩa rõ ràng, mỗi cái có audit trail:
```
POST /api/v1/admin/inventory/{skuId}/adjust
Body: { "delta": +50, "reason": "Nhập hàng mới", "note": "PO-2026-001" }
→ on_hand += 50, tạo audit log

POST /api/v1/admin/inventory/{skuId}/adjust  
Body: { "delta": -5, "reason": "Hàng lỗi loại bỏ", "note": "..." }
→ on_hand -= 5, tạo audit log
```

Admin KHÔNG được phép set `reserved` hoặc `available` trực tiếp — đây là derived values.

---

### 🟡 4.5 `POST /api/v1/admin/shipments/{shipmentId}/status` nên là `PATCH`

**Vấn đề:**

Cập nhật trạng thái một resource đang tồn tại là `PATCH` operation (partial update), không phải `POST` (create). Đây là vấn đề REST semantic consistency.

SRD nên nhất quán: dùng `PATCH` cho state update, dùng `POST` cho action execution có tên rõ ràng:
```
PATCH /api/v1/admin/shipments/{shipmentId}        ← Update fields
POST  /api/v1/admin/shipments/{shipmentId}/pack   ← Explicit state transition
POST  /api/v1/admin/shipments/{shipmentId}/pickup ← Explicit state transition
```

---

### 🟡 4.6 SRD thiếu `POST /api/v1/auth/change-password`

**Vấn đề:**

Endpoint này đã được implement đầy đủ trong codebase (auth_handler.go) với flow chi tiết (revoke sessions, JTI blacklist) nhưng **không có trong API surface section 11** của SRD.

SRD đang **lagging behind** implementation — nguy hiểm vì:
- Onboarding developer mới đọc SRD sẽ không biết endpoint này tồn tại
- QA test theo SRD sẽ miss endpoint này

**Thiết kế đúng:** Thêm vào Section 11.1:
```
POST /api/v1/me/change-password   ← Đổi mật khẩu (Strict Auth required)
```

---

## 5. Thiếu Specification

### 🔴 5.1 Forgot-password / Reset-password flow không có sequence và security rules

**Vấn đề:**

Section 11.1 list 2 endpoints:
```
POST /api/v1/auth/forgot-password
POST /api/v1/auth/reset-password
```

Nhưng **không có flow nào** (section 9) mô tả cách chúng hoạt động. Không có security requirements cho reset token. Đây là một trong những flow bảo mật nhạy cảm nhất trong hệ thống.

**Điều cần specify:**

```
9.7 Forgot-password / Reset-password Flow

1. User gửi email tới POST /auth/forgot-password
2. Server kiểm tra email tồn tại không (không được tiết lộ "email không tồn tại" → enumeration attack)
3. Nếu tồn tại: tạo reset token an toàn (cryptographically random, 32 bytes entropy)
4. Lưu hash(token) vào DB với expires_at = NOW() + 15 phút
5. Gửi link chứa token qua email
6. Luôn trả HTTP 200 kể cả email không tồn tại (chống enumeration)
7. User click link → gửi token tới POST /auth/reset-password kèm new_password
8. Server verify: token tồn tại, chưa expire, chưa dùng
9. Update password, đánh dấu token đã dùng (single-use)
10. Revoke toàn bộ sessions cũ (password changed → force re-login mọi nơi)
11. Ghi audit log

Security Rules:
- Reset token phải single-use (dùng GetDel hoặc soft-delete)
- TTL tối đa 15 phút
- Không được dùng sequential ID làm token
- Response time phải constant (tránh timing attack phân biệt email tồn tại/không tồn tại)
- Rate limit cho forgot-password endpoint: max 3 requests / email / giờ
```

**Bài học tư duy:**

> **Enumeration attack:** Nếu server trả "Email không tồn tại" với forgot-password, attacker có thể dùng endpoint này để kiểm tra xem email nào đã đăng ký hệ thống. Luôn trả 200 OK và thông điệp chung chung: "Nếu email tồn tại, bạn sẽ nhận được link reset trong vài phút."

---

### 🟠 5.2 Download quota design hoàn toàn chưa được specify

**Vấn đề:**

Section 13.3 có:
```
membership_plans: ..., max_downloads_total, ...
```

Và Section 9.5 nói "Hệ thống kiểm tra quota" nhưng không define:
- `max_downloads_total` là tổng lifetime hay per-period (mỗi tháng reset)?
- Quota áp dụng per-book (mỗi book tối đa N lần tải) hay total (tổng tất cả book N lần)?
- Cùng format tính riêng hay chung (PDF + EPUB = 2 lần hay 1 lần)?
- Non-membership purchase có quota riêng không?

**Thiết kế đúng:** Phải có sub-section rõ ràng:

```
8.9 Download Quota Rules

- max_downloads_per_book: Số lần tải tối đa per book, per membership period
  (ví dụ: 3 lần/book/tháng với monthly plan)
- Quota reset theo membership renewal/billing cycle
- PDF và EPUB tính tách biệt (tải cả hai = 2 lần)
- Khi hết quota: trả lỗi với remaining_count=0 và next_reset_at
- Purchase-based entitlement: không có quota limit (permanent)
- Track bằng: đếm records trong `downloads` table với status=completed 
  trong khoảng thời gian của period
```

---

### 🟠 5.3 Kafka `scheduler.commands.v1` — Kafka không hỗ trợ delayed delivery

**Vấn đề:**

Section 14.2 list topic `scheduler.commands.v1` với mục đích "delayed / periodic commands". Nhưng **Kafka không có cơ chế delay message native**. Khi producer gửi message, consumer nhận ngay lập tức (theo offset order).

Ví dụ dùng case: "Sau 30 phút kể từ khi payment pending, nếu chưa có callback → release reservation". Không thể dùng Kafka để "delay 30 phút rồi mới consume".

**Tại sao nguy hiểm:**

Developer đọc spec này có thể implement sai: publish message lên `scheduler.commands.v1` và expect consumer chạy sau 30 phút → consumer chạy ngay lập tức → reservation bị release sớm trước khi user kịp thanh toán.

**Thiết kế đúng:**

Thay vì Kafka-based delayed scheduling, dùng **DB-based scheduler**:

```sql
-- Bảng scheduled_jobs thay vì Kafka topic
CREATE TABLE scheduled_jobs (
    id           BIGSERIAL PRIMARY KEY,
    job_type     VARCHAR(50) NOT NULL,
    payload      JSONB NOT NULL,
    scheduled_at TIMESTAMPTZ NOT NULL,  -- Thời điểm execute
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Worker quét bảng này theo interval, chỉ execute jobs có `scheduled_at <= NOW()`. Sau khi execute xong, có thể publish Kafka event để notify các module khác.

Kafka `scheduler.commands.v1` vẫn có thể dùng nhưng chỉ cho **immediate** commands từ scheduler → các service, không phải delayed delivery.

**Bài học tư duy:**

> **Hiểu đúng tool's capability:** Kafka là distributed log — FIFO, high throughput, persistent. Nó không phải message queue hỗ trợ delayed delivery (cái đó là RabbitMQ với DLX TTL, hoặc AWS SQS với delay queues). Dùng sai tool là nguồn gốc của bug khó tìm.

---

### 🟠 5.4 User events topic không tồn tại trong Topic Catalog

**Vấn đề:**

Section 14.3 list events:
```
user.registered   → Produced by: Identity → Consumed by: Notification, Audit
user.email_verified → Produced by: Identity → Consumed by: Audit
```

Nhưng Section 14.2 (Topics) không có topic nào cho user events! `user.registered` và `user.email_verified` sẽ được publish lên topic nào?

Nếu publish lên `notification.commands.v1` — đó là "commands" topic (instruction to do something), nhưng `user.registered` là "event" (fact that happened). Conflating events và commands là anti-pattern trong event-driven systems.

**Thiết kế đúng:**

Thêm topic vào Section 14.2:
```
| user.events.v1 | user lifecycle events | user_id |
```

Và cập nhật 14.3:
```
user.registered     → Topic: user.events.v1 → Consumed by: Notification, Audit
user.email_verified → Topic: user.events.v1 → Consumed by: Audit
```

**Bài học tư duy:**

> **Event vs Command naming:** Events (sự kiện đã xảy ra) dùng past tense: `user.registered`, `order.paid`. Commands (yêu cầu làm gì đó) dùng imperative: `send_email`, `release_reservation`. Tách chúng ra topic khác nhau vì routing logic khác nhau.

---

### 🟠 5.5 Pagination hoàn toàn không được specify

**Vấn đề:**

Tất cả list endpoints (`GET /api/v1/books`, `GET /api/v1/orders`, `GET /api/v1/admin/users`) không có bất kỳ specification nào về pagination. Với 100,000 books trong catalog, query không có LIMIT sẽ trả về toàn bộ trong 1 response → OOM, timeout, client crash.

**Thiết kế đúng:**

Thêm vào Section 11.8:

```
11.9 Pagination Standard

Tất cả collection endpoints phải hỗ trợ pagination:

Offset-based (cho admin endpoints với sorting phức tạp):
  GET /api/v1/admin/orders?page=1&per_page=20&sort=created_at&order=desc
  Response:
  {
    "data": [...],
    "pagination": {
      "page": 1, "per_page": 20, "total": 1500, "total_pages": 75
    }
  }

Cursor-based (cho public endpoints và real-time feeds):
  GET /api/v1/books?cursor=eyJpZCI6MTAwfQ&limit=20
  Response:
  {
    "data": [...],
    "next_cursor": "eyJpZCI6MTIwfQ",
    "has_more": true
  }

Default per_page: 20. Maximum per_page: 100.
```

**Bài học tư duy:**

> **Cursor vs Offset pagination:** Offset pagination (`LIMIT 20 OFFSET 100`) đơn giản nhưng có 2 vấn đề: (1) Performance tệ với large offset vì DB vẫn đọc 100 rows rồi bỏ đi. (2) Nếu có insert mới trong lúc user đang phân trang, trang 2 có thể lặp lại item từ trang 1. Cursor-based pagination (`WHERE id > last_seen_id`) giải quyết cả 2 vấn đề.

---

### 🟡 5.6 Admin role definitions hoàn toàn thiếu

**Vấn đề:**

SRD có admin actor, admin API, admin override — nhưng **không có bất kỳ chỗ nào định nghĩa các admin roles là gì, ai có quyền làm gì**. Developer implement RBAC sẽ không có baseline để implement đúng.

Cần thêm:
- Danh sách admin roles
- Permission matrix cho từng role
- Quy trình cấp/thu hồi admin access
- Ai có thể cấp super_admin?

---

### 🟡 5.7 Flash-sale guard mechanism không được specify

**Vấn đề:**

SRD nhắc đến "flash-sale stock guard" dùng Redis nhưng không specify:
- Mechanism là gì? (Redis DECR atomic? Lua script? WATCH/MULTI/EXEC?)
- Khi Redis counter và DB inventory diverge thì xử lý sao?
- Flash-sale guard có cần initializing từ DB khi Redis restart không?

**Thiết kế đúng phải specify:**

```
15.5 Flash-sale Guard Mechanism

1. Khi flash-sale bắt đầu: seed Redis counter
   SET flashsale:stock:{sku_id} <qty> EX <ttl>
   
2. Khi user checkout:
   counter = DECR flashsale:stock:{sku_id}
   IF counter < 0:
       INCR flashsale:stock:{sku_id}  -- Rollback
       Return HTTP 422: "Hết hàng flash-sale"
   ELSE:
       Tiếp tục checkout flow bình thường

3. Nếu checkout thất bại sau DECR:
   INCR flashsale:stock:{sku_id}  -- Compensation

4. Source of truth vẫn là inventory_items.available trong PostgreSQL
   Redis counter chỉ là fast pre-check để giảm tải DB

5. Sau flash-sale hoặc Redis restart: 
   Re-compute counter từ DB: 
   counter = inventory_items.available - count(active_reservations)
```

---

## 6. Inconsistency giữa SRD và Implementation

### 🟠 6.1 `addresses` table trong SRD khác implementation

**Vấn đề:**

SRD Section 13.1 list key columns của `addresses`:
```
id, user_id, full_name, phone, province, district, ward, line1, line2
```

Nhưng identity module implementation (và migration SQL) không có `full_name` và `phone` trên `addresses`.

**Phân tích:**

Với shipping address, cần phân biệt 2 loại thông tin:
- Thông tin người nhận (có thể khác người đặt): `recipient_name`, `recipient_phone`
- Địa chỉ địa lý: province, district, ward, line1, line2

User A có thể đặt hàng giao cho User B. User A dùng account của mình nhưng điền tên và SĐT của B vào địa chỉ giao hàng.

**Thiết kế đúng:**

```sql
addresses (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    recipient_name   VARCHAR(255) NOT NULL,   -- ← THÊM: tên người nhận
    recipient_phone  VARCHAR(20) NOT NULL,    -- ← THÊM: SĐT người nhận
    address_line1    VARCHAR(255) NOT NULL,
    address_line2    VARCHAR(255),
    province_code    VARCHAR(20) NOT NULL,
    district_code    VARCHAR(20) NOT NULL,
    ward_code        VARCHAR(20) NOT NULL,
    is_default       BOOLEAN NOT NULL DEFAULT false,
    -- NOTE: Địa chỉ cũng cần snapshot vào order khi đặt hàng
    -- vì user có thể edit address sau đó
);
```

Và `orders.shipping_address_snapshot` phải include cả `recipient_name` và `recipient_phone`.

---

### 🟡 6.2 Redis key pattern inconsistency giữa SRD và implementation

**Vấn đề:**

SRD Section 15.3:
```
session:{session_id}    ← SRD pattern
```

Actual implementation trong `redis_service.go`:
```
identity:session:{sessionID}    ← Implementation pattern
```

Tương tự:
- SRD: `quota:download:{user_id}:{book_id}`
- Implementation có thể dùng prefix khác

**Tại sao quan trọng:**

Nếu developer khác implement feature mới dựa trên SRD pattern (`session:{id}`) trong khi existing code dùng `identity:session:{id}`, sẽ có 2 key patterns cùng tồn tại trong Redis → bugs khó debug.

**Thiết kế đúng:**

SRD phải cập nhật để match implementation, và tất cả Redis key phải có **namespace prefix** theo module:

```
identity:session:{session_id}
identity:email_verify:{token}
catalog:book:{book_id}
cart:user:{user_id}
idem:{scope}:{key}
blacklist:access_token:{jti}
```

Thêm vào SRD:
```
15.6 Key Naming Convention

Format: {module}:{resource_type}:{identifier}
- Module prefix bắt buộc để tránh key collision giữa modules
- Tất cả key phải được document trong Redis spec với TTL rõ ràng
- Thay đổi key pattern phải update tất cả code liên quan (không để 2 pattern cùng tồn tại)
```

---

## Tổng kết và ưu tiên xử lý

### Critical — Fix ngay trước khi implement:

| # | Vấn đề | Section | Tác động |
|---|---|---|---|
| 1.1 | Webhook signature verification | §17 | Security breach — fake payment confirmation |
| 1.2 | CSRF protection | §17 | Session hijack, unauthorized actions |
| 2.1 | `chargeback_won → paid` state regression | §10.1 | Double entitlement, revenue reporting sai |
| 2.2 | Order item state machine thiếu | §10 | Partial refund logic broken |
| 3.1 | `available` stored separately | §13.5 | Silent inventory inconsistency |
| 3.2 | `inventory_reservations` thiếu `order_item_id` | §13.5 | Wrong reservation tracking |

### High — Fix trước launch:

| # | Vấn đề | Section | Tác động |
|---|---|---|---|
| 1.3 | Admin RBAC quá yếu | §17.2 | Privilege escalation risk |
| 2.4 | Grace period không có rules | §10.4 | Undefined behavior |
| 3.3 | `sellable_skus` polymorphic FKs | §13.2 | Data integrity risk |
| 3.4 | `entitlements` polymorphic source | §13.3 | Referential integrity |
| 4.3 | `/orders/{id}/pay` vague | §11.4 | Developer cannot implement correctly |
| 5.1 | Forgot-password security spec missing | §9, §17 | Reset token can be abused |
| 5.3 | Kafka delayed delivery misunderstanding | §14.2 | Wrong implementation of scheduler |

### Medium — Address in next SRD revision:

| # | Vấn đề |
|---|---|
| 1.4 | Payment raw payload PCI concern |
| 2.3 | `renewed` state unnecessary |
| 2.5 | Return shipment no inventory restock |
| 3.5 | Price table no uniqueness constraint |
| 4.1 | GET creates side effects |
| 4.2 | POST coupon should be PUT |
| 5.2 | Download quota undefined |
| 5.4 | User events topic missing |
| 5.5 | Pagination not specified |
| 5.7 | Flash-sale mechanism unspecified |
| 6.1 | `addresses` missing recipient fields |

---

## Bài học tổng hợp cho FE developer học System Design

Qua phân tích SRD này, đây là 7 mental model quan trọng nhất:

### 1. Security là requirement, không phải implementation detail
Signature verification, CSRF, rate limiting phải được ghi vào **requirements document** như business rules — không để implementation team tự quyết định có làm hay không.

### 2. State machine phải là DAG (không có vòng)
Mỗi transition phải có: trigger rõ ràng, actor có quyền, side effects được list, invariant không bị vi phạm. State machine với vòng lặp hoặc state vô nghĩa là dấu hiệu thiết kế chưa chín.

### 3. Derived data không được lưu như regular column
`available = on_hand - reserved` phải là computed column hoặc không lưu. Storing derived data tạo ra invariant cần application code maintain — và application code sẽ có bug.

### 4. Referential integrity qua DB constraint, không qua application code
Application code bị bug. DB constraint không bị bug. Polymorphic `type + id` là cách hy sinh DB enforcement để đổi lấy flexibility — thường không đáng.

### 5. HTTP semantics phải đúng
- GET: idempotent, safe, không có side effects (không tạo resource, không thay đổi state)
- POST: tạo resource mới, non-idempotent
- PUT: replace/upsert, idempotent
- PATCH: partial update, thường idempotent
- DELETE: xóa resource

Sai HTTP method nghĩa là caching bị sai, client retry bị sai, API client expectations bị sai.

### 6. Hiểu tool's capability trước khi chọn
Kafka không hỗ trợ delayed delivery. Redis không phải canonical store. PostgreSQL không scale ngang dễ. Chọn tool mà không hiểu limitation là nguồn gốc của kiến trúc sai.

### 7. Specification "vừa đủ" không phải "đủ"
"Hệ thống kiểm tra quota" không phải specification — không thể implement từ đó. Specification thực sự phải đủ để một developer chưa biết gì về domain implement đúng mà không cần hỏi lại. Nếu phải đoán → spec chưa đủ.
