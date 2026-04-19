# Tài liệu State Transition Specification Chi Tiết

## 1. Mục đích tài liệu

Tài liệu này đặc tả chi tiết **State Transition Specification** cho hệ thống backend monolith bán sách sử dụng Golang/Gin, PostgreSQL, Redis và Kafka. Mục tiêu của tài liệu là chuẩn hóa toàn bộ vòng đời trạng thái của các aggregate nghiệp vụ cốt lõi trong dự án bookstore, bảo đảm mọi module backend, worker, scheduler, webhook handler, admin operation và integration adapter đều cùng tuân theo một mô hình chuyển trạng thái thống nhất.

Tài liệu này đóng vai trò là cầu nối giữa:
- URD/SRD nghiệp vụ;
- ERD logic và physical schema;
- Kafka event contract;
- Redis coordination design;
- OpenAPI và service implementation về sau.

Tài liệu này phải được xem là nguồn chuẩn để trả lời các câu hỏi sau:
- trạng thái nào là hợp lệ cho từng aggregate;
- transition nào được phép;
- transition nào bị cấm;
- ai có quyền kích hoạt transition;
- transition nào là synchronous, transition nào kéo theo asynchronous side effects;
- retry, timeout, idempotency và audit được xử lý như thế nào.

## 2. Phạm vi tài liệu

Tài liệu bao phủ đầy đủ các aggregate cốt lõi sau:
- Order
- Payment
- Refund
- Chargeback
- Membership
- Entitlement
- Reader Session
- Download Request / Download Token Lifecycle
- Inventory Reservation
- Shipment
- E-invoice Export
- Notification Job
- Outbox Event

Ngoài phạm vi của tài liệu này:
- UI state phía frontend;
- low-level worker scheduling implementation;
- chi tiết giao thức API của payment/carrier/e-invoice provider;
- dữ liệu analytical/reporting không tham gia trực tiếp vào state nghiệp vụ chính.

## 3. Mục tiêu thiết kế state machine

State machine trong hệ thống này không chỉ là bảng trạng thái để tham khảo. Đây là cơ chế bảo vệ các **business invariants** quan trọng nhất của hệ thống, bao gồm:
- không double finalize payment;
- không double grant entitlement;
- không COD cho order chứa digital item;
- không để shipment tồn tại cho order không có physical line;
- không phát link tải mới khi membership đã hết hạn;
- không để inventory available âm do race condition;
- không để admin hoặc worker sửa trạng thái tùy tiện bỏ qua audit.

State machine còn giúp:
- cô lập ownership của từng aggregate;
- kiểm soát coupling giữa modules;
- chuẩn hóa event emission;
- hỗ trợ idempotency và retry an toàn;
- đơn giản hóa debugging trong production.

## 4. Nguyên tắc tổng quát

### 4.1 Nguyên tắc ownership

- Mỗi aggregate có đúng một module owner được quyền cập nhật canonical state.
- Module khác chỉ được yêu cầu transition qua service contract hoặc domain event/command event tương ứng.
- Worker hoặc webhook handler không được phép ghi xuyên trực tiếp vào state của aggregate mà không đi qua policy/state transition layer.

### 4.2 Nguyên tắc source of truth

- PostgreSQL là canonical source of truth cho mọi trạng thái nghiệp vụ bền vững.
- Redis chỉ dùng cho coordination, cache, quota, lock và hot-path transient state.
- Kafka chỉ dùng để phát tín hiệu bất đồng bộ, không phải nơi quyết định trạng thái cuối cùng.

### 4.3 Nguyên tắc idempotency

- Mọi transition có thể bị trigger lặp phải được thiết kế idempotent.
- Duplicate request, duplicate webhook, duplicate consumer processing không được gây sai business state.
- Mọi transition có side effect tài chính, entitlement hoặc inventory đều phải có idempotency guard.

### 4.4 Nguyên tắc auditability

- Mọi transition quan trọng phải có actor, trigger, timestamp, reason code và audit record tương ứng.
- Admin override bắt buộc có before/after snapshot và justification rõ ràng.

### 4.5 Nguyên tắc transition safety

- Chỉ cho phép transition nằm trong ma trận hợp lệ.
- Không cho phép “nhảy trạng thái” tùy tiện nếu chưa qua trạng thái trung gian có ý nghĩa nghiệp vụ.
- Không rollback ngược terminal state bằng update trực tiếp; nếu cần sửa sai phải đi qua corrective action hoặc compensating flow.

## 5. Thuật ngữ và định nghĩa

| Thuật ngữ | Định nghĩa |
|---|---|
| Aggregate | Thực thể nghiệp vụ có vòng đời trạng thái riêng, ví dụ Order hoặc Payment |
| Transition | Sự thay đổi từ trạng thái hiện tại sang trạng thái kế tiếp |
| Trigger | Tác nhân gây ra transition, có thể là API call, webhook, scheduler, worker hoặc admin action |
| Guard Condition | Điều kiện bắt buộc phải đúng trước khi transition xảy ra |
| Terminal State | Trạng thái kết thúc nghiệp vụ, không được chuyển tiếp bình thường |
| Self-transition | Trigger xảy ra nhưng aggregate vẫn giữ nguyên state, thường dùng để cập nhật metadata hoặc heartbeat |
| Compensating Action | Hành động bù trừ khi một side effect sau transition thất bại |
| Canonical State | Trạng thái nguồn sự thật cuối cùng của aggregate |
| Derived State | Trạng thái được suy ra hoặc cache hóa từ canonical state |
| Side Effect | Hành động phụ sau transition như gửi email, phát Kafka event, invalidate cache |

## 6. Trigger taxonomy

### 6.1 Nhóm trigger chuẩn

| Nhóm trigger | Mô tả | Ví dụ |
|---|---|---|
| User API Trigger | Trigger do người dùng chủ động gây ra | submit checkout, cancel order, request refund |
| Admin Trigger | Trigger do admin/operator kích hoạt | manual refund approve, revoke entitlement |
| Gateway Webhook Trigger | Trigger từ payment/invoice provider | payment captured, refund success |
| Carrier Trigger | Trigger từ carrier webhook/polling | shipment delivered |
| Scheduler Trigger | Trigger theo thời gian | membership expiry check, payment timeout |
| Worker Trigger | Trigger từ async consumer | grant entitlement, send email |
| Internal System Trigger | Trigger sinh ra từ domain service | auto finalize order, release reservation |

