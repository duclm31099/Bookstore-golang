package kafka

import "time"

// Config chứa toàn bộ cấu hình kết nối và tuning Kafka.
type Config struct {
	// Base
	Brokers  []string `mapstructure:"brokers"`
	ClientID string   `mapstructure:"client_id"` // Giúp dễ dàng trace log trên Kafka Cluster

	// Producer
	ProducerMaxRetries   int           `mapstructure:"producer_max_retries"`
	ProducerRetryBackoff time.Duration `mapstructure:"producer_retry_backoff"`
	ProducerWriteTimeout time.Duration `mapstructure:"producer_write_timeout"`
	ProducerRequiredAcks int           `mapstructure:"producer_required_acks"` // -1 = all ISR

	// Tối ưu Producer: Chống trùng lặp và Tăng tốc
	ProducerIdempotent bool          `mapstructure:"producer_idempotent"`
	ProducerBatchSize  int           `mapstructure:"producer_batch_size"` // Kích thước batch tối đa (bytes)
	ProducerLingerMs   time.Duration `mapstructure:"producer_linger_ms"`  // Thời gian chờ gom batch tối đa

	// Consumer
	ConsumerGroupID      string        `mapstructure:"consumer_group_id"`
	ConsumerReadTimeout  time.Duration `mapstructure:"consumer_read_timeout"`
	ConsumerMaxRetries   int           `mapstructure:"consumer_max_retries"`
	ConsumerRetryBackoff time.Duration `mapstructure:"consumer_retry_backoff"`

	// Tối ưu Consumer: Kiểm soát an toàn dữ liệu
	ConsumerAutoOffsetReset  string `mapstructure:"consumer_auto_offset_reset"`  // "earliest" hoặc "latest"
	ConsumerEnableAutoCommit bool   `mapstructure:"consumer_enable_auto_commit"` // Bắt buộc false trong hệ thống quan trọng

	// DLQ
	DLQTopic string `mapstructure:"dlq_topic"`

	// SASL/TLS (production)
	SASLUsername string `mapstructure:"sasl_username"`
	SASLPassword string `mapstructure:"sasl_password"`
	TLSEnabled   bool   `mapstructure:"tls_enabled"`
}

func DefaultConfig() Config {
	return Config{
		Brokers:  []string{"localhost:9092"},
		ClientID: "bookstore-backend",

		// Producer Defaults
		ProducerMaxRetries:   3,
		ProducerRetryBackoff: 500 * time.Millisecond,
		ProducerWriteTimeout: 10 * time.Second,
		ProducerRequiredAcks: -1,
		ProducerIdempotent:   true,                 // BẬT tính năng Exactly-Once semantics nội bộ Kafka
		ProducerBatchSize:    1048576,              // 1MB mỗi batch
		ProducerLingerMs:     5 * time.Millisecond, // Chờ 5ms để gom các message lại trước khi gửi

		// Consumer Defaults
		ConsumerGroupID:          "bookstore-kafka",
		ConsumerReadTimeout:      10 * time.Second,
		ConsumerMaxRetries:       5,
		ConsumerRetryBackoff:     1 * time.Second,
		ConsumerAutoOffsetReset:  "earliest", // Chạy lần đầu sẽ đọc lại dữ liệu cũ tránh mất mát
		ConsumerEnableAutoCommit: false,      // TẮT để App tự chủ động commit sau khi lưu DB/Redis thành công

		DLQTopic: "",
	}
}

/* ProducerRequiredAcks: -1
Đây là thiết lập "chân ái" của các hệ thống để đảm bảo không bao giờ mất dữ liệu
(tin nhắn chỉ được coi là thành công khi toàn bộ các Replica đã ghi nhận).



ProducerIdempotent (Cực kỳ quan trọng): Khi bật lên true, Kafka Producer sẽ gán một Sequence ID cho mỗi tin nhắn.
Dù mạng chập chờn và bị retry gửi 10 lần, Kafka Server vẫn nhận ra đó là cùng một tin nhắn nhờ Sequence ID và chỉ lưu 1 lần duy nhất.
 Triệt tiêu hoàn toàn lỗi Duplicate.



ProducerLingerMs & ProducerBatchSize: Thay vì gửi 100 request mạng cho 100 sự kiện tạo đơn hàng,
Producer sẽ nán lại chờ 5ms (Linger) hoặc đến khi dữ liệu gom đủ 1MB (Batch Size) rồi mới gửi đi 1 lần.
Đây là bí quyết giúp hệ thống bắn hàng chục ngàn tin nhắn mỗi giây (RPS) mà CPU và Network không bị thắt cổ chai.



ConsumerEnableAutoCommit = false: Cơ chế bảo vệ At-least-once Delivery.
Bạn nhận một OrderCreatedEvent, code của bạn đang gọi DB để xử lý trừ tồn kho thì Database bị timeout, ứng dụng văng lỗi.
Nếu để Auto Commit là true, Kafka đã đánh dấu tin nhắn đó là "Đã xử lý xong".
Bạn vĩnh viễn mất event đó. Bằng cách để false, bạn chỉ gọi lệnh Commit() khi và chỉ khi mọi logic DB đã chạy 100% thành công.
*/
