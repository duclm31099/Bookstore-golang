# Hybrid URD/SRD for Bookstore Backend Monolith

## 1. Document Control

### 1.1 Document purpose
This document defines the business requirements, system requirements, domain model, operational rules, technical architecture, and acceptance criteria for a modular monolith backend serving an online bookstore business in Vietnam. The system supports physical books, digital books, and membership plans, and is designed to be implemented with Golang, Gin, PostgreSQL, Redis, and Kafka. Vietnam mandates electronic invoicing for taxpayers, but this system will only prepare and export invoice data to an external e-invoice provider rather than issuing invoices natively.[web:48][web:61][web:62]

### 1.2 Intended audience
This document is intended for founders, business analysts, backend engineers, solution architects, DevOps engineers, QA engineers, and admin-operations stakeholders. It is written to be detailed enough for backend module decomposition, schema design, API design, event modeling, cache design, and implementation planning.

### 1.3 Product vision
The product is a commerce platform for selling books in Vietnam across three product lines: physical books, digital books, and membership plans. Registered customers can buy books individually; members can read and download eligible digital books that are flagged as part of the membership program; and the platform must operate with real payment integrations, COD for physical books, inventory, shipping, and a complete admin backoffice.[web:54][web:71]

### 1.4 Architectural vision
The system shall be implemented as a modular monolith with a single deployable backend runtime and clearly separated internal modules. Kafka is a core requirement, not only for asynchronous processing and background jobs, but also as a learning-oriented event backbone for workers, schedulers, retries, and domain event propagation in a modular monolith architecture.[web:37][web:43][web:46]

## 2. Objectives and Success Criteria

### 2.1 Business objectives
- Sell physical books nationwide in Vietnam.
- Sell digital books in PDF and EPUB formats.
- Sell monthly and yearly membership plans.
- Allow online reading for users who either purchased the relevant ebook or hold an active membership covering that title.
- Allow digital file downloads for users who purchased the ebook or hold an active membership covering that title.
- Support real payment methods including Stripe, MoMo, VNPay, PayPal, and COD for physical-only orders.
- Provide a complete admin backoffice under `/api/v1/admin/*`.

### 2.2 Engineering objectives
- Build the system so that correctness and auditability take priority over cache freshness.
- Use PostgreSQL as source of truth for transactional state.
- Use Redis to reduce latency, reduce load, support quotas, and improve concurrency control.
- Use Kafka for asynchronous background processing, retries, delayed workflows, and event-driven module integration.
- Support extension paths for search engine migration, carrier integration, and e-invoice provider integration.

### 2.3 Success criteria
- Users can successfully browse, search, purchase, read, and download according to entitlement rules.
- Admins can operate catalog, orders, payments, refunds, inventory, shipments, and reports with auditable actions.
- Duplicate payment callbacks, worker retries, and repeated download requests do not corrupt state.
- The system can degrade gracefully if Redis or Kafka becomes temporarily unavailable.
- The architecture remains monolithic in deployment but modular in code and event contracts.

## 3. Scope

### 3.1 In scope
- Public bookstore APIs.
- Admin APIs.
- User registration, login, session management, and device registry.
- Physical book catalog, ebook catalog, membership plans, pricing, couponing.
- Cart and checkout.
- Online payment integration and COD.
- Refunds, partial refunds, chargebacks.
- Inventory reservation and single-shipment delivery model.
- Digital entitlement engine.
- Online reading for PDF and EPUB.
- Download link issuance and download history.
- Email notifications.
- Outbox-driven Kafka publishing and Kafka consumers.
- Redis-based cache, idempotency, rate limiting, session, device, cart, and flash-sale guards.
- E-invoice export staging for an external provider.

### 3.2 Out of scope for phase 1
- Native e-invoice issuance engine.
- DRM watermarking and DRM-enforced local file revocation.
- Family account sharing.
- Native mobile apps.
- Multi-seller marketplace logic.
- Multi-shipment orders.
- Content CMS for chapter authoring, metadata authoring workflow, and digital file versioning.
- SMS, push, or in-app notifications.

## 4. Assumptions and Constraints

### 4.1 Business assumptions
- The seller is a single legal entity.
- The system serves Vietnam only.
- The settlement currency is VND only.
- Only books flagged as included in membership are unlocked through membership.
- Membership adds rights for reading and downloading digital books, but does not alter physical-book purchase rights.
- If a membership expires, files already downloaded remain in the user’s possession locally, but prior download links inside the account are invalidated and no new membership-based download link is issued after expiry.

### 4.2 Technical constraints
- PostgreSQL full-text search will be used initially; Elasticsearch/OpenSearch is reserved for later expansion.
- One order maps to one shipment.
- COD is in scope but only for physical-only orders.
- Email is the only notification channel in phase 1.
- The platform must be secure enough for a production-grade small system.

## 5. Glossary

