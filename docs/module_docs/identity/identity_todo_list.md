# Identity Module — Danh sách việc cần làm

> **Cập nhật:** 2026-05-15 | **Phiên bản:** 2.0
>
> Tài liệu này dành cho **mọi developer** — kể cả bạn vừa chuyển từ frontend sang backend.
> Mỗi task đều có: _tại sao phải làm_, _làm ở đâu_, _làm như thế nào_, và _bẫy thường gặp_.
> Đọc kỹ phần "Lưu ý" trước khi code — nó sẽ cứu bạn khỏi những lỗi mất nửa ngày debug.

---

## Trạng thái hiện tại (đã xong ✅)

Trước khi làm, hãy biết cái gì đang chạy được để không làm lại:

| Tính năng | Endpoint | Ghi chú |
|---|---|---|
| Đăng ký | `POST /api/v1/auth/register` | Có idempotency middleware |
| Đăng nhập | `POST /api/v1/auth/login` | Refresh token lưu cookie HttpOnly |
| Làm mới token | `POST /api/v1/auth/refresh-token` | SELECT FOR UPDATE chống race condition |
| Xác minh email | `POST /api/v1/auth/verify-email` | Single-use token qua Redis |
| Đăng xuất | `POST /api/v1/auth/logout` | JTI blacklist + xóa session |
| Đổi mật khẩu | `POST /api/v1/me/change-password` | Strict auth + thu hồi mọi session khác |
| Xem profile | `GET /api/v1/me` | |
| Danh sách session | `GET /api/v1/me/sessions` | |
| Thu hồi tất cả session | `DELETE /api/v1/me/sessions` | |
| Danh sách thiết bị | `GET /api/v1/me/devices` | |
| Thu hồi thiết bị | `DELETE /api/v1/me/devices/:id` | |
| Danh sách địa chỉ | `GET /api/v1/me/addresses` | |
| Thêm địa chỉ | `POST /api/v1/me/addresses` | |
| Cập nhật địa chỉ | `PATCH /api/v1/me/addresses/:id` | |
| Xóa địa chỉ | `DELETE /api/v1/me/addresses/:id` | |

---

## 🔴 NHÓM 1 — Bảo mật (Làm trước, không thương lượng)

### Task 1.1 — Quên mật khẩu & Đặt lại mật khẩu

**Tại sao quan trọng?**
Đây là flow nhạy cảm nhất. Nếu thiết kế sai, attacker có thể dùng API này để _dò xem email nào đã đăng ký_ (gọi là enumeration attack). SRD v3.0 §9.7 quy định rõ cách phòng chống.

**Kết quả cần đạt:**
- `POST /api/v1/auth/forgot-password` — nhận email, gửi link reset
- `POST /api/v1/auth/reset-password` — nhận token + mật khẩu mới, đổi xong thu hồi mọi session

---

**Bước 1 — Tạo Command** (`app/command/forgot_password.go`)
```go
type ForgotPasswordCommand struct {
    Email string
}

type ResetPasswordCommand struct {
    Token       string
    NewPassword string
}
```

**Bước 2 — Implement trong AuthService** (`app/service/auth_service.go`)

Cho `ForgotPassword`:
```
1. Tìm user theo email
2. Dù có tìm thấy hay không → đều trả về HTTP 200 với cùng message
   (Lý do: nếu trả về 404 khi không tìm thấy → attacker biết email chưa đăng ký)
3. Nếu tìm thấy user và email đã verified:
   → Sinh random token (crypto/rand, 32 bytes, hex encode)
   → Lưu vào Redis: key = "identity:pwd_reset:{token_hash}" , value = userID, TTL = 15 phút
   → Publish event "user.password_reset_requested" qua outbox
     (Notification worker sẽ gửi email từ event này)
```

