package redis

import (
	"context"
	"time"

	"github.com/google/uuid"
)

var releaseLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

type Locker struct {
	rdb Client
}

type Lock struct {
	Key   string
	Token string
	TTL   time.Duration
}

func NewLocker(rdb Client) *Locker {
	return &Locker{rdb: rdb}
}

func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	token := uuid.NewString()
	ok, err := l.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, ErrUnavailable
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}
	return &Lock{Key: key, Token: token, TTL: ttl}, nil
}

func (l *Locker) Release(ctx context.Context, lock *Lock) error {
	if lock == nil {
		return nil
	}
	n, err := l.rdb.Eval(ctx, releaseLockScript, []string{lock.Key}, lock.Token).Int64()
	if err != nil {
		return ErrUnavailable
	}
	if n == 0 {
		return ErrLockNotHeld
	}
	return nil
}
