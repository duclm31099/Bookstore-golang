# Module Identity — Tài liệu kỹ thuật đầy đủ

> **Phiên bản:** v2.0
> **Cập nhật:** 2026-05-15
> **Stack:** Go (pgx/v5), PostgreSQL 15+, Redis, JWT, Bcrypt
> **Đối tượng:** Frontend developer muốn hiểu cách backend vận hành

---

## Mục lục

1. [Giới thiệu — Đọc trước nếu bạn là Frontend Dev](#1-giới-thiệu--đọc-trước-nếu-bạn-là-frontend-dev)
2. [Tổng quan kiến trúc](#2-tổng-quan-kiến-trúc)
3. [Database Schema](#3-database-schema)
4. [Domain Model — Thực thể nghiệp vụ](#4-domain-model--thực-thể-nghiệp-vụ)
5. [Value Objects & Enums](#5-value-objects--enums)
6. [Domain Policies — Luật nghiệp vụ cứng](#6-domain-policies--luật-nghiệp-vụ-cứng)
7. [Repository Interfaces — Hợp đồng với Database](#7-repository-interfaces--hợp-đồng-với-database)
8. [Port Interfaces — Hợp đồng với Hạ tầng ngoài](#8-port-interfaces--hợp-đồng-với-hạ-tầng-ngoài)
9. [Application Services — Các Use Case nghiệp vụ](#9-application-services--các-use-case-nghiệp-vụ)
10. [HTTP Layer — Từ Request đến Response](#10-http-layer--từ-request-đến-response)
11. [HTTP API Specification — Đặc tả đầy đủ](#11-http-api-specification--đặc-tả-đầy-đủ)
12. [Bảo mật — Các cơ chế bảo vệ](#12-bảo-mật--các-cơ-chế-bảo-vệ)
13. [Error Catalogue — Danh sách mã lỗi](#13-error-catalogue--danh-sách-mã-lỗi)
14. [Dependency Injection & Wire](#14-dependency-injection--wire)
15. [Platform Dependencies](#15-platform-dependencies)

---

## 1. Giới thiệu — Đọc trước nếu bạn là Frontend Dev

Nếu bạn đang quen với React/Vue và muốn hiểu backend hoạt động như thế nào, đây là những khái niệm cốt lõi bạn cần nắm trước khi đọc tiếp.

### Backend làm gì mà Frontend không thấy?

Khi bạn gọi `POST /api/v1/auth/login`, phía frontend chỉ thấy: gửi email + password → nhận về access token. Nhưng bên trong backend, có khoảng **10 bước** diễn ra theo thứ tự nghiêm ngặt:

```
[Request từ browser]
      ↓
[Middleware kiểm tra header, IP, rate limit]
      ↓
[Handler: parse JSON → validate → tạo Command object]
      ↓
[AuthService.Login(): điều phối nghiệp vụ]
      ↓
[UserRepository: truy vấn PostgreSQL]
      ↓
[PasswordHasher: so sánh bcrypt hash]
      ↓
[DeviceRepository: upsert device fingerprint]
      ↓
[TokenManager: tạo JWT access token + opaque refresh token]
      ↓
[SessionRepository: ghi session vào PostgreSQL]
      ↓
[RedisSessionService: cache session vào Redis]
      ↓
[Handler: set HTTP-only cookie + trả JSON response]
      ↓
[Response về browser]
```

### Tại sao có nhiều lớp như vậy?

Câu trả lời ngắn: **để thay thế từng phần mà không ảnh hưởng phần còn lại**.

Ví dụ: hôm nay dùng bcrypt để hash password, mai muốn đổi sang argon2id → chỉ cần đổi 1 file `BcryptHasher`, toàn bộ logic nghiệp vụ ở `AuthService` không cần chạm vào. Điều này gọi là **Dependency Inversion**.

### Luồng dữ liệu: Frontend → Backend

```
Frontend gửi:  { "email": "...", "password": "..." }
                        ↓
Backend nhận:  LoginRequest  (validate JSON binding)
                        ↓
              LoginCommand  (plain struct, chuyển qua App layer)
                        ↓
              AuthService.Login(cmd)  (nghiệp vụ)
                        ↓
              LoginOutput  (DTO trả về)
                        ↓
Frontend nhận: { "access_token": "...", "expires_at": "..." }
              + HTTP-only Cookie: refresh_token=...
```

---

## 2. Tổng quan kiến trúc

Module **identity** được tổ chức theo **Modular Monolith** với 4 tầng phân lớp rõ ràng:

```
internal/modules/identity/
├── domain/               ← Tầng 1: Quy tắc nghiệp vụ thuần túy
│   ├── entity/           ← User, Credential, Session, Device, Address
│   ├── value_object/     ← Email, UserStatus (có validation + state machine)
│   ├── policy/           ← RegisterPolicy, DevicePolicy
│   ├── error/            ← Mã lỗi chuẩn hóa
│   └── repository.go     ← Hợp đồng interface với DB (không phải implementation)
│
├── app/                  ← Tầng 2: Điều phối use case
│   ├── command/          ← Input structs cho thao tác ghi (Login, Register...)
│   ├── query/            ← Input/Output cho thao tác đọc (CQRS-lite)
│   ├── dto/              ← Output structs trả về HTTP layer
│   ├── ports/            ← Hợp đồng với hạ tầng ngoài (Redis, JWT, Email...)
│   └── service/          ← AuthService, ProfileService, AddressService
│
├── infra/                ← Tầng 3: Implementation thực tế
│   ├── postgres/         ← Các Repository thực thi truy vấn SQL
│   └── adapters/         ← Adapter cho JWT, Redis, Event Publisher
│
└── http/                 ← Tầng 4: Giao tiếp HTTP
    ├── auth_handler.go   ← Xử lý request HTTP, gọi service
    ├── request.go        ← JSON binding + validation rules
    ├── response.go       ← Cấu trúc JSON response
    ├── router.go         ← Khai báo routes, gán middleware
    └── middleware/
        ├── auth_middleware.go  ← Kiểm tra JWT Bearer token
        └── strict_auth.go     ← Kiểm tra JTI blacklist (cho thao tác nhạy cảm)
```

### Nguyên tắc thiết kế cốt lõi

| Nguyên tắc | Ý nghĩa thực tế |
|---|---|
| **Domain không biết DB** | `entity.User` không có field `db:""` hay SQL. Nó chỉ có business logic. |
| **Dependency Inversion** | `AuthService` phụ thuộc vào `interface`, không phải `*PostgresRepo` cụ thể |
| **Transaction ở App layer** | `AuthService` quyết định khi nào mở/đóng transaction — không để DB tự quyết |
| **Cache-aside pattern** | Đọc session từ Redis trước, nếu miss thì đọc DB, rồi cache lại Redis |
| **Refresh token trong HTTP-only Cookie** | Browser không thể đọc cookie này qua JavaScript → chống XSS |
| **JTI Blacklist** | Access token bị revoke ngay lập tức thay vì phải chờ hết hạn |
| **Fail-closed security** | Nếu Redis lỗi khi kiểm tra blacklist → CHẶN request (không cho qua) |

---

## 3. Database Schema

### 3.1 Bảng `users`

Chứa thông tin cơ bản của người dùng. **Lưu ý quan trọng:** password KHÔNG được lưu ở đây.

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

> **Tại sao có field `version`?** Đây là kỹ thuật **Optimistic Locking**: khi 2 request đồng thời muốn cập nhật cùng 1 user, request nào gửi kèm `version` cũ hơn sẽ bị từ chối. Tránh ghi đè nhau.

| Column | Nullable | Ghi chú |
|---|---|---|
| `id` | NOT NULL | BIGSERIAL — tự tăng |
| `email` | NOT NULL | UNIQUE, lowercase |
| `user_type` | NOT NULL | `customer` / `admin` / `operator` |
| `account_status` | NOT NULL | State machine — xem §5.2 |
| `email_verified_at` | NULL | NULL = chưa xác minh email |
| `version` | NOT NULL | Optimistic locking counter |

---

### 3.2 Bảng `user_credentials`

Password được tách ra bảng riêng vì lý do bảo mật: khi query user bình thường, không bao giờ join bảng này.

```sql
CREATE TABLE IF NOT EXISTS user_credentials (
  user_id              BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_hash        TEXT        NOT NULL,
  password_algo        VARCHAR(30) NOT NULL DEFAULT 'argon2id',
  password_changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  failed_login_count   INT         NOT NULL DEFAULT 0,
  last_failed_login_at TIMESTAMPTZ
);
```

> **Tại sao lưu `failed_login_count`?** Phát hiện brute-force attack: sau N lần nhập sai liên tiếp, account bị khóa tạm thời.

> **Tại sao chỉ lưu hash, không lưu password?** Vì nếu DB bị leak, hacker không biết password thật. bcrypt hash có tính chất **one-way**: không thể đảo ngược từ hash về password gốc.

---

### 3.3 Bảng `user_devices`

Theo dõi thiết bị người dùng. Mỗi thiết bị có một "dấu vân tay" (fingerprint) duy nhất.

```sql
CREATE TABLE IF NOT EXISTS user_devices (
  id               BIGSERIAL PRIMARY KEY,
  user_id          BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  fingerprint_hash TEXT          NOT NULL,
  device_label     VARCHAR(255),
  first_seen_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
  last_seen_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
  revoked_at       TIMESTAMPTZ,
  UNIQUE(user_id, fingerprint_hash)
);
```

> **`fingerprint_hash` là gì?** Frontend tạo một chuỗi định danh thiết bị (từ user agent, screen resolution, timezone...) rồi hash lại. Backend nhận hash này, không bao giờ biết cấu thành thực sự.

> **Policy:** Tối đa **5 devices** active per user. Device thứ 6 sẽ bị từ chối đăng nhập.

---

### 3.4 Bảng `user_sessions`

Mỗi session = 1 phiên đăng nhập = 1 refresh token đang tồn tại.

```sql
CREATE TABLE IF NOT EXISTS user_sessions (
  id                BIGSERIAL PRIMARY KEY,
  user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id         BIGINT NOT NULL REFERENCES user_devices(id) ON DELETE CASCADE,
  refresh_token_hash TEXT NOT NULL,
  session_status    VARCHAR(20) NOT NULL DEFAULT 'active',
  expires_at        TIMESTAMPTZ NOT NULL,
  ip_address        VARCHAR(45),
  user_agent        TEXT,
  last_seen_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at        TIMESTAMPTZ,
  UNIQUE(refresh_token_hash),         -- tra cứu nhanh khi refresh
  UNIQUE(user_id, device_id)          -- 1 device = 1 session
);
```

> **Tại sao `UNIQUE(user_id, device_id)`?** Đảm bảo mỗi thiết bị chỉ có đúng 1 session. Khi login lại trên cùng thiết bị → session cũ bị ghi đè (Upsert), không tạo thêm.

> **Tại sao chỉ lưu `refresh_token_hash` thay vì token thật?** Token thật chỉ tồn tại trong RAM 1 lần duy nhất khi tạo, rồi gửi về client. DB chỉ lưu SHA-256 hash của nó. Nếu DB bị leak, hacker có hash nhưng không có token thật để dùng.

---

### 3.5 Bảng `addresses`

```sql
CREATE TABLE IF NOT EXISTS addresses (
  id             BIGSERIAL    PRIMARY KEY,
  user_id        BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  address_line1  VARCHAR(255) NOT NULL,
  address_line2  VARCHAR(255),
  province_code  VARCHAR(20)  NOT NULL,
  district_code  VARCHAR(20)  NOT NULL,
  ward_code      VARCHAR(20)  NOT NULL,
  is_default     BOOLEAN      NOT NULL DEFAULT false,
  version        BIGINT       NOT NULL DEFAULT 1
);

-- Đảm bảo mỗi user chỉ có đúng 1 địa chỉ mặc định
CREATE UNIQUE INDEX idx_addresses_unique_default ON addresses(user_id) WHERE is_default = true;
```

> **Partial unique index** `WHERE is_default = true` là một trick DB: chỉ enforce uniqueness trên những row có `is_default = true`. Các address khác (`is_default = false`) có thể nhiều tùy ý.

---

### 3.6 ERD tóm tắt

```
users (1) ──── (1) user_credentials
  │
  ├── (1) ──── (N) user_devices
  │                     │
  └── (1) ──── (N) user_sessions ── (1) user_devices
  │
  └── (1) ──── (N) addresses
```

---

## 4. Domain Model — Thực thể nghiệp vụ

Domain entity **không phải là DB row**. Chúng là đối tượng chứa business logic. Ví dụ: `User` không chỉ có data, nó còn có hành vi như `MarkEmailVerified()`, `CanLogin()`.

### 4.1 Entity `User`

```go
type User struct {
    ID              int64
    Email           valueobject.Email          // có validation
    FullName        string
    Phone           *string                    // con trỏ vì có thể null
    UserType        string                     // "customer" | "admin" | "operator"
    Status          valueobject.UserStatus     // state machine
    EmailVerifiedAt *time.Time                 // nil = chưa verify
    LastLoginAt     *time.Time
    LockedReason    *string
    Metadata        map[string]interface{}
    Version         int64                      // optimistic lock
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**Các method của User:**

| Method | Ý nghĩa |
|---|---|
| `IsEmailVerified() bool` | Kiểm tra email đã xác minh chưa |
| `CanPerformDigitalActions() error` | Cổng bảo vệ — yêu cầu email đã verify mới được làm những thứ quan trọng |
| `MarkEmailVerified(now) error` | Set EmailVerifiedAt, tự động chuyển status → active (idempotent) |

---

### 4.2 Entity `Credential` — Thực thể mật khẩu (tách biệt hoàn toàn)

```go
type Credential struct {
    UserID            int64
    PasswordHash      string     // NEVER log, NEVER trả ra ngoài
    PasswordAlgo      string     // "bcrypt" hoặc "argon2id"
    PasswordChangedAt time.Time
    FailedLoginCount  int
    LastFailedLoginAt *time.Time
}
```

> **Nguyên tắc bảo mật:** Sau khi verify password xong, caller chỉ nhận `true/false` hoặc `error`. `Credential` object không bao giờ được trả ra khỏi domain layer.

---

### 4.3 Entity `Session`

```go
type Session struct {
    ID               int64
    UserID           int64
    RefreshTokenHash string        // SHA-256(raw_refresh_token)
    DeviceID         *int64
    SessionStatus    string        // "active" | "revoked" | "expired"
    ExpiredAt        time.Time
    IPAddress        string
    UserAgent        string
    LastSeenAt       time.Time
    RevokedAt        *time.Time
}
```

**Các method của Session:**

| Method | Ý nghĩa |
|---|---|
| `IsRevoked() bool` | Session bị thu hồi chưa? |
| `IsExpired(now) bool` | Đã quá hạn chưa? (inject `now` để có thể test) |
| `Revoke(now)` | Thu hồi session (idempotent) |
| `Rotate(newHash, now)` | Cập nhật token hash + gia hạn thêm 30 ngày |

---

### 4.4 Entity `Device`

```go
type Device struct {
    ID            int64
    UserID        int64
    Fingerprint   string
    Label         string
    FirstSeenAt   time.Time
    LastSeenAt    time.Time
    RevokedAt     *time.Time
}
```

---

## 5. Value Objects & Enums

### 5.1 `Email` — Kiểu dữ liệu có validation tích hợp

```go
type Email struct { value string }
```

- Validated bằng `net/mail.ParseAddress`
- Tự động lowercase + trim whitespace
- Max 255 ký tự
- Tạo bằng: `NewEmail(raw string) (Email, error)`

> **Tại sao dùng Value Object thay vì `string`?** Vì `string` không tự kiểm tra. Nếu truyền `string` thì có thể vô tình truyền email chưa validate. Dùng `Email` type → compiler đảm bảo bạn không thể tạo `Email` bất hợp lệ.

---

### 5.2 `UserStatus` — State Machine (Máy trạng thái)

Đây là khái niệm quan trọng: account status không phải muốn đổi thành gì thì đổi. Có những chuyển trạng thái hợp lệ và không hợp lệ.

```
pending_verification ──→ active ──→ disabled (tạm khóa)
                           │              │
                           └──────────────┴──→ locked (cấm vĩnh viễn)
```

| DB Value | Hằng số Go | Ý nghĩa |
|---|---|---|
| `pending_verification` | `UserStatusPendingVerification` | Mới đăng ký, chưa xác minh email |
| `active` | `UserStatusActive` | Hoạt động bình thường |
| `disabled` | `UserStatusSuspended` | Bị khoá tạm thời |
| `locked` | `UserStatusBanned` | Cấm vĩnh viễn — KHÔNG thể hoàn tác |

**Quy tắc CanLogin:**
- `active` → ✅ đăng nhập được
- `pending_verification` → ✅ đăng nhập được (nhưng bị hạn chế tính năng)
- `disabled` / `locked` → ❌ bị từ chối

---

## 6. Domain Policies — Luật nghiệp vụ cứng

Policy là nơi chứa các **giới hạn nghiệp vụ** có thể cấu hình được. Khác với entity method (chỉ áp dụng cho 1 object), policy áp dụng ở cấp hệ thống.

### 6.1 `RegisterPolicy`

| Tham số | Giá trị | Ý nghĩa |
|---|---|---|
| `MinPassLength` | 8 | Password tối thiểu 8 ký tự |
| `MaxPassLength` | 72 | Giới hạn của bcrypt |

**Quy tắc validate password:**
- Độ dài: `[8, 72]`
- Phải có: ít nhất 1 chữ HOA, 1 chữ thường, 1 chữ số

### 6.2 `DevicePolicy`

| Tham số | Giá trị | Ý nghĩa |
|---|---|---|
| `MaxDevicesPerUser` | 5 | Tối đa 5 thiết bị active |
| `MaxSessionTTL` | 30 (ngày) | Refresh token tồn tại 30 ngày |

---

## 7. Repository Interfaces — Hợp đồng với Database

> **Hiểu nôm na:** Interface là "tờ hợp đồng" liệt kê những gì DB phải làm. `AuthService` chỉ biết hợp đồng này, không biết bên dưới dùng PostgreSQL hay MongoDB hay thứ gì khác.

Tất cả interfaces định nghĩa tại `domain/repository.go`. Implementation thực tế tại `infra/postgres/`.

### 7.1 `UserRepository`

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

---

### 7.2 `CredentialRepository`

```go
type CredentialRepository interface {
    GetByUserID(ctx, userID int64) (*entity.Credential, error)
    Insert(ctx, cred *entity.Credential) error
    UpdatePasswordHash(ctx, userID int64, hash string, changedAt time.Time) error
}
```

---

### 7.3 `SessionRepository`

```go
type SessionRepository interface {
    Upsert(ctx, session *entity.Session) error
    Update(ctx, session *entity.Session) error
    GetByRefreshTokenHash(ctx, hash string) (*entity.Session, error)
    GetByRefreshTokenHashForUpdate(ctx, hash string) (*entity.Session, error)
    GetByDeviceID(ctx, deviceID int64) (*entity.Session, error)
    ListActiveByUserID(ctx, userID int64) ([]*entity.Session, error)
    Revoke(ctx, id int64, revokedAt time.Time) error
    RevokeAllByUserID(ctx, userID int64, revokedAt time.Time) error
    RevokeAllExcept(ctx, userID int64, excludeSessionID int64, revokedAt time.Time) error
}
```

**Giải thích các method quan trọng:**

| Method | Tại sao cần |
|---|---|
| `Upsert` | Login lại trên cùng device → ghi đè session cũ thay vì tạo thêm |
| `GetByRefreshTokenHashForUpdate` | Thêm `FOR UPDATE` vào SQL — khóa row để chống 2 request refresh cùng lúc |
| `RevokeAllExcept` | Khi đổi password → revoke tất cả session NGOẠI TRỪ session hiện tại |
| `GetByDeviceID` | Tìm session theo device để dọn Redis cache |

---

### 7.4 `DeviceRepository`

```go
type DeviceRepository interface {
    GetByID(ctx, id int64) (*entity.Device, error)
    GetByFingerprint(ctx, userID int64, fingerprint string) (*entity.Device, error)
    ListActiveByUserID(ctx, userID int64) ([]*entity.Device, error)
    Upsert(ctx, device *entity.Device) error
    Revoke(ctx, id int64, revokedAt time.Time) error
}
```

**Upsert device SQL:**
```sql
INSERT INTO user_devices (user_id, fingerprint_hash, device_label, ...)
VALUES ($1, $2, $3, ...)
ON CONFLICT (user_id, fingerprint_hash) DO UPDATE SET
    device_label = EXCLUDED.device_label,
    last_seen_at = EXCLUDED.last_seen_at,
    revoked_at   = NULL    -- re-activate nếu trước đó đã revoke
RETURNING id, first_seen_at
```

---

## 8. Port Interfaces — Hợp đồng với Hạ tầng ngoài

Ports là interface cho các dependency bên ngoài DB: Redis, JWT, Email service...

### 8.1 `PasswordHasher`

```go
type PasswordHasher interface {
    Hash(ctx, raw string) (string, error)
    Verify(ctx, raw, hash string) error
}
```

**Implementation:** `platform/auth.BcryptHasher` (cost=12 từ config)

---

### 8.2 `TokenManager`

```go
type TokenManager interface {
    GenerateAccessToken(ctx, claims AccessTokenClaims) (token string, expiresAt time.Time, error)
    GenerateRefreshToken(ctx, userID int64) (string, error)
}

type AccessTokenClaims struct {
    UserID    int64
    Email     string
    Role      string
    Type      string
    SessionID int64
    DeviceID  int64
    JTI       string  // JWT ID — dùng cho blacklist
}
```

| Token | Kiểu | TTL | Ghi chú |
|---|---|---|---|
| Access token | JWT (HS256) | 15 phút | Chứa claims: user_id, email, role, jti |
| Refresh token | Opaque random hex 32 bytes | 30 ngày | KHÔNG phải JWT — chỉ là chuỗi ngẫu nhiên |

> **Tại sao refresh token không phải JWT?** JWT có thể bị decode để đọc payload. Refresh token là chuỗi ngẫu nhiên hoàn toàn, không chứa thông tin gì → không leak thông tin dù bị intercepted.

---

### 8.3 `RedisSessionService`

```go
type RedisSessionService interface {
    IssueVerifyToken(ctx, userID int64) (token string, error)
    ParseVerifyToken(ctx, token string) (userID int64, error)  // atomic GetDel
    SetUserSession(ctx, sessionID int64, data SessionData, ttl int64) error
    GetUserSession(ctx, sessionID int64) (*SessionData, error)
    DeleteSession(ctx, sessionID int64) error
    DeleteMultipleSessions(ctx, sessionIDs []int64) error
}
```

**Redis key patterns:**

| Mục đích | Key format | TTL |
|---|---|---|
| Email verification token | `identity:email_verify:{token}` | 24 giờ |
| Session cache | `identity:session:{sessionID}` | Bằng session TTL |

> **`ParseVerifyToken` dùng atomic `GetDel`**: Lấy giá trị và xóa key trong 1 thao tác atomic. Đảm bảo token chỉ dùng được đúng 1 lần — không thể verify email 2 lần với cùng 1 link.

---

### 8.4 `BlacklistPort`

```go
type BlacklistPort interface {
    AddToBlacklist(ctx, jti string, expiration time.Duration) error
    IsTokenBlacklisted(ctx, jti string) (bool, error)
}
```

**Implementation:** Redis với key `blacklist:access_token:{jti}`, TTL = thời gian còn lại của token.

> **Tại sao cần blacklist?** JWT access token tự xác thực (stateless) — server không lưu gì. Nếu user đổi password, access token cũ vẫn hợp lệ trong 15 phút. Blacklist giải quyết vấn đề này: thêm `jti` (JWT unique ID) vào Redis → token bị từ chối ngay lập tức.

---

### 8.5 `EventPublisher`

```go
type EventPublisher interface {
    PublishUserRegistered(ctx, payload UserRegisteredPayload) error
}

type UserRegisteredPayload struct {
    UserID int64
    Email  string
    Token  string  // email verification token
}
```

**Cơ chế Transactional Outbox:** Thay vì gửi email trực tiếp (có thể fail sau khi commit DB), event được ghi vào bảng `outbox_events` trong cùng transaction với việc tạo user. Background worker sau đó đọc và xử lý event này.

---

## 9. Application Services — Các Use Case nghiệp vụ

`AuthService` là "nhạc trưởng" — nó điều phối các repository và port để thực hiện các use case. Nó KHÔNG biết gì về HTTP, JSON, hay cookie.

### UC-01: Register — Đăng ký tài khoản

**Input:** `RegisterCommand{Email, Password, FullName, Phone}`

```
1. Validate: RegisterPolicy.ValidateRegistration(email, password)
2. Check trùng email: UserRepository.ExistsByEmail(email)
3. Hash password: PasswordHasher.Hash(password) → hash string
4. Tạo User entity với Status = pending_verification, Version = 1
5. [BEGIN TRANSACTION]
   a. UserRepository.Insert(user) → userID
   b. CredentialRepository.Insert(credential)
   c. RedisSessionService.IssueVerifyToken(userID) → verifyToken
   d. EventPublisher.PublishUserRegistered({userID, email, verifyToken})
      (ghi vào bảng outbox_events — atomic với transaction)
6. [COMMIT]
7. Return: RegisterOutput{UserID, Email}
```

> **Idempotency-Key:** Middleware kiểm tra header `Idempotency-Key` trong Redis. Nếu đã tồn tại → trả lại kết quả cũ, không thực thi lại. Chống double-click tạo 2 account.

---

### UC-02: Login — Đăng nhập

**Input:** `LoginCommand{Email, Password, DeviceFingerprint, DeviceLabel, IPAddress, UserAgent}`

```
1. UserRepository.GetByEmail(email)
2. Kiểm tra user.Status.CanLogin() → lỗi nếu account disabled/locked
3. CredentialRepository.GetByUserID(userID)
4. PasswordHasher.Verify(password, credential.PasswordHash)
   → lỗi INVALID_CREDENTIALS nếu sai
5. Nếu có DeviceFingerprint:
   a. DeviceRepository.ListActiveByUserID(userID) → đếm device
   b. DevicePolicy.CanRegisterNewDevice(count) → lỗi nếu đã đủ 5
   c. DeviceRepository.Upsert(device) → deviceID
6. TokenManager.GenerateAccessToken(claims) → accessToken, expiresAt
7. TokenManager.GenerateRefreshToken(userID) → rawRefreshToken
8. hash = SHA-256(rawRefreshToken)
9. SessionRepository.Upsert(session{userID, deviceID, hash, TTL=30days})
   → sessionID
10. RedisSessionService.SetUserSession(sessionID, sessionData, ttl)
11. Return: LoginOutput{AccessToken, RefreshToken=rawToken, ExpiresAt}
    (rawToken chỉ tồn tại 1 lần này, sau đó KHÔNG lưu đâu cả)
```

> **Lưu ý quan trọng:** Refresh token raw được trả về client rồi **không bao giờ xuất hiện lại trong backend**. DB chỉ lưu hash của nó. Khi client gửi lại, backend hash lại rồi so sánh.

---

### UC-03: RefreshToken — Làm mới Access Token

**Input:** `RefreshTokenCommand{RawRefreshToken}`

```
1. hash = SHA-256(rawRefreshToken)
2. Cache-aside: RedisSessionService.GetUserSession(sessionID)
   - Nếu hit: lấy sessionData từ Redis
   - Nếu miss: SessionRepository.GetByRefreshTokenHash(hash) → session
               rồi SetUserSession vào Redis cache
3. [BEGIN TRANSACTION]
   a. SessionRepository.GetByRefreshTokenHashForUpdate(hash) → SELECT FOR UPDATE
      (khóa row này lại để chống race condition khi 2 request refresh cùng lúc)
   b. Kiểm tra session.IsRevoked() và session.IsExpired(now)
   c. TokenManager.GenerateRefreshToken(userID) → newRawToken
   d. newHash = SHA-256(newRawToken)
   e. session.Rotate(newHash, now) → cập nhật hash + gia hạn 30 ngày
   f. SessionRepository.Update(session) → ghi hash mới vào DB
   g. TokenManager.GenerateAccessToken(claims) → newAccessToken
4. [COMMIT]
5. RedisSessionService.SetUserSession(sessionID, updatedData, newTTL)
6. Return: RefreshTokenOutput{AccessToken, RefreshToken=newRawToken, ExpiresAt}
```

> **SELECT FOR UPDATE:** Khóa pessimistic ở cấp DB row. Khi 2 tab browser đồng thời gọi /refresh-token, cái nào chạm vào lock trước sẽ xử lý, cái kia phải chờ. Tránh trường hợp cả 2 đều thấy token cũ hợp lệ và sinh ra 2 token mới từ 1 token cũ.

---

### UC-04: Logout — Đăng xuất

**Input:** `LogoutCommand{SessionID, DeviceID}` (từ JWT claims)

```
1. SessionRepository.Revoke(sessionID, now)
2. RedisSessionService.DeleteSession(sessionID)
3. Nếu có DeviceID: DeviceRepository.Revoke(deviceID, now)  [optional]
```

---

### UC-05: VerifyEmail — Xác minh email

**Input:** `VerifyEmailCommand{Token}`

```
1. RedisSessionService.ParseVerifyToken(token) → userID
   (atomic GetDel: lấy xong xóa luôn — token chỉ dùng được 1 lần)
2. UserRepository.MarkEmailVerified(userID, now)
   (idempotent: COALESCE(email_verified_at, $now) → gọi 2 lần cũng không sao)
```

---

### UC-06: ChangePassword — Đổi mật khẩu

**Input:** `ChangePasswordCommand{UserID, SessionID, CurrentPassword, NewPassword}`

```
1. CredentialRepository.GetByUserID(userID)
2. PasswordHasher.Verify(currentPassword, credential.PasswordHash)
   → lỗi INVALID_CREDENTIALS nếu sai
3. Kiểm tra newPassword != currentPassword (không cho đổi sang pass cũ)
4. RegisterPolicy.ValidatePassword(newPassword)
5. PasswordHasher.Hash(newPassword) → newHash
6. [BEGIN TRANSACTION]
   a. CredentialRepository.UpdatePasswordHash(userID, newHash, now)
   b. SessionRepository.RevokeAllExcept(userID, currentSessionID, now)
      (thu hồi tất cả session NGOẠI TRỪ session đang dùng hiện tại)
7. [COMMIT]
8. Lấy list sessionID đã bị revoke → xóa khỏi Redis cache
9. Thêm JTI của các access token cũ vào blacklist Redis
   (để chúng không thể dùng trong 15 phút còn lại của TTL)
```

> **Tại sao giữ lại session hiện tại?** UX tốt hơn — user không bị đăng xuất ngay sau khi đổi mật khẩu thành công. Nhưng tất cả thiết bị khác đều bị đăng xuất.

---

## 10. HTTP Layer — Từ Request đến Response

### 10.1 Cấu trúc file

```
http/
├── auth_handler.go        ← Xử lý tất cả endpoint auth
├── request.go             ← JSON binding structs + validation tags
├── response.go            ← Response format structs
├── router.go              ← Khai báo routes, gán middleware
└── middleware/
    ├── auth_middleware.go  ← Xác thực JWT Bearer token
    └── strict_auth.go      ← Kiểm tra JTI blacklist
```

### 10.2 Request structs (request.go)

```go
type RegisterRequest struct {
    Email    string `json:"email"    binding:"required,email,max=255"`
    Password string `json:"password" binding:"required,min=8,max=72"`
    FullName string `json:"full_name" binding:"required,max=255"`
    Phone    string `json:"phone"    binding:"omitempty,max=20"`
}

type LoginRequest struct {
    Email             string `json:"email"              binding:"required,email"`
    Password          string `json:"password"           binding:"required"`
    DeviceFingerprint string `json:"device_fingerprint" binding:"omitempty"`
    DeviceLabel       string `json:"device_label"       binding:"omitempty,max=255"`
}

type ChangePasswordRequest struct {
    CurrentPassword string `json:"current_password" binding:"required"`
    NewPassword     string `json:"new_password"     binding:"required,min=8,max=72"`
}
```

> **`binding:"required"`** là Gin validation tag — tự động trả 400 nếu thiếu field này mà không cần viết code kiểm tra thủ công.

### 10.3 Auth Context — Cách truyền thông tin user qua middleware

```go
type AuthContext struct {
    UserID    int64
    Email     string
    Role      string
    SessionID int64
    DeviceID  int64
    JTI       string  // JWT unique ID (dùng cho blacklist)
}
```

Middleware ghi `AuthContext` vào `gin.Context`. Handler đọc ra:

```go
// Trong handler:
authCtx := ctx.MustGet("auth_context").(*middleware.AuthContext)
userID := authCtx.UserID
```

### 10.4 Luồng cookie cho Refresh Token

**Login response:**
```
HTTP/1.1 200 OK
Set-Cookie: refresh_token=<raw_token>; HttpOnly; Secure; SameSite=Strict; Max-Age=2592000
Content-Type: application/json

{ "access_token": "eyJ...", "expires_at": "2026-05-15T..." }
```

**Refresh request:**
```
POST /api/v1/auth/refresh-token
Cookie: refresh_token=<raw_token>
```

> **HTTP-only cookie:** Browser tự động gửi cookie theo mỗi request mà không cần JavaScript xử lý. JavaScript cũng KHÔNG thể đọc cookie này (HttpOnly) → chống XSS attack đánh cắp refresh token.

---

## 11. HTTP API Specification — Đặc tả đầy đủ

### Base URL: `/api/v1`

### Route Groups

```
/auth/register         POST   — Public (+ Idempotency middleware)
/auth/login            POST   — Public
/auth/verify-email     POST   — Public
/auth/refresh-token    POST   — Auth middleware required
/auth/logout           POST   — Auth middleware required
/me                    GET    — Auth middleware required
/me/change-password    POST   — Auth + Strict Auth middleware required
```

---

### POST `/auth/register`

**Mô tả:** Tạo tài khoản mới. Sau khi đăng ký, user nhận email chứa link xác minh.

**Headers:**
```
Content-Type: application/json
Idempotency-Key: <uuid>   (bắt buộc — chống tạo account trùng khi double-click)
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "Secret123",
  "full_name": "Nguyen Van A",
  "phone": "0901234567"
}
```

**Validation rules:**
- `email`: required, valid email format, max 255 ký tự
- `password`: required, 8-72 ký tự, phải có chữ HOA + chữ thường + số
- `full_name`: required, max 255 ký tự
- `phone`: optional, max 20 ký tự

**Response 201 Created:**
```json
{
  "code": "REGISTER_SUCCESS",
  "data": {
    "user_id": 42,
    "email": "user@example.com",
    "message": "Registration successful. Please check your email to verify your account."
  }
}
```

**Response Header khi replay:**
```
X-Idempotency-Replayed: true
```

**Lỗi có thể gặp:**
| HTTP | Code | Nguyên nhân |
|---|---|---|
| 409 | `IDENTITY_EMAIL_ALREADY_EXIST` | Email đã tồn tại |
| 422 | `VALIDATION_ERROR` | Password yếu, email sai format |
| 400 | `MISSING_IDEMPOTENCY_KEY` | Thiếu header Idempotency-Key |

---

### POST `/auth/login`

**Mô tả:** Đăng nhập. Trả về access token trong body, refresh token trong HTTP-only cookie.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "Secret123",
  "device_fingerprint": "a1b2c3d4...",
  "device_label": "Chrome trên MacBook"
}
```

**Response 200 OK:**
```json
{
  "code": "LOGIN_SUCCESS",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2026-05-15T08:30:00Z"
  }
}
```

**Set-Cookie:**
```
refresh_token=<opaque_hex_64>; HttpOnly; Secure; SameSite=Strict; Max-Age=2592000
```

**Lỗi có thể gặp:**
| HTTP | Code | Nguyên nhân |
|---|---|---|
| 401 | `IDENTITY_INVALID_CREDENTIALS` | Sai email hoặc mật khẩu |
| 403 | `IDENTITY_ACCOUNT_LOCKED` | Account bị khóa |
| 422 | `IDENTITY_DEVICE_LIMIT` | Đã đăng nhập trên 5 thiết bị |

---

### POST `/auth/refresh-token`

**Mô tả:** Dùng refresh token (từ cookie) để lấy access token mới. Cookie tự động được cập nhật.

**Headers:** (Cookie tự động gửi kèm)
```
Authorization: Bearer <access_token>   (optional — chỉ cần Cookie)
```

**Request Body:** `{}` (trống)

**Response 200 OK:**
```json
{
  "code": "REFRESH_SUCCESS",
  "data": {
    "access_token": "eyJ...",
    "expires_at": "2026-05-15T08:45:00Z"
  }
}
```

**Set-Cookie:** (refresh token mới)
```
refresh_token=<new_opaque_hex>; HttpOnly; Secure; SameSite=Strict; Max-Age=2592000
```

**Lỗi có thể gặp:**
| HTTP | Code | Nguyên nhân |
|---|---|---|
| 401 | `IDENTITY_SESSION_EXPIRED` | Refresh token đã hết hạn |
| 401 | `IDENTITY_SESSION_REVOKED` | Session đã bị thu hồi |
| 401 | `MISSING_TOKEN` | Không có cookie refresh_token |

---

### POST `/auth/logout`

**Mô tả:** Đăng xuất — thu hồi session hiện tại.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response 200 OK:**
```json
{
  "code": "LOGOUT_SUCCESS",
  "message": "Logged out successfully"
}
```

**Hành vi:** Cookie `refresh_token` bị xóa trong response (`Max-Age=0`).

---

### POST `/auth/verify-email`

**Mô tả:** Xác minh email qua token trong link email.

**Request Body:**
```json
{
  "token": "a3f9c2e1..."
}
```

**Response 200 OK:**
```json
{
  "code": "EMAIL_VERIFIED",
  "message": "Email verified successfully"
}
```

**Lỗi có thể gặp:**
| HTTP | Code | Nguyên nhân |
|---|---|---|
| 400 | `INVALID_TOKEN` | Token không hợp lệ hoặc đã hết hạn |
| 400 | `TOKEN_ALREADY_USED` | Token đã dùng rồi (single-use) |

---

### GET `/me`

**Mô tả:** Lấy thông tin profile người dùng hiện tại.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response 200 OK:**
```json
{
  "code": "SUCCESS",
  "data": {
    "user_id": 42,
    "email": "user@example.com",
    "full_name": "Nguyen Van A",
    "status": "active",
    "email_verified_at": "2026-05-14T10:00:00Z",
    "created_at": "2026-05-14T09:00:00Z"
  }
}
```

---

### POST `/me/change-password`

**Mô tả:** Đổi mật khẩu. Yêu cầu **Strict Auth** — kiểm tra JTI blacklist thêm lần nữa.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Request Body:**
```json
{
  "current_password": "OldSecret123",
  "new_password": "NewSecret456"
}
```

**Response 200 OK:**
```json
{
  "code": "PASSWORD_CHANGED",
  "message": "Password changed successfully. Other sessions have been revoked."
}
```

**Hành vi sau khi đổi mật khẩu:**
- Session hiện tại: vẫn còn hiệu lực
- Tất cả session khác: bị revoke ngay lập tức
- Access token từ các session cũ: bị thêm vào JTI blacklist → hết hiệu lực ngay

**Lỗi có thể gặp:**
| HTTP | Code | Nguyên nhân |
|---|---|---|
| 401 | `IDENTITY_INVALID_CREDENTIALS` | Sai current_password |
| 422 | `SAME_AS_OLD_PASSWORD` | New password giống password cũ |
| 422 | `VALIDATION_ERROR` | New password không đủ mạnh |

---

## 12. Bảo mật — Các cơ chế bảo vệ

### 12.1 Middleware Stack

Mỗi request đi qua các middleware theo thứ tự:

```
Request
  ↓
[CORS Middleware]
  ↓
[Rate Limiting]        ← Giới hạn số request/giây
  ↓
[Auth Middleware]      ← Kiểm tra JWT (chỉ cho protected routes)
  ↓
[Strict Auth]          ← Kiểm tra JTI blacklist (chỉ cho /change-password)
  ↓
[Handler]
```

### 12.2 Auth Middleware (`auth_middleware.go`)

**Chức năng:** Xác thực JWT Bearer token.

```
Request Header: Authorization: Bearer eyJ...
                                       ↓
              [Tách lấy token sau "Bearer "]
                                       ↓
              [authManager.ValidateAccessToken(token)]
                 - Verify chữ ký HMAC-SHA256
                 - Kiểm tra exp (thời hạn)
                 - Parse claims: user_id, email, role, jti, session_id
                                       ↓
              [Ghi AuthContext vào gin.Context]
                                       ↓
              Handler tiếp tục xử lý
```

**Trả về 401 khi:**
- Thiếu header Authorization → `MISSING_TOKEN`
- Format sai (không có "Bearer ") → `INVALID_TOKEN_FORMAT`
- JWT không hợp lệ hoặc hết hạn → `INVALID_TOKEN`

### 12.3 Strict Auth Middleware (`strict_auth.go`)

**Chức năng:** Thêm lớp bảo vệ thứ 2 cho các thao tác nhạy cảm (đổi mật khẩu).

```
[Sau Auth Middleware — đã có AuthContext]
                ↓
    [Lấy JTI từ AuthContext]
                ↓
    [BlacklistPort.IsTokenBlacklisted(jti)]
                ↓
    Nếu có lỗi Redis → CHẶN (fail-closed)
    Nếu trong blacklist → 401
    Nếu không → cho qua
```

> **Fail-closed:** Nếu Redis không trả lời (timeout, down), middleware chặn request thay vì cho qua. Hy sinh UX để đảm bảo security.

### 12.4 Token Rotation (Chống Refresh Token Reuse)

Mỗi lần `/refresh-token` được gọi:
1. Refresh token cũ bị **vô hiệu hóa** trong DB (hash bị thay thế)
2. Refresh token mới được tạo và trả về

Nếu kẻ tấn công đánh cắp refresh token và dùng sau khi user đã refresh → token đó đã đổi → request thất bại.

### 12.5 SELECT FOR UPDATE (Chống Race Condition)

Scenario: User double-click nút "Refresh" → 2 request gửi cùng lúc.

```
Request A: SELECT session WHERE hash = 'abc'  → thấy hash 'abc' hợp lệ
Request B: SELECT session WHERE hash = 'abc'  → thấy hash 'abc' hợp lệ
Request A: UPDATE session SET hash = 'xyz'
Request B: UPDATE session SET hash = 'pqr'    ← ĐÈ LÊN CỦA A!
```

Với `SELECT FOR UPDATE`:
```
Request A: SELECT ... FOR UPDATE → khóa row
Request B: SELECT ... FOR UPDATE → PHẢI CHỜ A xong
Request A: UPDATE hash = 'xyz', COMMIT, giải phóng lock
Request B: SELECT lại → hash giờ là 'xyz', không phải 'abc' → token không khớp → lỗi
```

---

## 13. Error Catalogue — Danh sách mã lỗi

Tất cả domain errors định nghĩa tại `domain/error/errors.go`.

| HTTP Status | Error Code | Tình huống |
|---|---|---|
| 401 | `IDENTITY_INVALID_CREDENTIALS` | Sai email hoặc mật khẩu |
| 401 | `IDENTITY_SESSION_EXPIRED` | Refresh token đã hết hạn 30 ngày |
| 401 | `IDENTITY_SESSION_REVOKED` | Session đã bị thu hồi (logout, đổi pass) |
| 401 | `MISSING_TOKEN` | Thiếu Authorization header |
| 401 | `INVALID_TOKEN_FORMAT` | Header không đúng định dạng "Bearer ..." |
| 401 | `INVALID_TOKEN` | JWT không hợp lệ hoặc hết 15 phút |
| 403 | `IDENTITY_EMAIL_NOT_VERIFIED` | Thao tác yêu cầu email đã verify |
| 403 | `IDENTITY_ACCOUNT_LOCKED` | Account bị khóa/disabled |
| 403 | `IDENTITY_DEVICE_NOT_OWNED` | Device không thuộc user này |
| 403 | `IDENTITY_ADDRESS_NOT_OWNED` | Địa chỉ không thuộc user này |
| 404 | `IDENTITY_USER_NOT_FOUND` | Không tìm thấy user |
| 404 | `IDENTITY_ADDRESS_NOT_FOUND` | Địa chỉ không tồn tại |
| 409 | `IDENTITY_EMAIL_ALREADY_EXIST` | Email đã được đăng ký |
| 422 | `IDENTITY_DEVICE_LIMIT` | Đã đăng nhập trên 5 thiết bị |
| 422 | `IDENTITY_INVALID_STATUS_TRANSITION` | Chuyển trạng thái user không hợp lệ |
| 422 | `SAME_AS_OLD_PASSWORD` | Mật khẩu mới giống mật khẩu cũ |

---

## 14. Dependency Injection & Wire

> **Giải thích đơn giản:** Wire là công cụ tự động "lắp ráp" các dependency. Bạn khai báo "AuthService cần UserRepository, CredentialRepository, TokenManager..." — Wire tự viết code khởi tạo.

### Sơ đồ dependency

```
bootstrap/providers.go
└── APISet
    ├── PlatformSet
    │   ├── Config (đọc từ env)
    │   ├── Logger (zap)
    │   ├── PostgreSQL pool (pgxpool)
    │   ├── TxManager (quản lý transaction)
    │   └── Redis client (goredis)
    │
    ├── identity/infra/postgres (DB implementations)
    │   ├── UserRepository
    │   ├── CredentialRepository
    │   ├── SessionRepository
    │   ├── DeviceRepository
    │   ├── AddressRepository
    │   └── QueryRepository
    │
    ├── identity/infra/adapters (Port implementations)
    │   ├── BcryptHasher           → ports.PasswordHasher
    │   ├── jwtTokenManagerBridge  → ports.TokenManager
    │   ├── RedisSessionService    → ports.RedisSessionService
    │   ├── RedisBlacklistAdapter  → ports.BlacklistPort
    │   ├── OutboxEventPublisher   → ports.EventPublisher
    │   └── RealClock              → ports.Clock
    │
    └── identity/app/service (Use Cases)
        ├── AuthService (nhận tất cả repos + ports ở trên)
        ├── ProfileService
        └── AddressService
```

### Tại sao cần `jwtTokenManagerBridge`?

`platform/auth.JWTTokenManager` sử dụng kiểu `auth.JWTClaims`.
`identity/app/ports.TokenManager` yêu cầu kiểu `ports.AccessTokenClaims`.

Hai kiểu này không thể tự chuyển đổi vì `platform/auth` không được phép import `identity/app/ports` (vi phạm nguyên tắc phân lớp). Bridge là adapter trung gian:

```go
type jwtTokenManagerBridge struct{ inner *auth.JWTTokenManager }

func (b *jwtTokenManagerBridge) GenerateAccessToken(ctx, c ports.AccessTokenClaims) (string, time.Time, error) {
    return b.inner.GenerateAccessToken(ctx, auth.JWTClaims{
        UserID: c.UserID, Email: c.Email, Role: c.Role, Type: c.Type,
    })
}
```

---

## 15. Platform Dependencies

Module identity không tự implement các thành phần infrastructure cấp thấp. Chúng được cung cấp từ `internal/platform/`:

| Package | Vai trò | Được dùng bởi |
|---|---|---|
| `platform/auth` | JWT tạo/xác thực, bcrypt hash, Redis verify token | `identity/infra/adapters` |
| `platform/tx` | Transaction manager (`WithinTransaction`) | `AuthService` |
| `platform/db` | PostgreSQL connection pool | Tất cả repositories |
| `platform/redis` | Redis client | `RedisSessionService`, `BlacklistPort` |
| `platform/config` | JWT config (secret, TTL, bcrypt cost) | Token manager, hasher |
| `platform/outbox` | Transactional Outbox (ghi event vào DB) | `EventPublisher` |
| `platform/idempotency` | Middleware + Redis store cho Idempotency-Key | Route `/register` |

**Token lifetimes (từ config/env):**

| Token | Env Variable | Default |
|---|---|---|
| Access token | `JWT_ACCESS_TOKEN_TTL_MINUTES` | 15 phút |
| Refresh token / Session | Hardcoded trong `DevicePolicy.MaxSessionTTL` | 30 ngày |
| Email verify token | Hardcoded trong `RedisVerificationTokenService` | 24 giờ |