Cho `ResetPassword`:
```
1. Hash token nhận được
2. GetDel từ Redis: "identity:pwd_reset:{token_hash}"
   → GetDel là atomic: lấy giá trị xong xóa luôn → token chỉ dùng được 1 lần
3. Nếu không tìm thấy → trả lỗi INVALID_RESET_TOKEN
4. Parse userID từ value
5. Hash mật khẩu mới bằng argon2id
6. Trong transaction:
   → UpdatePasswordHash trên CredentialRepository
   → RevokeAllByUserID trên SessionRepository (đăng xuất khỏi tất cả thiết bị)
7. Publish event "user.password_changed" qua outbox
```

**Bước 3 — HTTP Handlers** (`http/auth_handler.go`)

```
POST /api/v1/auth/forgot-password
- Body: { "email": "..." }
- Response: 200 OK dù có user hay không (chống enumeration)
- Thêm rate limiting: không được spam endpoint này

POST /api/v1/auth/reset-password  
- Body: { "token": "...", "new_password": "..." }
- Response: 200 OK nếu thành công
- Validate new_password: min 8 ký tự
```

**Bước 4 — Thêm route** (`http/router.go`)
```go
auth.POST("/forgot-password", idempotencyMiddleware, authHandler.ForgotPassword)
auth.POST("/reset-password", idempotencyMiddleware, authHandler.ResetPassword)
```

> ⚠️ **Bẫy hay gặp:**
> - **Không dùng time.Sleep để "giả vờ" mất thời gian xử lý** — attacker vẫn có thể đo thời gian phản hồi (timing attack). Response phải nhanh và giống nhau cho cả 2 trường hợp.
> - **Token phải hash trước khi lưu Redis** — nếu Redis bị lộ, attacker không lấy được token gốc. Dùng sha256(token) làm key.
> - **GetDel thay vì Get rồi Del** — nếu dùng Get+Del thì 2 request đồng thời có thể cùng verify thành công (race condition).

---

### Task 1.2 — Tự động khóa tài khoản sau đăng nhập sai nhiều lần

**Tại sao quan trọng?**
Không có cơ chế này, attacker có thể thử mật khẩu mãi mãi (brute-force). SRD §17.1 yêu cầu tài khoản bị khóa sau 5 lần sai liên tiếp.

**Chỉnh sửa `Login` trong `auth_service.go`:**
```
Sau khi verify password thất bại:
1. CredentialRepository.IncrementFailedCount(ctx, userID)
2. Lấy failed_login_count từ DB
3. Nếu failed_login_count >= 5:
   → UserRepository.LockAccount(ctx, userID, "too_many_failed_attempts")
   → Publish event "user.account_locked" qua outbox
   → Trả lỗi ACCOUNT_LOCKED (thay vì WRONG_PASSWORD)

Sau khi verify password thành công:
1. Nếu failed_login_count > 0: Reset về 0
   → CredentialRepository.ResetFailedCount(ctx, userID)
```

**Thêm methods vào Repository** (`domain/repository.go`):
```go
// CredentialRepository — thêm 2 method mới
IncrementFailedLoginCount(ctx context.Context, userID int64) (int, error)
ResetFailedLoginCount(ctx context.Context, userID int64) error

// UserRepository — thêm 1 method mới
LockAccount(ctx context.Context, userID int64, reason string) error
```

**Implement SQL** (`infra/postgres/credential_repo.go`):
```sql
-- IncrementFailedLoginCount
UPDATE user_credentials
SET failed_login_count = failed_login_count + 1,
    last_failed_login_at = NOW()
WHERE user_id = $1
RETURNING failed_login_count

-- ResetFailedLoginCount
UPDATE user_credentials
SET failed_login_count = 0
WHERE user_id = $1
```

**Thêm bẫy: Không khóa tài khoản vĩnh viễn**
Nên có cơ chế unlock sau X giờ (hoặc chỉ admin unlock). Đây là tradeoff giữa security và UX — quyết định business.

