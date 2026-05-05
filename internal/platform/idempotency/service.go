package idempotency

import (
	"context"
	"strings"
	"time"
)

// Phần service là “bộ não” của package: nó quyết định request nào được chạy, request nào phải replay lại response cũ, và request nào phải bị chặn vì đang xử lý dở.
// Middleware thì đứng ở rìa HTTP để chặn duplicate ngay từ đầu, đúng với yêu cầu các mutation nhạy cảm phải có idempotency key thay vì chờ business layer xử lý sau

type Config struct {
	InProgressTTL time.Duration
	CompletedTTL  time.Duration
}

type service struct {
	store      Store
	eventStore ProcessedEventStore
	clock      Clock
	config     Config
}

func NewService(store Store, eventStore ProcessedEventStore, clock Clock, config Config) Service {

	if config.InProgressTTL <= 0 {
		config.InProgressTTL = 3 * time.Minute
	}

	if config.CompletedTTL <= 0 {
		config.CompletedTTL = 24 * time.Hour
	}
	return &service{
		store:      store,
		eventStore: eventStore,
		clock:      clock,
		config:     config,
	}
}

func (s *service) BeginHTTP(ctx context.Context, scope, rawKey, requestHash string) (BeginResult, error) {
	if strings.TrimSpace(rawKey) == "" {
		return BeginResult{}, ErrMissingKey
	}
	scope = NormalizeScope(scope)
	key := BuildKey(scope, rawKey)
	now := s.clock.Now()
	// 1. Create record
	record := Record{
		Key:         key,
		Scope:       scope,
		Status:      StatusInProgress,
		RequestHash: requestHash,
		StartedAt:   now,
	}

	// 2. Try acquire lock (Redis SETNX)
	decision, existing, err := s.store.Reserve(ctx, record, s.config.InProgressTTL)
	if err != nil {
		return BeginResult{}, err
	}

	// 3. If lock acquired => return proceed and record
	if decision == ReserveAcquired {
		return BeginResult{
			Decision: BeginProceed,
			Record:   &record,
		}, nil
	}
	// 4. If lock not acquired => check existing record

	// 4.1 If record not found => return error
	if existing == nil {
		return BeginResult{}, ErrRecordNotFound
	}
	// 4.2 If hash doesn't match => return error
	if existing.RequestHash != "" && requestHash != existing.RequestHash {
		return BeginResult{}, ErrRequestHashMismatch
	}

	switch existing.Status {
	// 4.3 If record is completed => return replay
	case StatusCompleted:
		return BeginResult{
			Decision: BeginReplay,
			Record:   existing,
		}, nil

	// 4.4 If record is in progress => return conflict
	case StatusInProgress:
		return BeginResult{
			Decision: BeginConflict,
			Record:   existing,
		}, ErrRequestInProgress
	// 4.5 For other statuses => return conflict
	default:
		return BeginResult{
			Decision: BeginConflict,
			Record:   existing,
		}, ErrRequestInProgress
	}
}

// CompleteHTTP marks the record as completed and sets the response
func (s *service) CompleteHTTP(ctx context.Context, scope, rawKey string, result Result) error {
	if strings.TrimSpace(rawKey) == "" {
		return ErrMissingKey
	}
	key := BuildKey(scope, rawKey)
	return s.store.Complete(ctx, key, result, s.config.CompletedTTL, s.clock.Now())
}

func (s *service) ReleaseHTTP(ctx context.Context, scope, rawKey string) error {
	if strings.TrimSpace(rawKey) == "" {
		return ErrMissingKey
	}
	key := BuildKey(scope, rawKey)
	return s.store.Release(ctx, key)
}

func (s *service) GetHTTP(ctx context.Context, scope, rawKey string) (*Record, error) {
	if strings.TrimSpace(rawKey) == "" {
		return nil, ErrMissingKey
	}
	key := BuildKey(scope, rawKey)
	return s.store.Get(ctx, key)
}

func (s *service) TryProcessEvent(
	ctx context.Context,
	consumerName string,
	eventID string,
	metadata map[string]any,
) (bool, error) {
	if strings.TrimSpace(consumerName) == "" {
		return false, ErrMissingConsumerName
	}
	if strings.TrimSpace(eventID) == "" {
		return false, ErrMissingEventID
	}
	return s.eventStore.TryMarkProcessed(ctx, consumerName, eventID, s.clock.Now(), metadata)
}
