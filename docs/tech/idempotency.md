`internal/platform/idempotency/` nên được thiết kế như một lớp **chống xử lý lặp** dùng chung cho cả HTTP mutation và event consumer, bám đúng tinh thần của dự án là request nhạy cảm phải có idempotency key, còn consumer phải chống duplicate bằng `processedevents`.[^1][^2]
Nói ngắn gọn, package này không chỉ “chặn request gửi hai lần”, mà còn là nơi chuẩn hóa cách hệ thống trả lại cùng một kết quả cho cùng một key và cách worker bỏ qua event đã xử lý rồi mà không làm sai state.[^2][^1]

## Mục tiêu package

Theo SRD, các endpoint mutation nhạy cảm như checkout hoặc tạo order phải yêu cầu idempotency key, và Redis được dùng cho idempotency key storage, rate limiting, session, lock và các hot-path lookup; trong khi đó PostgreSQL vẫn là nguồn sự thật cuối cùng cho transactional state.[^1][^2]
Theo ERD, bảng `processedevents` đã được thiết kế riêng cho consumer idempotency với khóa chính `(consumername, eventid)`, vì vậy package này nên chia rõ hai nhánh: HTTP idempotency thiên về Redis để nhanh, còn consumer idempotency thiên về PostgreSQL để bền và audit được.[^2][^1]

```text
internal/platform/idempotency/
├── interface.go
├── key.go
├── record.go
├── service.go
├── middleware.go
├── redis_store.go
├── processed_event_store.go
├── errors.go
└── provider.go
```

## Cơ chế hoạt động

Luồng HTTP nên chạy theo 4 bước: nhận `Idempotency-Key` từ client, chuẩn hóa key theo scope nghiệp vụ, thử “giữ chỗ” key trong Redis bằng TTL, rồi hoặc trả lại kết quả cũ nếu key đã hoàn tất, hoặc cho request chạy tiếp và lưu kết quả để những lần gọi sau replay lại đúng response cũ.[^1]
Luồng consumer nên khác hơn: trước khi làm side effect, consumer ghi một dấu mốc vào `processedevents`; nếu trùng khóa chính thì hiểu rằng event này đã xử lý trước đó và bỏ qua an toàn, đúng với yêu cầu consumer idempotent và duplicate delivery không được làm sai state.[^2][^1]
Dưới góc nhìn backend production, ý nghĩa lớn nhất của package này là tách “policy chống duplicate” ra khỏi business module, để Order, Payment, Identity hay Notification đều dùng chung một cơ chế thay vì mỗi module tự vá lỗi theo kiểu riêng lẻ.[^3][^1][^2]

## Files lõi

Bảng dưới đây là phần “xương sống” của package, tức những file quyết định package này nói ngôn ngữ gì, lưu dữ liệu gì, và điều phối luồng idempotency ra sao.[^1][^2]