> ⚠️ **Bẫy hay gặp:**
> - **Đừng trả lỗi khác nhau** cho "sai mật khẩu" vs "tài khoản locked" — attacker sẽ biết khi nào tài khoản đã bị khóa và thôi thử. Thực tế bạn nên trả cùng 1 message chung chung.
> - **failed_login_count là trên DB, không phải Redis** — vì cần persist qua restart.

---

### Task 1.3 — Thu hồi một session cụ thể

**Tại sao cần?**
Hiện tại có `DELETE /me/sessions` (xóa tất cả) nhưng chưa có endpoint xóa một session cụ thể (ví dụ: "Đăng xuất khỏi iPhone 12 của tôi").

**Endpoint cần thêm:** `DELETE /api/v1/me/sessions/:sessionId`

**Implement trong ProfileService:**
```
1. Lấy session theo sessionId
2. Kiểm tra session.UserID == userID từ token (chống IDOR)
3. Revoke session
4. Nếu session đang revoke là session hiện tại → cần handle gracefully
```

**Thêm method vào SessionRepository** (`domain/repository.go`):
```go
GetByID(ctx context.Context, id int64) (*entity.Session, error)
```

**Route cần thêm** (`http/router.go`):
```go
me.DELETE("/sessions/:sessionId", profileHandler.RevokeSession)
```

> ⚠️ **Bẫy IDOR (Insecure Direct Object Reference):**
> Bắt buộc phải check `session.UserID == currentUserID` trước khi revoke.
> Nếu không check, user A có thể xóa session của user B bằng cách đoán sessionId.
> Đây là lỗi bảo mật cực kỳ phổ biến và nguy hiểm.

---

## 🟠 NHÓM 2 — Profile và Quản lý tài khoản

### Task 2.1 — Cập nhật thông tin profile

**Tại sao cần?**
Hiện tại user đăng ký xong không đổi được tên hay SĐT. `GET /me` đã có nhưng `PUT /me` chưa implement.

**Endpoint:** `PUT /api/v1/me`
```json
// Request body
{
  "full_name": "Nguyễn Văn A",
  "phone": "0901234567"
}

// Response
{
  "code": "PROFILE_UPDATED",
  "data": { "user_id": 1, "full_name": "...", "phone": "..." }
}
```

**Bước 1 — Tạo Command** (`app/command/update_profile.go`):
```go
type UpdateProfileCommand struct {
    UserID   int64
    FullName string
    Phone    *string  // Pointer vì phone có thể để trống
    Version  int64    // Dùng cho optimistic locking
}
```

**Bước 2 — Thêm method vào UserRepository** (`domain/repository.go`):
```go
UpdateProfile(ctx context.Context, user *entity.User) error
```

**Implement SQL** (`infra/postgres/user_repo.go`):
```sql
UPDATE users
SET full_name = $2, phone = $3, updated_at = NOW(), version = version + 1
WHERE id = $1 AND version = $4  -- optimistic locking
RETURNING version
```
Nếu `RowsAffected() == 0` → có 2 khả năng: user không tồn tại, hoặc có người khác đã update trước. Trả lỗi `CONFLICT` để client retry.

**Bước 3 — Implement trong ProfileService:**
```
1. GetByID lấy user hiện tại
2. Cập nhật full_name, phone trên entity
3. Gọi UserRepository.UpdateProfile với version từ entity
4. Nếu lỗi conflict → trả 409 Conflict
```

**Bước 4 — Handler và Route:**
```go
// http/profile_handler.go
me.PUT("", profileHandler.UpdateProfile)
```

> ⚠️ **Tại sao cần Optimistic Locking (version)?**
> Nếu user mở 2 tab cùng lúc và đều update profile → tab nào save sau sẽ ghi đè tab trước mà không báo lỗi. Version giải quyết vấn đề này: DB chỉ update nếu version khớp, bảo đảm không có lost update silently.

---

