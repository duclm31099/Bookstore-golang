package idempotency

import "time"

type RecordStatus string

const (
	StatusInProgress RecordStatus = "in_progress"
	StatusCompleted  RecordStatus = "completed"
)

type ReserveDecision string

const (
	ReserveAcquired ReserveDecision = "acquired"
	ReserveExisting ReserveDecision = "existing"
)

type BeginDecision string

const (
	BeginProceed  BeginDecision = "proceed"
	BeginReplay   BeginDecision = "replay"
	BeginConflict BeginDecision = "conflict"
)

type Record struct {
	Key          string            `json:"key"`
	Scope        string            `json:"scope"`
	Status       RecordStatus      `json:"status"`
	RequestHash  string            `json:"request_hash,omitempty"`
	ResponseCode int               `json:"response_code,omitempty"`
	ResponseBody []byte            `json:"response_body,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	CompletedAt  time.Time         `json:"completed_at"`
}

type Result struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

type BeginResult struct {
	Decision BeginDecision
	Record   *Record
}