| File           | Vai trò chính                                                                                                                                                             | Ý nghĩa nghiệp vụ / kỹ thuật                                                                                                                                                                                                                         |
| :------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `interface.go` | Khai báo các contract như `Service`, `Store`, `ProcessedEventStore` để app layer và middleware phụ thuộc vào abstraction thay vì dính chặt Redis hay PostgreSQL. [^3][^1] | Đây là điểm rất “master-level” vì nó giữ được layering: business service biết mình cần chống duplicate, nhưng không cần biết dữ liệu đang nằm ở Redis hay bảng `processedevents`. [^3][^2]                                                           |
| `key.go`       | Chuẩn hóa cách tạo key theo scope, ví dụ `checkout:user:123:abc-key` hoặc `payment_webhook:provider:ref`. [^1]                                                            | File này quan trọng vì nếu key không có scope rõ ràng thì hai hành động khác nhau có thể đụng nhau, dẫn tới replay nhầm kết quả; nói cách khác, key builder là nơi biến “chuỗi client gửi lên” thành “định danh kỹ thuật an toàn cho hệ thống”. [^1] |
| `record.go`    | Định nghĩa model lưu trạng thái của một idempotent request như `key`, `scope`, `status`, `response_code`, `response_body`, `created_at`, `expires_at`. [^1]               | Về kỹ thuật, đây là “biên nhận” cho mỗi request; về nghiệp vụ, nó giúp hệ thống trả lại đúng kết quả cũ thay vì thực hiện lại hành động mua hàng, thanh toán hay cấp link tải. [^1]                                                                  |
| `service.go`   | Chứa orchestration chính: `Reserve`, `ExecuteOnce`, `Complete`, `Fail`, `ReplayResult`, `MarkInProgress`. [^1][^2]                                                        | File này là “bộ não” của package, vì mọi policy chống duplicate phải đi qua đây; nó giúp business code gọi một API nhất quán thay vì tự viết đi viết lại logic check key, set TTL, lưu response và xử lý race condition. [^1][^2]                    |
| `errors.go`    | Chuẩn hóa các lỗi như thiếu key, key đang được xử lý, replay conflict, store unavailable, processed event already exists. [^1][^3]                                        | Đây là chỗ biến các tình huống kỹ thuật thành ngôn ngữ mà middleware, HTTP layer và worker có thể hiểu để map ra status code, log và retry policy thống nhất. [^1][^3]                                                                               |

Nếu nhìn như một kiến trúc sư backend, 5 file trên chính là phần “domain của platform”: nó mô tả package này làm gì, dữ liệu nó hiểu là gì, và các trạng thái sống của một idempotent operation ra sao.[^3][^2][^1]
Nhờ tách như vậy, về sau anh có thể thay Redis bằng store khác hoặc mở rộng thêm SQL-backed request idempotency mà business code gần như không đổi.[^3][^1]

## Files adapter và hạ tầng

Các file dưới đây là nơi nối phần lõi ở trên với hạ tầng thật của dự án: Redis cho HTTP hot-path, PostgreSQL cho consumer dedup, và bootstrap để DI.[^2][^3][^1]

| File                       | Vai trò chính                                                                                                                                        | Ý nghĩa nghiệp vụ / kỹ thuật                                                                                                                                                                            |
| :------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `middleware.go`            | Middleware đọc `Idempotency-Key`, xác định scope theo route hoặc use case, gọi `service.go` để reserve/replay rồi bọc handler mutation. [^1]         | Đây là nơi package “đứng gác cổng”; đặt ở middleware giúp chống duplicate ngay từ mép hệ thống, trước khi request chạm sâu vào order creation, checkout hay payment flow. [^1]                          |
| `redis_store.go`           | Cài đặt `Store` bằng Redis theo đúng vai trò Redis trong dự án: lưu dữ liệu ngắn hạn, hot-path, TTL-based và phải chịu tải concurrency tốt. [^1][^2] | File này nên tối ưu cho thao tác atomic như `SET NX EX`, vì ý nghĩa của nó là giữ chỗ một request đang chạy để hai request trùng key không cùng lúc chui vào business logic. [^1]                       |
| `processed_event_store.go` | Cài đặt store cho consumer idempotency bằng bảng `processedevents`, nơi mỗi consumer ghi dấu `consumername + eventid`. [^2][^1]                      | Đây là chốt an toàn ở tầng DB: nếu Kafka retry hoặc rebalance làm event tới lại, insert trùng khóa chính sẽ báo “đã xử lý”, từ đó side effect như gửi mail hay grant entitlement không bị lặp. [^1][^2] |
| `provider.go`              | Wire các implementation lại với nhau để bootstrap có thể cấp `Service`, `Store`, `ProcessedEventStore` cho HTTP app hoặc worker. [^3]                | File này tưởng nhỏ nhưng rất quan trọng vì nó giữ nguyên triết lý dependency injection đã có ở Identity: app layer dùng interface, hạ tầng mới bind implementation cụ thể. [^3]                         |