### 6.2 Trigger naming guideline

- Trigger phải đặt tên theo hành động nghiệp vụ, không theo tên hàm code.
- Ví dụ đúng: `payment_captured`, `membership_expired`, `release_timeout`, `cancel_before_pay`.
- Ví dụ không nên dùng: `handleCallback`, `updateStatus`, `syncData`.

## 7. Chuẩn mô tả transition

Mỗi transition trong tài liệu này phải có đủ các trường sau:
- Current State
- Trigger
- Actor / Source
- Guard Conditions
- Next State
- Synchronous Actions
- Asynchronous Side Effects
- Idempotency Rule
- Audit Requirement
- Notes / Special Cases

## 8. Global business invariants

Các bất biến nghiệp vụ toàn hệ thống phải luôn đúng:

1. COD chỉ được áp dụng cho **physical-only order**.
2. Order chứa ebook hoặc membership không được đi COD path.
3. Payment captured chỉ được finalize order đúng một lần.
4. Membership active không đồng nghĩa với quyền vĩnh viễn; quyền từ membership phải hết hạn khi membership hết hạn.
5. Lịch sử download vẫn giữ khi membership hết hạn, nhưng mọi link tải cũ phải bị vô hiệu hóa và không phát link mới từ quyền đã hết hạn.
6. Entitlement là lớp canonical trả lời quyền đọc/tải hiện tại của user đối với từng book.
7. Shipment không được tạo cho order không có physical line.
8. Inventory reservation không được làm âm available stock.
9. Refund hoặc chargeback có thể làm revoke quyền số theo policy, nhưng không giả định thu hồi được file đã tải về local trước đó.
10. Admin override được phép tồn tại nhưng phải audit tuyệt đối.

## 9. Order State Transition Specification

### 9.1 Aggregate definition

- Aggregate: `Order`
- Owner module: `order` / `checkout`
- Canonical tables: `orders`, `order_items`
- External dependencies: payment, entitlement, membership, inventory, shipment, refund, invoice

### 9.2 Danh sách trạng thái Order

| State | Ý nghĩa | Terminal |
|---|---|---|
| `draft` | Order mới hình thành trong context nội bộ, chưa commit business flow chính | Không |
| `pending_payment` | Order chờ thanh toán online | Không |
| `payment_processing` | Đã khởi tạo payment, đang chờ kết quả cuối | Không |
| `confirmed_cod` | Order COD physical-only đã được chấp nhận | Không |
| `paid` | Payment thành công, order đã được ghi nhận đã thanh toán | Không |
| `partially_fulfilled` | Một phần line đã được fulfill, phần còn lại chưa | Không |
| `fulfilled` | Toàn bộ line đã hoàn tất fulfillment | Có |
| `cancel_requested` | Có yêu cầu hủy, đang chờ review/processing | Không |
| `cancelled` | Order bị hủy trước khi hoàn tất | Có |
| `refund_pending` | Đang xử lý hoàn tiền | Không |
| `partially_refunded` | Đã hoàn tiền một phần | Không |
| `refunded` | Đã hoàn tiền toàn phần | Có |
| `failed` | Thanh toán thất bại hoặc flow thất bại | Có |
| `cod_in_delivery` | Đơn COD đang trong luồng giao hàng | Không |
| `failed_cod` | Đơn COD giao thất bại/từ chối nhận | Có |
| `chargeback_open` | Có tranh chấp thanh toán mở | Không |
| `chargeback_resolved` | Tranh chấp đã kết thúc | Có điều kiện |

### 9.3 Allowed transitions cho Order

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Synchronous Actions | Asynchronous Side Effects | Idempotency Rule | Audit |
|---|---|---|---|---|---|---|---|---|
| `draft` | `submit_checkout_online` | user/system | Cart hợp lệ, re-price pass, digital/COD rule hợp lệ | `pending_payment` | Tạo `orders`, `order_items`, snapshot buyer/address/pricing | `order.created` | Idempotency theo checkout key | Ghi audit order create |
| `draft` | `submit_checkout_cod` | user/system | Order physical-only, địa chỉ hợp lệ, stock pass | `confirmed_cod` | Tạo order và reserve path ban đầu nếu policy yêu cầu | `order.created` | Idempotency theo checkout key | Ghi audit |
| `pending_payment` | `payment_init_success` | system | Payment intent tạo thành công | `payment_processing` | Tạo `payments`, `payment_attempts` | `payment.initiated` | Không tạo nhiều payment object cho cùng request key | Ghi audit nếu cần |
| `pending_payment` | `cancel_before_pay` | user/admin/system | Chưa có capture | `cancelled` | Mark cancelled, release soft holds nếu có | `order.cancelled` | Duplicate cancel phải no-op | Ghi reason |
| `payment_processing` | `payment_captured` | gateway/system | Webhook verified, chưa finalize trước đó | `paid` | Mark order paid, freeze line states phù hợp | `order.paid` | Lock theo `order_id` hoặc `payment_id` | Bắt buộc |
| `payment_processing` | `payment_failed` | gateway/system | Webhook verified | `failed` | Save fail state and reason | `payment.failed`, `order.payment_failed` | Duplicate fail không override captured | Bắt buộc |
| `payment_processing` | `payment_expired` | scheduler/system | Chưa captured, quá hạn | `failed` | Mark expired/fail | `payment.expired` | Nếu đã captured thì no-op | Bắt buộc |
| `paid` | `digital_only_fulfillment_done` | system | Tất cả digital line được entitled, không có physical line | `fulfilled` | Update line states -> fulfilled | `order.fulfilled` | Idempotent theo order | Ghi audit nội bộ |
| `paid` | `mixed_partial_done` | system | Digital line done, physical line chưa delivered | `partially_fulfilled` | Update partial states | `order.partially_fulfilled` | Idempotent | Ghi audit |
| `paid` | `shipment_delivered_all` | carrier/system | Mọi physical line delivered, digital line done nếu có | `fulfilled` | Finalize fulfillment | `order.fulfilled` | Duplicate delivery no-op | Ghi audit |
| `paid` | `refund_requested` | user/admin/system | Theo refund policy | `refund_pending` | Tạo refund request | `refund.requested` | Idempotency theo refund request key | Bắt buộc |
| `partially_fulfilled` | `refund_requested` | user/admin/system | Có line eligible | `refund_pending` | Tạo refund request | `refund.requested` | Idempotent | Bắt buộc |
| `refund_pending` | `refund_partial_done` | gateway/system | Refund một phần thành công | `partially_refunded` | Update refunded totals | `refund.completed` | Theo refund_id | Bắt buộc |
| `refund_pending` / `partially_refunded` | `refund_full_done` | gateway/system | Refund full thành công | `refunded` | Finalize refunded totals | `refund.completed` | Theo refund_id | Bắt buộc |
| `confirmed_cod` | `shipment_created` | system | Shipment tạo thành công | `cod_in_delivery` | Reserve stock nếu chưa reserve | `shipment.created` | Duplicate create blocked bởi unique shipment/order | Ghi audit |
| `cod_in_delivery` | `delivery_success_cod_collected` | carrier/system | Carrier xác nhận delivered & COD collected | `fulfilled` | Mark order as paid by COD | `order.fulfilled` | Duplicate delivery no-op | Bắt buộc |
| `cod_in_delivery` | `delivery_failed` | carrier/system | Carrier xác nhận fail | `failed_cod` | Mark fail, chờ stock return flow nếu cần | `shipment.status_changed` | Idempotent theo shipment status log | Bắt buộc |
| `paid` / `partially_fulfilled` | `chargeback_opened` | gateway/system | Dispute opened | `chargeback_open` | Mark flag dispute | `chargeback.opened` | Theo case ref | Bắt buộc |
| `chargeback_open` | `chargeback_resolved` | gateway/system | Resolution final | `chargeback_resolved` | Save resolution | `chargeback.resolved` | Theo case ref | Bắt buộc |

