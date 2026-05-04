Dưới đây là sơ đồ trực quan và cách giải thích "bình dân" nhất để bạn nắm trọn vẹn kiến trúc này.

---

### 1. Dàn nhân vật trong Hệ thống Giao hàng

- **Envelope (Bao bì/Thùng carton):** Trước khi gửi hàng, mọi món đồ (Payload JSON) đều phải được đóng vào một cái thùng carton chuẩn có in sẵn form mẫu: Mã vận đơn (EventID), Mã theo dõi (TraceID), Giờ đóng gói (ProducedAt).
- **Producer (Bưu cục gửi):** Nhân viên gom hàng. Nhiệm vụ của anh này là lấy hàng từ công ty (hệ thống của bạn), dán mã vạch (Event Key), đóng vào thùng (Envelope) và quăng lên xe tải chuyển ra trung tâm.
- **Kafka Cluster (Trung tâm phân loại):** Cái kho khổng lồ chứa các băng chuyền (Topics/Partitions) chạy không ngừng nghỉ.
- **ConsumerGroup (Đội shipper):** Một đội gồm nhiều shipper cùng chia nhau đi giao hàng từ một băng chuyền. Họ phối hợp nhịp nhàng, không ai giành hàng của ai.
- **Middleware (Trạm kiểm dịch):** Trạm gác trước khi shipper đi giao. Trạm này kiểm tra mã theo dõi (Tracing), và chuẩn bị sẵn hộp cứu thương (Recovery) lỡ shipper bị tai nạn (Panic).
- **DLQ - Dead Letter Queue (Kho hàng hoàn):** Nơi chứa những gói hàng bị rách nát (Parse Error) hoặc bị khách bom (Retry Exhausted).

---

### 2. Trực quan hóa Luồng Gửi Hàng (Producer Flow)

Đây là những gì xảy ra khi bạn gọi hàm `Producer.PublishEnvelope()`.

```text
[ Business Logic ] (Ví dụ: Tạo Đơn Hàng Thành Công)
        │
        ▼
1. Tạo Thùng Hàng (Envelope):
   Gói {"order_id": 1} vào hộp, dán TraceID.
        │
        ▼
2. Bưu cục (Producer) nhận hộp:
   Dịch các thông tin ra mã vạch (Kafka Headers).
   Gắn ID Đơn Hàng làm "Chìa khóa phân loại" (Partition Key).
        │
        ▼
3. Vận chuyển: Gửi qua mạng TCP lên băng chuyền (Topic: order.events).
        │
        ▼
[ KAFKA CLUSTER (Kho Tổng) ]
```

**Cơ chế cốt lõi:** Việc dán **Partition Key** cực kỳ quan trọng. Giống như quy định: "Mọi gói hàng của Đơn hàng #1 phải được đẩy vào chung Băng chuyền số 3". Điều này đảm bảo shipper luôn giao hàng theo đúng thứ tự (Tạo đơn -> Thanh toán -> Hủy đơn), không bị lộn xộn.

---

### 3. Trực quan hóa Luồng Nhận & Xử lý (Consumer Flow) - Trái tim của hệ thống

Đây là vòng lặp vô tận chạy ngầm trong hàm `ConsumerGroup.consume()`. Đây cũng là nơi chứa nhiều "chất xám" nhất của một Master Backend.

```text
[ KAFKA CLUSTER (Kho Tổng) ]
        │
        ▼ (1) Shipper lấy gói hàng từ băng chuyền (FetchMessage)
        │
        ▼ (2) Kiểm tra vỏ hộp (Parse Envelope)
   [Hộp rách/Sai format JSON?] ──────(CÓ)─────▶ [ Bắn vào KHO HÀNG HOÀN (DLQ) ]
        │                                                     │
     (KHÔNG)                                                  │
        │                                                     ▼ (Chặn đứng lỗi)
        ▼                                           (6) Báo cáo đã dọn xong (Commit Offset)
  (3) Đi qua trạm gác (Middleware)                            │
   - Tracing (Bơm mã theo dõi vào balo)                       ▼
   - Recovery (Bảo hiểm rủi ro chống sập app)          [ BỎ QUA GÓI HÀNG NÀY ]
        │
        ▼
  (4) Giao hàng cho khách (Business Handler)
   (Ví dụ: Cập nhật Tồn kho vào Database)
        │
   [Gặp sự cố / Lỗi Database?] ──────(CÓ)─────┐
        │                                     ▼
     (KHÔNG)                       (5) Shipper chờ một lát rồi giao lại (Retry Backoff)
        │                             - Thử lần 1: chờ 1 giây
        │                             - Thử lần 2: chờ 2 giây...
        │                             - Hết 5 lần vẫn lỗi? ─▶ [ Bắn vào DLQ ]
        │                                                             │
        ▼                                                             │
  (6) Khách ký nhận thành công                                        │
      Shipper báo cáo hoàn tất (Commit Offset) ◀──────────────────────┘
        │
        ▼
[ Chờ lấy gói hàng tiếp theo... ]
```

### 4. Giải nghĩa các "Kỹ năng đặc biệt" cho người mới

Nhìn vào sơ đồ trên, bạn sẽ hiểu tại sao chúng ta phải viết code phức tạp như vậy:

