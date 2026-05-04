/*
Chào bạn, sau khi đi qua Outbox (bắn event) và Inbox (nhận event), bảng `notification_jobs` này cho thấy bạn đang xây dựng một **Job Queue (Hàng đợi công việc)** nội bộ bằng PostgreSQL. Đây là một kiến trúc rất thực dụng và phổ biến để xử lý các tác vụ gửi Email, SMS, Push Notification (vốn thường có độ trễ cao và hay bị lỗi từ bên thứ 3).

Nhìn chung, cấu trúc của bạn đã bao phủ được 80% yêu cầu của một hệ thống Notification chuẩn mực. Dưới đây là phần phân tích chuyên sâu cho 20% còn lại để biến nó thành một hệ thống **High-performance & Fault-tolerant (Chịu tải cao và Chống chịu lỗi)**.

### 🌟 Điểm cộng xuất sắc (The Good)

1.  **Thiết kế decoupled (Tách rời):** Bạn tách bạch `channel` (email, sms) và `template_code`. Điều này giúp hệ thống rất linh hoạt. Code gửi thông báo không cần hardcode nội dung, chỉ cần ném payload vào là xong.
2.  **Có cơ chế Retry rõ ràng:** Sự kết hợp giữa `attempts` và `last_error` là **bắt buộc phải có** khi làm việc với Notification. Các API của SendGrid, Twilio, hay Firebase rất hay bị timeout hoặc rate-limit. Đếm số lần thử để có chiến lược dừng lại (chuyển sang `failed`) là thiết kế chuẩn.
3.  **State Machine với CHECK Constraint:** Giới hạn chặt chẽ vòng đời của 1 job (`pending` ➡️ `sent` / `failed` / `cancelled`) giúp tránh dữ liệu rác.
4.  **Index `idx_notification_jobs_recipient`:** Rất tinh tế! Index này phục vụ hoàn hảo cho tính năng: *"Cho tôi xem lịch sử nhận thông báo của user A"*.

---

### 💣 3 Điểm nghẽn cần tối ưu (The Master Tweaks)

**1. Thiếu cơ chế "Exponential Backoff" (Lùi lịch thử lại)**
Khi gửi một Email thất bại do mạng chậm, nếu bạn để nguyên trạng thái `pending`, Worker của bạn sẽ ngay lập tức "bốc" job đó lên và thử lại liên tục trong cùng 1 giây. Việc spam retry này sẽ khiến bạn bị block API từ đối tác.
👉 **Giải pháp:** Bạn cần thêm cột `next_run_at TIMESTAMPTZ`. Mỗi khi lỗi, bạn cộng dồn thời gian chờ (lần 1 chờ 1 phút, lần 2 chờ 5 phút, lần 3 chờ 15 phút) vào cột này. Worker chỉ được phép quét những job có `next_run_at <= NOW()`.

**2. Ảo giác về cột `updated_at` trong PostgreSQL**
Khác với MySQL (có tính năng `ON UPDATE CURRENT_TIMESTAMP`), trong PostgreSQL, việc gán `DEFAULT NOW()` cho cột `updated_at` **chỉ có tác dụng khi INSERT**. Khi bạn chạy lệnh `UPDATE`, cột này sẽ không tự đổi thời gian trừ khi bạn tự truyền giá trị mới vào bằng code Go, hoặc bạn phải viết một Database Trigger.
👉 **Giải pháp:** Luôn nhớ truyền `updated_at = NOW()` trong câu lệnh Update của Go, hoặc tạo Trigger cho bảng này.

**3. Bài toán "Đàn cừu giẫm đạp" (Thundering Herd) của Worker**
Bạn có Index trên `(state, created_at)`, điều này tốt nhưng chưa đủ. Nếu bạn có 5 tiến trình Notification Worker cùng chạy song song, chúng sẽ cùng `SELECT` ra 100 job giống hệt nhau, và gửi đi 5 cái email y chang nhau cho cùng 1 user (Gây thảm họa spam).

---

### Bản vá đề xuất (The Ultimate Schema):

```sql
CREATE TABLE IF NOT EXISTS notification_jobs (
    id BIGSERIAL PRIMARY KEY,
    channel VARCHAR(30) NOT NULL,
    template_code VARCHAR(100) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    state VARCHAR(30) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- MỚI: Dành cho Retry Backoff
    CHECK (state IN ('pending', 'processing', 'sent', 'failed', 'cancelled')), -- MỚI: Thêm state 'processing'
    CHECK (attempts >= 0)
);

-- Index này giờ đây cực kỳ quyền lực cho Worker
CREATE INDEX IF NOT EXISTS idx_notification_jobs_queue 
    ON notification_jobs (state, next_run_at);

CREATE INDEX IF NOT EXISTS idx_notification_jobs_recipient 
    ON notification_jobs (recipient);
```

### 💡 Tips Master: Cách lấy Job an toàn bằng Golang (Chống xử lý trùng)

Để giải quyết bài toán "Đàn cừu giẫm đạp" ở trên, PostgreSQL có một "phép thuật" tên là **`FOR UPDATE SKIP LOCKED`**. 

Khi Worker của bạn muốn lấy job ra để gửi thông báo, HÃY DÙNG CÂU QUERY NÀY:

```go
// Câu lệnh này làm 3 việc CÙNG MỘT LÚC (Atomic):
// 1. Tìm các job đang 'pending' và đến giờ chạy (next_run_at <= NOW)
// 2. Bỏ qua ngay lập tức những job mà các Worker khác đang nắm giữ (SKIP LOCKED)
// 3. Đổi trạng thái job đó sang 'processing' và update next_run_at để không ai lấy được nữa
query := `
    UPDATE notification_jobs
    SET state = 'processing',
        updated_at = NOW()
    WHERE id IN (
        SELECT id
        FROM notification_jobs
        WHERE state = 'pending' AND next_run_at <= NOW()
        ORDER BY next_run_at ASC
        LIMIT $1
        FOR UPDATE SKIP LOCKED
    )
    RETURNING id, channel, template_code, recipient, payload, attempts
`

// worker.go
rows, err := db.QueryContext(ctx, query, batchSize)
// ... Sau đó xử lý gửi thông báo ...
```

**Tại sao nó thần thánh?**
Với `SKIP LOCKED`, bạn có thể bật 100 cái pod Notification Worker trên Kubernetes. Khi chúng cùng lao vào Database, mỗi Worker sẽ "nhặt" được một tập hợp job hoàn toàn khác nhau mà không bao giờ bị đụng độ (lock contention) hay gửi trùng email. Đây là tiêu chuẩn vàng khi dùng PostgreSQL làm Message Queue!
*/