### 9.4 Illegal transitions cho Order

Các transition sau bị cấm tuyệt đối:
- `cancelled -> paid`
- `failed -> paid`
- `fulfilled -> pending_payment`
- `refunded -> fulfilled`
- `confirmed_cod -> paid` qua payment online callback
- `pending_payment -> fulfilled` bỏ qua `paid`
- `draft -> fulfilled`

Nếu business cần sửa sai sau terminal state, phải dùng corrective flow riêng, không update thẳng state.

### 9.5 Order item state tương ứng

| Item State | Ý nghĩa |
|---|---|
| `created` | Item vừa được chụp snapshot trong order |
| `paid` | Item đã được thanh toán |
| `entitled` | Item digital đã được cấp quyền |
| `inventory_reserved` | Item physical đã được giữ tồn |
| `shipped` | Item physical đã giao cho carrier |
| `delivered` | Item physical đã giao thành công |
| `fulfilled` | Item đã hoàn tất nghiệp vụ cuối |
| `cancelled` | Item bị hủy |
| `refunded` | Item đã hoàn tiền |

Rule: `orders.order_state` phải luôn là hàm tổng hợp hợp lệ từ tập `order_items.item_state`; không được để summary state mâu thuẫn với detail states.

## 10. Payment State Transition Specification

### 10.1 Aggregate definition

- Aggregate: `Payment`
- Owner module: `payment`
- Canonical tables: `payments`, `payment_attempts`
- External dependencies: gateway adapters, order, refund, chargeback

### 10.2 Danh sách trạng thái Payment

| State | Ý nghĩa | Terminal |
|---|---|---|
| `initiated` | Payment object vừa được tạo | Không |
| `pending` | Đã gửi request đến gateway, chưa rõ kết quả cuối | Không |
| `authorized` | Gateway đã authorize thành công, chưa capture | Không |
| `captured` | Đã thu tiền thành công | Có điều kiện |
| `failed` | Payment thất bại | Có |
| `expired` | Payment hết hiệu lực | Có |
| `cancelled` | Payment bị hủy trước khi thành công | Có |
| `refund_pending` | Đang xử lý hoàn tiền | Không |
| `refunded_partial` | Đã hoàn tiền một phần | Không |
| `refunded_full` | Đã hoàn tiền toàn bộ | Có |
| `chargeback_open` | Có dispute mở | Không |
| `chargeback_resolved` | Dispute đã kết thúc | Có điều kiện |

### 10.3 Allowed transitions cho Payment

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Idempotency | Audit |
|---|---|---|---|---|---|---|---|---|
| `initiated` | `gateway_request_sent` | system | Provider request hợp lệ | `pending` | Save provider request id | none hoặc `payment.pending` | Theo payment attempt | Log internal |
| `pending` | `gateway_authorized` | gateway | Verify callback | `authorized` | Save auth ref | `payment.authorized` | Duplicate auth no-op | Audit |
| `pending` / `authorized` | `gateway_captured` | gateway | Verify callback, chưa captured trước đó | `captured` | Save capture refs, captured_at | `payment.captured` | Lock finalize theo payment | Bắt buộc |
| `pending` / `authorized` | `gateway_failed` | gateway | Verify callback | `failed` | Save error code/message | `payment.failed` | Không override captured | Bắt buộc |
| `pending` / `authorized` | `gateway_expired` | scheduler/gateway | Chưa captured | `expired` | Save expired_at | `payment.expired` | Idempotent | Bắt buộc |
| `pending` | `cancel_payment` | user/admin/system | Gateway cho cancel, chưa captured | `cancelled` | Save cancel reason | `payment.cancelled` | Idempotent | Audit |
| `captured` | `refund_requested` | user/admin/system | Refund policy pass | `refund_pending` | Link refund process | `refund.requested` | Theo refund request key | Audit |
| `refund_pending` | `refund_partial_done` | gateway/system | Partial refund complete | `refunded_partial` | Update refunded amount | `refund.completed` | Theo refund_id | Audit |
| `refund_pending` / `refunded_partial` | `refund_full_done` | gateway/system | Full refund complete | `refunded_full` | Update refunded amount total | `refund.completed` | Theo refund_id | Audit |
| `captured` / `refunded_partial` | `chargeback_opened` | gateway | Case opened | `chargeback_open` | Create case link | `chargeback.opened` | Theo case ref | Audit |
| `chargeback_open` | `chargeback_resolved` | gateway | Final result received | `chargeback_resolved` | Save resolution | `chargeback.resolved` | Theo case ref | Audit |

### 10.4 Payment invariants

