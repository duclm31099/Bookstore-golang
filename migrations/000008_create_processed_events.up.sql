CREATE TABLE IF NOT EXISTS processed_events (
    consumer_name VARCHAR(100) NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (consumer_name, event_id)
);

/*
### 🌟 Điểm cộng xuất sắc (The Good)

1.  **Sức mạnh tuyệt đối của `PRIMARY KEY (consumer_name, event_id)`:** Đây là điểm sáng giá nhất trong file migration này. Thay vì tự code logic `SELECT` kiểm tra xem event đã tồn tại chưa rồi mới `INSERT` (rất dễ dính lỗi Race Condition khi có nhiều luồng chạy song song), bạn đang dùng chính **Database Unique Constraint** làm lá chắn. Database sẽ khóa chặt mọi nỗ lực xử lý lặp lại với chi phí rẻ nhất và an toàn nhất.
2.  **Thiết kế Đa nhiệm (Multi-tenant) bằng `consumer_name`:** Một event `OrderCreated` có thể được xử lý bởi nhiều service khác nhau (ví dụ: `EmailService`, `InventoryService`). Việc kẹp thêm `consumer_name` vào Primary Key đảm bảo rằng mỗi service sẽ có một "bộ nhớ" độc lập về việc nó đã xử lý event đó hay chưa, không giẫm chân lên nhau.
3.  **`event_id` dùng `VARCHAR(255)`:** Rất an toàn. Nó hứng được mọi chuẩn ID từ các hệ thống khác đẩy tới (UUID, Snowflake, XID...) mà không lo lỗi tràn kiểu số.
4.  **`metadata JSONB`:** Giống như bảng Outbox, cột này rất lý tưởng để bạn lưu lại dấu vết (TraceID) hoặc nguyên nhân nếu sự kiện có xử lý thành công nhưng kèm theo warning nghiệp vụ.

---

### 💣 Quả bom nổ chậm cần tháo gỡ (The Master Tweaks)

Về mặt logic, bảng này hoàn hảo. Tuy nhiên, về mặt **Vận hành (Operations) và Quản trị Database**, bảng này đang ẩn chứa một "quả bom nổ chậm": **Dữ liệu phình to vô hạn (Infinite Growth)**.

Khác với bảng dữ liệu nghiệp vụ (như `users` hay `orders`), bảng `processed_events` là dạng dữ liệu *Log*. Nếu hệ thống của bạn nhận 1 triệu event mỗi ngày, bảng này sẽ tăng thêm 30 triệu dòng mỗi tháng.
Khi Primary Key Index (B-Tree) phình to đến mức không thể chứa vừa trong bộ nhớ RAM, tốc độ `INSERT` của toàn bộ Consumer sẽ tụt thê thảm, kéo theo hiện tượng Consumer Lag trên Kafka.

**Giải pháp đề xuất:**
Bạn bắt buộc phải có chiến lược **Data Retention (Dọn rác)** cho bảng này.
*   *Lý thuyết:* Kafka chỉ giữ log trong một khoảng thời gian (mặc định thường là 7 ngày). Do đó, bạn không cần phải nhớ một event đã xử lý từ 1 tháng trước làm gì cả, vì Kafka làm gì còn event đó mà gửi lại cho bạn nữa!
*   *Thực thi:* Bạn cần cấu hình một Cronjob chạy ngầm (hoặc dùng pg_cron nếu xài PostgreSQL extension) để xóa dữ liệu cũ.
    ```sql
    -- Ví dụ câu lệnh dọn rác chạy lúc nửa đêm
    DELETE FROM processed_events 
    WHERE processed_at < NOW() - INTERVAL '14 days';
    ```
    *Lưu ý:* Có thể cân nhắc dùng cơ chế **Table Partitioning** (phân vùng bảng) theo thời gian (ví dụ: mỗi tuần 1 partition). Khi dọn rác, bạn chỉ cần `DROP PARTITION` là data bay màu trong 1 phần nghìn giây mà không bị lock bảng như lệnh `DELETE`.

---

⚠️ 3 Quy Tắc Vàng Khi Xóa Dữ Liệu Bằng Go

1. KHÔNG BAO GIỜ xóa tất cả trong 1 câu lệnh (Never delete all at once)
Nếu bạn chạy DELETE FROM processed_events WHERE processed_at < ... khi bảng có 10 triệu dòng rác, Postgres sẽ tạo ra một Transaction khổng lồ, phình to bộ nhớ, khóa (lock) các index, và làm chậm/treo toàn bộ các Consumer đang Insert vào.
👉 Giải pháp: Phải xóa theo từng mẻ nhỏ (Batch Delete/Chunking), ví dụ 5000 dòng mỗi lần.

2. Tận dụng ctid của PostgreSQL để tối ưu Batch Delete
Vì bảng của bạn dùng Composite Primary Key (consumer_name, event_id), việc viết câu query xóa theo batch khá cồng kềnh. Trong Postgres có một "vũ khí bí mật" là cột ẩn ctid (chỉ định vị trí vật lý của record trên ổ cứng). Dùng ctid để xóa batch là cách nhanh và nhẹ nhất.

3. Bài toán đa bản sao (Distributed Concurrency)
Ứng dụng Go của bạn khi deploy lên K8s có thể chạy 5-10 Pods (replicas). Nếu bạn dùng cronjob nội bộ trong Go (như time.Ticker), lúc 12h đêm, cả 10 Pods sẽ cùng tranh nhau lao vào Database để xóa dữ liệu, gây ra hiện tượng Deadlock hoặc lãng phí tài nguyên.
👉 Giải pháp: Sử dụng PostgreSQL Advisory Lock để đảm bảo tại 1 thời điểm, chỉ có DUY NHẤT 1 tiến trình Go được phép chạy hàm dọn rác.
```
*/