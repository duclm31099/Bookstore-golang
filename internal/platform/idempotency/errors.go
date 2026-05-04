package idempotency

import "errors"

var (
	ErrMissingKey          = errors.New("idempotency key is required")
	ErrInvalidScope        = errors.New("idempotency scope is invalid")
	ErrRequestInProgress   = errors.New("request with same idempotency key is in progress")
	ErrRequestHashMismatch = errors.New("same idempotency key used with different payload")
	ErrRecordNotFound      = errors.New("idempotency record not found")
	ErrMissingConsumerName = errors.New("consumer name is required")
	ErrMissingEventID      = errors.New("event ID is required")
)