- Không transition sang `captured` dựa trên redirect URL từ frontend.
- Chỉ verified webhook hoặc reconciliation mới được đổi sang `captured`.
- `captured` là cột mốc tài chính mạnh; mọi flow hậu kỳ phải bám vào đó.
- `payment_attempts` không được dùng để suy luận canonical state nếu `payments.payment_state` khác.

## 11. Refund State Transition Specification

### 11.1 Aggregate definition

- Aggregate: `Refund`
- Owner module: `refund`
- Canonical table: `refunds`

### 11.2 Danh sách trạng thái Refund

| State | Ý nghĩa | Terminal |
|---|---|---|
| `requested` | Vừa tạo yêu cầu hoàn tiền | Không |
| `under_review` | Chờ người/logic xét duyệt | Không |
| `approved` | Đã được duyệt để xử lý | Không |
| `processing_gateway` | Đã gửi yêu cầu ra gateway | Không |
| `completed` | Hoàn tiền thành công | Có |
| `failed` | Hoàn tiền thất bại | Có điều kiện |
| `rejected` | Bị từ chối | Có |
| `cancelled` | Yêu cầu hoàn bị hủy trước khi gửi gateway | Có |

### 11.3 Allowed transitions cho Refund

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Idempotency | Audit |
|---|---|---|---|---|---|---|---|---|
| `requested` | `auto_review_pass` | system | Rule auto-approve pass | `approved` | Save decision | none | Theo refund request key | Audit |
| `requested` | `manual_review_required` | system/admin | Rule yêu cầu review | `under_review` | Assign review state | none | Idempotent | Audit |
| `under_review` | `approve_refund` | admin/system | Reviewer hợp lệ | `approved` | Save approver/reason | none | Idempotent | Bắt buộc |
| `under_review` | `reject_refund` | admin/system | Reviewer hợp lệ | `rejected` | Save reject reason | `refund.rejected` | Idempotent | Bắt buộc |
| `approved` | `send_to_gateway` | system | Gateway operation required | `processing_gateway` | Save provider request info | none | Theo refund_id | Audit |
| `processing_gateway` | `gateway_refund_success` | gateway/system | Verified | `completed` | Save completed_at | `refund.completed` | Theo provider refund ref | Bắt buộc |
| `processing_gateway` | `gateway_refund_fail` | gateway/system | Verified | `failed` | Save error | `refund.failed` | Theo provider refund ref | Bắt buộc |
| `requested` / `under_review` | `cancel_refund_request` | user/admin | Chưa gửi gateway | `cancelled` | Save cancel reason | none | Idempotent | Audit |

### 11.4 Refund-specific rules

- Không cho `completed -> failed` hoặc `completed -> cancelled`.
- Refund partial và full phải được phản ánh ở payment/order aggregate nhưng không được viết tắt bằng cách bỏ qua `refunds` table.
- Nếu refund liên quan digital entitlement, sau `completed` phải kích hoạt evaluation revoke policy.

## 12. Chargeback State Transition Specification

### 12.1 Aggregate definition

- Aggregate: `Chargeback`
- Owner module: `chargeback`
- Canonical table: `chargebacks`

### 12.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `opened` | Tranh chấp mới mở | Không |
| `under_review` | Đang thu thập/chuẩn bị evidence | Không |
| `submitted` | Đã nộp evidence cho provider | Không |
| `won` | Doanh nghiệp thắng tranh chấp | Có |
| `lost` | Doanh nghiệp thua tranh chấp | Có |
| `cancelled` | Case bị hủy/voided | Có |

### 12.3 Allowed transitions cho Chargeback

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Notes |
|---|---|---|---|---|---|---|---|
| `opened` | `collect_evidence` | system/admin | Case hợp lệ | `under_review` | Save review state | none | |
| `opened` / `under_review` | `submit_evidence` | admin/system | Evidence đủ | `submitted` | Save evidence payload | none | |
| `opened` / `under_review` / `submitted` | `provider_resolution_won` | gateway | Verified | `won` | Save outcome | `chargeback.resolved` | Có thể khôi phục quyền nếu đã hold |
| `opened` / `under_review` / `submitted` | `provider_resolution_lost` | gateway | Verified | `lost` | Save outcome | `chargeback.resolved` | Trigger revoke/correction |
| `opened` | `void_case` | gateway/system | Provider voids case | `cancelled` | Save reason | none | |

### 12.4 Chargeback rules

- Chargeback không thay thế refund; đây là flow độc lập.
- `lost` có thể kéo theo revoke entitlement và accounting adjustment.
- Nếu hệ thống từng “hold” quyền tạm thời khi `opened`, thì khi `won` phải có corrective restore flow.

## 13. Membership State Transition Specification

### 13.1 Aggregate definition

- Aggregate: `Membership`
- Owner module: `membership`
- Canonical tables: `memberships`, `membership_plans`

### 13.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `pending_activation` | Đã có record nhưng chưa active | Không |
| `active` | Membership đang hiệu lực | Không |
| `suspended` | Tạm ngừng vì risk/manual policy | Không |
| `expired` | Hết hạn bình thường | Có điều kiện |
| `revoked` | Bị thu hồi trước hạn | Có |

### 13.3 Allowed transitions cho Membership

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Idempotency | Audit |
|---|---|---|---|---|---|---|---|---|
| `pending_activation` | `activate_after_paid_order` | system | Source order paid, plan valid | `active` | Set starts_at/expires_at | `membership.activated` | Theo source order | Bắt buộc |
| `active` | `renew_membership` | system | Renewal paid, plan renewable | `active` | Extend expires_at | `membership.renewed` | Theo renewal order | Bắt buộc |
| `active` | `suspend_membership` | admin/system | Fraud/risk/manual trigger | `suspended` | Save reason | `membership.suspended` | Idempotent | Bắt buộc |
| `suspended` | `restore_membership` | admin/system | Review pass | `active` | Restore state | `membership.restored` | Idempotent | Bắt buộc |
| `active` | `membership_expiry_timeout` | scheduler | `now > expires_at` | `expired` | Mark expired | `membership.expired` | Scheduler rerun safe | Bắt buộc |
| `active` / `suspended` | `revoke_membership` | refund/chargeback/admin | Valid revoke cause | `revoked` | Save revoked_at/reason | `membership.revoked` | Idempotent | Bắt buộc |
| `expired` | `renew_after_expiry` | system | Renewal paid | `active` | Set new active window | `membership.renewed` | Theo new source order | Bắt buộc |

