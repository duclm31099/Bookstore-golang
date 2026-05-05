# Identity Module Implementation - Master TODO List

> **Version:** 1.0 (Hyper-Detailed)  
> **Principles:** Clean Architecture, DDD, Event-Driven (Outbox), CQRS Read-Side, Zero-Trust Security.

---

## 🟥 PHASE 1: SECURITY & AUTHENTICATION REFINEMENT

### 1.1 Password Reset Flow (Quên mật khẩu)
- [ ] **Domain**: 
    - [ ] Định nghĩa `PasswordResetPolicy` (TTL, độ phức tạp mật khẩu mới).
- [ ] **Application**:
    - [ ] Implement `RequestPasswordResetCommand`: Sinh token ngẫu nhiên, lưu Redis (TTL 15p), bắn event `auth.password_reset_requested`.
    - [ ] Implement `ResetPasswordCommand`: Verify token, Hash mật khẩu mới, cập nhật `user_credentials`, **thu hồi toàn bộ sessions cũ**.
- [ ] **HTTP**:
    - [ ] Endpoint `POST /api/v1/auth/forgot-password` (Idempotent).
    - [ ] Endpoint `POST /api/v1/auth/reset-password`.

### 1.2 Multi-Factor Authentication (MFA) - Future Proofing
- [ ] **Domain**: Bổ sung `MFASecret`, `MFAEnabled` vào `User` entity.
- [ ] **App**: Logic verify TOTP token.

### 1.3 Security Hardening
- [ ] **Brute-force Protection**: 
    - [ ] Cập nhật `Login` use case: Nếu sai pass > 5 lần, chuyển `account_status` sang `locked` và bắn event `user.account_locked`.
- [ ] **JWT Blacklisting**: Triển khai cơ chế thu hồi Access Token qua Redis (cho logout/revoke ngay lập tức).

---

## 🟧 PHASE 2: PROFILE & ACCOUNT MANAGEMENT (APIs)

### 2.1 Profile Handlers
- [ ] **GET `/api/v1/me`**:
    - [ ] Fix bug `QueryRepository`: Bổ sung các field `Phone`, `UserType`, `Metadata`.
- [ ] **PUT `/api/v1/me`**:
    - [ ] Use Case `UpdateProfile`: Cho phép đổi `FullName`, `Phone`, `Metadata`.
    - [ ] **Enforce Optimistic Locking**: Phải truyền `version` từ client lên để check.

### 2.2 Session & Device Management (Audit-ability)
- [ ] **GET `/api/v1/me/sessions`**: Liệt kê các session đang active (thông tin từ bảng `user_sessions`).
- [ ] **DELETE `/api/v1/me/sessions/:id`**: Thu hồi một session cụ thể.
- [ ] **DELETE `/api/v1/me/sessions`**: Thu hồi toàn bộ session (Logout từ mọi nơi).
- [ ] **GET `/api/v1/me/devices`**: Liệt kê thiết bị đã tin cậy.
- [ ] **DELETE `/api/v1/me/devices/:id`**: Xóa thiết bị khỏi danh sách tin cậy.

---

## 🟨 PHASE 3: ADDRESS MANAGEMENT SYSTEM (Hyper-Detailed CRUD)

Tuân thủ nghiêm ngặt Invariant: **Chỉ duy nhất 1 địa chỉ là `is_default = true`**.

### 3.1 Domain & Infra
- [ ] **Address Repository**: 
    - [ ] Implement `UnsetDefaultExcept(ctx, userID, addressID)` dùng trong Transaction.
    - [ ] Implement `CountActiveByUserID(ctx, userID)` (Giới hạn tối đa 10 địa chỉ).

### 3.2 HTTP Endpoints
- [ ] **POST `/api/v1/me/addresses`**:
    - [ ] Validation: Province/District/Ward code phải hợp lệ.
    - [ ] Idempotency: Chống tạo trùng địa chỉ khi mạng lag.
- [ ] **PUT `/api/v1/me/addresses/:id`**: Cập nhật thông tin địa chỉ.
- [ ] **DELETE `/api/v1/me/addresses/:id`**: Xóa địa chỉ (Nếu là địa chỉ mặc định, tự động gán địa chỉ cũ nhất còn lại làm mặc định).
- [ ] **PATCH `/api/v1/me/addresses/:id/default`**: Đặt làm mặc định.

---

## 🟩 PHASE 4: TRANSACTIONAL OUTBOX & EVENT CONSISTENCY

### 4.1 Dispatcher Implementation (`cmd/worker`)
- [ ] **Polling Mechanism**: 
    - [ ] Dùng `FOR UPDATE SKIP LOCKED` để lấy 20 events mỗi batch.
    - [ ] Publish lên Kafka với retry logic (Exponential Backoff).
- [ ] **Event Schema Registry**: Đảm bảo payload của mọi event (`user.registered`, `user.updated`, `auth.password_changed`) tuân thủ đúng spec trong `docs/specs/kafka_spec.md`.

### 4.2 Cleanup Worker
- [ ] **Job**: Tự động xóa các bản ghi `processed_events` trong Redis/DB sau 7 ngày để tránh phình database.

---

## 🟦 PHASE 5: OBSERVABILITY & QUALITY (Master Mindset)

### 5.1 Monitoring
- [ ] **Metrics (Prometheus)**:
    - [ ] `identity_registration_total`: Số lượng đăng ký.
    - [ ] `identity_login_failures_total`: Theo dõi dấu hiệu tấn công Brute-force.
    - [ ] `outbox_pending_events_count`: Theo dõi độ trễ của Worker.
- [ ] **Tracing (OpenTelemetry)**: Gắn `TraceID` xuyên suốt từ HTTP Middleware -> Service -> Outbox -> Kafka.

### 5.2 Automated Testing
- [ ] **Unit Test**: Phủ 100% logic trong `domain/policy`.
- [ ] **Integration Test**: 
    - [ ] Flow: Register -> Verify Email -> Login -> Refresh Token.
    - [ ] Flow: Concurrent address updates (Test Concurrency).

---

## 🚀 ROADMAP SUMMARY
1. [ ] Hoàn thiện 100% API (Sprint 2 & 3).
2. [ ] Chạy thực tế Outbox Worker trong `cmd/worker`.
3. [ ] Viết bộ Integration Test Suite.
4. [ ] Security Audit (Review JWT & Bcrypt configuration).
