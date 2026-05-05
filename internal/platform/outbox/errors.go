package outbox

import "errors"

var (
	ErrNoRows      = errors.New("outbox: no rows affected")
	ErrMarshal     = errors.New("outbox: payload marshal failed")
	ErrAlreadyDone = errors.New("outbox: event already published or failed")
)