### 13.4 Membership rules

- `expired` là tự nhiên; `revoked` là bất thường hoặc do corrective/financial action.
- Khi `expired` hoặc `revoked`, các entitlement phụ thuộc membership phải được đánh giá lại.
- Không nên update ngược `revoked -> active` trực tiếp; nếu admin sửa sai, nên tạo corrective membership grant/restore flow có audit rất rõ.

## 14. Entitlement State Transition Specification

### 14.1 Aggregate definition

- Aggregate: `Entitlement`
- Owner module: `entitlement`
- Canonical table: `entitlements`
- Purpose: canonical business-right layer cho quyền đọc/tải theo user-book-source

### 14.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `pending_grant` | Quyền đang chờ materialize/confirm | Không |
| `active` | Quyền đang hiệu lực | Không |
| `suspended` | Quyền tạm khóa | Không |
| `expired` | Quyền hết hạn bình thường | Có điều kiện |
| `revoked` | Quyền bị thu hồi | Có |

### 14.3 Allowed transitions cho Entitlement

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Idempotency | Audit |
|---|---|---|---|---|---|---|---|---|
| `pending_grant` | `grant_from_ebook_purchase` | system | Source order item paid | `active` | Save allow_read/allow_download/expiry | `entitlement.granted` | Theo source type/id + book_id + user_id | Bắt buộc |
| `pending_grant` | `grant_from_membership` | system | Membership active, book included in membership | `active` | Save rights with membership expiry | `entitlement.granted_from_membership` | Theo membership source | Bắt buộc |
| `active` | `suspend_entitlement` | admin/system | Fraud/risk | `suspended` | Save reason | `entitlement.suspended` | Idempotent | Bắt buộc |
| `suspended` | `restore_entitlement` | admin/system | Review pass | `active` | Restore | `entitlement.restored` | Idempotent | Bắt buộc |
| `active` | `natural_expiry` | scheduler/system | `now > expires_at` | `expired` | Mark expired | `entitlement.expired` | Scheduler rerun safe | Bắt buộc |
| `active` / `suspended` | `revoke_entitlement` | refund/chargeback/admin/system | Valid revoke cause | `revoked` | Save revoked info | `entitlement.revoked` | Theo source revoke action | Bắt buộc |

### 14.4 Effective-right evaluation rules

Nếu user có nhiều entitlement cho cùng một book:
- effective `can_read` = true nếu tồn tại ít nhất một entitlement `active` cho đọc;
- effective `can_download` = true nếu tồn tại ít nhất một entitlement `active` cho tải;
- effective expiry không phải là một trường duy nhất cố định nếu nhiều nguồn cùng tồn tại; service phải evaluate theo tập entitlement active.

### 14.5 Entitlement-specific rules

- Quyền từ ebook purchase thường có `expires_at = null` trừ khi business policy khác.
- Quyền từ membership phải gắn `expires_at` theo membership hoặc theo effective access window của plan.
- Khi membership hết hạn, không xóa lịch sử download; chỉ chặn token/link mới và đánh dấu entitlement tương ứng là `expired` hoặc `revoked` theo nguyên nhân.

## 15. Reader Session State Transition Specification

### 15.1 Aggregate definition

- Aggregate: `Reader Session`
- Owner module: `reader`
- Canonical table: `reader_sessions`
- Redis hot path: active counters, active session registry, heartbeat cache

### 15.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `created` | Session record vừa hình thành | Không |
| `active` | User đang đọc | Không |
| `expired` | Session timeout do mất heartbeat | Có |
| `ended` | User đóng reader bình thường | Có |
| `kicked` | Session bị ép đóng | Có |

### 15.3 Allowed transitions cho Reader Session

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Notes |
|---|---|---|---|---|---|---|---|
| `created` | `open_reader` | user/system | Entitlement active, device valid, concurrent limit chưa vượt | `active` | Create DB row, update Redis active registry | `reader.session_started` | Lock/atomic quota check |
| `active` | `heartbeat` | user/system | Session chưa terminal | `active` | Update last heartbeat & last position | `reader.position_updated` | Self-transition |
| `active` | `close_reader` | user | Session active | `ended` | Mark ended, release Redis counters | `reader.session_ended` | Normal path |
| `active` | `session_timeout` | scheduler/system | Heartbeat stale quá threshold | `expired` | Mark expired, cleanup counters | optional | Recovery path |
| `active` | `force_kick` | admin/system | Entitlement/device/session policy invalidated | `kicked` | Save kick reason, cleanup counters | `reader.session_kicked` | Used on revoke |

### 15.4 Reader rules

- Reader concurrent limit phải được kiểm soát atomically, ưu tiên Redis + DB reconciliation.
- Không được tin Redis counter 100%; phải có periodic reconciler giữa Redis và DB session log.
- Self-transition `heartbeat` không được bị xem là meaningless; đây là transition cập nhật sống còn cho timeout logic.

## 16. Download Lifecycle State Transition Specification

### 16.1 Aggregate definition

- Aggregate: `Download Record`
- Owner module: `download`
- Canonical table: `downloads`
- Redis support: token metadata, single-use marker, quota counters

### 16.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `requested` | User yêu cầu tải | Không |
| `issued` | Đã phát token/link tải | Không |
| `consumed` | Link/token đã được dùng | Có điều kiện |
| `expired` | Token hết hạn | Có |
| `revoked` | Token bị thu hồi | Có |
| `failed` | Không phát được token do rule fail | Có |

### 16.3 Allowed transitions cho Download

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Idempotency | Audit |
|---|---|---|---|---|---|---|---|---|
| `requested` | `issue_download_link` | user/system | Entitlement active, quota pass, device pass | `issued` | Create token metadata, set expiry | `download.link_issued` | Theo request key và business policy | Bắt buộc |
| `requested` | `validation_fail` | system | Entitlement/limit invalid | `failed` | Save fail reason | none hoặc failure event | Idempotent | Audit nhẹ |
| `issued` | `consume_download_token` | file gateway/system | Token valid, not used nếu single-use | `consumed` | Mark consumed, set used marker | `download.consumed` | Lua/atomic consume required | Bắt buộc |
| `issued` | `token_expired` | scheduler/system | TTL reached | `expired` | Mark expired | optional | Idempotent | Không bắt buộc mạnh |
| `issued` | `revoke_download_link` | system/admin | Membership expired/revoked or admin action | `revoked` | Mark revoked | `download.link_revoked` | Idempotent | Bắt buộc |

