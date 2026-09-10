package bootstrap

import (
	"context"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
)

type consumerTestHandler struct{}

func (consumerTestHandler) Claim(context.Context, taskdispatch.Message) (taskdispatch.ClaimResult, error) {
	return taskdispatch.ClaimAcquired, nil
}

func (consumerTestHandler) Execute(context.Context, taskdispatch.Message) error { return nil }

func TestRequireTaskHandlersRejectsIdleConsumer(t *testing.T) {
	t.Parallel()
	registry := taskdispatch.NewRegistry()
	if err := requireTaskHandlers(registry); err == nil {
		t.Fatal("requireTaskHandlers() error = nil, want missing-handler failure")
	}
}

func TestRequireTaskHandlersAcceptsRegisteredConsumer(t *testing.T) {
	t.Parallel()
	registry := taskdispatch.NewRegistry()
	if err := registry.Register("test.task", 1, consumerTestHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := requireTaskHandlers(registry); err != nil {
		t.Fatalf("requireTaskHandlers() error = %v", err)
	}
}