CREATE TABLE IF NOT EXISTS notification_jobs (
    id BIGSERIAL PRIMARY KEY,
    channel VARCHAR(30) NOT NULL,
    template_code VARCHAR(100) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    state VARCHAR(30) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- MỚI: Dành cho Retry Backoff
    CHECK (state IN ('pending', 'processing', 'sent', 'failed', 'cancelled')), -- MỚI: Thêm state 'processing'
    CHECK (attempts >= 0)
);

-- Index này giờ đây cực kỳ quyền lực cho Worker
CREATE INDEX IF NOT EXISTS idx_notification_jobs_queue 
    ON notification_jobs (state, next_run_at);

CREATE INDEX IF NOT EXISTS idx_notification_jobs_recipient 
    ON notification_jobs (recipient);


/*
### 🌟 Điểm cộng xuất sắc (The Good)

1.  **Thiết kế decoupled (Tách rời):** Bạn tách bạch `channel` (email, sms) và `template_code`. Điều này giúp hệ thống rất linh hoạt. Code gửi thông báo không cần hardcode nội dung, chỉ cần ném payload vào là xong.
2.  **Có cơ chế Retry rõ ràng:** Sự kết hợp giữa `attempts` và `last_error` là **bắt buộc phải có** khi làm việc với Notification. Các API của SendGrid, Twilio, hay Firebase rất hay bị timeout hoặc rate-limit. Đếm số lần thử để có chiến lược dừng lại (chuyển sang `failed`) là thiết kế chuẩn.
3.  **State Machine với CHECK Constraint:** Giới hạn chặt chẽ vòng đời của 1 job (`pending` ➡️ `sent` / `failed` / `cancelled`) giúp tránh dữ liệu rác.
4.  **Index `idx_notification_jobs_recipient`:** Rất tinh tế! Index này phục vụ hoàn hảo cho tính năng: *"Cho tôi xem lịch sử nhận thông báo của user A"*.

---

### 💣 3 Điểm nghẽn cần tối ưu (The Master Tweaks)

**1. Thiếu cơ chế "Exponential Backoff" (Lùi lịch thử lại)**
Khi gửi một Email thất bại do mạng chậm, nếu bạn để nguyên trạng thái `pending`, Worker của bạn sẽ ngay lập tức "bốc" job đó lên và thử lại liên tục trong cùng 1 giây. Việc spam retry này sẽ khiến bạn bị block API từ đối tác.
👉 **Giải pháp:** Bạn cần thêm cột `next_run_at TIMESTAMPTZ`. Mỗi khi lỗi, bạn cộng dồn thời gian chờ (lần 1 chờ 1 phút, lần 2 chờ 5 phút, lần 3 chờ 15 phút) vào cột này. Worker chỉ được phép quét những job có `next_run_at <= NOW()`.

**2. Ảo giác về cột `updated_at` trong PostgreSQL**
Khác với MySQL (có tính năng `ON UPDATE CURRENT_TIMESTAMP`), trong PostgreSQL, việc gán `DEFAULT NOW()` cho cột `updated_at` **chỉ có tác dụng khi INSERT**. Khi bạn chạy lệnh `UPDATE`, cột này sẽ không tự đổi thời gian trừ khi bạn tự truyền giá trị mới vào bằng code Go, hoặc bạn phải viết một Database Trigger.
👉 **Giải pháp:** Luôn nhớ truyền `updated_at = NOW()` trong câu lệnh Update của Go, hoặc tạo Trigger cho bảng này.

**3. Bài toán "Đàn cừu giẫm đạp" (Thundering Herd) của Worker**
Bạn có Index trên `(state, created_at)`, điều này tốt nhưng chưa đủ. Nếu bạn có 5 tiến trình Notification Worker cùng chạy song song, chúng sẽ cùng `SELECT` ra 100 job giống hệt nhau, và gửi đi 5 cái email y chang nhau cho cùng 1 user (Gây thảm họa spam).

---


### 💡 Tips Master: Cách lấy Job an toàn bằng Golang (Chống xử lý trùng)

Để giải quyết bài toán "Đàn cừu giẫm đạp" ở trên, PostgreSQL có một "phép thuật" tên là **`FOR UPDATE SKIP LOCKED`**. 

Khi Worker của bạn muốn lấy job ra để gửi thông báo, HÃY DÙNG CÂU QUERY NÀY:

```go
// Câu lệnh này làm 3 việc CÙNG MỘT LÚC (Atomic):
// 1. Tìm các job đang 'pending' và đến giờ chạy (next_run_at <= NOW)
// 2. Bỏ qua ngay lập tức những job mà các Worker khác đang nắm giữ (SKIP LOCKED)
// 3. Đổi trạng thái job đó sang 'processing' và update next_run_at để không ai lấy được nữa
query := `
    UPDATE notification_jobs
    SET state = 'processing',
        updated_at = NOW()
    WHERE id IN (
        SELECT id
        FROM notification_jobs
        WHERE state = 'pending' AND next_run_at <= NOW()
        ORDER BY next_run_at ASC
        LIMIT $1
        FOR UPDATE SKIP LOCKED
    )
    RETURNING id, channel, template_code, recipient, payload, attempts
`

// worker.go
rows, err := db.QueryContext(ctx, query, batchSize)
// ... Sau đó xử lý gửi thông báo ...
```

**Tại sao nó thần thánh?**
Với `SKIP LOCKED`, bạn có thể bật 100 cái pod Notification Worker trên Kubernetes. Khi chúng cùng lao vào Database, mỗi Worker sẽ "nhặt" được một tập hợp job hoàn toàn khác nhau mà không bao giờ bị đụng độ (lock contention) hay gửi trùng email. Đây là tiêu chuẩn vàng khi dùng PostgreSQL làm Message Queue!*/