### 16.4 Download rules

- Nếu chính sách là single-use thì `issued -> consumed` phải atomic để chống double-click/race.
- `consumed` không có nghĩa file tải chắc chắn hoàn tất 100%, nhưng đủ để xem token đã bị dùng.
- Không được tái sử dụng token sau `consumed`, `expired` hoặc `revoked`.

## 17. Inventory Reservation State Transition Specification

### 17.1 Aggregate definition

- Aggregate: `Inventory Reservation`
- Owner module: `inventory`
- Canonical tables: `inventory_items`, `inventory_reservations`

### 17.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `pending` | Bắt đầu flow giữ tồn | Không |
| `reserved` | Đã giữ tồn thành công | Không |
| `released` | Đã trả tồn về available | Có |
| `consumed` | Reservation đã được tiêu thụ cho outbound shipment | Có điều kiện |
| `returned` | Hàng đã quay lại kho sau flow reverse logistics | Có |

### 17.3 Allowed transitions cho Inventory Reservation

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Notes |
|---|---|---|---|---|---|---|---|
| `pending` | `reserve_stock` | system | Available đủ, row lock/version check pass | `reserved` | Decrease available, increase reserved | `inventory.reserved` | Strong consistency bắt buộc |
| `reserved` | `release_timeout` | scheduler/system | Payment failed/expired/cancelled | `released` | Increase available, decrease reserved | `inventory.released` | Timeout release |
| `reserved` | `release_cancel` | system/admin | Order cancelled before outbound | `released` | Same as above | `inventory.released` | |
| `reserved` | `consume_for_shipment` | system | Outbound shipping confirmed | `consumed` | Convert reserved -> outbound consumed | `inventory.consumed` | |
| `consumed` | `return_to_stock` | system/admin | Returned goods physically received | `returned` | Increase stock by policy | `inventory.returned` | Future-friendly |

### 17.4 Inventory rules

- `reserve_stock` phải chạy trong DB transaction mạnh hoặc optimistic locking chuẩn; không dùng eventual consistency thuần túy.
- Redis stock guard chỉ là pre-check performance trong flash sale, không thay inventory transaction.
- Cần lưu movement reason để audit stock adjustments và reservation flows.

## 18. Shipment State Transition Specification

### 18.1 Aggregate definition

- Aggregate: `Shipment`
- Owner module: `shipment`
- Canonical tables: `shipments`, `shipment_status_logs`

### 18.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `pending_pack` | Chờ đóng gói | Không |
| `packed` | Đã đóng gói | Không |
| `awaiting_pickup` | Chờ carrier lấy hàng | Không |
| `in_transit` | Đang vận chuyển | Không |
| `delivered` | Giao thành công | Có |
| `delivery_failed` | Giao thất bại | Không |
| `returning` | Đang hoàn về | Không |
| `returned` | Đã hoàn về kho/người bán | Có |
| `cancelled` | Shipment bị hủy trước khi thực sự vận chuyển | Có |

### 18.3 Allowed transitions cho Shipment

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Notes |
|---|---|---|---|---|---|---|---|
| `pending_pack` | `pack_completed` | admin/system | Physical items available | `packed` | Save packed timestamp | `shipment.packed` | |
| `packed` | `handover_to_carrier` | admin/system | Carrier/tracking assigned | `awaiting_pickup` | Save carrier info | `shipment.created` / `shipment.assigned` | |
| `awaiting_pickup` | `carrier_pickup_confirmed` | carrier/system | Verified | `in_transit` | Append status log | `shipment.status_changed` | |
| `in_transit` | `carrier_delivered` | carrier/system | Verified | `delivered` | Save delivered_at | `shipment.status_changed` | May finalize order |
| `in_transit` | `carrier_delivery_failed` | carrier/system | Verified | `delivery_failed` | Save fail reason | `shipment.status_changed` | |
| `delivery_failed` | `retry_dispatch` | admin/system | Policy cho phép reattempt | `in_transit` | Append log | `shipment.status_changed` | Optional but supported |
| `delivery_failed` / `in_transit` | `start_return` | carrier/system | Carrier begins return flow | `returning` | Append log | `shipment.status_changed` | |
| `returning` | `return_completed` | carrier/system | Goods returned physically | `returned` | Append log | `shipment.status_changed` | |
| `pending_pack` / `packed` / `awaiting_pickup` | `cancel_shipment` | admin/system | Chưa thực vào transit | `cancelled` | Save cancel reason | `shipment.cancelled` | |

### 18.4 Shipment rules

- Một `order` chỉ có một `shipment` trong phase hiện tại.
- Không cho `shipment` tồn tại nếu order không có physical line.
- Carrier webhook/polling chỉ append facts và transition hợp lệ; không được sửa order trực tiếp bỏ qua shipment service.

## 19. E-invoice Export State Transition Specification

### 19.1 Aggregate definition

- Aggregate: `EInvoiceExport`
- Owner module: `invoice`
- Canonical table: `e_invoice_exports`

### 19.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `pending_export` | Chờ gửi sang provider | Không |
| `exporting` | Worker đang xử lý gửi | Không |
| `exported` | Provider xác nhận thành công | Có |
| `failed` | Export thất bại | Không |
| `cancelled` | Bị hủy do rule hoặc admin | Có |

### 19.3 Allowed transitions

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Notes |
|---|---|---|---|---|---|---|---|
| `pending_export` | `start_export` | worker/system | Order đủ điều kiện hóa đơn | `exporting` | Save attempt info | none | |
| `exporting` | `provider_export_success` | provider/worker | Verified | `exported` | Save provider ref | `invoice.export_completed` | |
| `exporting` | `provider_export_fail` | provider/worker | Verified | `failed` | Save error + retryable flag | `invoice.export_failed` | |
| `failed` | `retry_export` | worker/admin | Retryable | `exporting` | Increment retry count | none | |
| `pending_export` / `failed` | `cancel_export` | admin/system | Rule cho phép | `cancelled` | Save reason | none | |

### 19.4 Invoice rules

- Hệ thống chỉ export dữ liệu; không nội bộ hóa vai trò nhà cung cấp hóa đơn điện tử.
- `exported` không được tự động rollback khi order có dispute về sau; cần policy kế toán/điều chỉnh riêng.

