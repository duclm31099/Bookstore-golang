package kafka

// 1. TYPE SAFETY: Định nghĩa kiểu dữ liệu riêng thay vì dùng string tự do
type Topic string
type AggregateType string

// 2. Định nghĩa các Topic dựa trên kiểu Topic vừa tạo
const (
	TopicOrderEvents       Topic = "order.events.v1"
	TopicPaymentEvents     Topic = "payment.events.v1"
	TopicMembershipEvents  Topic = "membership.events.v1"
	TopicEntitlementEvents Topic = "entitlement.events.v1"
	TopicInventoryEvents   Topic = "inventory.events.v1"
	TopicShipmentEvents    Topic = "shipment.events.v1"
	TopicNotificationCmds  Topic = "notification.commands.v1"
	TopicSchedulerCmds     Topic = "scheduler.commands.v1"
	TopicAuditEvents       Topic = "audit.events.v1"
	TopicReportingEvents   Topic = "reporting.events.v1"
	TopicDLQGeneral        Topic = "dlq.general.v1"
)

// 3. Định nghĩa chặt chẽ các AggregateType được phép sử dụng
const (
	AggOrder      AggregateType = "order"
	AggPayment    AggregateType = "payment"
	AggUser       AggregateType = "user"
	AggMembership AggregateType = "membership"
	AggInventory  AggregateType = "inventory"
	AggShipment   AggregateType = "shipment"
)

// EventKey nhận vào AggregateType thay vì string tự do.
// Bất kỳ ai truyền chuỗi bậy bạ vào hàm này sẽ bị Go Compiler báo lỗi đỏ lòm ngay lập tức!
func EventKey(aggType AggregateType, aggID string) string {
	// Trình biên dịch Go (từ bản 1.15+) tối ưu hóa toán tử "+" cho chuỗi rất tốt
	// Nó sẽ tính toán độ dài trước và chỉ cấp phát bộ nhớ (allocate) đúng 1 lần.
	return string(aggType) + ":" + aggID
}

var TopicKeyStrategy = map[Topic]string{
	TopicOrderEvents:       "order_id",
	TopicPaymentEvents:     "order_id or payment_id",
	TopicMembershipEvents:  "user_id",
	TopicEntitlementEvents: "user_id",
	TopicInventoryEvents:   "book_id",
	TopicShipmentEvents:    "shipment_id",
	TopicNotificationCmds:  "recipient or business_key",
	TopicSchedulerCmds:     "command_key",
	TopicAuditEvents:       "resource_id",
	TopicReportingEvents:   "business_key",
}