1. **Tại sao không Commit ngay lúc lấy hàng (FetchMessage)?**
   Nếu shipper vừa lấy hàng ra khỏi kho đã ký báo cáo "Giao xong" (Auto Commit), lỡ đi nửa đường xe bị lủng lốp (Server sập), gói hàng đó sẽ rơi ngoài đường và mất vĩnh viễn. Chúng ta thiết kế **Manual Commit** (Bước 6) để đảm bảo: Giao tận tay khách (hoặc vứt vào DLQ) xong mới được ký xác nhận. (Cơ chế _At-least-once_).
2. **"Poison Pill" (Viên thuốc độc) là gì?**
   Đó là những gói hàng ở Bước (2). Mở ra thấy không phải JSON hợp lệ. Nếu shipper cố gắng giao đi giao lại một gói hàng mà khách không bao giờ nhận, anh ta sẽ đứng đó vĩnh viễn (Kẹt luồng). Bắn nó sang kho DLQ và Commit là cách để shipper "nuốt trôi" rắc rối và đi giao đơn tiếp theo.
3. **Graceful Shutdown (Nghỉ việc có văn hóa) là gì?**
   Là khi Công ty gọi điện: _"Hết giờ làm, các shipper về đi!"_ (Tắt Server). Lúc này, nhờ có `context` và `WaitGroup`, shipper sẽ từ chối nhận thêm hàng mới từ kho, nhưng sẽ cố gắng **giao nốt gói hàng đang cầm trên tay** rồi mới về nhà ngủ. Tránh tình trạng vứt bỏ hàng hóa giữa ngã tư.

Với hệ thống Platform vững chắc này, các bạn Dev mới vào làm (Business Developer) không cần quan tâm đến Retry, DLQ hay Graceful Shutdown nữa. Các bạn ấy chỉ cần viết đúng logic: _"Nếu có hàng tới, tôi lấy sách khỏi kho"_, mọi giông bão hạ tầng đã có Platform Kafka chống đỡ.

Dưới đây là tài liệu đầy đủ — từ khái niệm nền tảng đến kiến trúc `platform/kafka` chi tiết, viết cho người đọc từ mức zero đến hiểu được code production.

---

# 📘 Tài liệu toàn diện: Apache Kafka & platform/kafka

---

## PHẦN I — KAFKA CƠ BẢN CHO NGƯỜI MỚI

### 1. Kafka là gì?