### Task 2.2 — Giới hạn số lượng địa chỉ (tối đa 10)

**Tại sao cần?**
Không giới hạn → user có thể tạo hàng nghìn địa chỉ → spam database.

**Thêm method vào AddressRepository** (`domain/repository.go`):
```go
CountByUserID(ctx context.Context, userID int64) (int, error)
```

**Implement SQL** (`infra/postgres/address_repo.go`):
```sql
SELECT COUNT(*) FROM addresses WHERE user_id = $1
```

**Sửa AddressService.AddAddress** (`app/service/address_service.go`):
```
Trước khi Insert:
count, err := s.address.CountByUserID(ctx, cmd.UserID)
if count >= 10 {
    return 0, err_domain.ErrAddressLimitReached
}
```

**Thêm error mới** (`domain/error/errors.go`):
```go
ErrAddressLimitReached = errors.New("address limit reached: maximum 10 addresses per user")
```

**Xử lý trong handler** (`http/address_handler.go`):
```go
case errors.Is(err, identity_err.ErrAddressLimitReached):
    httpx.Error(c, http.StatusUnprocessableEntity, "ADDRESS_LIMIT_REACHED", "maximum 10 addresses allowed")
```

---

### Task 2.3 — Đặt địa chỉ mặc định (endpoint riêng)

**Tại sao cần?**
Hiện tại muốn đặt default phải gọi `PATCH /addresses/:id` với toàn bộ body. Nên có endpoint riêng gọn hơn.

**Endpoint:** `PATCH /api/v1/me/addresses/:addressId/default`
```
- Không cần request body
- Logic: UnsetDefaultByUserID rồi SetDefault(addressId)
- Trả 200 OK
```

**Thêm method vào AddressRepository** (`domain/repository.go`):
```go
SetDefault(ctx context.Context, id int64) error
```

**SQL:**
```sql
UPDATE addresses SET is_default = true WHERE id = $1
```

**Route:**
```go
me.PATCH("/addresses/:addressId/default", addressHandler.SetDefaultAddress)
```

> ℹ️ **Lưu ý transaction:**
> UnsetDefaultByUserID và SetDefault phải nằm trong cùng một transaction. Nếu SetDefault thất bại mà UnsetDefault đã chạy → không còn địa chỉ mặc định nào cả. Dùng `s.txManager.WithinTransaction`.

---

## 🟡 NHÓM 3 — Outbox Worker và Background Jobs

### Task 3.1 — Outbox Dispatcher Worker

**Tại sao cần?**
Events đang được lưu vào bảng `outbox_events` sau mỗi transaction (register, login, v.v.) nhưng chưa có worker để đọc và publish lên Kafka. Nếu không có worker, mọi event sẽ mãi ở trạng thái `pending`.

**Tư duy:**
```
Outbox pattern hoạt động như thế này:
  [Business Transaction] → lưu event vào outbox_events (cùng transaction)
  [Worker chạy ngầm]    → đọc events pending → publish Kafka → đánh dấu published
```

**Tạo file mới:** `cmd/worker/outbox_dispatcher.go`

**Logic của worker:**
```go
// Chạy vòng lặp mỗi X giây (ví dụ 2 giây)
for {
    // Bước 1: Lấy batch events pending (dùng FOR UPDATE SKIP LOCKED)
    // → FOR UPDATE SKIP LOCKED đảm bảo nếu chạy nhiều pod, mỗi pod lấy batch khác nhau
    events := db.Query(`
        SELECT id, topic, event_key, aggregate_type, aggregate_id, event_type, payload, metadata
        FROM outbox_events
        WHERE state = 'pending'
        ORDER BY created_at ASC
        LIMIT 20
        FOR UPDATE SKIP LOCKED
    `)
    
    for _, event := range events {
        // Bước 2: Publish lên Kafka
        err := kafkaProducer.Publish(event.Topic, event.EventKey, event.Payload)
        
        if err != nil {
            // Bước 3a: Nếu fail → đánh dấu failed, tăng retry count
            db.Exec("UPDATE outbox_events SET state='failed', last_error=$1 WHERE id=$2", 
                err.Error(), event.ID)
        } else {
            // Bước 3b: Nếu thành công → đánh dấu published
            db.Exec("UPDATE outbox_events SET state='published', published_at=NOW() WHERE id=$2",
                event.ID)
        }
    }
    
    time.Sleep(2 * time.Second)
}
```

