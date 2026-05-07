package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	rdb    redis.UniversalClient
	prefix string
}

func NewRedisStore(rdb redis.UniversalClient) *RedisStore {
	return &RedisStore{
		rdb:    rdb,
		prefix: DefaultRedisKeyPrefix,
	}
}

func NewRedisStoreWithPrefix(rdb redis.UniversalClient, prefix string) *RedisStore {
	return &RedisStore{
		rdb:    rdb,
		prefix: prefix,
	}
}

// Reserve dùng Redis SETNX để “xí chỗ” một request trước khi xử lý.
// Nếu key đã tồn tại → request này là duplicate → trả về existing.
// Nếu chưa tồn tại → set key với TTL → đánh dấu đang xử lý.
// Chỉ dùng prefix cho Redis key để dễ debug, không dùng trong logic business.
func (s *RedisStore) Reserve(ctx context.Context, rec Record, ttl time.Duration) (ReserveDecision, *Record, error) {
	// 1. Tạo idempotency key từ config
	key := s.redisKey(rec.Key)

	// 2. Marshal record struct to JSON
	payload, err := json.Marshal(rec)
	if err != nil {
		return "", nil, err
	}

	// 3. SetNX là atomic: chỉ set nếu key chưa tồn tại.
	// 4. ttl = 5 phút (thời gian sống của request)
	ok, err := s.rdb.SetNX(ctx, key, payload, ttl).Result()
	if err != nil {
		return "", nil, err
	}

	if ok {
		// 5. Nếu chưa có key -> return acquired
		return ReserveAcquired, &rec, nil
	}

	// 6. Nếu đã có key -> return existing
	existing, err := s.Get(ctx, rec.Key)
	if err != nil {
		return "", nil, err
	}
	return ReserveExisting, existing, nil
}

func (s *RedisStore) Get(ctx context.Context, key string) (*Record, error) {
	raw, err := s.rdb.Get(ctx, s.redisKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *RedisStore) Complete(ctx context.Context, key string, result Result, ttl time.Duration, now time.Time) error {
	// 1. Get record từ redis cache
	rec, err := s.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			// Nếu record không tồn tại => return nil
			return nil
		}
		return err
	}

	// 2. Update record status
	rec.Status = StatusCompleted
	// 3. Set completed at
	rec.CompletedAt = now
	// 4. Set response code
	rec.ResponseCode = result.StatusCode
	// 5. Set response body
	rec.ResponseBody = append([]byte(nil), result.Body...)
	// 6. Set headers
	rec.Headers = cloneHeaders(result.Headers)

	// 7. Marshal record struct to JSON
	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	// 8. Set record to redis
	if err := s.rdb.Set(ctx, s.redisKey(key), payload, ttl).Err(); err != nil {
		return wrapRedisErr(err)
	}
	return nil
}

func (s *RedisStore) Release(ctx context.Context, key string) error {
	if err := s.rdb.Del(ctx, s.redisKey(key)).Err(); err != nil {
		return wrapRedisErr(err)
	}
	return nil
}

func (s *RedisStore) redisKey(key string) string {
	return key
}

func cloneHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func wrapRedisErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, redis.Nil) {
		return ErrRecordNotFound
	}
	return err
}
