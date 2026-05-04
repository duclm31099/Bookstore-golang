package redis

import (
	"encoding/json"
	"errors"
)

var (
	ErrCacheMiss       = errors.New("redis: cache miss")
	ErrLockNotHeld     = errors.New("redis: lock not held")
	ErrLockNotAcquired = errors.New("redis: lock not acquired")
	ErrUnavailable     = errors.New("redis: unavailable")
)

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func Unmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}