**Retry logic (Exponential Backoff):**
```
Lần 1 fail: thử lại sau 30 giây
Lần 2 fail: thử lại sau 2 phút
Lần 3 fail: thử lại sau 10 phút
Lần 4+ fail: chuyển sang dead-letter, cần manual review
```
Để implement được, cần thêm cột `retry_count INT DEFAULT 0` và `next_retry_at TIMESTAMPTZ` vào `outbox_events`.

> ⚠️ **Bẫy phổ biến — At-least-once delivery:**
> Kafka có thể nhận được cùng 1 message 2 lần (do network retry). Consumer phải dùng bảng `processed_events` để dedup (kiểm tra event đã xử lý chưa trước khi process). Đây không phải lỗi — đây là thiết kế đúng.

---

### Task 3.2 — Dọn dẹp processed_events định kỳ

**Tại sao cần?**
Bảng `processed_events` chỉ cần nhớ events trong vòng 14 ngày (Kafka mặc định giữ message 7 ngày, ta nhớ 2x để an toàn). Không dọn → bảng phình vô hạn → query chậm dần.

**Tạo Scheduled Job:** `cmd/worker/cleanup_worker.go`

```go
// Chạy 1 lần/ngày, lúc ít traffic (ví dụ 2 giờ sáng)
func cleanupProcessedEvents(db *pgxpool.Pool) {
    // Xóa theo batch nhỏ để không lock bảng
    for {
        result := db.Exec(`
            DELETE FROM processed_events
            WHERE processed_at < NOW() - INTERVAL '14 days'
            AND ctid IN (
                SELECT ctid FROM processed_events
                WHERE processed_at < NOW() - INTERVAL '14 days'
                LIMIT 1000   -- Mỗi lần chỉ xóa 1000 dòng
            )
        `)
        if result.RowsAffected() == 0 {
            break  // Không còn gì để xóa
        }
        time.Sleep(100 * time.Millisecond)  // Nghỉ giữa các batch
    }
}
```

> ℹ️ **Tại sao xóa theo batch?**
> Nếu xóa 10 triệu dòng cùng lúc bằng 1 câu DELETE → PostgreSQL sẽ lock bảng, tạo transaction khổng lồ, làm chậm toàn bộ hệ thống. Batch nhỏ + sleep giữa các batch = an toàn hơn nhiều.

---

### Task 3.3 — Dọn dẹp session và reservation hết hạn

**Tại sao cần?**
Sessions với `expires_at < NOW()` và `session_status = 'active'` vẫn nằm trong DB dù không còn dùng được. Tương tự với outbox events đã published lâu ngày.

**Job cleanup sessions:**
```sql
UPDATE user_sessions
SET session_status = 'expired'
WHERE session_status = 'active'
  AND expires_at < NOW()
LIMIT 500
```

**Job cleanup outbox đã published (> 30 ngày):**
```sql
DELETE FROM outbox_events
WHERE state = 'published'
  AND published_at < NOW() - INTERVAL '30 days'
LIMIT 1000
```

---

## 🟢 NHÓM 4 — Cải thiện chất lượng code

### Task 4.1 — Chuẩn hóa Error Handling ở HTTP layer

**Vấn đề hiện tại:**
Nhiều chỗ trong handler dùng `switch-case` lặp đi lặp lại. Nên tập trung vào một hàm map error → HTTP response.