| Term | Definition |
|---|---|
| Book | A catalog item representing a title sold by the platform. |
| Physical Book | A shippable printed book. |
| Ebook | A digital book sold in PDF and/or EPUB format. |
| Membership | A time-bounded plan granting access to eligible digital books. |
| Membership-Eligible Book | A digital title flagged as included in the membership program. |
| Entitlement | A computed right allowing a user to read or download a specific digital title through purchase, membership, or admin grant. |
| Device | A user-recognized client environment used for login, reading, or downloading. |
| Reader Session | An online reading session tracked for concurrency enforcement. |
| Download Token | A short-lived signed token granting access to a downloadable asset. |
| Order | A commercial transaction containing one or more purchasable items. |
| Payment Attempt | A concrete payment interaction with a gateway for an order. |
| Refund | Full or partial reversal of captured value. |
| Chargeback | A payment dispute initiated outside the platform. |
| Inventory Reservation | A temporary hold on stock for a physical order. |
| Outbox Event | A database-persisted event that is published to Kafka after transaction commit. |
| Idempotency Key | A client or server-generated key used to make repeated requests safe. |
| Admin Override | A privileged operator action that bypasses normal business automation but remains audited. |

## 6. Actor Matrix

| Actor | Capabilities | Restrictions |
|---|---|---|
| Guest | Browse books, search catalog, view plans, register, login | Cannot buy, read, download, or access account data |
| Registered Customer | Buy books, manage cart, manage address, pay online, view orders, read owned books, download owned books | Cannot access membership-only rights without active membership |
| Member | All customer actions plus read/download eligible membership books while membership is active | Cannot retain new membership download rights after membership expiry |
| Admin Operator | Manage catalog, pricing, plans, users, orders, refunds, payments, inventory, shipment, reports, audit logs, notifications, overrides | Must be RBAC-controlled and fully audited |
| Scheduler / Worker | Process background tasks, retry jobs, reconcile payments, send emails, update statuses | No UI access; only machine credentials and scoped permissions |

## 7. Domain Model

### 7.1 Core bounded modules
- Identity
- Catalog
- Pricing
- Membership
- Cart
- Checkout
- Order
- Payment
- Refund
- Chargeback
- Entitlement
- Reader
- Inventory
- Shipment
- Notification
- Audit
- Reporting
- E-invoice Export
- Admin
- Scheduler/Worker
- Integration

### 7.2 Aggregate overview

| Aggregate | Description | Primary ownership |
|---|---|---|
| User | Identity, profile, account state | Identity |
| Session | Access/refresh context, revocation | Identity |
| Device | Device registration and cap enforcement | Identity/Entitlement |
| Book | Commercial and reading metadata | Catalog |
| MembershipPlan | Plan configuration and quotas | Membership |
| MembershipSubscription | User membership lifecycle | Membership |
| Cart | Pre-order mutable selection | Cart |
| Order | Purchase commitment | Order |
| Payment | Gateway-facing transaction state | Payment |
| Refund | Money reversal state | Refund |
| ChargebackCase | Dispute and evidence state | Chargeback |
| Entitlement | Read/download right | Entitlement |
| ReaderSession | Active online read state | Reader |
| DownloadRecord | Download attempts and completions | Entitlement |
| InventoryItem | Current physical stock summary | Inventory |
| InventoryReservation | Temporary stock hold | Inventory |
| Shipment | Delivery fulfillment | Shipment |
| NotificationJob | Outbound communication request | Notification |
| AuditLog | Immutable activity trail | Audit |
| OutboxEvent | Event relay record | Integration |

## 8. Business Rules

### 8.1 Product rules
1. A book may be physical-only, digital-only, or hybrid.
2. A digital title may expose PDF, EPUB, or both.
3. A book can be marked as membership-eligible or not.
4. Membership never unlocks physical-book fulfillment.
5. A user may still buy a physical book even if membership is active.
6. A user who buys an ebook individually receives permanent commercial access to that ebook unless revoked due to refund, chargeback loss, fraud, or admin enforcement.

### 8.2 Membership rules
1. Membership plans are sold in monthly and yearly durations.
2. Membership grants read and download rights only for titles flagged as membership-eligible.
3. Membership benefits are active only within the subscription validity window.
4. On expiry, previously issued membership download links become unusable, and no new membership-based download link may be issued.
5. The user may continue to keep previously downloaded local files because the system does not implement device-enforced DRM or remote file revocation.[web:64][web:66]
6. Membership may be renewed or extended.
7. Membership stacking policy is allowed and extends end date from the current expiry if still active.

### 8.3 Ebook purchase rules
1. A customer who buys an ebook individually can read it online in all purchased formats.
2. A customer who buys an ebook individually can download it, subject to download, device, and session policies.
3. Ebook downloads are always mediated by signed temporary links.
4. Download history must remain visible even if future downloads are disabled.