Apache Kafka là một **distributed event streaming platform** — hiểu đơn giản là một "đường ống dữ liệu" cực kỳ nhanh và bền vững, cho phép các ứng dụng gửi và nhận sự kiện (events) với nhau theo thời gian thực. [viblo](https://viblo.asia/p/tong-quan-ve-apache-kafka-he-thong-xu-ly-du-lieu-thoi-gian-thuc-phan-tan-5OXLA5XkLGr)

Được tạo ra tại LinkedIn, Kafka hiện được sử dụng bởi hơn 80% công ty Fortune 100 và xử lý hàng triệu message mỗi giây với độ trễ chỉ ~10ms. [youtube](https://www.youtube.com/watch?v=Z5w8E7Z9fh8)

Hãy hình dung Kafka như **bưu điện trung tâm**:

- Người gửi thư (Producer) gửi thư vào hòm thư theo chủ đề
- Bưu điện (Kafka Broker) lưu và quản lý thư
- Người nhận (Consumer) đến lấy thư bất cứ khi nào họ muốn
- Quan trọng: **thư không bị xóa sau khi đọc** — người nhận khác vẫn đọc được

---

### 2. Tại sao cần Kafka?

Trước Kafka, nếu service A muốn gửi dữ liệu cho service B, C, D — A phải tự gọi HTTP đến từng service. [news.cloud365](https://news.cloud365.vn/kafka-phan-4-kien-truc-va-su-hoat-dong-cua-kafka/)

**Vấn đề:**

- Nếu B đang down → A mất dữ liệu hoặc phải retry phức tạp
- Nếu thêm service E sau này → phải sửa code của A
- A phải chờ B, C, D xử lý xong → chậm

**Kafka giải quyết bằng cách:**

- A chỉ publish event vào Kafka rồi tiếp tục công việc
- B, C, D tự đọc khi sẵn sàng
- Kafka lưu lại event → kể cả B đang down, sau khi B khởi động lại vẫn đọc được

---

### 3. Các khái niệm cốt lõi

#### 3.1 Event (Sự kiện)

Event là đơn vị dữ liệu nhỏ nhất trong Kafka, ghi lại **"điều gì đó đã xảy ra"**. Mỗi event gồm:

```
Key:   "order:123"
Value: {"order_id": 123, "status": "paid", "amount": 250000}
Headers: {"trace-id": "abc", "event-type": "order.paid"}
Timestamp: 2026-04-28T10:00:00Z
```

- **Key** → quyết định message vào partition nào (quan trọng cho ordering)
- **Value** → nội dung business chính
- **Headers** → metadata transport (không phải business data)

#### 3.2 Topic

Topic là **"kênh"** hoặc **"danh mục"** để nhóm các event cùng loại. [news.cloud365](https://news.cloud365.vn/kafka-phan-4-kien-truc-va-su-hoat-dong-cua-kafka/)

```
order.events.v1      → tất cả event liên quan đến đơn hàng
payment.events.v1    → tất cả event thanh toán
notification.commands.v1 → lệnh gửi email/notification
```

Một topic giống như một **nhật ký append-only** — dữ liệu chỉ được thêm vào cuối, không sửa không xóa (theo thời gian retention).

#### 3.3 Partition

Mỗi topic được chia thành nhiều **partition** — đây là đơn vị lưu trữ vật lý. [redpanda](https://www.redpanda.com/guides/kafka-architecture)

```
Topic: order.events.v1
├── Partition 0: [event1, event4, event7, ...]
├── Partition 1: [event2, event5, event8, ...]
└── Partition 2: [event3, event6, event9, ...]
```

**Tại sao cần partition?**

- **Parallelism**: nhiều consumer đọc song song từ các partition khác nhau
- **Scalability**: partition có thể nằm trên nhiều server khác nhau
- **Ordering**: Kafka **chỉ đảm bảo ordering trong một partition** [redpanda](https://www.redpanda.com/guides/kafka-architecture)

**Partition key quyết định ordering:**

```
Nếu key = "order:123" → luôn vào Partition 1
Nếu key = "order:456" → luôn vào Partition 0

→ Tất cả event của order 123 sẽ được đọc theo đúng thứ tự
```

#### 3.4 Broker

Broker là **Kafka server** — nơi lưu trữ và phục vụ các message. [viblo](https://viblo.asia/p/tong-quan-ve-apache-kafka-he-thong-xu-ly-du-lieu-thoi-gian-thuc-phan-tan-5OXLA5XkLGr)

```
Kafka Cluster
├── Broker 1 (lưu Partition 0 của order.events.v1)
├── Broker 2 (lưu Partition 1 của order.events.v1)
└── Broker 3 (lưu Partition 2 của order.events.v1, replica)
```

Trong môi trường production, thường có 3+ brokers để fault-tolerance. Nếu Broker 1 die → Broker khác có replica sẽ tiếp quản. [redpanda](https://www.redpanda.com/guides/kafka-architecture)

#### 3.5 Offset

Offset là **số thứ tự** của mỗi message trong một partition — giống số trang trong cuốn sách. [techdata](https://techdata.ai/apache-kafka-cho-nguoi-moi-bat-dau/)

```
Partition 0:
  Offset 0: event "user.registered" (user_id=1)
  Offset 1: event "user.email_verified" (user_id=1)
  Offset 2: event "user.registered" (user_id=2)
  Offset 3: ...
```

Consumer ghi nhớ offset đã đọc đến đâu:

- **Auto commit**: consumer tự động commit offset định kỳ → có thể miss message nếu crash
- **Manual commit**: consumer commit thủ công sau khi xử lý xong → at-least-once guarantee

#### 3.6 Producer

Producer là **ứng dụng gửi message** vào Kafka topic. [viblo](https://viblo.asia/p/tong-quan-ve-apache-kafka-he-thong-xu-ly-du-lieu-thoi-gian-thuc-phan-tan-5OXLA5XkLGr)

Producer quyết định:

- Gửi vào topic nào
- Key là gì (để routing partition)
- Cần bao nhiêu ACK từ broker (reliability vs performance)

```
RequiredAcks = 0  → fire and forget, không cần confirm (nhanh nhất, mất data)
RequiredAcks = 1  → chỉ cần leader confirm (mặc định)
RequiredAcks = -1 → tất cả ISR replicas confirm (chậm nhất, an toàn nhất)
```

Trong hệ thống bookstore, ta dùng `RequiredAcks = -1` cho outbox relay vì correctness quan trọng hơn tốc độ.

#### 3.7 Consumer & Consumer Group

Consumer là **ứng dụng đọc message** từ Kafka topic. [news.cloud365](https://news.cloud365.vn/kafka-phan-4-kien-truc-va-su-hoat-dong-cua-kafka/)

Consumer Group là **nhóm consumer cùng làm một việc**:

```
Topic: order.events.v1 (3 partitions)
Consumer Group: "notification-worker"
├── Consumer A → đọc Partition 0
├── Consumer B → đọc Partition 1
└── Consumer C → đọc Partition 2

→ Mỗi message chỉ được xử lý BỞI MỘT consumer trong group
→ Scale horizontally bằng cách thêm consumer vào group
```

Nếu có 2 consumer groups khác nhau subscribe cùng topic:

```
Consumer Group "notification-worker"  → xử lý gửi email
Consumer Group "reporting-worker"     → xử lý analytics
→ Cả hai đều nhận ĐẦY ĐỦ tất cả messages
```

---

### 4. Delivery Semantics

Đây là khái niệm quan trọng nhất khi làm việc với Kafka:

| Semantic          | Nghĩa             | Rủi ro             | Khi nào dùng                               |
| ----------------- | ----------------- | ------------------ | ------------------------------------------ |
| **At most once**  | Gửi tối đa 1 lần  | Có thể mất message | Analytics không quan trọng                 |
| **At least once** | Gửi ít nhất 1 lần | Có thể duplicate   | Hầu hết use case — handler phải idempotent |
| **Exactly once**  | Đúng 1 lần        | Phức tạp, chậm     | Tài chính, inventory critical              |

**Hệ thống bookstore dùng At-least-once** + idempotent consumer qua bảng `processedevents`. [ppl-ai-file-upload.s3.amazonaws](https://ppl-ai-file-upload.s3.amazonaws.com/web/direct-files/attachments/157776878/22e302d4-962e-45dc-b66e-44aa4a6a03ee/1.srd_specs.md)

---

### 5. Transactional Outbox Pattern

Đây là pattern quan trọng nhất mà hệ thống bookstore sử dụng. Vấn đề cần giải quyết:

**Vấn đề:** Làm sao đảm bảo khi DB commit thành công thì Kafka cũng publish được — và không publish event của transaction chưa commit?

```
❌ Cách sai:
BEGIN TRANSACTION
  INSERT order INTO db
  kafka.publish("order.created")  ← nếu crash ở đây?
COMMIT

→ DB không có order, nhưng Kafka đã nhận event
→ Hoặc: DB có order, nhưng Kafka publish fail → event mất
```

```
✅ Cách đúng — Transactional Outbox:
BEGIN TRANSACTION
  INSERT order INTO orders
  INSERT event INTO outboxevents (state="pending")
COMMIT  ← cả hai hoặc không cái nào

→ Worker riêng đọc outboxevents.state="pending"
→ Worker publish lên Kafka
→ Nếu thành công → UPDATE outboxevents.state="published"
→ Nếu Kafka down → event vẫn an toàn trong DB, retry sau
```

**Kafka down không block transaction business** — đây là core principle. [ppl-ai-file-upload.s3.amazonaws](https://ppl-ai-file-upload.s3.amazonaws.com/web/direct-files/attachments/157776878/22e302d4-962e-45dc-b66e-44aa4a6a03ee/1.srd_specs.md)

---

### 6. Consumer Idempotency

Vì ta dùng at-least-once, cùng một event có thể được consumer nhận nhiều lần (khi retry, khi rebalance...). Consumer phải **idempotent** — xử lý cùng event nhiều lần cho cùng kết quả.

**Cơ chế**: bảng `processedevents` trong PostgreSQL:

```sql
CREATE TABLE processedevents (
  consumer_name VARCHAR(100) NOT NULL,
  event_id      VARCHAR(255) NOT NULL,
  processed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  PRIMARY KEY   (consumer_name, event_id)
);
```

**Logic consumer:**

```
Nhận event với event_id = "abc-123"
→ SELECT FROM processedevents WHERE consumer_name='notification' AND event_id='abc-123'
→ Đã có → skip (idempotent)
→ Chưa có →
    BEGIN TRANSACTION
      INSERT notification_jobs
      INSERT INTO processedevents
    COMMIT
```

---

## PHẦN II — KIẾN TRÚC platform/kafka

### 7. Tổng quan kiến trúc

`platform/kafka` là tầng **infrastructure thuần túy** — nó không biết gì về business logic của bookstore. Nó chỉ cung cấp:

- Cách publish message lên Kafka đúng format
- Cách consume message với retry, DLQ, graceful shutdown
- Contract (interface) để module business plug vào

```
┌─────────────────────────────────────────────────────────┐
│                    Business Modules                      │
│  auth │ order │ payment │ notification │ inventory...    │
└──────────────────┬──────────────────┬────────────────────┘
                   │ implements       │ calls
                   ▼                  ▼
┌─────────────────────────────────────────────────────────┐
│                   platform/kafka                         │
│                                                          │
│  config.go    envelope.go    topic.go    headers.go      │
│  producer.go  consumer.go   handler.go  dlq.go           │
│                          provider.go                     │
└──────────────────────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────┐
│              Apache Kafka Cluster                        │
│  Broker 1 │ Broker 2 │ Broker 3                         │
└─────────────────────────────────────────────────────────┘
```

---

### 8. Vai trò và chức năng từng file

#### 📄 `config.go` — Cấu hình kết nối

**Vai trò:** Tập trung toàn bộ config Kafka vào một struct duy nhất.

```go
type Config struct {
    Brokers              []string      // Địa chỉ Kafka brokers
    ProducerRequiredAcks int           // -1 = tất cả replica confirm
    ConsumerGroupID      string        // ID nhóm consumer
    ConsumerMaxRetries   int           // Số lần retry tối đa
    DLQTopic             string        // Topic nhận message lỗi
    TLSEnabled           bool          // Bật TLS cho production
    ...
}
```

**Tại sao cần file này?**

- Không hardcode bất kỳ giá trị nào trong business code
- Thay đổi behavior (retry, timeout, broker address) chỉ cần sửa config file, không đụng code
- Dễ inject mock config trong test

---

#### 📄 `topic.go` — Registry tên topic và keying strategy

**Vai trò:** Đăng ký toàn bộ 11 topics theo spec như một nguồn duy nhất của sự thật.

```go
const (
    TopicOrderEvents      = "order.events.v1"
    TopicPaymentEvents    = "payment.events.v1"
    TopicMembershipEvents = "membership.events.v1"
    TopicDLQGeneral       = "dlq.general.v1"
    // ... 7 topics khác
)

func EventKey(aggregateType string, aggregateID int64) string {
    return aggregateType + ":" + strconv.FormatInt(aggregateID, 10)
    // Ví dụ: "order:123", "user:456", "payment:789"
}
```

**Tại sao cần file này?**

- Tránh magic string `"order.events.v1"` rải rác trong code
- Khi rename topic → sửa 1 chỗ duy nhất
- `EventKey` chuẩn hóa partition key → đảm bảo ordering per aggregate

**Keying strategy quan trọng:**

```
TopicOrderEvents    → key = "order:<orderID>"
TopicMembershipEvents → key = "user:<userID>"
TopicShipmentEvents → key = "shipment:<shipmentID>"
```

→ Mọi event của cùng `order:123` sẽ vào **cùng partition** → consumer xử lý **đúng thứ tự**

---

#### 📄 `envelope.go` — Bao bì chuẩn cho mọi message

**Vai trò:** Định nghĩa 14-field envelope theo SRD spec, cùng factory function để tạo.

```
┌─────────────────────────────────────┐
│           Envelope                  │
├─────────────────────────────────────┤
│ event_id       (UUID, globally unique)     ← Identity
│ event_type     ("order.paid")              ← Routing
│ aggregate_type ("order")                   ← Domain context
│ aggregate_id   (123)                       ← Which entity
│ occurred_at    (when in domain)            ← Business time
│ produced_at    (when published)            ← Transport time
│ trace_id       (distributed trace)         ← Observability
│ correlation_id (same business flow)        ← Grouping
│ causation_id   (which event caused this)   ← Lineage
│ actor_type     ("user"/"admin"/"system")   ← Who
│ actor_id       (456)                       ← Who exactly
│ schema_version (1)                         ← Evolution
│ idempotency_key (for consumer dedup)       ← Safety
│ payload        (JSON business data)        ← Content
└─────────────────────────────────────┘
```

**Tại sao cần đủ 14 fields?**

| Field                          | Mục đích thực tế                                                 |
| ------------------------------ | ---------------------------------------------------------------- |
| `event_id`                     | Consumer dùng để dedup trong `processedevents`                   |
| `trace_id`                     | Tìm toàn bộ log của 1 request xuyên HTTP → DB → Kafka            |
| `correlation_id`               | Nhóm tất cả event của 1 business flow (checkout → pay → ship)    |
| `causation_id`                 | Biết event nào gây ra event này để debug chain                   |
| `occurred_at` vs `produced_at` | Audit log dùng `occurred_at`; monitoring lag dùng `produced_at`  |
| `schema_version`               | Khi thay đổi payload format, consumer cũ không bị break          |
| `actor_type/id`                | Biết ai trigger event: user tự làm hay admin override hay system |

**Builder pattern để không bỏ sót field:**

```go
// Thay vì viết struct thủ công (dễ quên field)
envelope := kafka.NewEnvelope(
    "order.paid",
    "order", orderID,
    occurredAt, payloadBytes,
    kafka.WithTraceID(traceID),
    kafka.WithCorrelationID(correlationID),
    kafka.WithActor("user", userID),
)
```

---

#### 📄 `headers.go` — Kafka message headers

**Vai trò:** Đặt và đọc metadata transport-level từ Kafka message headers.

```go
// Headers = metadata ngoài bì thư
// Payload = nội dung bên trong
const (
    HeaderTraceID       = "x-trace-id"
    HeaderCorrelationID = "x-correlation-id"
    HeaderEventType     = "x-event-type"
    HeaderSchemaVersion = "x-schema-version"
)
```

**Tại sao tách headers ra khỏi payload?**

```
Không có headers:
  Consumer muốn check event type → phải unmarshal toàn bộ JSON payload
  Tốn CPU cho mọi message, kể cả message consumer không xử lý

Có headers:
  Consumer middleware đọc header "x-event-type" → O(1)
  Nếu event type không phù hợp → skip ngay, không parse payload
  Logging middleware đọc "x-trace-id" → không cần đụng payload
```

---

#### 📄 `producer.go` — Gửi message lên Kafka

**Vai trò:** Sync producer dùng cho outbox relay worker.

```
Outbox Relay Worker
       │
       ▼
producer.PublishEnvelope(ctx, topic, envelope)
       │
       ├─► Serialize envelope → JSON
       ├─► Tạo kafka.Message với key, headers, value
       ├─► writer.WriteMessages(ctx, msg)
       │         │
       │         ├─► Kafka confirms all ISR replicas ✓
       │         │   └─► return nil
       │         │
       │         └─► Kafka timeout / broker down ✗
       │             └─► return error
       │
       ├─► nil   → outboxRepo.MarkPublished(id) ✓
       └─► error → outboxRepo.IncrementRetry(id), retry later
```

**Tại sao SYNC producer?**

```
ASYNC producer:
  Fire → return immediately
  Kafka xử lý background
  → Không biết thành công hay thất bại
  → Không thể cập nhật outboxevents.state
  → At risk of losing messages

SYNC producer (ta dùng):
  Fire → wait for confirm
  → Biết chắc published → UPDATE state = "published"
  → Fail → giữ state = "pending" → relay retry sau
```

**`PublishBatch` cho hiệu năng:**

```go
// Thay vì gọi 100 lần PublishEnvelope (100 round-trips)
producer.PublishBatch(ctx, []TopicMessage{
    {Topic: "order.events.v1", Envelope: e1},
    {Topic: "payment.events.v1", Envelope: e2},
    // ... 98 messages khác
})
// → 1 round-trip duy nhất → giảm latency 100x
```

---

#### 📄 `handler.go` — Interface contract cho consumer

**Vai trò:** Định nghĩa contract để platform và module business không biết nhau.

```go
// Platform định nghĩa interface
type Handler interface {
    Handle(ctx context.Context, msg Message) error
    Topic() string
}

// Module notification implement interface
type NotificationHandler struct { ... }
func (h *NotificationHandler) Handle(ctx context.Context, msg Message) error {
    // business logic: parse event, create email job
}
func (h *NotificationHandler) Topic() string {
    return kafka.TopicNotificationCmds
}

// Module inventory implement interface
type InventoryHandler struct { ... }
func (h *InventoryHandler) Handle(ctx context.Context, msg Message) error {
    // business logic: release reservation
}
```

**Tại sao cần interface?**

- Platform không import code của bất kỳ module nào
- Module không import kafka-go library trực tiếp
- Dễ swap implementation: test dùng `HandlerFunc`, production dùng struct thật

**Middleware chain — giống HTTP middleware:**

```
Mỗi message đi qua:
  RecoveryMiddleware
      └─► LoggingMiddleware
              └─► TracingMiddleware
                      └─► BusinessHandler

Nếu handler panic → RecoveryMiddleware bắt, convert thành error → DLQ
```

---

#### 📄 `consumer.go` — Vòng đời consumer

**Vai trò:** Quản lý toàn bộ vòng đời consume: fetch → parse → retry → DLQ → commit.

**Luồng xử lý một message:**

```
reader.FetchMessage()
        │
        ▼
   Parse Envelope
        │
        ├─► Parse fail (poison pill)
        │       └─► DLQ ngay
        │       └─► CommitMessages (không loop)
        │
        └─► Parse OK
                │
                ▼
        Inject trace context vào ctx
                │
                ▼
        handleWithRetry(handler, msg, maxRetries=5)
                │
                ├─► Attempt 1: handler.Handle(ctx, msg)
                │       ├─► nil   → return nil ✓
                │       └─► error → retry?
                │                   ├─► non-retryable → break
                │                   └─► retryable → sleep, attempt 2...
                │
                ├─► Attempt 2, 3, 4, 5... (exponential backoff)
                │
                ├─► Thành công → return nil ✓
                └─► Exhausted → DLQ
                │
                ▼
        reader.CommitMessages()  ← Chỉ commit SAU KHI xong
```

**Tại sao manual commit QUAN TRỌNG:**

```
Auto commit (sai):
  FetchMessage offset=100
  Auto commit offset=101
  handler.Handle() → panic/crash
  → Consumer restart → đọc từ offset 101
  → MESSAGE 100 BỊ MẤT VĨNH VIỄN

Manual commit (đúng):
  FetchMessage offset=100
  handler.Handle() → success/DLQ
  CommitMessages offset=100
  → Consumer restart → đọc lại từ 100 (nếu chưa commit)
  → At-least-once: handler phải idempotent
```

**Retry với exponential backoff:**

```
Attempt 1: ngay lập tức
Attempt 2: sleep 1s
Attempt 3: sleep 2s
Attempt 4: sleep 3s
Attempt 5: sleep 4s
Exhausted → DLQ
```

→ Tránh hammer external service đang bị lỗi tạm thời

**Phân biệt retryable vs non-retryable:**

```
Retryable (nên retry):
  - DB timeout
  - Redis connection error
  - External API 503

Non-retryable (DLQ ngay):
  - Invalid payload format
  - Business validation error ("user not found")
  - Authorization error
```

---

#### 📄 `dlq.go` — Dead Letter Queue

**Vai trò:** Nơi "an toàn" cho message không xử lý được sau tất cả retry.

```
DLQEnvelope = {
  original_topic:    "order.events.v1"    ← Topic gốc
  original_key:      "order:123"          ← Key gốc (cho replay ordering)
  original_payload:  {...}                ← Payload gốc (nguyên vẹn)
  original_headers:  [...]                ← Headers gốc
  failure_reason:    "retry_exhausted"    ← Tại sao vào DLQ
  failure_error:     "db: connection..."  ← Error cụ thể
  failed_at:         "2026-04-28T..."     ← Khi nào fail
  consumer_group:    "notification-worker"
}
```

**DLQ không phải "thùng rác":**

- Ops team monitor DLQ growth rate → alert khi DLQ tăng bất thường
- Khi fix bug → replay từ DLQ: `original_payload` vẫn nguyên vẹn
- `original_key` được giữ → replay vào đúng partition → ordering đảm bảo
- `failure_reason` phân loại: `parse_error` vs `retry_exhausted` → debug khác nhau

---

#### 📄 `provider.go` — Dependency Injection (Wire)

**Vai trò:** Khai báo dependencies cho Google Wire tự động inject.

```go
var ProviderSet = wire.NewSet(
    ProvideProducer,         // Config → Producer
    ProvideDLQPublisher,     // Producer + Config → DLQPublisher
    ProvideConsumerGroup,    // Config + DLQPublisher → ConsumerGroup
)
```

**Dependency graph:**

```
Config
  └─► Producer
        └─► DLQPublisher
              └─► ConsumerGroup
```

Wire tự động resolve graph này → không cần viết constructor manually trong `main.go`

---

### 9. Luồng chạy đầy đủ end-to-end

Lấy ví dụ user đặt hàng thành công, cần gửi email xác nhận:

```
┌─────────────────────────────────────────────────────────────┐
│ PHASE 1: Business Transaction (PostgreSQL)                   │
│                                                              │
│  POST /api/v1/orders                                         │
│       │                                                      │
│       ▼                                                      │
│  OrderService.CreateOrder(ctx, cmd)                         │
│       │                                                      │
│       ▼                                                      │
│  tx.TxManager.WithinTransaction(ctx, func(ctx) error {      │
│      orderRepo.Insert(order)                                 │
│      inventoryRepo.Reserve(items)                            │
│      outboxRepo.Insert({                                     │
│          topic:   "notification.commands.v1",                │
│          key:     "user:456",                                │
│          type:    "order.created",                           │
│          payload: {orderID: 123, email: "user@...", ...},    │
│          state:   "pending"                                  │
│      })                                                      │
│  })                                                          │
│  ← COMMIT PostgreSQL ✓                                       │
│  ← Return 201 Created to client ✓                           │
│                                                              │
│  [Kafka trạng thái: chưa biết gì]                           │
└─────────────────────────────────────────────────────────────┘
                    │
                    │ (vài giây sau)
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ PHASE 2: Outbox Relay Worker (Background)                    │
│                                                              │
│  OutboxRelayWorker.Run() [chạy mỗi 2 giây]                 │
│       │                                                      │
│       ▼                                                      │
│  outboxRepo.ListPending(limit=100)                           │
│  → [{id:1, topic:"notification.commands.v1", ...}]          │
│       │                                                      │
│       ▼                                                      │
│  Tạo Envelope{                                               │
│      event_id:       "uuid-abc",                             │
│      event_type:     "order.created",                        │
│      aggregate_type: "order",                                │
│      aggregate_id:   123,                                    │
│      trace_id:       "trace-xyz",                            │
│      payload:        {orderID:123, email:"user@..."},        │
│  }                                                           │
│       │                                                      │
│       ▼                                                      │
│  producer.PublishBatch(ctx, [{                               │
│      Topic:    "notification.commands.v1",                   │
│      Envelope: envelope,                                     │
│  }])                                                         │
│       │                                                      │
│       ├─► Kafka OK → outboxRepo.MarkPublished( [viblo](https://viblo.asia/p/tong-quan-ve-apache-kafka-he-thong-xu-ly-du-lieu-thoi-gian-thuc-phan-tan-5OXLA5XkLGr)) ✓        │
│       └─► Kafka DOWN → giữ state="pending", retry sau       │
└─────────────────────────────────────────────────────────────┘
                    │
                    │ (milliseconds sau)
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ PHASE 3: Notification Consumer (Background)                  │
│                                                              │
│  ConsumerGroup.consume(reader, "notification.commands.v1")   │
│       │                                                      │
│       ▼                                                      │
│  reader.FetchMessage()                                       │
│  → kafka.Message{key:"user:456", value:envelope_json}       │
│       │                                                      │
│       ▼                                                      │
│  parseMessage() → Message{Envelope, TraceID, CorrelationID} │
│       │                                                      │
│       ▼                                                      │
│  injectTraceContext(ctx, msg)                                │
│       │                                                      │
│       ▼                                                      │
│  Chain(RecoveryMW, LoggingMW)(NotificationHandler)          │
│       │                                                      │
│       ▼                                                      │
│  NotificationHandler.Handle(ctx, msg):                       │
│      │                                                       │
│      ├─► processedevents.IsProcessed(                       │
│      │       "notification-worker", "uuid-abc")              │
│      │   ├─► TRUE  → return nil (skip, idempotent) ✓        │
│      │   └─► FALSE →                                         │
│      │           BEGIN TRANSACTION                           │
│      │             INSERT notification_jobs {                │
│      │               channel: "email",                       │
│      │               recipient: "user@...",                  │
│      │               template: "order_confirmation",         │
│      │             }                                         │
│      │             INSERT processedevents {                  │
│      │               consumer_name: "notification-worker",   │
│      │               event_id: "uuid-abc"                    │
│      │             }                                         │
│      │           COMMIT ✓                                    │
│      │           return nil ✓                                │
│      │                                                       │
│      └─► return nil                                          │
│       │                                                      │
│       ▼                                                      │
│  reader.CommitMessages() ← offset committed ✓               │
└─────────────────────────────────────────────────────────────┘
                    │
                    │ (vài giây sau)
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ PHASE 4: Email Sender Worker (Background)                    │
│                                                              │
│  EmailSenderWorker.Run()                                     │
│  → SELECT * FROM notification_jobs WHERE state='pending'    │
│  → SendEmail(to: "user@...", template: "order_confirmation") │
│  → UPDATE notification_jobs SET state='sent' ✓              │
└─────────────────────────────────────────────────────────────┘
```

---

### 10. Graceful Shutdown

Khi nhận `SIGTERM` (deploy mới, restart container):

```
OS → SIGTERM
  │
  ▼
main.go: cancel(ctx)
  │
  ├─► HTTP Server: graceful shutdown (drain in-flight requests)
  │
  ├─► ConsumerGroup.Close()
  │     └─► Mỗi reader.Close()
  │           └─► consumer loop nhận context.Canceled
  │               └─► "kafka consumer: shutting down"
  │               └─► commit offset hiện tại
  │               └─► return (goroutine kết thúc sạch)
  │
  └─► Producer.Close()
        └─► Flush pending writes
        └─► Close connection
```

**Quan trọng:** Không forcefully kill — phải drain message đang xử lý và commit offset trước khi thoát.

---

### 11. Kafka Graceful Degrade

Spec yêu cầu: **Kafka down không được làm corrupt primary transactional state**. [ppl-ai-file-upload.s3.amazonaws](https://ppl-ai-file-upload.s3.amazonaws.com/web/direct-files/attachments/157776878/22e302d4-962e-45dc-b66e-44aa4a6a03ee/1.srd_specs.md)

```
Bình thường:
  Commit DB → Relay Worker → Kafka publish (vài giây sau)

Kafka down:
  Commit DB ✓ (vẫn thành công, user nhận 201)
  Outbox event state = "pending" (lưu trong PostgreSQL)
  Relay Worker retry liên tục (backoff)
  Kafka up lại → relay flush tất cả pending events
  → Chỉ trễ hơn vài phút, KHÔNG mất data

Redis down (tương tự):
  Cache miss → fallback to DB (chậm hơn, không sai)
  Idempotency check fail → reject request (safe)
```

---

### 12. Sơ đồ quan hệ các file

```
                    ┌─────────────┐
                    │  config.go  │
                    │  (Config)   │
                    └──────┬──────┘
                           │ injected into
              ┌────────────┼────────────────┐
              │            │                │
              ▼            ▼                ▼
      ┌──────────────┐ ┌──────────┐ ┌──────────────┐
      │ producer.go  │ │  dlq.go  │ │ consumer.go  │
      │  (Producer)  │ │  (DLQ    │ │ (Consumer    │
      │              │ │Publisher)│ │   Group)     │
      └──────┬───────┘ └────┬─────┘ └──────┬───────┘
             │              │              │
             │  uses        │ uses         │ uses
             ▼              ▼              ▼
      ┌──────────────────────────────────────────┐
      │              envelope.go                 │
      │          topic.go + headers.go           │
      └──────────────────────────────────────────┘
                           │
                           │ typed by
                           ▼
                   ┌───────────────┐
                   │  handler.go   │
                   │  (Handler     │
                   │   interface)  │
                   └───────────────┘
                           │
                           │ implemented by
                           ▼
         ┌─────────────────────────────────┐
         │  modules/notification/consumer  │
         │  modules/inventory/consumer     │
         │  modules/entitlement/consumer   │
         │  ...                            │
         └─────────────────────────────────┘
```

---

### 13. Quick reference: What goes where

| Bạn muốn...                       | File cần sửa                             |
| --------------------------------- | ---------------------------------------- |
| Thêm topic mới                    | `topic.go`                               |
| Thay đổi Kafka broker/credentials | `config.go`                              |
| Thay đổi format envelope          | `envelope.go`                            |
| Thêm header mới cho tracing       | `headers.go`                             |
| Thay đổi retry policy consumer    | `config.go` + `consumer.go`              |
| Thêm middleware mới (metrics...)  | `handler.go` (thêm Middleware func)      |
| Thay đổi DLQ topic                | `config.go` → `DLQTopic` field           |
| Implement consumer mới            | Tạo struct implement `Handler` interface |
| Thay đổi batch size relay         | `producer.go` → `PublishBatch` caller    |

---

### 14. Checklist trước khi production

Dựa trên spec và các nguyên tắc đã xây dựng: [ppl-ai-file-upload.s3.amazonaws](https://ppl-ai-file-upload.s3.amazonaws.com/web/direct-files/attachments/157776878/17f0a5d3-99b9-4c7f-8d7f-56331c85b423/2.erd_specs.md)

- [ ] Tất cả topics đã được tạo sẵn qua Kafka CLI/Terraform — `AllowAutoTopicCreation: false`
- [ ] `RequiredAcks = -1` cho outbox relay producer
- [ ] `CommitInterval = 0` (manual commit) cho tất cả consumer
- [ ] Bảng `processedevents` đã được migrate
- [ ] Mỗi consumer implement idempotency check trước khi write
- [ ] DLQ topic tạo với retention dài (7-30 ngày)
- [ ] Alert khi DLQ lag tăng bất thường
- [ ] SASL + TLS enabled trên production brokers
- [ ] Graceful shutdown signal handler đã đăng ký
- [ ] Outbox relay worker retry backoff đã config
- [ ] `schemaVersion` tăng khi thay đổi payload format
