package idempotency

import "time"

type RecordStatus string

const (
	StatusInProgress RecordStatus = "in_progress"
	StatusCompleted  RecordStatus = "completed"
)

type ReserveDecision string

const (
	ReserveAcquired ReserveDecision = "acquired" // acquired means there is no record for this key
	ReserveExisting ReserveDecision = "existing" // existing means there is a record for this key
)

type BeginDecision string

const (
	BeginProceed  BeginDecision = "proceed"  // proceed means there is no record for this key
	BeginReplay   BeginDecision = "replay"   // replay means there is a record for this key
	BeginConflict BeginDecision = "conflict" // conflict means there is a record for this key but the request hash does not match
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
