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
	if prefix == "" {
		prefix = DefaultRedisKeyPrefix
	}
	return &RedisStore{
		rdb:    rdb,
		prefix: prefix,
	}
}

func (s *RedisStore) Reserve(ctx context.Context, rec Record, ttl time.Duration) (ReserveDecision, *Record, error) {
	key := s.redisKey(rec.Key)

	payload, err := json.Marshal(rec)
	if err != nil {
		return "", nil, err
	}

	ok, err := s.rdb.SetNX(ctx, key, payload, ttl).Result()
	if err != nil {
		return "", nil, err
	}
	if ok {
		return ReserveAcquired, &rec, nil
	}

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
	rec, err := s.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return nil
		}
		return err
	}

	rec.Status = StatusCompleted
	rec.CompletedAt = now
	rec.ResponseCode = result.StatusCode
	rec.ResponseBody = append([]byte(nil), result.Body...)
	rec.Headers = cloneHeaders(result.Headers)

	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}

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
	return s.prefix + key
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
