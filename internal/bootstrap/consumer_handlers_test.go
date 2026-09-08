package bootstrap

import (
	"context"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/foundation/eventing"
)

type consumerTestHandler struct{}

func (consumerTestHandler) Handle(context.Context, eventing.Envelope) (eventing.Result, error) {
	return eventing.Success, nil
}

func TestRequireEventHandlersRejectsIdleConsumer(t *testing.T) {
	t.Parallel()
	registry := eventing.NewRegistry()
	if err := requireEventHandlers(registry); err == nil {
		t.Fatal("requireEventHandlers() error = nil, want missing-handler failure")
	}
}

func TestRequireEventHandlersAcceptsRegisteredConsumer(t *testing.T) {
	t.Parallel()
	registry := eventing.NewRegistry()
	if err := registry.Register("test.event", 1, consumerTestHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := requireEventHandlers(registry); err != nil {
		t.Fatalf("requireEventHandlers() error = %v", err)
	}
}