Dưới góc nhìn kỹ thuật, `redis_store.go` và `processed_event_store.go` không nên trộn vào một file vì chúng giải quyết hai lớp vấn đề khác nhau: một bên là request replay trong thời gian ngắn, bên kia là event dedup có tính bền vững hơn.[^1][^2]
Tách riêng như vậy cũng giúp hệ thống degrade đẹp hơn: Redis lỗi thì HTTP idempotency có thể bị hạn chế hoặc fallback theo policy, nhưng consumer dedup dựa trên PostgreSQL vẫn còn nguyên và không làm mất correctness của event processing.[^2][^1]

## Nguyên tắc triển khai

Ở mức triển khai thật, tôi khuyên package này phải tuân thủ 4 nguyên tắc: scope key rõ ràng theo nghiệp vụ; response replay phải trả lại đúng kết quả cũ; trạng thái “in progress” phải có TTL để tránh key treo mãi; và consumer dedup phải làm trước side effect chứ không phải sau.[^1][^2]
Một nguyên tắc rất đáng giữ nữa là Redis chỉ giữ dữ liệu idempotency ngắn hạn cho request hot-path, còn những thứ liên quan correctness dài hơi như trạng thái giao dịch, reservation hay event-processed marker quan trọng phải bám PostgreSQL, đúng với định hướng “Redis hỗ trợ, PostgreSQL quyết định”.[^2][^1]
Nếu làm đúng, `internal/platform/idempotency/` sẽ trở thành lớp “ổn định hóa hành vi hệ thống” cho toàn dự án: người dùng bấm lại nút không tạo thêm order, webhook gửi lại không double-transition payment, và consumer nhận lại event không bắn lại side effect.[^1][^2]

Tôi có thể viết tiếp ngay phần **code skeleton hoàn chỉnh cho toàn bộ `internal/platform/idempotency/`** theo đúng các file ở trên.

Hoàn toàn có thể, và thực tế đây là **cách được rất nhiều công ty công nghệ ưa chuộng nhất** (đặc biệt là khi dùng các Cloud Database như AWS RDS, nơi việc cài đặt các extension như `pg_cron` đôi khi bị hạn chế hoặc rườm rà).

Việc đưa logic dọn rác (Garbage Collection) về phía Application (Golang) mang lại lợi ích lớn nhất là: **Mọi logic nghiệp vụ và vận hành đều nằm chung trong một source code**, dễ dàng version control, deploy và monitor.

Tuy nhiên, vì hệ thống của bạn là High-performance, nếu code Go dọn rác không khéo, bạn có thể tự tay "đánh sập" Database của mình. Dưới đây là **3 quy tắc vàng** và bản code mẫu chuẩn Master để chạy dọn rác bằng Golang.

### ⚠️ 3 Quy Tắc Vàng Khi Xóa Dữ Liệu Bằng Go

**1. KHÔNG BAO GIỜ xóa tất cả trong 1 câu lệnh (Never delete all at once)**
Nếu bạn chạy `DELETE FROM processed_events WHERE processed_at < ...` khi bảng có 10 triệu dòng rác, Postgres sẽ tạo ra một Transaction khổng lồ, phình to bộ nhớ, khóa (lock) các index, và làm chậm/treo toàn bộ các Consumer đang Insert vào.
👉 **Giải pháp:** Phải xóa theo từng mẻ nhỏ (Batch Delete/Chunking), ví dụ 5000 dòng mỗi lần.

**2. Tận dụng `ctid` của PostgreSQL để tối ưu Batch Delete**
Vì bảng của bạn dùng Composite Primary Key (`consumer_name`, `event_id`), việc viết câu query xóa theo batch khá cồng kềnh. Trong Postgres có một "vũ khí bí mật" là cột ẩn `ctid` (chỉ định vị trí vật lý của record trên ổ cứng). Dùng `ctid` để xóa batch là cách nhanh và nhẹ nhất.

