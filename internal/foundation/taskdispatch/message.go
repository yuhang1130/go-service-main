package taskdispatch

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Message struct {
	DispatchID string `json:"dispatch_id"`
	TaskType   string `json:"task_type"`
	TaskID     string `json:"task_id"`
	Version    int    `json:"version"`
}

func (m Message) Validate() error {
	if strings.TrimSpace(m.DispatchID) == "" {
		return fmt.Errorf("dispatch id is required")
	}
	if strings.TrimSpace(m.TaskType) == "" {
		return fmt.Errorf("task type is required")
	}
	if strings.TrimSpace(m.TaskID) == "" {
		return fmt.Errorf("task id is required")
	}
	if m.Version <= 0 {
		return fmt.Errorf("task version must be positive")
	}
	return nil
}

type ClaimResult uint8

const (
	ClaimAcquired ClaimResult = iota + 1
	ClaimIgnored
)

type Handler interface {
	Claim(context.Context, Message) (ClaimResult, error)
	Execute(context.Context, Message) error
}

type Outbound struct {
	Stream   string
	Message  Message
	Attempts int
}

type Outbox interface {
	Claim(context.Context, string, int, time.Duration) ([]Outbound, error)
	ClaimOne(context.Context, string, string, time.Duration) (*Outbound, error)
	MarkPublished(context.Context, string, string) error
	MarkRetry(context.Context, string, string, string, time.Time, bool) error
	CleanupPublished(context.Context, time.Time, int, int) error
}

type Publisher interface {
	Publish(context.Context, string, Message) error
}
