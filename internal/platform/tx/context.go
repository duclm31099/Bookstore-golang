package tx

import (
	"context"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func WithTx(ctx context.Context, t pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, t)
}

func ExtractTx(ctx context.Context) (pgx.Tx, bool) {
	t, ok := ctx.Value(txKey{}).(pgx.Tx)
	return t, ok
}

func GetExecutor(ctx context.Context, pool *pgxpool.Pool) db.Executor {
	if t, ok := ExtractTx(ctx); ok {
		return t
	}
	return pool
}

// Mục đích trong thiết kế hệ thống: application mở transaction, nhét pgx.Tx vào context, rồi repository tự dùng executor phù hợp;
// như vậy transaction boundary vẫn ở app layer, đúng nguyên tắc kiến trúc.
// Khái niệm context.Context có thể coi là "Trái tim của lập trình đồng thời (Concurrency)" trong Golang. Thiếu nó, bạn không thể kiểm soát được hàng ngàn Goroutine đang chạy hỗn loạn.

// Dưới góc nhìn của một người thiết kế hệ thống, hãy tưởng tượng Context giống như một "Hồ sơ Nhiệm vụ" (Mission Dossier) được truyền tay từ cấp trên (API Handler) xuống các cấp dưới (Service, Repository, Database).

// Chúng ta sẽ bóc tách 4 hàm cốt lõi trong interface này một cách cặn kẽ và thực tế nhất:
// 1. Deadline() (deadline time.Time, ok bool)

//     Ý nghĩa: Trả về "Thời hạn chót" của nhiệm vụ.

//     Cách hoạt động: Hàm này cho Goroutine biết: "Nhiệm vụ này bắt buộc phải hoàn thành trước thời điểm X, nếu qua thời điểm đó thì bỏ đi, không cần làm nữa".

//         Nếu ok == true: Context này có set deadline (ví dụ: bị giới hạn 5 giây).

//         Nếu ok == false: Context này là vô thời hạn (ví dụ: context.Background()).

//     Tư duy Master: Lợi ích của nó là giúp bạn chủ động "cắt lỗ". Nếu bạn biết API gọi sang hệ thống bên thứ 3 chỉ cho phép tối đa 3 giây, bạn check Deadline(), nếu thấy sắp hết giờ rồi thì bạn chủ động dừng luôn đoạn code nặng nhọc phía sau để đỡ tốn CPU.

// 2. Done() <-chan struct{} (Quan trọng nhất)

//     Ý nghĩa: Là một "Còi báo động" (Cancellation Signal).

//     Cách hoạt động: Hàm này trả về một Channel (kênh giao tiếp) chỉ-đọc. Bạn không nhận được dữ liệu gì từ channel này cả (đó là lý do nó dùng struct{}, kiểu dữ liệu chiếm 0 byte bộ nhớ).

//         Kênh này sẽ bị đóng (closed) khi nhiệm vụ bị hủy (do timeout, do hàm cancel() được gọi, hoặc do tiến trình cha bị hủy).

//         Trong Go, khi một channel bị đóng, bất kỳ ai đang "lắng nghe" nó thông qua khối lệnh select đều sẽ nhận được tín hiệu ngay lập tức.

//     Tư duy Master: Đây là cơ chế giải phóng tài nguyên. Ví dụ bạn có một vòng lặp vô tận xử lý dữ liệu. Bạn phải chèn khối select vào để lắng nghe ctx.Done(). Khi còi báo động vang lên, hàm của bạn phải lập tức return để giải phóng Goroutine, tránh rò rỉ bộ nhớ (Goroutine Leak).

// 3. Err() error

//     Ý nghĩa: Trả lời cho câu hỏi: "Tại sao nhiệm vụ này lại bị hủy?"

//     Cách hoạt động: * Nếu Done() chưa bị đóng (nhiệm vụ vẫn đang chạy): Hàm này trả về nil.

//         Nếu Done() đã đóng, nó sẽ báo lỗi chi tiết: Hoặc là context.DeadlineExceeded (Do hết giờ), hoặc là context.Canceled (Do bị ai đó chủ động gọi hàm hủy).

// 4. Value(key any) any

//     Ý nghĩa: Chiếc "Túi đồ nghề" mang theo các dữ liệu gắn liền với Request (Request-scoped data).

//     Cách hoạt động: Cho phép bạn nhét một giá trị vào Context ở đầu nguồn (ví dụ: Middleware lấy UserID từ JWT), và lấy giá trị đó ra ở cuối nguồn (ví dụ: Repository muốn biết ai đang thực hiện query).

//     ⚠️ Lời cảnh báo của Master (như trong comment gốc): 1. KHÔNG DÙNG nó để truyền tham số tùy chọn (optional parameters) cho hàm. Nó làm mất đi tính rõ ràng (type-safety) của code. Chỉ dùng cho các dữ liệu mang tính "vô hình" chạy xuyên suốt hệ thống (như Request ID, User/Session Data, Tracing ID).
//     2. Chống đụng độ Key (Collision): Vì Key là kiểu any, nếu bạn dùng chuỗi "user" làm key, thư viện khác cũng dùng chữ "user", dữ liệu sẽ bị ghi đè! Do đó, tác giả Go đã chỉ cho bạn một Trick: Khai báo một kiểu dữ liệu ẩn (unexported type) như type key int, rồi dùng nó làm khóa. Đảm bảo 100% không bao giờ đụng độ.
