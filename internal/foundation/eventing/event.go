package eventing

import (
	"context"
	"encoding/json"
	"time"
)

type Envelope struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Producer      string          `json:"producer"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	AggregateID   string          `json:"aggregate_id"`
	TenantID      string          `json:"tenant_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

type Result string

const (
	Success            Result = "success"
	RetryableFailure   Result = "retryable_failure"
	PermanentRejection Result = "permanent_rejection"
	InvalidMessage     Result = "invalid_message"
)

type Handler interface {
	Handle(ctx context.Context, event Envelope) (Result, error)
}
