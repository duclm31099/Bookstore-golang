package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// ConsumerGroup quản lý vòng đời của Kafka consumer group.
// Hỗ trợ graceful shutdown an toàn tuyệt đối, retry, và DLQ routing.
type ConsumerGroup struct {
	cfg     Config
	log     *zap.Logger
	dlq     *DLQPublisher
	readers map[string]*kafka.Reader // topic → reader

	// Cơ chế kiểm soát Goroutine và Graceful Shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewConsumerGroup khởi tạo ConsumerGroup với context cấp cha có thể cancel được.
func NewConsumerGroup(cfg Config, log *zap.Logger, dlq *DLQPublisher) *ConsumerGroup {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConsumerGroup{
		cfg:     cfg,
		log:     log,
		dlq:     dlq,
		readers: make(map[string]*kafka.Reader),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Register đăng ký một handler cho các topic.
// Khởi chạy Goroutine an toàn để consume message.
func (c *ConsumerGroup) Register(handler Handler, middlewares ...Middleware) {
	// Giả định Handler hiện tại trả về danh sách các topic (hỗ trợ Multi-topic)
	topics := handler.Topics()

	for _, topic := range topics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        c.cfg.Brokers,
			GroupID:        c.cfg.ConsumerGroupID,
			Topic:          topic,
			MinBytes:       1,
			MaxBytes:       10e6, // 10MB max message size
			MaxWait:        c.cfg.ConsumerReadTimeout,
			StartOffset:    kafka.LastOffset,
			CommitInterval: 0, // Bắt buộc: manual commit để đảm bảo at-least-once
		})

		c.readers[topic] = reader
		wrapped := Chain(middlewares...)(handler)

		c.wg.Add(1) // Đánh dấu 1 worker bắt đầu chạy
		go c.consume(reader, topic, wrapped)
	}
}

// consume là vòng lặp chính của mỗi topic reader.
// Nó chỉ dừng lại khi nhận được tín hiệu shutdown từ context.
func (c *ConsumerGroup) consume(reader *kafka.Reader, topic string, handler Handler) {
	defer c.wg.Done() // Báo cáo hoàn thành khi thoát vòng lặp

	for {
		// 1. Kiểm tra tín hiệu shutdown
		select {
		case <-c.ctx.Done():
			c.log.Info("kafka consumer: received shutdown signal, exiting loop", zap.String("topic", topic))
			return
		default:
		}

		// 2. FetchMessage KHÔNG commit offset. Sử dụng c.ctx để hủy blocking khi cần.
		rawMsg, err := reader.FetchMessage(c.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				c.log.Info("kafka consumer: fetch interrupted by shutdown", zap.String("topic", topic))
				return
			}
			c.log.Error("kafka consumer: fetch error",
				zap.String("topic", topic),
				zap.Error(err),
			)
			time.Sleep(c.cfg.ConsumerRetryBackoff)
			continue
		}

		// 3. Xử lý Poison pill
		msg, parseErr := c.parseMessage(rawMsg)
		if parseErr != nil {
			c.log.Error("kafka consumer: parse envelope failed — sending to DLQ",
				zap.String("topic", topic),
				zap.String("offset", fmt.Sprintf("%d", rawMsg.Offset)),
				zap.Error(parseErr),
			)
			_ = c.dlq.Publish(context.Background(), rawMsg, "parse_error", parseErr)

			// Commit để nuốt trôi poison pill, không kẹt lại
			_ = reader.CommitMessages(c.ctx, rawMsg)
			continue
		}

		// 4. Inject trace context
		ctx := injectTraceContext(c.ctx, msg)

		// 5. Retry loop với finite attempts
		if err := c.handleWithRetry(ctx, handler, msg, rawMsg, reader); err != nil {
			c.log.Error("kafka consumer: retry exhausted — sending to DLQ",
				zap.String("topic", topic),
				zap.String("event_id", msg.Envelope.EventID),
				zap.String("event_type", msg.Envelope.EventType),
				zap.Error(err),
			)
			_ = c.dlq.Publish(ctx, rawMsg, "retry_exhausted", err)
		}

		// 6. Commit offset sau khi xử lý xong (success hoặc đã đẩy vào DLQ)
		if commitErr := reader.CommitMessages(c.ctx, rawMsg); commitErr != nil {
			c.log.Error("kafka consumer: commit offset failed",
				zap.String("topic", topic),
				zap.Error(commitErr),
			)
		}
	}
}

// handleWithRetry thực thi handler với finite retry.
func (c *ConsumerGroup) handleWithRetry(
	ctx context.Context,
	handler Handler,
	msg Message,
	rawMsg kafka.Message,
	reader *kafka.Reader,
) error {
	var lastErr error

	for attempt := 0; attempt <= c.cfg.ConsumerMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.cfg.ConsumerRetryBackoff * time.Duration(attempt)
			c.log.Warn("kafka consumer: retrying",
				zap.String("event_id", msg.Envelope.EventID),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(lastErr),
			)
			time.Sleep(backoff)
		}

		if err := handler.Handle(ctx, msg); err != nil {
			lastErr = err
			// Break ngay nếu lỗi không thể retry (Domain errors)
			if !shouldRetryConsumer(err) {
				break
			}
			continue
		}

		return nil
	}

	return lastErr
}

// parseMessage giải mã kafka.Message thành Message có Envelope.
func (c *ConsumerGroup) parseMessage(rawMsg kafka.Message) (Message, error) {
	var envelope Envelope
	if err := json.Unmarshal(rawMsg.Value, &envelope); err != nil {
		return Message{}, fmt.Errorf("unmarshal envelope: %w", err)
	}

	// MASTER FIX: Quét header ĐÚNG 1 LẦN để lấy toàn bộ Metadata
	meta := ExtractMetadata(rawMsg.Headers)

	return Message{
		Envelope:      envelope,
		Raw:           rawMsg,
		TraceID:       meta.TraceID,       // Lấy thẳng từ struct meta
		CorrelationID: meta.CorrelationID, // Lấy thẳng từ struct meta
	}, nil
}

// Close thực thi Graceful Shutdown.
func (c *ConsumerGroup) Close() error {
	c.log.Info("kafka consumer: initiating graceful shutdown...")

	// 1. Phóng tín hiệu hủy vào Context
	c.cancel()

	// 2. Chờ tất cả vòng lặp hoàn tất công việc đang dở dang
	c.wg.Wait()
	c.log.Info("kafka consumer: all message processing loops stopped")

	// 3. Đóng kết nối
	var errs []error
	for topic, reader := range c.readers {
		if err := reader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close reader [%s]: %w", topic, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("consumer group close errors: %v", errs)
	}

	c.log.Info("kafka consumer: shutdown completed successfully")
	return nil
}

// injectTraceContext đặt trace context vào Go context.
func injectTraceContext(ctx context.Context, msg Message) context.Context {
	// Dùng custom types để làm key tránh đụng độ bộ nhớ
	type traceKey struct{}
	type correlationKey struct{}
	ctx = context.WithValue(ctx, traceKey{}, msg.TraceID)
	ctx = context.WithValue(ctx, correlationKey{}, msg.CorrelationID)
	return ctx
}

// shouldRetryConsumer phân biệt lỗi retryable và non-retryable.
func shouldRetryConsumer(err error) bool {
	// Placeholder: tích hợp với hệ thống error chung của nền tảng
	return true
}
func ExtractCorrelationID(headers []kafka.Header) string {
	for _, h := range headers {
		if h.Key == HeaderCorrelationID {
			return string(h.Value)
		}
	}
	return ""
}
