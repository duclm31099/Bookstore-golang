package adapters

import (
	"context" // Quản lý context cho các lệnh gọi DB/Redis
	"time"    // Xử lý thời gian hết hạn của token

	// Import interface đã định nghĩa

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	"github.com/redis/go-redis/v9" // Thư viện Redis client cho Go
)

// redisBlacklistAdapter là struct thực thi BlacklistPort sử dụng Redis
type redisBlacklistAdapter struct {
	client *redis.Client // Giữ kết nối (connection pool) tới Redis server
}

// NewRedisBlacklistAdapter là hàm khởi tạo (constructor) cho adapter
// Nó nhận vào redis.Client và trả về interface BlacklistPort để che giấu implementation
func NewRedisBlacklistAdapter(client *redis.Client) ports.BlacklistPort {
	return &redisBlacklistAdapter{client: client} // Trả về con trỏ cấu trúc chứa kết nối Redis
}

// AddToBlacklist đẩy jti (JWT ID) vào Redis để cấm sử dụng
func (r *redisBlacklistAdapter) AddToBlacklist(ctx context.Context, jti string, expiration time.Duration) error {
	blacklistKey := "blacklist:access_token:" + jti // Tạo key có prefix rõ ràng để dễ quản lý, tránh trùng lặp

	// Lệnh Set lưu key vào Redis. Giá trị là "revoked" (bị thu hồi). Quan trọng nhất là expiration để key tự xóa.
	err := r.client.Set(ctx, blacklistKey, "revoked", expiration).Err()

	return err // Trả về lỗi nếu Redis gặp sự cố (timeout, mất mạng...)
}

// IsTokenBlacklisted tra cứu xem jti có tồn tại trong Redis không
func (r *redisBlacklistAdapter) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	blacklistKey := "blacklist:access_token:" + jti // Cùng quy tắc tạo key như hàm AddToBlacklist

	// Lệnh Exists đếm số lượng key tồn tại (tốc độ O(1) rất nhanh)
	result, err := r.client.Exists(ctx, blacklistKey).Result()

	if err != nil { // Nếu gọi Redis lỗi
		return false, err // Trả về false kèm theo lỗi để hàm gọi tự quyết định xử lý
	}

	return result > 0, nil // Nếu result > 0 nghĩa là key tồn tại -> token đã bị khóa
}