**Tạo file:** `http/error_map.go`
```go
// writeIdentityError là hàm duy nhất xử lý lỗi cho toàn bộ identity module HTTP layer
func writeIdentityError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, identity_err.ErrUserNotFound):
        httpx.Error(c, 404, "USER_NOT_FOUND", "user not found")
    case errors.Is(err, identity_err.ErrWrongPassword):
        httpx.Error(c, 401, "WRONG_CREDENTIALS", "invalid email or password")
    case errors.Is(err, identity_err.ErrAccountLocked):
        httpx.Error(c, 403, "ACCOUNT_LOCKED", "account has been locked")
    case errors.Is(err, identity_err.ErrAddressNotFound):
        httpx.Error(c, 404, "ADDRESS_NOT_FOUND", "address not found")
    case errors.Is(err, identity_err.ErrAddressLimitReached):
        httpx.Error(c, 422, "ADDRESS_LIMIT_REACHED", "maximum 10 addresses allowed")
    case errors.Is(err, identity_err.ErrAddressNotOwned):
        httpx.Error(c, 403, "FORBIDDEN", "you don't own this resource")
    // ... các lỗi khác
    default:
        // Log lỗi thật ở đây (logger.Error)
        httpx.Error(c, 500, "INTERNAL_ERROR", "internal server error")
    }
}
```

---

### Task 4.2 — Cải thiện GetMe response

**Vấn đề hiện tại:**
`GET /me` trả về `query.MeView` nhưng thiếu `phone` và `user_type`.

**Sửa `query/model.go`** — thêm field vào `MeView`:
```go
type MeView struct {
    UserID          int64
    Email           string
    FullName        string
    Phone           *string    // thêm
    UserType        string     // thêm
    Status          string
    EmailVerifiedAt *time.Time
    CreatedAt       time.Time
}
```

**Sửa `mapper.go`** — `mapUserRowToMeView` bổ sung các field mới.

**Sửa response JSON** để client nhận đủ thông tin cần thiết.

> ℹ️ **Lưu ý:** Query SQL trong `query_repo.go` đã chọn đủ các column này rồi (đã fix ở session trước). Chỉ cần thêm field vào struct và mapper.

---

### Task 4.3 — Chuẩn hóa Redis key prefix

**Vấn đề hiện tại:**
Code dùng `refresh_token:` làm prefix cho session Redis nhưng SRD v3.0 §15.3 quy định phải dùng `identity:session:`. Cần thống nhất để dễ quản lý và tránh collision khi thêm module mới.

**File cần sửa:** `app/ports/redis_manager.go`
```go
// Cũ:
const RedisSessionKeyPrefix = "refresh_token:"

// Mới (theo SRD v3.0 §15.3):
const RedisSessionKeyPrefix = "identity:session:"
```

**Lưu ý:** Sau khi đổi prefix, các session đang lưu ở Redis với key cũ sẽ không tìm được nữa → tất cả user sẽ bị đăng xuất. Đây là **breaking change** cần deploy vào giờ thấp điểm.

---

### Task 4.4 — Viết Integration Test cho flow cốt lõi

**Tại sao quan trọng?**
Unit test chỉ test từng hàm riêng lẻ. Integration test chạy flow đầu-đến-cuối thật sự qua DB, đảm bảo mọi thứ hoạt động cùng nhau.

**Tạo file:** `tests/integration/identity_test.go`