### 8.4 COD rules
1. COD is supported only for physical-only orders.
2. COD must not be offered for digital-only orders.
3. COD must not be offered for mixed digital and physical orders in phase 1.
4. Inventory is reserved only after COD order confirmation.
5. COD payment is considered collected only when shipment reaches a delivered-and-collected business state.
6. COD refusal, failed delivery, or return-to-origin must update shipment and order states and may trigger stock adjustments.
7. COD remains commercially important in Vietnam and requires explicit handling for failed delivery and completion risk.[web:54][web:71]

### 8.5 Payment rules
1. Frontend redirect success is not sufficient proof of payment success.
2. Gateway webhooks and reconciliation checks are the authoritative payment confirmation sources.
3. All payment callbacks and retries must be idempotent.
4. The same order may have multiple payment attempts but only one terminal commercial outcome.
5. Order totals are always computed server-side.

### 8.6 Refund and chargeback rules
1. Full and partial refunds are supported.
2. Refund eligibility depends on item type, fulfillment state, and policy.
3. Membership refund may revoke or shorten rights depending on refund timing and policy.
4. Chargebacks create a dispute case and may freeze or revoke related entitlement.
5. Admins may override default refund paths, but all overrides must be audited.

### 8.7 Shipping and inventory rules
1. One order maps to one shipment.
2. Physical inventory must track on-hand, reserved, and available quantities.
3. Inventory reservation must be released if payment fails or an order is cancelled before shipment.
4. Shipment carrier integration is abstracted in phase 1 and supports webhook/polling contracts for future providers.

### 8.8 Security and access rules
1. Email verification is required before sensitive digital actions such as download.
2. Session concurrency limits and device limits must be enforced.
3. Family sharing is not supported.
4. Admin overrides require elevated permissions and audit logging.
5. Signed links must be short-lived.
6. Abuse patterns such as repeated download-link generation, excessive login attempts, and coupon abuse must be rate-limited.

## 9. Sequence Flows

### 9.1 User registration and verification
1. Guest submits registration details.
2. System validates uniqueness and password policy.
3. System creates user in unverified state.
4. System emits verification-email command via Kafka outbox.
5. Email worker sends verification email.
6. User clicks verification link.
7. System marks email as verified.

### 9.2 Physical-only COD order
1. User adds physical books to cart.
2. System validates stock and address.
3. User selects COD.
4. System creates order with COD-eligible state.
5. System reserves stock.
6. System creates shipment in pending state.
7. Admin/ops process shipment.
8. Carrier delivers and COD is collected.
9. System marks order as paid and fulfilled.

### 9.3 Online paid ebook order
1. User adds ebook to cart.
2. System validates ownership conflicts and reprices cart.
3. User chooses payment gateway.
4. System creates order and payment attempt.
5. User is redirected to gateway.
6. Gateway returns callback/webhook.
7. System verifies authenticity and transitions payment.
8. On successful capture, order becomes paid.
9. Entitlement is granted in the same business transaction or through immediate reliable post-payment process.
10. System emits email and audit events.

### 9.4 Membership purchase
1. User selects membership plan.
2. System validates plan status and pricing.
3. Payment flow completes.
4. Membership subscription is created or extended.
5. Entitlement engine starts considering membership rights immediately.
6. Email confirmation is sent asynchronously.

### 9.5 Download request
1. Authenticated user requests a download for a book and format.
2. System verifies email, session, device, entitlement, quota, and policy.
3. System creates download request record.
4. System issues a short-lived signed link.
5. User consumes the link.
6. System records token use and completion when observable.
7. If membership expires later, existing link records remain visible in history, but new membership-based links cannot be generated.

### 9.6 Refund workflow
1. Refund request is triggered by admin or rule-based workflow.
2. System validates item-level refund eligibility.
3. Refund request is sent to payment gateway when applicable.
4. Gateway webhook/reconciliation confirms refund status.
5. System updates refund state, order state, and item states.
6. Entitlement and shipment/inventory side effects are applied.
7. Email and audit events are emitted.

## 10. State Models (Table Form)

### 10.1 Order states

| State | Meaning | Allowed transitions |
|---|---|---|
| draft | Cart converted but not submitted | pending_payment, cancelled |
| pending_payment | Awaiting payment initiation or completion | payment_processing, cancelled, expired |
| payment_processing | Awaiting callback/reconciliation | paid, failed, cancelled |
| paid | Monetary commitment confirmed | partially_fulfilled, fulfilled, refund_pending, partially_refunded, refunded, chargeback_open |
| partially_fulfilled | Some but not all lines fulfilled | fulfilled, refund_pending, partially_refunded |
| fulfilled | All lines fulfilled | refund_pending, partially_refunded, refunded, chargeback_open |
| cancel_requested | Cancellation under review | cancelled, paid |
| cancelled | Terminal canceled state | none |
| failed | Terminal failure state | none |
| refund_pending | Refund initiated | partially_refunded, refunded |
| partially_refunded | Some amount reversed | refunded, chargeback_open |
| refunded | Full amount reversed | none |
| chargeback_open | Dispute active | chargeback_won, chargeback_lost |
| chargeback_won | Merchant wins dispute | paid, fulfilled, partially_refunded |
| chargeback_lost | Merchant loses dispute | refunded, failed |

