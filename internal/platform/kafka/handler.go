package kafka

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Khai báo kiểu Context Key tĩnh để chống đụng độ bộ nhớ
type contextKey string

const (
	TraceIDKey       contextKey = "trace_id"
	CorrelationIDKey contextKey = "correlation_id"
)

// Handler interface được nâng cấp để hỗ trợ nghe nhiều Topic cùng lúc.
type Handler interface {
	Handle(ctx context.Context, msg Message) error
	Topics() []string // MASTER FIX: Cho phép đăng ký []string thay vì string
}

// Message wrap envelope. Đã lược bỏ TraceID/CorrelationID vì chúng sẽ nằm trong Context.
type Message struct {
	Envelope      Envelope
	Raw           kafka.Message
	TraceID       string
	CorrelationID string
}

// HandlerFunc Adapter
type HandlerFunc struct {
	topics  []string
	handler func(ctx context.Context, msg Message) error
}

// Hỗ trợ truyền vào nhiều topic bằng variadic parameter (...string)
func NewHandlerFunc(handler func(ctx context.Context, msg Message) error, topics ...string) *HandlerFunc {
	return &HandlerFunc{topics: topics, handler: handler}
}

func (h *HandlerFunc) Handle(ctx context.Context, msg Message) error {
	return h.handler(ctx, msg)
}

func (h *HandlerFunc) Topics() []string {
	return h.topics
}

// --- Middleware Engine ---

type Middleware func(Handler) Handler

func Chain(middlewares ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// ==============================================================================
// CÁC MIDDLEWARE BẮT BUỘC PHẢI CÓ TRONG HỆ THỐNG ENTERPRISE (MASTER TIPS)
// ==============================================================================

// 1. TracingMiddleware: "Bơm" TraceID từ Kafka Header vào Go Context
func TracingMiddleware() Middleware {
	return func(next Handler) Handler {
		return NewHandlerFunc(func(ctx context.Context, msg Message) error {
			// Bóc metadata từ Header (Dùng hàm ExtractMetadata ở file trước)
			meta := ExtractMetadata(msg.Raw.Headers)

			// Nếu không có TraceID, tự sinh ra một cái mới
			if meta.TraceID == "" {
				meta.TraceID = "generated-trace-" + time.Now().String() // Dùng UUID generator
			}

			// MASTER FIX: Bơm thẳng vào Context.
			// Giờ đây mọi hàm Repo, DB bên dưới gọi ctx.Value(TraceIDKey) đều thấy!
			ctx = context.WithValue(ctx, TraceIDKey, meta.TraceID)
			ctx = context.WithValue(ctx, CorrelationIDKey, meta.CorrelationID)

			return next.Handle(ctx, msg)
		}, next.Topics()...)
	}
}

// 2. RecoveryMiddleware: "Cứu mạng" hệ thống khi logic nghiệp vụ bị Panic
func RecoveryMiddleware(logger *zap.Logger) Middleware {
	return func(next Handler) Handler {
		return NewHandlerFunc(func(ctx context.Context, msg Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					// Bắt Panic, log ra stack trace, và ép nó thành 1 error thông thường
					// Hệ thống không bị sập, message này sẽ được đưa vào DLQ (Dead Letter Queue)
					logger.Error("kafka consumer: panic recovered",
						zap.Any("panic", r),
						zap.ByteString("stack", debug.Stack()),
						zap.String("topic", msg.Raw.Topic),
					)
					err = fmt.Errorf("panic in consumer handler: %v", r)
				}
			}()

			return next.Handle(ctx, msg)
		}, next.Topics()...)
	}
}