## 20. Notification Job State Transition Specification

### 20.1 Aggregate definition

- Aggregate: `Notification Job`
- Owner module: `notification`
- Canonical table: `notifications`

### 20.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `queued` | Chờ worker xử lý | Không |
| `processing` | Worker đang gửi | Không |
| `sent` | Gửi thành công | Có |
| `retry_waiting` | Lỗi tạm thời, chờ retry | Không |
| `failed` | Gửi thất bại vĩnh viễn | Có |
| `cancelled` | Bị hủy | Có |

### 20.3 Allowed transitions

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Async Side Effects | Notes |
|---|---|---|---|---|---|---|---|
| `queued` | `worker_pickup` | worker | Job hợp lệ | `processing` | Increment attempt | none | |
| `processing` | `send_success` | worker/provider | Provider success | `sent` | Save sent_at | none | Terminal |
| `processing` | `temporary_failure` | worker/provider | Retryable error | `retry_waiting` | Save error, next_retry_at | none | |
| `retry_waiting` | `retry_due` | worker | Time due | `processing` | Increment attempt | none | |
| `processing` / `retry_waiting` | `permanent_failure` | worker/provider | Non-retryable or max attempts exceeded | `failed` | Save final error | none | Terminal |
| `queued` / `retry_waiting` | `cancel_notification` | admin/system | Chưa sent | `cancelled` | Save reason | none | |

## 21. Outbox Event State Transition Specification

### 21.1 Aggregate definition

- Aggregate: `Outbox Event`
- Owner module: `integration/outbox`
- Canonical table: `outbox_events`

### 21.2 States

| State | Ý nghĩa | Terminal |
|---|---|---|
| `pending` | Chờ publish Kafka | Không |
| `publishing` | Worker đang publish | Không |
| `published` | Đã publish thành công | Có |
| `failed` | Publish thất bại, chờ retry | Không |
| `parked` | Tạm dừng cần người can thiệp | Có điều kiện |

### 21.3 Allowed transitions

| Current State | Trigger | Actor / Source | Guard Conditions | Next State | Sync Actions | Notes |
|---|---|---|---|---|---|---|
| `pending` | `publisher_claim` | worker | Event hợp lệ | `publishing` | Claim row / increment try | |
| `publishing` | `publish_success` | worker | Broker ack success | `published` | Save published_at | Terminal |
| `publishing` | `publish_fail_retryable` | worker | Retryable error | `failed` | Save last_error | |
| `failed` | `retry_publish` | worker | Retry budget còn | `publishing` | Increment retry | |
| `failed` | `park_event` | worker/admin | Retry exhausted or non-recoverable config/schema issue | `parked` | Save reason | |
| `parked` | `manual_requeue` | admin/system | Issue fixed | `pending` | Reset or increment control fields | |

## 22. Cross-aggregate transition mapping

### 22.1 Chuỗi transition quan trọng: online digital order

1. `Order: draft -> pending_payment`
2. `Payment: initiated -> pending -> captured`
3. `Order: payment_processing -> paid`
4. `Membership: pending_activation -> active` nếu có membership line
5. `Entitlement: pending_grant -> active` cho ebook purchase hoặc membership rights
6. `Order: paid -> fulfilled` nếu digital-only đã complete right grant
7. `Notification: queued -> sent`
8. `EInvoiceExport: pending_export -> exported` nếu policy tạo invoice

### 22.2 Chuỗi transition quan trọng: physical COD order

1. `Order: draft -> confirmed_cod`
2. `InventoryReservation: pending -> reserved`
3. `Shipment: pending_pack -> packed -> awaiting_pickup -> in_transit -> delivered`
4. `Order: confirmed_cod -> cod_in_delivery -> fulfilled`

### 22.3 Chuỗi transition khi membership hết hạn

1. `Membership: active -> expired`
2. `Entitlement: active -> expired` cho các quyền phụ thuộc membership
3. `Download: issued -> revoked` cho token/link đang còn hiệu lực cần vô hiệu hóa theo policy
4. `ReaderSession: active -> kicked` nếu đang đọc mà policy yêu cầu đóng phiên ngay

### 22.4 Chuỗi transition khi refund digital line thành công

1. `Refund: approved -> processing_gateway -> completed`
2. `Payment: captured -> refund_pending -> refunded_partial/refunded_full`
3. `Entitlement: active -> revoked` nếu policy yêu cầu
4. `Order: paid/partially_fulfilled -> refund_pending -> partially_refunded/refunded`
5. `Notification: queued -> sent`

## 23. Timeout policies

| Aggregate | Timeout / SLA logic | Hành động khi timeout |
|---|---|---|
| Payment pending | Theo gateway TTL | Move sang `expired` hoặc `reconcile_requested` |
| Reader session active | Mất heartbeat quá ngưỡng | Move sang `expired` |
| Download token issued | TTL ngắn 5–15 phút | Move sang `expired` |
| Inventory reservation reserved | Quá hạn chưa trả kết quả thanh toán | Move sang `released` |
| Notification retry_waiting | Quá next_retry_at | Move sang `processing` |
| Outbox failed | Theo worker retry schedule | Retry hoặc `parked` |
| Membership active | Quá `expires_at` | Move sang `expired` |
| Shipment polling wait | Theo carrier SLA nội bộ | Re-poll hoặc escalate |

## 24. Idempotency guidelines theo aggregate

| Aggregate | Idempotency key / strategy |
|---|---|
| Order create | checkout idempotency key |
| Payment capture | `provider + external_payment_ref` hoặc verified event ref |
| Refund complete | `provider_refund_ref` |
| Membership activation | `source_order_id + plan_id + user_id` |
| Entitlement grant | `source_type + source_id + user_id + book_id` |
| Download consume | `token_id` atomic consume |
| Shipment status change | `carrier + external_event_id` hoặc dedup payload signature |
| Invoice export | `order_id` hoặc `invoice_export_id` |
| Notification send | `notification_id` |
| Outbox publish | `event_id` |

## 25. Audit requirements

Mọi transition sau bắt buộc có audit log cấp business:
- Order cancel, paid, refund, fulfilled, failed_cod
- Payment captured, failed, refunded, chargeback opened/resolved
- Membership activated, expired, revoked, restored
- Entitlement granted, revoked, expired, restored
- Inventory manual adjustment, reserve/release anomalies
- Shipment status manual override
- Invoice export manual retry/cancel
- Admin override bất kỳ

