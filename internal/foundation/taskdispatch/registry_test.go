package taskdispatch

import (
	"context"
	"testing"
)

type testHandler struct{}

func (testHandler) Claim(context.Context, Message) (ClaimResult, error) {
	return ClaimAcquired, nil
}

func (testHandler) Execute(context.Context, Message) error { return nil }

func TestRegistryRejectsDuplicateHandler(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	if err := registry.Register("material.collect", 1, testHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("material.collect", 1, testHandler{}); err == nil {
		t.Fatal("duplicate registration error = nil")
	}
}

func TestMessageValidate(t *testing.T) {
	t.Parallel()
	if err := (Message{DispatchID: "dispatch-1", TaskType: "material.collect", TaskID: "task-1", Version: 1}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Message{DispatchID: "dispatch-1", TaskType: "material.collect", Version: 1}).Validate(); err == nil {
		t.Fatal("missing task id error = nil")
	}
}