**Flow cần test:**
```
Flow 1: Happy path Authentication
  → POST /register → 201
  → POST /login → 401 (email chưa verified)
  → POST /verify-email → 200
  → POST /login → 200 (nhận access token + cookie)
  → GET /me → 200 (xem profile)
  → POST /refresh-token → 200 (token mới)
  → POST /logout → 200

Flow 2: Security - Brute Force
  → POST /login với sai password 5 lần → 5 lần đầu 401
  → Lần 6 → tài khoản bị lock → 403

Flow 3: Password Reset
  → POST /forgot-password → 200 (dù email có tồn tại hay không)
  → (Lấy token từ Redis trong test) POST /reset-password → 200
  → POST /login với password cũ → 401
  → POST /login với password mới → 200

Flow 4: Concurrent Token Refresh
  → Tạo 2 goroutine cùng gọi POST /refresh-token với cùng token
  → Chỉ 1 request thành công, request còn lại nhận lỗi INVALID_TOKEN
  → Đảm bảo không bị double-refresh (race condition)
```

**Setup test:**
```go
// Dùng testcontainers để tạo PostgreSQL và Redis thật trong Docker
// Sau mỗi test: truncate tất cả bảng để test độc lập
```

---

## 📋 Thứ tự ưu tiên thực hiện

```
Tuần 1 (Bảo mật - Phải làm ngay):
  ✦ Task 1.1 — Forgot/Reset Password
  ✦ Task 1.2 — Brute-force lockout
  ✦ Task 1.3 — Thu hồi session cụ thể

Tuần 2 (UX và nghiệp vụ):
  ✦ Task 2.1 — Cập nhật profile
  ✦ Task 2.2 — Giới hạn 10 địa chỉ
  ✦ Task 2.3 — Endpoint set default address

Tuần 3 (Background và chất lượng):
  ✦ Task 3.1 — Outbox Worker
  ✦ Task 3.2 — Cleanup processed_events
  ✦ Task 4.1 — Chuẩn hóa error handling

Tuần 4 (Test và hoàn thiện):
  ✦ Task 4.4 — Integration tests
  ✦ Task 4.2 — Cải thiện GetMe response
  ✦ Task 4.3 — Chuẩn hóa Redis key (deploy giờ thấp điểm)
```

---

## 🗺️ Sơ đồ file cần chạm cho mỗi tính năng mới

Mỗi khi thêm một tính năng mới vào module identity, bạn thường sẽ chạm vào các file theo thứ tự từ trong ra ngoài:

```
1. domain/error/errors.go          ← Thêm error mới (nếu cần)
2. domain/entity/*.go              ← Thêm field hoặc method vào entity (nếu cần)
3. domain/repository.go            ← Khai báo method mới vào interface
4. app/command/new_command.go      ← Tạo Command struct mới
5. app/service/auth_service.go     ← Implement use case
6. infra/postgres/*_repo.go        ← Implement SQL query
7. http/request.go                 ← Thêm request struct
8. http/auth_handler.go            ← Thêm handler method
9. http/router.go                  ← Đăng ký route mới
```

> 📌 **Nguyên tắc vàng:** Bắt đầu từ **domain** (bên trong), đi dần ra **HTTP** (bên ngoài). Không bao giờ viết SQL trước khi xác định được interface domain cần gì.

---

## 🚨 Những điều tuyệt đối KHÔNG làm

| Sai lầm | Hậu quả | Cách đúng |
|---|---|---|
| Query DB trực tiếp trong HTTP handler | Phá vỡ Clean Architecture, không test được | Luôn gọi qua Service layer |
| Trả lỗi "User not found" cho forgot-password | Attacker dò được email nào đã đăng ký | Trả 200 OK dù có user hay không |
| Lưu refresh token gốc vào DB | Nếu DB bị lộ, attacker login được | Chỉ lưu hash của token |
| Dùng `bcrypt` mặc định cost factor 10 | Quá yếu so với argon2id | Dùng `argon2id` như đã cấu hình |
| Bypass `version` check khi update | Lost update silently | Luôn check version (optimistic locking) |
| Xóa 10 triệu dòng bằng 1 câu DELETE | Lock bảng, hệ thống chết | Xóa theo batch 1000 dòng |
| Không check ownership trước khi xóa | User A xóa địa chỉ của user B (IDOR) | Luôn `AssertOwnership` trước khi mutate |
