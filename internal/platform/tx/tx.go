package tx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

/*
Transaction boundary hoàn toàn thuộc về Application layer
Vì vậy platform/tx phải cung cấp cơ chế chuẩn để mở transaction một lần ở service,
rồi repository bên dưới tự lấy pgx.Tx từ context nếu có
*/
type txKey struct{}

type Manager struct {
	pool *pgxpool.Pool
}

func NewPoolManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

// Quy tắc vàng: Context mang theo Transaction là một vật phẩm "có hạn sử dụng" gắn chặt với luồng chạy đồng bộ (Synchronous).
// Tuyệt đối không bao giờ được ném nó sang một Goroutine chạy ngầm (Asynchronous).
// Nếu cần chạy ngầm gọi DB, bạn phải dùng context.Background() để tạo một luồng riêng không có transaction.
func (m *Manager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// 1. Xử lý Nested Transaction
	if _, ok := ExtractTx(ctx); ok {
		return fn(ctx)
	}

	// 2. Khởi tạo Transaction
	txx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	// 3. Tấm khiên bảo vệ (QUAN TRỌNG NHẤT)
	// pgx cực kỳ thông minh: Nếu transaction ĐÃ được Commit thành công ở dưới,
	// hàm Rollback này sẽ tự động hiểu và bỏ qua (no-op) mà không văng lỗi.
	// Nếu hàm bị Panic hoặc return error sớm, hàm này sẽ dọn dẹp sạch sẽ.
	defer func() {
		_ = txx.Rollback(context.Background())
	}()

	ctx = inject(ctx, txx)

	// 4. Thực thi Business Logic
	if err := fn(ctx); err != nil {
		// Không cần gọi txx.Rollback thủ công ở đây nữa, defer sẽ lo việc đó!
		return err
	}

	// 5. Chốt hạ Transaction
	if err := txx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func inject(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}