### 10.2 Payment states

| State | Meaning | Allowed transitions |
|---|---|---|
| initiated | Payment attempt created | pending, failed, cancelled |
| pending | Waiting for user or gateway result | authorized, captured, failed, expired, cancelled |
| authorized | Funds authorized only | captured, cancelled, expired |
| captured | Payment complete | refunded_partial, refunded_full, chargeback_open |
| failed | Payment failed | none |
| expired | Payment timed out | none |
| cancelled | Payment intentionally canceled | none |
| refunded_partial | Partial reversal | refunded_full, chargeback_open |
| refunded_full | Full reversal | none |
| chargeback_open | External dispute | chargeback_resolved |
| chargeback_resolved | Dispute closed | none |

### 10.3 Membership states

| State | Meaning | Allowed transitions |
|---|---|---|
| pending_activation | Paid but not yet activated | active, cancelled |
| active | Valid membership | grace, expired, revoked |
| grace | Optional grace period | active, expired |
| expired | Naturally ended | renewed |
| renewed | Transitional extension event | active |
| revoked | Force-terminated | none |
| cancelled | Purchase reversed before activation | none |

### 10.4 Entitlement states

| State | Meaning | Allowed transitions |
|---|---|---|
| active | Can read/download under policy | suspended, expired, revoked |
| suspended | Temporarily disabled | active, revoked |
| expired | Time-based right ended | none |
| revoked | Removed due to refund, chargeback, fraud, or admin | none |

### 10.5 Shipment states

| State | Meaning | Allowed transitions |
|---|---|---|
| pending_pack | Order approved but not packed | packed, cancelled |
| packed | Packed and ready | awaiting_pickup, cancelled |
| awaiting_pickup | Waiting carrier handoff | in_transit, cancelled |
| in_transit | Moving through carrier network | delivered, delivery_failed, returning |
| delivered | Completed delivery | none |
| delivery_failed | Failed attempt | in_transit, returning, cancelled |
| returning | Return to origin in progress | returned |
| returned | Returned to warehouse | cancelled, refund_pending |
| cancelled | Shipment canceled | none |

## 11. Public API Surface

### 11.1 Auth and account
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/verify-email`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/forgot-password`
- `POST /api/v1/auth/reset-password`
- `GET /api/v1/me`
- `GET /api/v1/me/sessions`
- `DELETE /api/v1/me/sessions/{sessionId}`
- `GET /api/v1/me/devices`
- `DELETE /api/v1/me/devices/{deviceId}`

### 11.2 Catalog and search
- `GET /api/v1/books`
- `GET /api/v1/books/{bookId}`
- `GET /api/v1/books/{bookId}/formats`
- `GET /api/v1/authors`
- `GET /api/v1/categories`
- `GET /api/v1/search/books`
- `GET /api/v1/membership/plans`

### 11.3 Cart and checkout
- `GET /api/v1/cart`
- `POST /api/v1/cart/items`
- `PATCH /api/v1/cart/items/{itemId}`
- `DELETE /api/v1/cart/items/{itemId}`
- `POST /api/v1/cart/coupon`
- `DELETE /api/v1/cart/coupon`
- `POST /api/v1/checkout/preview`
- `POST /api/v1/checkout/orders`

### 11.4 Orders and payments
- `GET /api/v1/orders`
- `GET /api/v1/orders/{orderId}`
- `POST /api/v1/orders/{orderId}/cancel`
- `POST /api/v1/orders/{orderId}/pay`
- `GET /api/v1/payments/{paymentId}`
- `POST /api/v1/payments/webhooks/{provider}`

### 11.5 Membership and library
- `GET /api/v1/me/membership`
- `GET /api/v1/me/library`
- `GET /api/v1/me/downloads`

### 11.6 Reader and download
- `POST /api/v1/reader/sessions`
- `POST /api/v1/reader/sessions/{sessionId}/heartbeat`
- `DELETE /api/v1/reader/sessions/{sessionId}`
- `GET /api/v1/books/{bookId}/read`
- `POST /api/v1/books/{bookId}/download-links`

### 11.7 Address and shipment view
- `GET /api/v1/me/addresses`
- `POST /api/v1/me/addresses`
- `PATCH /api/v1/me/addresses/{addressId}`
- `DELETE /api/v1/me/addresses/{addressId}`
- `GET /api/v1/orders/{orderId}/shipment`

## 12. Admin API Surface

