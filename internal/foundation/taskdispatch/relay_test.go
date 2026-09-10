package taskdispatch

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type relayStore struct {
	outbound  *Outbound
	published int
	retried   int
}

func (s *relayStore) Claim(context.Context, string, int, time.Duration) ([]Outbound, error) {
	if s.outbound == nil {
		return nil, nil
	}
	return []Outbound{*s.outbound}, nil
}

func (s *relayStore) ClaimOne(context.Context, string, string, time.Duration) (*Outbound, error) {
	return s.outbound, nil
}

func (s *relayStore) MarkPublished(context.Context, string, string) error {
	s.published++
	s.outbound = nil
	return nil
}

func (s *relayStore) MarkRetry(context.Context, string, string, string, time.Time, bool) error {
	s.retried++
	s.outbound = nil
	return nil
}

func (*relayStore) CleanupPublished(context.Context, time.Time, int, int) error { return nil }

type relayPublisher struct{ err error }

func (p relayPublisher) Publish(context.Context, string, Message) error { return p.err }

func TestRelayMarksPublished(t *testing.T) {
	t.Parallel()
	store := &relayStore{outbound: &Outbound{
		Stream:  "material:collection:v1",
		Message: Message{DispatchID: "dispatch-1", TaskType: "material.collect", TaskID: "task-1", Version: 1},
	}}
	relay := NewRelay(store, relayPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := relay.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.published != 1 || store.retried != 0 {
		t.Fatalf("published/retried = %d/%d, want 1/0", store.published, store.retried)
	}
}

func TestRelaySchedulesRetry(t *testing.T) {
	t.Parallel()
	store := &relayStore{outbound: &Outbound{
		Stream:  "material:collection:v1",
		Message: Message{DispatchID: "dispatch-1", TaskType: "material.collect", TaskID: "task-1", Version: 1},
	}}
	relay := NewRelay(store, relayPublisher{err: errors.New("redis unavailable")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := relay.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.published != 0 || store.retried != 1 {
		t.Fatalf("published/retried = %d/%d, want 0/1", store.published, store.retried)
	}
}