**3. Bài toán đa bản sao (Distributed Concurrency)**
Ứng dụng Go của bạn khi deploy lên K8s có thể chạy 5-10 Pods (replicas). Nếu bạn dùng cronjob nội bộ trong Go (như `time.Ticker`), lúc 12h đêm, cả 10 Pods sẽ cùng tranh nhau lao vào Database để xóa dữ liệu, gây ra hiện tượng _Deadlock_ hoặc lãng phí tài nguyên.
👉 **Giải pháp:** Sử dụng **PostgreSQL Advisory Lock** để đảm bảo tại 1 thời điểm, chỉ có DUY NHẤT 1 tiến trình Go được phép chạy hàm dọn rác.

---

### 💻 Code Mẫu Chuẩn: Dọn rác bằng Go Background Worker

Bạn có thể tạo một Goroutine chạy ngầm (Background Worker) khi khởi động ứng dụng Go như sau:

```go
package worker

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// StartGarbageCollector khởi chạy tiến trình dọn rác ngầm
func StartGarbageCollector(ctx context.Context, db *sql.DB) {
	// Chạy mỗi ngày 1 lần vào lúc ít người dùng (hoặc cấu hình tùy ý)
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Garbage Collector stopped.")
			return
		case <-ticker.C:
			log.Println("Starting scheduled cleanup for processed_events...")
			runCleanup(ctx, db)
		}
	}
}

func runCleanup(ctx context.Context, db *sql.DB) {
	// 1. Lấy Distributed Lock (Advisory Lock) để chống chạy đụng nhau giữa các Pods
	// Số 9999 là một ID định danh tùy chọn cho tiến trình này
	var locked bool
	err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(9999)").Scan(&locked)
	if err != nil || !locked {
		log.Println("Another instance is running the cleanup. Skipping...")
		return
	}

	// Đảm bảo mở khóa khi chạy xong
	defer func() {
		_, _ = db.ExecContext(ctx, "SELECT pg_advisory_unlock(9999)")
	}()

	// 2. Chạy Batch Deletion
	batchSize := 5000
	totalDeleted := 0

	for {
		// Dùng ctid kết hợp LIMIT để xóa từng mẻ nhỏ cực kỳ an toàn cho DB
		// Xóa các event đã xử lý cũ hơn 14 ngày
		query := `
			DELETE FROM processed_events
			WHERE ctid IN (
				SELECT ctid
				FROM processed_events
				WHERE processed_at < NOW() - INTERVAL '14 days'
				LIMIT $1
			)
		`

		res, err := db.ExecContext(ctx, query, batchSize)
		if err != nil {
			log.Printf("Error during batch delete: %v\n", err)
			break
		}

		rowsAffected, _ := res.RowsAffected()
		totalDeleted += int(rowsAffected)

		// Nếu số dòng xóa được nhỏ hơn batchSize, nghĩa là đã xóa sạch rác
		if rowsAffected < int64(batchSize) {
			break
		}

		// (Tùy chọn) Ngủ 100ms - 500ms giữa các mẻ xóa để nhường CPU/Disk IO cho các Query chính
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Cleanup finished. Total old events deleted: %d\n", totalDeleted)
}
```

### 🧠 Tóm tắt lợi ích của đoạn code này:

1. **`pg_try_advisory_lock`**: Cho dù bạn scale app Go lên 100 instances, cũng chỉ có đúng 1 instance "giành" được quyền dọn rác. Các instance khác sẽ in log `Skipping...` và bỏ qua. Bạn không cần setup Redis hay Zookeeper phức tạp.
2. **`ctid` + `LIMIT`**: Giúp Postgres xóa cực mượt, không hề khóa bảng `processed_events`. Các Consumer khác vẫn `INSERT` data bình thường mà không bị khựng lại (Lag).
3. **`time.Sleep`**: Hành động cực kỳ tinh tế trong High-performance system. Nhường tài nguyên Database cho các giao dịch quan trọng của User đang thực hiện giữa các lần xóa.