### 12.1 Catalog admin
- `GET /api/v1/admin/books`
- `POST /api/v1/admin/books`
- `GET /api/v1/admin/books/{bookId}`
- `PATCH /api/v1/admin/books/{bookId}`
- `POST /api/v1/admin/books/{bookId}/publish`
- `POST /api/v1/admin/books/{bookId}/unpublish`
- `GET /api/v1/admin/authors`
- `POST /api/v1/admin/authors`
- `GET /api/v1/admin/categories`
- `POST /api/v1/admin/categories`

### 12.2 Membership admin
- `GET /api/v1/admin/membership/plans`
- `POST /api/v1/admin/membership/plans`
- `PATCH /api/v1/admin/membership/plans/{planId}`
- `POST /api/v1/admin/users/{userId}/membership/grant`
- `POST /api/v1/admin/users/{userId}/membership/revoke`
- `POST /api/v1/admin/users/{userId}/membership/extend`

### 12.3 Order, payment, refund, chargeback admin
- `GET /api/v1/admin/orders`
- `GET /api/v1/admin/orders/{orderId}`
- `POST /api/v1/admin/orders/{orderId}/cancel`
- `POST /api/v1/admin/orders/{orderId}/refunds`
- `GET /api/v1/admin/payments`
- `POST /api/v1/admin/payments/{paymentId}/reconcile`
- `GET /api/v1/admin/chargebacks`
- `POST /api/v1/admin/chargebacks/{chargebackId}/resolve`

### 12.4 Inventory and shipment admin
- `GET /api/v1/admin/inventory`
- `PATCH /api/v1/admin/inventory/{bookId}`
- `GET /api/v1/admin/reservations`
- `GET /api/v1/admin/shipments`
- `POST /api/v1/admin/shipments/{shipmentId}/status`
- `POST /api/v1/admin/shipments/{shipmentId}/carrier-sync`

