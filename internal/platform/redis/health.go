package redis

import "context"

type HealthChecker struct {
	rdb Client
}

func NewHealthChecker(rdb Client) *HealthChecker {
	return &HealthChecker{rdb: rdb}
}

func (h *HealthChecker) Check(ctx context.Context) error {
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		return ErrUnavailable
	}
	return nil
}
