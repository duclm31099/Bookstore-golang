package kafka

import (
	"strconv"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

const (
	HeaderTraceID       = "x-trace-id"
	HeaderCorrelationID = "x-correlation-id"
	HeaderCausationID   = "x-causation-id"
	HeaderEventType     = "x-event-type"
	HeaderSchemaVersion = "x-schema-version"
	HeaderProducedAt    = "x-produced-at"
)

// BuildHeaders tạo mảng Header thông minh.
// Chỉ đính kèm những metadata THỰC SỰ CÓ DỮ LIỆU để tiết kiệm băng thông.
func BuildHeaders(e Envelope) []kafka.Header {
	// MASTER TIP: Pre-allocate capacity = 6 để tránh slice phải tự động phình to (grow)
	// gây ra các lệnh copy memory ngầm đắt đỏ dưới background.
	headers := make([]kafka.Header, 0, 6)

	// Các field bắt buộc
	headers = append(headers, kafka.Header{Key: HeaderEventType, Value: []byte(e.EventType)})
	headers = append(headers, kafka.Header{Key: HeaderSchemaVersion, Value: []byte(strconv.Itoa(e.SchemaVersion))})
	headers = append(headers, kafka.Header{Key: HeaderProducedAt, Value: []byte(e.ProducedAt.Format(time.RFC3339Nano))})

	// Các field tùy chọn (Observability) - Bỏ qua nếu rỗng
	if e.TraceID != "" {
		headers = append(headers, kafka.Header{Key: HeaderTraceID, Value: []byte(e.TraceID)})
	}
	if e.CorrelationID != "" {
		headers = append(headers, kafka.Header{Key: HeaderCorrelationID, Value: []byte(e.CorrelationID)})
	}
	if e.CausationID != "" {
		headers = append(headers, kafka.Header{Key: HeaderCausationID, Value: []byte(e.CausationID)})
	}

	return headers
}

// ExtractMetadata là bản nâng cấp của extractHeader.
// Nó quét mảng headers ĐÚNG 1 LẦN và bốc toàn bộ dữ liệu cần thiết ra một struct.
// Phù hợp cho Middleware Consumer muốn lấy tất cả Trace Context cùng lúc.
type MessageMetadata struct {
	TraceID       string
	CorrelationID string
	CausationID   string
	EventType     string
}

func ExtractMetadata(headers []kafka.Header) MessageMetadata {
	var meta MessageMetadata
	for _, h := range headers {
		switch h.Key {
		case HeaderTraceID:
			meta.TraceID = string(h.Value)
		case HeaderCorrelationID:
			meta.CorrelationID = string(h.Value)
		case HeaderCausationID:
			meta.CausationID = string(h.Value)
		case HeaderEventType:
			meta.EventType = string(h.Value)
		}
	}
	return meta
}

// Giữ lại hàm cũ cho các trường hợp chỉ cần bóc lẻ 1 ID nhanh chóng
func ExtractTraceID(headers []kafka.Header) string {
	for _, h := range headers {
		if h.Key == HeaderTraceID {
			return string(h.Value)
		}
	}
	return ""
}