### 12.5 User and entitlement admin
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/users/{userId}`
- `POST /api/v1/admin/users/{userId}/lock`
- `POST /api/v1/admin/users/{userId}/unlock`
- `POST /api/v1/admin/users/{userId}/force-logout`
- `POST /api/v1/admin/users/{userId}/entitlements/grant`
- `POST /api/v1/admin/users/{userId}/entitlements/revoke`
- `GET /api/v1/admin/users/{userId}/sessions`
- `GET /api/v1/admin/users/{userId}/devices`

### 12.6 Audit, reporting, notification, invoice export
- `GET /api/v1/admin/audit-logs`
- `GET /api/v1/admin/reports/sales`
- `GET /api/v1/admin/reports/memberships`
- `GET /api/v1/admin/reports/downloads`
- `POST /api/v1/admin/notifications/email`
- `GET /api/v1/admin/e-invoice/exports`
- `POST /api/v1/admin/e-invoice/exports/{exportId}/retry`

## 13. Main Database Schema

### 13.1 Identity tables

| Table | Purpose | Key columns |
|---|---|---|
| users | User profile and status | id, email, email_verified_at, status, created_at |
| user_credentials | Password hash and auth metadata | user_id, password_hash, password_changed_at |
| user_sessions | Refresh/session registry | id, user_id, device_id, refresh_token_hash, ip, ua, expires_at, revoked_at |
| user_devices | Device registry | id, user_id, fingerprint, label, first_seen_at, last_seen_at, revoked_at |
| addresses | Shipping addresses | id, user_id, full_name, phone, province, district, ward, line1, line2 |

### 13.2 Catalog tables

| Table | Purpose | Key columns |
|---|---|---|
| books | Core catalog entity | id, slug, title, description, product_type, published, membership_eligible |
| book_formats | Digital/physical availability | id, book_id, format, asset_path, reader_mode, downloadable |
| authors | Author records | id, name, slug |
| categories | Category records | id, name, slug |
| book_authors | Many-to-many join | book_id, author_id |
| book_categories | Many-to-many join | book_id, category_id |
| prices | Price history or effective price table | id, subject_type, subject_id, list_price_vnd, sale_price_vnd, starts_at, ends_at |
| search_documents | FTS support if normalized separately | subject_type, subject_id, tsv |

### 13.3 Membership and entitlement tables

| Table | Purpose | Key columns |
|---|---|---|
| membership_plans | Plan definitions | id, code, name, duration_days, price_vnd, max_devices, max_concurrent_sessions, max_downloads |
| memberships | User subscription instances | id, user_id, plan_id, starts_at, expires_at, state, source_order_id |
| entitlements | Effective commercial rights | id, user_id, book_id, source_type, source_id, state, starts_at, expires_at |
| reader_sessions | Active reading sessions | id, user_id, book_id, device_id, started_at, last_heartbeat_at, ended_at |
| downloads | Download history and token use | id, user_id, book_id, format, source_type, token_id, requested_at, consumed_at, status |

### 13.4 Cart, order, payment tables

| Table | Purpose | Key columns |
|---|---|---|
| carts | Durable cart snapshot | id, user_id, state, coupon_code, expires_at |
| cart_items | Cart lines | id, cart_id, sku_type, sku_id, qty, price_snapshot_vnd |
| orders | Commercial order header | id, user_id, order_no, currency, subtotal_vnd, shipping_fee_vnd, discount_vnd, total_vnd, state |
| order_items | Order lines | id, order_id, sku_type, sku_id, item_state, qty, line_total_vnd |
| payments | Canonical payment object | id, order_id, provider, state, amount_vnd, external_ref |
| payment_attempts | Gateway attempts | id, payment_id, attempt_no, state, request_payload, response_payload |
| refunds | Refund records | id, order_id, payment_id, amount_vnd, state, reason_code |
| chargebacks | Dispute cases | id, payment_id, order_id, state, amount_vnd, opened_at, resolved_at |
| coupons | Coupon definitions | id, code, type, value_vnd, usage_limit, starts_at, ends_at |
| coupon_redemptions | Coupon usage history | id, coupon_id, user_id, order_id, redeemed_at |

### 13.5 Inventory and shipping tables

| Table | Purpose | Key columns |
|---|---|---|
| inventory_items | Stock summary | book_id, on_hand, reserved, available, version |
| inventory_reservations | Stock holds | id, order_id, book_id, qty, state, expires_at |
| shipments | Shipment header | id, order_id, carrier_code, tracking_no, state |
| shipment_status_logs | Shipment history | id, shipment_id, state, source, raw_payload, created_at |

### 13.6 Operations tables

| Table | Purpose | Key columns |
|---|---|---|
| notifications | Email job and delivery status | id, channel, template_code, recipient, state, attempts |
| audit_logs | Immutable audit trail | id, actor_type, actor_id, action, resource_type, resource_id, metadata, created_at |
| outbox_events | Guaranteed event publishing | id, topic, event_key, payload, state, created_at, published_at |
| e_invoice_exports | Invoice export staging | id, order_id, buyer_payload, invoice_payload, provider_state, provider_ref |
| processed_events | Consumer idempotency table | consumer_name, event_id, processed_at |

## 14. Kafka Event Catalog

### 14.1 Event envelope
Every Kafka message must include a common envelope:
- `event_id`
- `event_type`
- `aggregate_type`
- `aggregate_id`
- `occurred_at`
- `produced_at`
- `trace_id`
- `correlation_id`
- `causation_id`
- `actor_type`
- `actor_id`
- `schema_version`
- `idempotency_key`
- `payload`

### 14.2 Topics

| Topic | Purpose | Keying strategy |
|---|---|---|
| `order.events.v1` | Order state changes | order_id |
| `payment.events.v1` | Payment lifecycle | order_id or payment_id |
| `membership.events.v1` | Membership lifecycle | user_id |
| `entitlement.events.v1` | Grant/revoke/suspend | user_id |
| `inventory.events.v1` | Reservation and stock updates | book_id |
| `shipment.events.v1` | Shipment lifecycle | shipment_id |
| `notification.commands.v1` | Email job commands | recipient or logical business key |
| `scheduler.commands.v1` | Delayed or periodic commands | command key |
| `audit.events.v1` | Audit fan-out | resource_id |
| `reporting.events.v1` | Analytics/report aggregation | business key |
| `dlq.general.v1` | Dead-letter topic | original key |

### 14.3 Mandatory domain events

| Event | Produced by | Consumed by |
|---|---|---|
| `user.registered` | Identity | Notification, Audit |
| `user.email_verified` | Identity | Audit |
| `order.created` | Order | Audit, Reporting |
| `order.paid` | Payment/Order | Entitlement, Inventory, Notification, Invoice Export, Reporting |
| `order.cancelled` | Order | Inventory, Notification, Reporting |
| `payment.captured` | Payment | Order, Notification, Reporting |
| `payment.failed` | Payment | Notification, Reporting |
| `payment.refund_requested` | Refund | Payment adapter worker, Audit |
| `payment.refunded` | Payment | Order, Entitlement, Notification, Reporting |
| `membership.activated` | Membership | Notification, Reporting |
| `membership.expired` | Scheduler/Membership | Entitlement, Notification |
| `entitlement.granted` | Entitlement | Audit, Reporting |
| `entitlement.revoked` | Entitlement | Reader, Notification, Audit |
| `download.link_issued` | Entitlement | Audit, Reporting |
| `download.completed` | Entitlement/CDN callback layer if available | Reporting |
| `inventory.reserved` | Inventory | Audit |
| `inventory.released` | Inventory | Audit |
| `shipment.status_changed` | Shipment | Notification, Reporting |
| `invoice.export_requested` | Order/Invoice Export | E-invoice integration worker |
| `invoice.export_failed` | Invoice Export | Admin alert/reporting |

### 14.4 Kafka processing rules
- Events are published only after DB commit through the outbox pattern.[web:40][web:43]
- Consumers must be idempotent using `processed_events` or Redis dedup.
- Consumers must support retry with bounded attempts and DLQ fallback.
- Side effects such as email sending and cache invalidation must tolerate duplicate delivery.
- Scheduler does not execute domain mutations directly; it produces commands that trigger normal service flows.

## 15. Redis Design

### 15.1 Redis objectives
Redis is used to reduce latency, reduce direct database load, provide concurrency-friendly primitives, and support high-churn ephemeral state. PostgreSQL remains the source of truth for durable business state.

### 15.2 Primary Redis use cases
- Catalog and book detail caching.
- Search-result caching.
- Cart caching.
- Session and token hot-path lookup.
- Device registry hot-path checks.
- Rate limiting.
- Idempotency key storage.
- Distributed locks.
- Flash-sale stock guard.
- Job deduplication.
- Short-lived download token metadata.

### 15.3 Key design

| Key pattern | Purpose | TTL |
|---|---|---|
| `catalog:list:{hash}` | Cached list results | 5–15 min |
| `book:detail:{book_id}` | Book detail cache | 10–30 min |
| `search:books:{hash}` | Search cache | 1–5 min |
| `cart:{user_id}` | Hot cart snapshot | 1–7 days or active sliding TTL |
| `session:{session_id}` | Session metadata | aligned to session expiry |
| `user:sessions:{user_id}` | Active sessions index | aligned to latest session expiry |
| `device:{user_id}:{device_id}` | Device registry hot lookup | long TTL or explicit invalidation |
| `idem:{scope}:{key}` | Idempotency result | 5 min to 24 h by scope |
| `lock:{resource}` | Distributed lock | short TTL, seconds |
| `quota:download:{user_id}:{book_id}` | Download counters | policy-based |
| `rate:{scope}:{subject}` | Rate limiting bucket | seconds to minutes |
| `jobdedup:{job_type}:{biz_key}` | Worker dedup | policy-based |
| `dltoken:{token_id}` | Download token metadata | short TTL |

### 15.4 Redis rules
- Cache-aside strategy for catalog data.
- Event-driven invalidation after admin mutations.
- Redis outage must degrade to PostgreSQL for correctness-critical flows where possible.
- Distributed locks must be used only for short critical sections.
- Do not store canonical order/payment/refund state only in Redis.
- Quota increments and compare-and-set operations must be atomic.

## 16. NFR (Non-Functional Requirements)

### 16.1 Performance
- p95 for typical authenticated API calls without external gateway dependency should be under 300 ms.
- p95 for cached catalog/detail/search endpoints should target under 100 ms.
- Cart operations should remain responsive under moderate concurrent access.
- Reader session heartbeat should be lightweight and scalable.

### 16.2 Availability and resilience
- System should degrade gracefully if Redis is unavailable.
- Kafka outages must not corrupt primary transactional state; outbox backlog is acceptable.
- Background retries must prevent data loss for notifications and provider-sync tasks.
- Payment uncertainty must be reconciled asynchronously.

### 16.3 Reliability
- Duplicate webhook delivery must not duplicate money capture, entitlement grant, or refund effects.
- Inventory reservation must be transactionally safe against overselling.
- Entitlement checks must be correct even when cache is stale.
- Audit logs must exist for privileged actions.

### 16.4 Maintainability
- Modular internal boundaries must be respected.
- Every module must expose service interfaces and domain events.
- Error codes must be standardized.
- API versioning must use `/api/v1` prefixing.

### 16.5 Compliance and retention
Vietnam requires e-invoice use by taxpayers and imposes operational and data obligations around electronic invoice handling, but this platform will externalize issuance while retaining the business data needed for provider integration.[web:48][web:53][web:62] Invoice export data and business transaction records must be retained according to company policy and legal obligations.[web:62]

### 16.6 Logging and observability
- Structured JSON logs.
- Correlation IDs and trace IDs across HTTP, DB, Kafka, and provider calls.
- Metrics for Redis hit rate, DB pool, HTTP latency, Kafka lag, payment success/failure, refund errors, entitlement failures, and notification retries.
- Health endpoints for liveness, readiness, and dependency readiness.

## 17. Security Requirements

### 17.1 Authentication and session security
- Passwords must be hashed using a strong password hashing algorithm such as Argon2id or a production-grade bcrypt configuration.
- Access tokens must be short-lived.
- Refresh tokens must be rotatable and revocable.
- Concurrent sessions must be observable and terminable by user and admin.

### 17.2 Authorization
- Public and admin APIs must be fully separated.
- Admin endpoints require RBAC and optional finer-grained permissions.
- Admin override actions must be audited with actor, resource, before/after context, and rationale.

### 17.3 Transport and secret protection
- TLS is mandatory in production.
- Provider secrets must be managed outside source code.
- Webhook verification secrets must be rotated safely.

### 17.4 Abuse controls
- Rate limiting by IP, endpoint, and user.
- Login brute-force protection.
- Download-link generation throttling.
- Coupon abuse detection and order-submission idempotency.
- Session and device anomaly logging.

### 17.5 Digital content protection posture
The platform intentionally does not implement watermarking or strong DRM. This means server-side access control can prevent issuing new links or revoke account-based access, but it cannot guarantee revocation of already downloaded local files, which is a known limitation of DRM-free distribution and weak browser-based protection models.[web:64][web:66]

## 18. Acceptance Criteria by Module

### 18.1 Identity module
- A guest can register with a unique email.
- Unverified users cannot perform protected digital download actions.
- Refresh token rotation invalidates previous refresh token chain according to policy.
- Users can list and revoke active sessions.
- Device registration is enforced and capped.

### 18.2 Catalog module
- Guests and users can list and search books with filters.
- Membership eligibility is visible in book detail and queryable in filters.
- Admin changes invalidate cached catalog data predictably.
- Search uses PostgreSQL full-text search and returns relevant results.

### 18.3 Cart module
- Cart supports physical books, ebooks, and membership plans.
- Duplicate-ineligible items are blocked according to policy.
- Server recalculates totals before checkout.
- Idempotent cart updates do not create duplicates under retry.

### 18.4 Checkout and order module
- Physical-only cart can choose COD.
- Digital-only or mixed digital cart cannot choose COD.
- Online-payment orders enter pending payment state correctly.
- Successful payment transitions order to paid.
- Failed or expired payments do not create entitlement.

### 18.5 Payment module
- Stripe, MoMo, VNPay, and PayPal integrations can create payment attempts.
- Duplicate gateway callbacks do not double-transition payment state.
- Reconciliation job can resolve uncertain payment states.
- Payment state changes are auditable.

### 18.6 Membership module
- User can purchase monthly or yearly plans.
- Membership extends from current expiry if already active.
- Membership unlocks only books flagged as membership-eligible.
- Membership expiry stops new membership-based downloads.

### 18.7 Entitlement and reader module
- Individually purchased ebooks are readable online.
- Membership-eligible ebooks are readable online while membership is active.
- Download link issuance checks entitlement, device, session, quota, and verification.
- Reader concurrency limit is enforced.
- Last read position is persisted for EPUB and PDF reader context where applicable.

### 18.8 Inventory and shipment module
- Stock is reserved on eligible order confirmation.
- Reservation is released on cancellation or payment failure.
- Shipment state changes are tracked in history.
- COD-delivered shipment marks order as paid according to COD completion rule.

### 18.9 Refund and chargeback module
- Full and partial refunds are possible where policy allows.
- Refund completion updates financial and entitlement state.
- Chargeback case creation freezes or adjusts rights according to policy.
- All override actions are audited.

### 18.10 Notification module
- Email jobs are produced asynchronously.
- Worker retries transient failures.
- Duplicate event delivery does not send duplicate emails when dedup policy applies.

### 18.11 Audit module
- Every admin override generates an immutable audit log.
- Sensitive business transitions produce searchable audit entries.
- Audit logs can be filtered by actor, resource, date range, and action.

### 18.12 Integration, Kafka, and Redis module
- Transactional outbox guarantees that committed business events are eventually published.
- Kafka consumers are idempotent.
- DLQ is populated after retry exhaustion.
- Redis cache misses do not break correctness.
- Idempotency and rate limiting work under concurrent load.

## 19. Testing Strategy Requirements

### 19.1 Unit testing
- Domain services for pricing, entitlement, membership extension, refund eligibility, and order policy.
- Redis utility logic and key policy logic.
- Payment adapter mappers.

### 19.2 Integration testing
- PostgreSQL transaction flows for checkout, payment finalization, inventory reservation, entitlement grant, refund.
- Redis-integrated quota and idempotency checks.
- Kafka outbox publishing and consumer idempotency.
- Webhook verification and duplicate callback handling.

### 19.3 End-to-end and scenario testing
- Register → verify → buy ebook → read → download.
- Buy membership → read eligible book → download → expire membership.
- Physical-only COD order → ship → deliver → mark paid.
- Online payment success and failed reconciliation cases.
- Full refund and partial refund cases.
- Chargeback case handling.

### 19.4 Performance and resilience testing
- Search and catalog cache hit load.
- Concurrent checkout on limited stock.
- Reader heartbeat at scale.
- Redis failure fallback.
- Kafka backlog and recovery behavior.

## 20. Open Extension Paths

### 20.1 Planned future evolution
- Elasticsearch/OpenSearch for advanced search.
- Real carrier integrations.
- More notification channels.
- Stronger content protection if business later chooses DRM tooling.
- Advanced recommendation and reporting pipelines.
- Event streaming to external consumers.

### 20.2 Migration safety
The current design intentionally favors a modular monolith so that business rules remain centralized while async infrastructure is introduced through Kafka. This preserves implementation simplicity in the early stages while still allowing eventual extraction of specific modules if scaling or team structure later demands it.[web:37][web:43][web:46]
