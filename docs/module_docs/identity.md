# Module Identity — Schema & API Specification

> **Phiên bản:** v1.1  
> **Cập nhật:** 2026-04-27  
> **Stack:** Go (pgx/v5), PostgreSQL 15+, JWT (HS256), Bcrypt

---

## Mục lục

1. [Tổng quan kiến trúc](#1-tổng-quan-kiến-trúc)
2. [Database Schema](#2-database-schema)
3. [Domain Model](#3-domain-model)
4. [Value Objects & Enums](#4-value-objects--enums)
5. [Domain Policies](#5-domain-policies)
6. [Repository Interfaces](#6-repository-interfaces)
7. [Query Layer (CQRS Read-side)](#7-query-layer-cqrs-read-side)
8. [Application Services (Use Cases)](#8-application-services-use-cases)
9. [Port Adapters (infra/adapters)](#9-port-adapters-infraadapters)
10. [Dependency Injection & Wire](#10-dependency-injection--wire)
11. [HTTP API Specification](#11-http-api-specification)
12. [Error Catalogue](#12-error-catalogue)
13. [Platform Dependencies](#13-platform-dependencies)
14. [Ghi chú & Known Issues](#14-ghi-chú--known-issues)

---

## 1. Tổng quan kiến trúc

Module **identity** được tổ chức theo **Modular Monolith** với kiến trúc phân lớp rõ ràng:

```
internal/modules/identity/
├── domain/               ← Core business logic, entities, repo interfaces, policies
│   ├── entity/
│   ├── value_object/
│   ├── policy/
│   ├── error/
│   └── repository.go
├── app/                  ← Application layer (use cases)
│   ├── command/
│   ├── query/
│   ├── service/          ← ✅ AuthService, ProfileService, AddressService
│   ├── ports/            ← ✅ 5 port interfaces
│   └── dto/
├── infra/
│   ├── postgres/         ← Repository implementations
│   │   ├── rows.go / scan.go / mapper.go
│   │   ├── *_repo.go (5 repos + query_repo)
│   │   └── provider.go
│   └── adapters/         ← ✅ Bridge layer (identity-specific adapters + thin wrappers)
│       ├── log_event_publisher.go  ← EventPublisher (identity domain event)
│       ├── real_clock.go           ← Clock
│       └── provider.go             ← wire.ProviderSet + jwtTokenManagerBridge
└── http/                 ← HTTP handlers (chưa implement)

internal/platform/auth/               ← ✅ Platform-level auth implementations
├── auth.go               ← *Auth (JWT validate cho HTTP middleware)
├── bcrypt_hasher.go      ← BcryptHasher (password hashing)
├── jwt_token_manager.go  ← JWTTokenManager + JWTClaims
├── redis_verify_token.go ← RedisVerificationTokenService
└── rand.go               ← generateSecureToken() helper
```

### Nguyên tắc thiết kế

| Nguyên tắc                           | Áp dụng                                                                                                                        |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| **Domain không biết DB**             | Repo interface ở domain, implementation ở infra                                                                                |
| **Transaction boundary ở App layer** | `tx.TxManager.WithinTransaction()` → `tx.GetExecutor(ctx, pool)`                                                               |
| **Ports & Adapters**                 | App layer phụ thuộc vào interface (`ports/`), infra cung cấp implementation                                                    |
| **Platform vs Module separation**    | Logic không gắn business (crypto, JWT, Redis token) ở `platform/auth/`; chỉ adapters identity-specific mới ở `infra/adapters/` |
| **Credential tách riêng**            | `Credential` entity độc lập, không bao giờ serialize ra ngoài                                                                  |
| **Refresh token không lưu raw**      | Chỉ lưu hash SHA-256(random hex 32 bytes) vào DB                                                                               |
| **Verification token single-use**    | Redis `GetDel` đảm bảo token chỉ dùng được 1 lần                                                                               |
| **Anti-corruption boundary**         | `rows.go` tách biệt DB shape khỏi domain entity                                                                                |
| **CQRS read-side**                   | `QueryRepository` trả view model, không qua domain entity                                                                      |
| **Clock injectable**                 | `ports.Clock` thay cho `time.Now()` trực tiếp → testable                                                                       |

---

## 2. Database Schema

### 2.1 Bảng `users`

**Migration:** `000002_create_user_table.up.sql`

```sql
CREATE TABLE IF NOT EXISTS users (
  id               BIGSERIAL PRIMARY KEY,
  email            VARCHAR(255) NOT NULL UNIQUE,
  full_name        VARCHAR(255) NOT NULL,
  phone            VARCHAR(20),
  user_type        VARCHAR(30)  NOT NULL DEFAULT 'customer'
                     CHECK (user_type IN ('customer', 'admin', 'operator')),
  account_status   VARCHAR(30)  NOT NULL DEFAULT 'pending_verification'
                     CHECK (account_status IN ('pending_verification', 'active', 'locked', 'disabled')),
  email_verified_at TIMESTAMPTZ,
  last_login_at    TIMESTAMPTZ,
  locked_reason    VARCHAR(255),
  metadata         JSONB        DEFAULT '{}'::jsonb,
  version          BIGINT       NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

**Indexes:**

```sql
CREATE INDEX idx_users_account_status    ON users(account_status);
CREATE INDEX idx_users_user_type         ON users(user_type);
CREATE INDEX idx_users_email_verified_at ON users(email_verified_at);
```

**Trigger:** `trg_users_update_updated_at` — tự động set `updated_at = NOW()` trước mỗi UPDATE.

| Column              | Type         | Nullable | Default                | Ghi chú                         |
| ------------------- | ------------ | -------- | ---------------------- | ------------------------------- |
| `id`                | BIGSERIAL    | NOT NULL | auto                   | PK                              |
| `email`             | VARCHAR(255) | NOT NULL | —                      | UNIQUE, normalized lowercase    |
| `full_name`         | VARCHAR(255) | NOT NULL | —                      |                                 |
| `phone`             | VARCHAR(20)  | NULL     | —                      |                                 |
| `user_type`         | VARCHAR(30)  | NOT NULL | `customer`             | enum: customer, admin, operator |
| `account_status`    | VARCHAR(30)  | NOT NULL | `pending_verification` | xem §4                          |
| `email_verified_at` | TIMESTAMPTZ  | NULL     | —                      | NULL = chưa verify              |
| `last_login_at`     | TIMESTAMPTZ  | NULL     | —                      |                                 |
| `locked_reason`     | VARCHAR(255) | NULL     | —                      |                                 |
| `metadata`          | JSONB        | NULL     | `{}`                   | Thông tin phụ trợ mở rộng       |
| `version`           | BIGINT       | NOT NULL | 1                      | Optimistic locking              |
| `created_at`        | TIMESTAMPTZ  | NOT NULL | NOW()                  |                                 |
| `updated_at`        | TIMESTAMPTZ  | NOT NULL | NOW()                  | auto-trigger                    |

> ✅ **Đã đồng bộ (v1.1):** Go constants được cập nhật để khớp với DB CHECK constraint:
>
> - `UserStatusBanned = "locked"` (thay vì `"banned"`)
> - `UserStatusSuspended = "disabled"` (thay vì `"suspended"`)

---

### 2.2 Bảng `user_credentials`

**Migration:** `000003_create_user_credentials_table.up.sql`

```sql
CREATE TABLE IF NOT EXISTS user_credentials (
  user_id              BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_hash        TEXT        NOT NULL,
  password_algo        VARCHAR(30) NOT NULL DEFAULT 'argon2id',
  password_changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  failed_login_count   INT         NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  last_failed_login_at TIMESTAMPTZ
);
```

| Column                 | Type        | Nullable | Default    | Ghi chú                               |
| ---------------------- | ----------- | -------- | ---------- | ------------------------------------- |
| `user_id`              | BIGINT      | NOT NULL | —          | PK + FK → users(id) ON DELETE CASCADE |
| `password_hash`        | TEXT        | NOT NULL | —          | bcrypt/argon2id hash, NEVER log       |
| `password_algo`        | VARCHAR(30) | NOT NULL | `argon2id` | Hỗ trợ mở rộng thuật toán             |
| `password_changed_at`  | TIMESTAMPTZ | NOT NULL | NOW()      | Dùng invalidate old sessions          |
| `failed_login_count`   | INT         | NOT NULL | 0          | Brute-force protection counter        |
| `last_failed_login_at` | TIMESTAMPTZ | NULL     | —          |                                       |

---

### 2.3 Bảng `user_devices`

**Migration:** `000004_create_user_devices_table.up.sql`

```sql
CREATE TABLE IF NOT EXISTS user_devices (
  id               BIGSERIAL PRIMARY KEY,
  user_id          BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  fingerprint_hash TEXT          NOT NULL,
  device_label     VARCHAR(255),
  first_seen_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
  last_seen_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
  revoked_at       TIMESTAMPTZ,
  revoked_reason   VARCHAR(255),
  metadata         JSONB         NOT NULL DEFAULT '{}'::jsonb,
  created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),

  UNIQUE(user_id, fingerprint_hash)
);
```

**Trigger:** `trg_user_devices_update_updated_at`

| Column             | Type         | Nullable | Default | Ghi chú                     |
| ------------------ | ------------ | -------- | ------- | --------------------------- |
| `id`               | BIGSERIAL    | NOT NULL | auto    | PK                          |
| `user_id`          | BIGINT       | NOT NULL | —       | FK → users(id)              |
| `fingerprint_hash` | TEXT         | NOT NULL | —       | Hash của device fingerprint |
| `device_label`     | VARCHAR(255) | NULL     | —       | Tên hiển thị device         |
| `first_seen_at`    | TIMESTAMPTZ  | NOT NULL | now()   | Lần đầu xuất hiện           |
| `last_seen_at`     | TIMESTAMPTZ  | NOT NULL | now()   | Lần cuối hoạt động          |
| `revoked_at`       | TIMESTAMPTZ  | NULL     | —       | NULL = chưa bị thu hồi      |
| `revoked_reason`   | VARCHAR(255) | NULL     | —       |                             |
| `metadata`         | JSONB        | NOT NULL | `{}`    |                             |
| `created_at`       | TIMESTAMPTZ  | NOT NULL | now()   |                             |
| `updated_at`       | TIMESTAMPTZ  | NOT NULL | now()   | auto-trigger                |

**Lưu ý:** Composite UNIQUE `(user_id, fingerprint_hash)` tự động tạo composite index — không cần index riêng cho `user_id`.  
**Policy:** Tối đa **5 devices** active per user (`DevicePolicy.MaxDevicesPerUser = 5`).

---

### 2.4 Bảng `user_sessions`

**Migration:** `000005_create_user_sesions_table.up.sql`

```sql
CREATE TABLE IF NOT EXISTS user_sessions (
  id                 BIGSERIAL    PRIMARY KEY,
  user_id            BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id          BIGINT       REFERENCES user_devices(id) ON DELETE SET NULL,
  refresh_token_hash TEXT         NOT NULL,
  session_status     VARCHAR(20)  NOT NULL DEFAULT 'active'
                       CHECK (session_status IN ('active', 'revoked', 'expired')),
  expires_at         TIMESTAMPTZ  NOT NULL,
  ip_address         INET,
  user_agent         TEXT,
  last_seen_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  revoked_at         TIMESTAMPTZ,
  revoked_reason     VARCHAR(255),
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

  UNIQUE(refresh_token_hash)
);
```

**Indexes:**

```sql
CREATE INDEX idx_user_sessions_id_status ON user_sessions(user_id, session_status);
CREATE INDEX idx_user_sessions_expires_at ON user_sessions(expires_at);
```

**Trigger:** `trg_user_sessions_update_updated_at`

| Column               | Type         | Nullable | Default  | Ghi chú                                  |
| -------------------- | ------------ | -------- | -------- | ---------------------------------------- |
| `id`                 | BIGSERIAL    | NOT NULL | auto     | PK                                       |
| `user_id`            | BIGINT       | NOT NULL | —        | FK → users(id)                           |
| `device_id`          | BIGINT       | NULL     | —        | FK → user_devices(id) ON DELETE SET NULL |
| `refresh_token_hash` | TEXT         | NOT NULL | —        | SHA-256 hash, UNIQUE                     |
| `session_status`     | VARCHAR(20)  | NOT NULL | `active` | active / revoked / expired               |
| `expires_at`         | TIMESTAMPTZ  | NOT NULL | —        | Thời điểm hết hạn session                |
| `ip_address`         | INET         | NULL     | —        | IP của client                            |
| `user_agent`         | TEXT         | NULL     | —        | Browser/app user agent                   |
| `last_seen_at`       | TIMESTAMPTZ  | NOT NULL | NOW()    |                                          |
| `revoked_at`         | TIMESTAMPTZ  | NULL     | —        | NULL = chưa bị thu hồi                   |
| `revoked_reason`     | VARCHAR(255) | NULL     | —        |                                          |
| `created_at`         | TIMESTAMPTZ  | NOT NULL | NOW()    |                                          |
| `updated_at`         | TIMESTAMPTZ  | NOT NULL | NOW()    | auto-trigger                             |

**Session lifetime:** Rotate mỗi lần refresh → `expires_at += 30 ngày` (xem `Session.Rotate()`).

---

### 2.5 Bảng `addresses`

**Migration:** `000006_create_address_table.up.sql`

```sql
CREATE TABLE IF NOT EXISTS addresses (
  id             BIGSERIAL    PRIMARY KEY,
  user_id        BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  address_line1  VARCHAR(255) NOT NULL,
  address_line2  VARCHAR(255),
  province_code  VARCHAR(20)  NOT NULL,
  district_code  VARCHAR(20)  NOT NULL,
  ward_code      VARCHAR(20)  NOT NULL,
  postal_code    VARCHAR(10),
  country_code   VARCHAR(10)  NOT NULL DEFAULT 'VN',
  is_default     BOOLEAN      NOT NULL DEFAULT false,
  version        BIGINT       NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

**Indexes:**

```sql
CREATE INDEX idx_addresses_user_id ON addresses(user_id);
-- Đảm bảo mỗi user chỉ có 1 địa chỉ mặc định
CREATE UNIQUE INDEX idx_addresses_unique_default ON addresses(user_id) WHERE is_default = true;
```

**Trigger:** `trg_addresses_update_updated_at`

| Column          | Type         | Nullable | Default | Ghi chú                                   |
| --------------- | ------------ | -------- | ------- | ----------------------------------------- |
| `id`            | BIGSERIAL    | NOT NULL | auto    | PK                                        |
| `user_id`       | BIGINT       | NOT NULL | —       | FK → users(id)                            |
| `address_line1` | VARCHAR(255) | NOT NULL | —       | Số nhà, tên đường                         |
| `address_line2` | VARCHAR(255) | NULL     | —       | Tầng, phòng...                            |
| `province_code` | VARCHAR(20)  | NOT NULL | —       | Mã tỉnh/thành                             |
| `district_code` | VARCHAR(20)  | NOT NULL | —       | Mã quận/huyện                             |
| `ward_code`     | VARCHAR(20)  | NOT NULL | —       | Mã phường/xã                              |
| `postal_code`   | VARCHAR(10)  | NULL     | —       |                                           |
| `country_code`  | VARCHAR(10)  | NOT NULL | `VN`    | ISO 3166-1 alpha-2                        |
| `is_default`    | BOOLEAN      | NOT NULL | false   | Partial unique index → max 1 default/user |
| `version`       | BIGINT       | NOT NULL | 1       | Optimistic locking                        |
| `created_at`    | TIMESTAMPTZ  | NOT NULL | NOW()   |                                           |
| `updated_at`    | TIMESTAMPTZ  | NOT NULL | NOW()   | auto-trigger                              |

---

### 2.6 ERD tóm tắt

```
users (1) ──────────────────────────── (1) user_credentials
  │
  ├── (1) ──── (N) user_devices
  │                  │
  ├── (1) ──── (N) user_sessions ──── (0..1) user_devices
  │
  └── (1) ──── (N) addresses
```

---

## 3. Domain Model

### 3.1 Entity `User`

```go
type User struct {
    ID              int64
    Email           valueobject.Email          // validated, normalized lowercase
    FullName        string
    Phone           *string                    // optional
    UserType        string                     // "customer" | "admin" | "operator"
    Status          valueobject.UserStatus     // state machine
    EmailVerifiedAt *time.Time                 // nil = chưa verify
    LastLoginAt     *time.Time
    LockedReason    *string
    Metadata        map[string]interface{}     // JSONB, extensible
    Version         int64                      // optimistic lock
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**Domain methods:**

| Method                                   | Mô tả                                                        |
| ---------------------------------------- | ------------------------------------------------------------ |
| `IsEmailVerified() bool`                 | Email đã được xác minh?                                      |
| `CanPerformDigitalActions() error`       | Gate check — yêu cầu email đã verify                         |
| `MarkEmailVerified(now time.Time) error` | Idempotent — set `EmailVerifiedAt`, chuyển status → `active` |
| `Suspend() error`                        | Chuyển sang `suspended` nếu state machine cho phép           |
| `Ban() error`                            | Chuyển sang `banned` — terminal state                        |

---

### 3.2 Entity `Credential`

```go
type Credential struct {
    UserID            int64
    PasswordHash      string     // bcrypt/argon2id — NEVER log
    PasswordAlgo      string
    PasswordChangedAt time.Time
    FailedLoginCount  int
    LastFailedLoginAt *time.Time
}
```

> **Security rule:** Sau khi verify, caller nhận `bool` hoặc `error` — KHÔNG bao giờ trả `Credential` ra ngoài domain.

**Domain methods:**

| Method                                           | Mô tả                                 |
| ------------------------------------------------ | ------------------------------------- |
| `IsPasswordChangeRequired(now, maxAgeDays) bool` | Kiểm tra password quá hạn theo policy |
| `MarkPasswordChanged(newHash, algo, now)`        | Cập nhật hash + timestamp             |

---

### 3.3 Entity `Device`

```go
type Device struct {
    ID            int64
    UserID        int64
    Fingerprint   string             // mapped từ fingerprint_hash
    Label         string             // mapped từ device_label
    FirstSeenAt   time.Time
    LastSeenAt    time.Time
    RevokedAt     *time.Time
    RevokedReason string
    Metadata      map[string]interface{}
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

**Domain methods:**

| Method                          | Mô tả                                      |
| ------------------------------- | ------------------------------------------ |
| `IsRevoked() bool`              | Device đã bị thu hồi?                      |
| `Revoke(now) error`             | Idempotent revoke                          |
| `AssertOwnership(userID) error` | Enforce ownership — gọi trước mọi mutation |
| `UpdateLastSeen(now)`           | Cập nhật `last_seen_at`                    |

---

### 3.4 Entity `Session`

```go
type Session struct {
    ID               int64
    UserID           int64
    RefreshTokenHash string     // SHA-256(raw_refresh_token)
    DeviceID         *int64     // nil nếu device chưa đăng ký
    SessionStatus    string     // "active" | "revoked" | "expired"
    ExpiredAt        time.Time
    IPAddress        string
    UserAgent        string
    LastSeenAt       time.Time
    RevokedAt        *time.Time
    RevokedReason    *string
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

**Domain methods:**

| Method                          | Mô tả                                        |
| ------------------------------- | -------------------------------------------- |
| `IsRevoked() bool`              | Session bị revoke chưa?                      |
| `IsExpired(now time.Time) bool` | So sánh với `now` được inject vào — testable |
| `Revoke(now)`                   | Idempotent revoke                            |
| `Rotate(newHash, now)`          | Cập nhật hash + gia hạn 30 ngày              |

---

### 3.5 Entity `Address`

```go
type Address struct {
    ID          int64
    UserID      int64
    Line1       string   // address_line1
    Line2       string   // address_line2
    Province    string   // province_code
    District    string   // district_code
    Ward        string   // ward_code
    PostalCode  string
    CountryCode string
    IsDefault   bool
    Version     int64    // optimistic lock
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Domain methods:**

| Method                          | Mô tả                                     |
| ------------------------------- | ----------------------------------------- |
| `AssertOwnership(userID) error` | Enforce ownership boundary trước mutation |

---

## 4. Value Objects & Enums

### 4.1 `Email`

```go
type Email struct { value string }
```

- Validated bằng `net/mail.ParseAddress`
- Normalized: `strings.ToLower(strings.TrimSpace(raw))`
- Max length: 255 ký tự
- Factory: `NewEmail(raw string) (Email, error)`

### 4.2 `UserStatus` — State Machine

```
pending_verification ──→ active ──→ suspended ──→ active
                           │                │
                           └───────── banned (terminal)
                                     suspended ──→ banned
```

| DB Value               | Hằng số Go                      | Mô tả                                   |
| ---------------------- | ------------------------------- | --------------------------------------- |
| `pending_verification` | `UserStatusPendingVerification` | Mặc định khi đăng ký, chưa verify email |
| `active`               | `UserStatusActive`              | Đã verify, hoạt động bình thường        |
| `disabled`             | `UserStatusSuspended`           | Bị khoá tạm thời                        |
| `locked`               | `UserStatusBanned`              | Cấm vĩnh viễn — KHÔNG thể hoàn tác      |

**CanLogin rules:**

- `active` → ✅ có thể login
- `pending_verification` → ✅ có thể login (nhưng bị hạn chế digital actions)
- `disabled` / `locked` → ❌ không thể login

> ✅ **Đã đồng bộ (v1.1):** Go constants đã được cập nhật để match DB CHECK constraint.

---

## 5. Domain Policies

### 5.1 `RegisterPolicy`

| Setting         | Giá trị |
| --------------- | ------- |
| `MinPassLength` | 8       |
| `MaxPassLength` | 72      |

**`ValidatePassword(password) error`**

- Kiểm tra độ dài: `[8, 72]`
- Bắt buộc có: ít nhất 1 chữ HOA, 1 chữ thường, 1 chữ số

**`ValidateRegistration(email, password) error`**

- Email không được rỗng
- Gọi `ValidatePassword`

---

### 5.2 `DevicePolicy`

| Setting             | Giá trị   |
| ------------------- | --------- |
| `MaxDevicesPerUser` | 5         |
| `MaxSessionTTL`     | 30 (ngày) |

**`CanRegisterNewDevice(activeDevices) error`**

- Trả `ErrDeviceLimitReached` nếu số device active ≥ 5

**`MaxSessionTTL` usage:**

```go
ExpiredAt: now.Add(time.Duration(s.devicePolicy.MaxSessionTTL) * 24 * time.Hour)
```

---

## 6. Repository Interfaces

Tất cả interfaces được định nghĩa trong `domain/repository.go`. Implementations ở `infra/postgres/`.

### 6.1 `UserRepository`

```go
type UserRepository interface {
    GetByID(ctx, id int64) (*entity.User, error)
    GetByEmail(ctx, email Email) (*entity.User, error)
    ExistsByEmail(ctx, email Email) (bool, error)
    Insert(ctx, user *entity.User) error
    UpdateStatus(ctx, id int64, status UserStatus) error
    MarkEmailVerified(ctx, id int64, verifiedAt time.Time) error
}
```

**SQL queries key:**

| Method              | Query pattern                                                  |
| ------------------- | -------------------------------------------------------------- |
| `GetByID`           | `SELECT ... FROM users WHERE id = $1`                          |
| `GetByEmail`        | `SELECT ... FROM users WHERE email = $1`                       |
| `ExistsByEmail`     | `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`          |
| `Insert`            | `INSERT INTO users (...) VALUES (...) RETURNING id`            |
| `UpdateStatus`      | `UPDATE users SET account_status = $1 WHERE id = $2`           |
| `MarkEmailVerified` | Idempotent COALESCE update + tự động active nếu lần đầu verify |

---

### 6.2 `CredentialRepository`

```go
type CredentialRepository interface {
    GetByUserID(ctx, userID int64) (*entity.Credential, error)
    Insert(ctx, cred *entity.Credential) error
    UpdatePasswordHash(ctx, userID int64, hash string, changedAt time.Time) error
}
```

---

### 6.3 `SessionRepository`

```go
type SessionRepository interface {
    Update(ctx, session *entity.Session) error                                  // ✅ thêm mới v1.1
    Insert(ctx, session *entity.Session) error
    GetByRefreshTokenHash(ctx, hash string) (*entity.Session, error)
    GetByRefreshTokenHashForUpdate(ctx, hash string) (*entity.Session, error)  // SELECT FOR UPDATE
    ListActiveByUserID(ctx, userID int64) ([]*entity.Session, error)
    Revoke(ctx, id int64, revokedAt time.Time) error
    RevokeAllByUserID(ctx, userID int64, revokedAt time.Time) error
}
```

**Lưu ý:**

- `Update`: cập nhật `refresh_token_hash`, `expires_at`, `last_seen_at` sau khi rotate session
- `ListActiveByUserID`: lọc `revoked_at IS NULL AND expires_at > NOW()`
- `GetByRefreshTokenHashForUpdate`: thêm `FOR UPDATE` — dùng trong Rotate flow trong transaction
- `Revoke` dùng `COALESCE(revoked_at, $2)` — idempotent

---

### 6.4 `DeviceRepository`

```go
type DeviceRepository interface {
    GetByID(ctx, id int64) (*entity.Device, error)
    GetByFingerprint(ctx, userID int64, fingerprint string) (*entity.Device, error)
    ListActiveByUserID(ctx, userID int64) ([]*entity.Device, error)
    Upsert(ctx, device *entity.Device) error
    Revoke(ctx, id int64, revokedAt time.Time) error
}
```

**Upsert device query:**

```sql
INSERT INTO user_devices (user_id, fingerprint_hash, device_label, first_seen_at, last_seen_at, revoked_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, fingerprint_hash) DO UPDATE SET
    device_label = EXCLUDED.device_label,
    last_seen_at = EXCLUDED.last_seen_at,
    revoked_at   = NULL              -- re-activate nếu trước đó đã revoke
RETURNING id, first_seen_at
```

---

### 6.5 `AddressRepository`

```go
type AddressRepository interface {
    GetByID(ctx, id int64) (*entity.Address, error)
    ListByUserID(ctx, userID int64) ([]*entity.Address, error)
    Insert(ctx, address *entity.Address) error
    Update(ctx, address *entity.Address) error
    Delete(ctx, id int64) error
    UnsetDefaultByUserID(ctx, userID int64) error
}
```

**Lưu ý:** `UnsetDefaultByUserID` được gọi trước khi set một address mới làm default, đảm bảo invariant "chỉ 1 default/user".

---

## 7. Query Layer (CQRS Read-side)

### 7.1 Interface

```go
type QueryRepository interface {
    GetMe(ctx, userID int64) (*MeView, error)
    ListSessions(ctx, userID int64) ([]*SessionView, error)
    ListDevices(ctx, userID int64) ([]*DeviceView, error)
    ListAddresses(ctx, userID int64) ([]*AddressView, error)
}
```

### 7.2 View Models

#### `MeView`

```go
type MeView struct {
    UserID          int64
    Email           string
    FullName        string
    Status          string
    EmailVerifiedAt *time.Time
    CreatedAt       time.Time
}
```

#### `SessionView`

```go
type SessionView struct {
    ID        int64
    DeviceID  *int64
    IPAddress string
    UserAgent string
    ExpiredAt time.Time
    RevokedAt *time.Time
    CreatedAt time.Time
}
```

#### `DeviceView`

```go
type DeviceView struct {
    ID          int64
    Fingerprint string
    Label       string
    FirstSeenAt time.Time
    LastSeenAt  time.Time
    RevokedAt   *time.Time
}
```

#### `AddressView`

```go
type AddressView struct {
    ID        int64
    Province  string
    District  string
    Ward      string
    Line1     string
    Line2     string
    IsDefault bool
}
```

> ✅ **Đã fix (v1.1):** Đã xóa field `Phone` không tồn tại trong bảng `addresses`.

---

## 8. Application Services (Use Cases)

> **Trạng thái (v1.1):** Đã implement đầy đủ `AuthService`, `ProfileService`, `AddressService` trong `app/service/`. DTOs, Commands, và Port interfaces đã sẵn sàng.

### 8.1 Auth Use Cases

#### UC-01: Register (Đăng ký tài khoản)

**Input:**

- `email: string`
- `password: string`
- `full_name: string`
- `phone?: string`

**Flow:**

1. `RegisterPolicy.ValidateRegistration(email, password)` — validate input
2. `UserRepository.ExistsByEmail(email)` — check trùng email
3. `PasswordHasher.Hash(ctx, password)` — hash bằng bcrypt
4. Tạo `User` entity với `Status = pending_verification`
5. `tx.TxManager.WithinTransaction`:
   - `UserRepository.Insert(user)`
   - `CredentialRepository.Insert(credential)`
   - `VerificationTokenService.IssueEmailVerificationToken(userID)` — tạo token Redis
   - `EventPublisher.PublishUserRegistered(payload)` — phát event (Phase 1: log)

**Output:** `dto.RegisterOutput{UserID, Email, Message}`

---

#### UC-02: Login (Đăng nhập)

**Input:**

- `email: string`
- `password: string`
- `device_fingerprint?: string`
- `device_label?: string`
- `ip_address: string`
- `user_agent: string`

**Flow:**

1. `UserRepository.GetByEmail(email)`
2. Kiểm tra `UserStatus.CanLogin()`
3. `CredentialRepository.GetByUserID(userID)`
4. `PasswordHasher.Verify(ctx, password, hash)` — bcrypt compare
5. Nếu có `device_fingerprint`:
   - `DeviceRepository.ListActiveByUserID` + `DevicePolicy.CanRegisterNewDevice`
   - `DeviceRepository.GetByFingerprint` → `DeviceRepository.Upsert`
6. `TokenManager.GenerateAccessToken(claims)` → `(token, expiresAt)`
7. `TokenManager.GenerateRefreshToken(userID)` → raw random hex 32 bytes
8. Hash refresh token: `sha256(rawToken)`
9. `SessionRepository.Insert(session)` với `refresh_token_hash`, TTL từ `DevicePolicy.MaxSessionTTL`
10. Trả token raw về HTTP layer (ONE-TIME)

**Output:** `dto.LoginOutput{AccessToken, RefreshToken, ExpiresAt}`

---

#### UC-03: Refresh Token

**Input:**

- `refresh_token: string` (raw)

**Flow:**

1. `hashToken(rawRefreshToken)` → SHA-256 hex
2. `tx.TxManager.WithinTransaction`:
   - `SessionRepository.GetByRefreshTokenHashForUpdate(hash)` — `SELECT FOR UPDATE` chống race condition
   - Kiểm tra `session.IsRevoked()` và `session.IsExpired(now)`
   - `TokenManager.GenerateRefreshToken(userID)` → new raw token
   - `session.Rotate(hashToken(newRaw), now)` → gia hạn session 30 ngày
   - `SessionRepository.Update(session)` — ghi hash mới vào DB
   - `TokenManager.GenerateAccessToken(claims)` → new access token
3. Trả về `dto.RefreshTokenOutput{AccessToken, RefreshToken, ExpiresAt}`

---

#### UC-04: Logout (Thu hồi session)

**Input:** `session_id: int64` (từ JWT claims hoặc query param)

**Flow:**

1. `SessionRepository.Revoke(id, now)`
2. Optionally: `DeviceRepository.Revoke(device_id, now)` nếu user muốn đăng xuất khỏi device

---

#### UC-05: Verify Email

**Input:** `token: string` (email verification token)

**Flow:**

1. Validate và decode token (Redis/JWT)
2. Extract `user_id` từ token
3. `UserRepository.MarkEmailVerified(user_id, now)` — idempotent

---

### 8.2 Profile Use Cases

#### UC-06: Get Me

**Input:** `user_id` (từ JWT)

**Flow:** `QueryRepository.GetMe(user_id)` → `MeView`

---

## 9. Port Adapters

Sau khi tái cấu trúc (v1.2), các implementation được phân chia theo nguyên tắc:

- **`platform/auth/`**: logic không gắn domain bất kỳ (crypto, JWT, Redis token)
- **`identity/infra/adapters/`**: adapters identity-specific + thin bridge wrappers

### 9.1 `platform/auth.BcryptHasher` → `ports.PasswordHasher`

```go
// platform/auth/bcrypt_hasher.go
type BcryptHasher struct{ cost int }
func NewBcryptHasher(cost int) *BcryptHasher
func (h *BcryptHasher) Hash(ctx context.Context, raw string) (string, error)
func (h *BcryptHasher) Verify(ctx context.Context, raw, hash string) error
```

| Chi tiết          | Giá trị                                                                      |
| ----------------- | ---------------------------------------------------------------------------- |
| Thuật toán        | bcrypt                                                                       |
| Cost              | Lấy từ `config.JWT.BcryptCost` (mặc định: 12)                                |
| Structural typing | `*BcryptHasher` satisfy `ports.PasswordHasher` trực tiếp — không cần adapter |

---

### 9.2 `platform/auth.JWTTokenManager` → `ports.TokenManager`

```go
// platform/auth/jwt_token_manager.go
type JWTClaims struct {
    UserID int64; Email string; Role string; Type string
}
type JWTTokenManager struct{ cfg config.JWTConfig }
func NewJWTTokenManager(cfg config.JWTConfig) *JWTTokenManager
func (m *JWTTokenManager) GenerateAccessToken(ctx, claims JWTClaims) (string, time.Time, error)
func (m *JWTTokenManager) GenerateRefreshToken(ctx, userID int64) (string, error)
```

| Chi tiết      | Giá trị                                                                                                     |
| ------------- | ----------------------------------------------------------------------------------------------------------- |
| Access token  | HS256 JWT, claim: `user_id`, `email`, `role`, `type="access"`                                               |
| Access TTL    | `config.JWT.AccessTokenTTL` (mặc định: 15 phút)                                                             |
| Refresh token | **Opaque** random hex 32 bytes (không phải JWT)                                                             |
| Bridge layer  | `jwtTokenManagerBridge` trong `adapters/provider.go` — convert `ports.AccessTokenClaims` → `auth.JWTClaims` |

> **Tại sao cần bridge?** `platform/auth` không được import `identity/app/ports` (vi phạm layering). `JWTClaims` và `ports.AccessTokenClaims` có cùng fields, `jwtTokenManagerBridge` trong adapters chuyển đổi giữa hai kiểu.

```go
// identity/infra/adapters/provider.go
type jwtTokenManagerBridge struct{ inner *auth.JWTTokenManager }
func (b *jwtTokenManagerBridge) GenerateAccessToken(ctx, c ports.AccessTokenClaims) (string, time.Time, error) {
    return b.inner.GenerateAccessToken(ctx, auth.JWTClaims{
        UserID: c.UserID, Email: c.Email, Role: c.Role, Type: c.Type,
    })
}
```

---

### 9.3 `platform/auth.RedisVerificationTokenService` → `ports.VerificationTokenService`

```go
// platform/auth/redis_verify_token.go
type RedisVerificationTokenService struct{ rdb *goredis.Client }
func NewRedisVerificationTokenService(rdb *goredis.Client) *RedisVerificationTokenService
func (s *RedisVerificationTokenService) IssueEmailVerificationToken(ctx, userID int64) (string, error)
func (s *RedisVerificationTokenService) ParseEmailVerificationToken(ctx, token string) (int64, error)
```

| Chi tiết          | Giá trị                                                                             |
| ----------------- | ----------------------------------------------------------------------------------- |
| Token             | Random hex 32 bytes (dùng `generateSecureToken()` từ `platform/auth/rand.go`)       |
| Storage           | Redis key: `identity:email_verify:{token}`                                          |
| TTL               | 24 giờ                                                                              |
| Single-use        | `GetDel` — lấy và xóa trong cùng 1 thác tác → chống replay attack                   |
| Structural typing | `*RedisVerificationTokenService` satisfy `ports.VerificationTokenService` trực tiếp |

---

### 9.4 `LogEventPublisher` → `ports.EventPublisher` _(identity-specific)_

```go
// identity/infra/adapters/log_event_publisher.go
type LogEventPublisher struct{ log *zap.Logger }
```

> **Phase 1:** Chỉ log sự kiện bằng `zap.Logger`. Không gửi email thật.
> **Phase 2:** Swap sang `KafkaEventPublisher` hoặc `SMTPPublisher` mà không đụng vào `AuthService`.

---

### 9.5 `RealClock` → `ports.Clock` _(identity-specific)_

```go
func (c *RealClock) Now() time.Time { return time.Now() }
```

- Test: inject `MockClock` với giá trị cố định — không cần mock time package toàn cục

---

### 9.4 `LogEventPublisher` → `ports.EventPublisher`

```go
type EventPublisher interface {
    PublishUserRegistered(ctx context.Context, payload UserRegisteredPayload) error
}

type UserRegisteredPayload struct {
    UserID int64
    Email  string
    Token  string  // email verification token
}
```

> **Phase 1:** Chỉ log sự kiện bằng `zap.Logger`. Không gửi email thật.
> **Phase 2:** Swap sang `KafkaEventPublisher` hoặc `SMTPPublisher` mà không đụng vào `AuthService`.

---

### 9.5 `RealClock` → `ports.Clock`

```go
type Clock interface {
    Now() time.Time
}
```

- Production: `RealClock.Now()` = `time.Now()`
- Test: inject `MockClock` với giá trị cố định — không cần mock time package toàn cục

---

## 10. Dependency Injection & Wire

### 10.1 `tx.TxManager` interface

Thêm interface `TxManager` vào `platform/tx/manager.go`:

```go
type TxManager interface {
    WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

`*tx.Manager` implicitly implements `TxManager` — wire bind qua `ProvideTxManagerInterface`.

### 10.2 Layering Rule: Platform ≠ Module

`platform/auth` không được import `identity/app/ports`.
Layer flow: `bootstrap` → `platform/auth` + `identity/infra/adapters` → `identity/app/ports` (interface).

```
[platform/auth]         [identity/infra/adapters]
  BcryptHasher    ───│──► (direct) → ports.PasswordHasher
  JWTTokenManager ───│──► jwtTokenManagerBridge → ports.TokenManager
  RedisVerifyTokenSvc ─│──► (direct) → ports.VerificationTokenService
                       │
  [adapters-only]      │
  LogEventPublisher ──► ports.EventPublisher
  RealClock ────────► ports.Clock
```

### 10.3 Wire Provider Sets

```
internal/bootstrap/providers.go
└── APISet
    ├── PlatformSet
    │   ├── ProvideConfig() *config.Config
    │   ├── ProvideLogger() *zap.Logger
    │   ├── ProvideDBPool() *pgxpool.Pool
    │   ├── ProvideTxManager() *tx.Manager
    │   ├── ProvideTxManagerInterface(*tx.Manager) tx.TxManager
    │   ├── ProvideRedis() *goredis.Client
    │   └── ProvideAuthManager() *auth.Auth          ← giữ cho HTTP middleware
    ├── HTTPSet
    │   ├── ProvideGinEngine()
    │   └── ProvideHTTPServer()
    ├── ModuleSet
    │   ├── identity_postgres.ProviderSet
    │   │   └── 5 repos + QueryRepository
    │   ├── identity_adapters.ProviderSet
    │   │   ├── ProvideBcryptHasher(cfg)                ← dùng platform/auth.NewBcryptHasher
    │   │   ├── ProvideJWTTokenManager(cfg)             ← wrap platform/auth.NewJWTTokenManager với bridge
    │   │   ├── ProvideRedisVerificationTokenService(rdb) ← dùng platform/auth.NewRedisVerificationTokenService
    │   │   ├── ProvideLogEventPublisher(log)           ← adapters-only
    │   │   └── ProvideRealClock()                      ← adapters-only
    │   └── identity_service.ProviderSet
    │       ├── NewAuthService(...) *AuthService
    │       ├── NewProfileService(...) *ProfileService
    │       ├── NewAddressService(...) *AddressService
    │       ├── ProvideRegisterPolicy() policy.RegisterPolicy
    │       └── ProvideDevicePolicy() policy.DevicePolicy
    └── ProvideAPIApp()
```

---

## 11. HTTP API Specification

> **Trạng thái:** Chưa implement HTTP handlers. Phần này là đặc tả để hướng dẫn implement sau.
>
> Tham khảo spec chi tiết trong tài liệu `docs/specs/` hoặc từ bản gốc của file này.

---

## 12. Error Catalogue

Tất cả lỗi domain được định nghĩa trong `domain/error/errors.go`:

| Error Code                           | HTTP Mapping | Mô tả                               |
| ------------------------------------ | ------------ | ----------------------------------- |
| `IDENTITY_USER_NOT_FOUND`            | 404          | Không tìm thấy user                 |
| `IDENTITY_EMAIL_ALREADY_EXIST`       | 409          | Email đã tồn tại                    |
| `IDENTITY_INVALID_CREDENTIALS`       | 401          | Sai email hoặc mật khẩu             |
| `IDENTITY_EMAIL_NOT_VERIFIED`        | 403          | Email chưa được xác minh            |
| `IDENTITY_SESSION_EXPIRED`           | 401          | Session đã hết hạn                  |
| `IDENTITY_SESSION_REVOKED`           | 401          | Session đã bị thu hồi               |
| `IDENTITY_DEVICE_LIMIT`              | 422          | Đã đạt giới hạn số device (5)       |
| `IDENTITY_DEVICE_NOT_OWNED`          | 403          | Device không thuộc user này         |
| `IDENTITY_ADDRESS_NOT_FOUND`         | 404          | Địa chỉ không tồn tại               |
| `IDENTITY_ADDRESS_NOT_OWNED`         | 403          | Địa chỉ không thuộc user này        |
| `IDENTITY_INVALID_STATUS_TRANSITION` | 422          | Chuyển trạng thái user không hợp lệ |

---

## 13. Platform Dependencies

Module identity phụ thuộc vào các platform packages:

| Package           | Vai trò                                                            |
| ----------------- | ------------------------------------------------------------------ |
| `platform/auth`   | `*auth.Auth` giữ cho HTTP middleware — validate access token       |
| `platform/tx`     | `TxManager` interface + `*Manager` impl + `GetExecutor(ctx, pool)` |
| `platform/db`     | pgxpool connection pool                                            |
| `platform/redis`  | `*goredis.Client` — dùng cho verification token (Redis)            |
| `platform/config` | JWT config (secret, AccessTokenTTL, RefreshTokenTTL, BcryptCost)   |

**Token lifetimes:**

- Access token TTL: env `JWT_ACCESS_TOKEN_TTL_MINUTES` (mặc định: 15 phút) ✅
- Session TTL: `DevicePolicy.MaxSessionTTL` × 24h (mặc định: **30 ngày**)

---

## 14. Ghi chú & Known Issues

### ⚠️ Tồn động cần xử lý

| #   | Vị trí                    | Vấn đề                                                                                                               | Mức độ     |
| --- | ------------------------- | -------------------------------------------------------------------------------------------------------------------- | ---------- |
| 1   | `query_repo.go` `queryMe` | SELECT thiếu các cột `phone`, `user_type`, `locked_reason`, `metadata`, `version`, `last_login_at` so với `scanUser` | **MEDIUM** |
| 2   | `auth.go` `*auth.Auth`    | Chỉ giữ lại để HTTP middleware validate access token — không còn dùng trong service                                  | INFO       |
| 3   | `http/`                   | HTTP handlers chưa implement                                                                                         | **HIGH**   |
| 4   | `EventPublisher`          | Phase 1 chỉ log — chưa gửi email thật cho user                                                                       | **HIGH**   |

### 📌 Việc cần làm tiếp theo

```
http/
  handler/    ← AuthHandler, ProfileHandler, AddressHandler
  middleware/ ← AuthMiddleware (dùng *auth.Auth để validate JWT, inject userID vào ctx)
  router.go   ← Route registration
```

### ✅ Đã implement (v1.1)

- Domain entities với behavior methods đầy đủ
- State machine `UserStatus` với `CanTransitionTo`
- Repository implementations (5 repo + 1 query repo)
- Anti-corruption layer (`rows.go` → `mapper.go`)
- Transaction management (`tx.TxManager.WithinTransaction`)
- **Port interfaces** (`PasswordHasher`, `TokenManager`, `VerificationTokenService`, `EventPublisher`, `Clock`)
- **Port adapters** (`BcryptHasher`, `JWTTokenManager`, `RedisVerificationTokenService`, `LogEventPublisher`, `RealClock`)
- **Application services** (`AuthService` — Register, Login, RefreshToken, Logout, VerifyEmail)
- **DTOs** (`RegisterInput/Output`, `LoginInput/Output`, `RefreshTokenInput/Output`...)
- **Commands** (9 command structs)
- Idempotent operations (revoke, verify email)
- `SELECT FOR UPDATE` trong refresh token rotation flow
- Verification token single-use (Redis `GetDel`)
- Partial unique index cho default address
- Ownership enforcement (`AssertOwnership`) trước mọi mutation
- Wire DI đầy đủ (PlatformSet + ModuleSet)

bookstore-backend-v2/internal/modules/identity/
│
├── domain/ # 1. TẦNG CỐT LÕI (CORE) - Bất biến, không phụ thuộc framework
│ ├── entity/ # Các thực thể mang dữ liệu và trạng thái
│ │ ├── user.go  
│ │ ├── credential.go  
│ │ ├── session.go  
│ │ ├── device.go  
│ │ └── address.go  
│ ├── value_object/ # Các kiểu dữ liệu ràng buộc nghiệp vụ
│ │ ├── email.go  
│ │ └── user_status.go  
│ ├── policy/ # Luật nghiệp vụ (Business Rules)
│ │ ├── register_policy.go  
│ │ └── device_policy.go  
│ ├── error/ # Mã lỗi chuẩn hóa của riêng module
│ │ └── errors.go  
│ └── repository.go # CÁC INTERFACE ĐỂ GHI (Write-side contracts)
│
├── app/ # 2. TẦNG ĐIỀU PHỐI (APPLICATION) - Orchestration & Use Cases
│ ├── command/ # Định nghĩa Input cho thao tác GHI
│ │ ├── register_command.go  
│ │ ├── login_command.go  
│ │ └── refresh_command.go  
│ ├── query/ # Định nghĩa Input/Output & Interface cho thao tác ĐỌC (CQRS-lite)
│ │ ├── query_repository.go  
│ │ └── views.go # (MeView, SessionView, DeviceView...)
│ ├── dto/ # Data Transfer Objects trả về cho HTTP
│ │ ├── register_output.go  
│ │ └── login_output.go  
│ ├── ports/ # CÁC INTERFACE YÊU CẦU NGOẠI VI (Outbound Ports)
│ │ ├── token_manager.go  
│ │ ├── password_hasher.go  
│ │ └── event_publisher.go  
│ └── service/ # Kẻ nhạc trưởng điều phối nghiệp vụ
│ ├── auth_service.go  
│ ├── user_service.go  
│ └── providers.go # (Wire ProviderSet cho tầng App)
│
├── infra/ # 3. TẦNG HẠ TẦNG (INFRASTRUCTURE) - Các Implementation thực tế
│ ├── postgres/ # Giao tiếp với DB (Sử dụng pgx)
│ │ ├── user_repository.go  
│ │ ├── session_repository.go
│ │ ├── device_repository.go
│ │ ├── address_repository.go
│ │ ├── query_repository.go # Cắm thẳng SQL để lấy View tối ưu đọc
│ │ ├── mapper.go # Map từ DB row -> Entity / View
│ │ └── providers.go # (Wire ProviderSet cho DB)
│ └── adapter/ # Các công cụ bên thứ 3 (Thợ xây đắp ứng dụng)
│ ├── redis_verify_token.go# Implement VerificationTokenService
│ ├── bcrypt_hasher.go # Implement PasswordHasher
│ ├── jwt_manager.go # Implement TokenManager
│ └── providers.go # (Wire ProviderSet cho Adapter)
│
└── interfaces/ # 4. TẦNG GIAO TIẾP NGOÀI (ENTRYPOINTS / INBOUND PORTS)
├── http/ # Nhận HTTP Request từ client
│ ├── auth_handler.go # Chuyển JSON -> Command -> gọi AuthService
│ ├── user_handler.go  
 │ └── routes.go # Khai báo các endpoint (/api/v1/auth/...)
└── consumer/ # (Nếu có) Nhận event từ Kafka
└── outbox_worker.go