Audit tối thiểu phải có:
- actor_type
- actor_id
- action / trigger
- resource_type
- resource_id
- before_state
- after_state
- reason_code / note
- created_at
- correlation_id nếu có

## 26. Admin override policy

### 26.1 Nguyên tắc

Admin operator có quyền override business theo requirement hiện tại, nhưng override chỉ hợp lệ khi:
- hành động nằm trong danh mục cho phép;
- actor có permission tương ứng;
- bắt buộc nhập reason code và note;
- ghi audit before/after;
- nếu override bypass normal flow thì phải phát corrective event nếu downstream cần biết.

### 26.2 Ví dụ override hợp lệ

- Revoke membership do fraud confirmed
- Revoke entitlement thủ công
- Retry invoice export
- Force close reader session
- Manual inventory adjustment
- Retry shipment sync

### 26.3 Ví dụ override không hợp lệ

- Sửa trực tiếp `payment_state = captured` khi không có chứng cứ gateway
- Sửa order `failed -> paid` bằng tay mà không tạo corrective financial flow
- Sửa entitlement active khi source order đã refunded mà không có grant policy mới

## 27. Illegal transition catalog tổng hợp

Các loại illegal transition phổ biến cần chặn ở service layer và test:
- terminal -> non-terminal không qua corrective flow
- unpaid order -> fulfilled trực tiếp
- expired/revoked entitlement -> active trực tiếp không qua new source hoặc restore flow hợp lệ
- shipment delivered -> pending_pack
- inventory released -> reserved lại trên cùng reservation record
- download consumed -> issued lại trên cùng token record
- refunded payment -> captured
- cancelled notification -> processing
- published outbox -> pending trực tiếp không qua requeue semantics chính thức

## 28. Failure handling và compensating actions

### 28.1 Nguyên tắc

Không phải mọi transition đều rollback được theo kiểu DB transaction thuần túy, nhất là khi đã có side effect ngoài hệ thống. Vì vậy cần định nghĩa rõ compensating strategy.

### 28.2 Ví dụ compensating strategy

| Tình huống | Chiến lược |
|---|---|
| Payment captured nhưng entitlement grant fail tạm thời | Giữ order `paid`, enqueue retry grant entitlement, alert nếu quá ngưỡng |
| Membership expired nhưng Redis cache chưa invalidate | Canonical DB vẫn thắng, request path phải fallback check DB khi cần |
| Shipment delivered webhook duplicate | Dedup theo external event id, no-op transition |
| Download token consume race | Lua/atomic consume, chỉ một flow thắng |
| Outbox publish fail sau DB commit | Retry publish từ outbox worker |
| Refund completed nhưng revoke entitlement fail | Retry revoke worker + manual alert |

## 29. Test matrix bắt buộc cho state machine

### 29.1 Loại test

- Unit test cho `CanTransition` và policy guards
- Integration test cho transaction boundary
- Concurrency test cho payment finalize, inventory reserve, download consume, reader concurrent limit
- Contract test giữa transition và event emission
- Replay test cho duplicate webhook / duplicate Kafka event
- Recovery test cho worker crash và retry

### 29.2 Test cases tối thiểu

| Aggregate | Test tối thiểu |
|---|---|
| Order | create online, create COD, cancel before pay, paid duplicate, refund partial/full |
| Payment | capture duplicate, fail after capture, expire before capture, reconcile success |
| Membership | activate, renew, expire, revoke, restore |
| Entitlement | grant purchase, grant membership, expire, revoke, multi-source effective access |
| Reader | open session over limit, heartbeat stale cleanup, force kick |
| Download | issue token, consume once, consume duplicate, revoke on membership expiry |
| Inventory | reserve race, release timeout, consume for shipment |
| Shipment | create, delivered duplicate, failed then return |
| Invoice | export success, fail, retry |
| Notification | retry then sent, max attempt fail |
| Outbox | publish fail then retry, parked then requeue |

## 30. Mapping tài liệu này với implementation

Tài liệu này phải được phản chiếu trong code dưới dạng:
- state enums per aggregate;
- transition policy tables hoặc state machine objects;
- domain service methods theo trigger semantics;
- centralized error codes cho illegal transition;
- outbox emission mapping;
- audit logging hooks;
- integration tests tương ứng.

Khuyến nghị package structure theo tư duy backend:
- `internal/domain/{aggregate}` cho state enum, policy, invariants
- `internal/application/{usecase}` cho orchestration
- `internal/infrastructure/repository` cho persistence
- `internal/infrastructure/events` cho outbox/publisher/consumer base
- `internal/infrastructure/redis` cho counters/locks/quota
- `internal/interfaces/http` cho API handlers
- `internal/interfaces/jobs` cho scheduler/worker entrypoints

## 31. Acceptance criteria

### 31.1 Tài liệu
- Mọi aggregate quan trọng đều có danh sách trạng thái rõ ràng.
- Mọi transition đều có trigger, guard, next state, side effect.
- Có catalog transition bị cấm.
- Có mapping timeout, idempotency, audit.

### 31.2 Implementation readiness
- Có thể sinh enum và validation rules từ tài liệu này.
- Có thể viết service methods và tests trực tiếp từ transition matrix.
- Có thể map sang Kafka event contract và Redis coordination mà không mâu thuẫn.

### 31.3 Business correctness
- Không double capture/double fulfill/double entitlement.
- Membership expiry không còn phát link tải mới.
- COD path không chạm digital order.
- Inventory race không làm available âm.
- Admin override luôn audit được.

## 32. Hạng mục nên làm tiếp ngay sau tài liệu này

Sau khi chốt State Transition Specification, các deliverable tiếp theo nên là:
- OpenAPI Specs cho public/admin domains
- ADR pack cho các quyết định kiến trúc chính
- Data Dictionary chi tiết cho mọi bảng
- Golang implementation blueprint cho module boundary, repository pattern, transaction policy, outbox, Redis coordination
- Test Strategy & QA Matrix
- Failure Recovery Runbook

Tài liệu này là lớp xương sống để chuyển từ planning nghiệp vụ sang triển khai backend thực chiến theo tư duy system design, thay vì coding theo từng màn hình hoặc từng endpoint rời rạc.
