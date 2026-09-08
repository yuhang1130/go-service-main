package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreakerOpensAndShortCircuits(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	breaker, err := New(Config{
		FailureThreshold: 2, OpenTimeout: time.Minute, HalfOpenMaxRequests: 1,
	}, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	operationErr := errors.New("upstream failed")
	for range 2 {
		if err := breaker.Execute(context.Background(), func(context.Context) error { return operationErr }); !errors.Is(err, operationErr) {
			t.Fatalf("Execute() error = %v, want %v", err, operationErr)
		}
	}
	if state := breaker.State(); state != StateOpen {
		t.Fatalf("state = %s, want open", state)
	}
	called := false
	if err := breaker.Execute(context.Background(), func(context.Context) error {
		called = true
		return nil
	}); !errors.Is(err, ErrOpen) {
		t.Fatalf("short-circuit error = %v, want ErrOpen", err)
	}
	if called {
		t.Fatal("operation ran while breaker was open")
	}
}

func TestBreakerClosesAfterSuccessfulHalfOpenProbe(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	breaker, err := New(Config{
		FailureThreshold: 1, OpenTimeout: time.Minute, HalfOpenMaxRequests: 1,
	}, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	operationErr := errors.New("upstream failed")
	_ = breaker.Execute(context.Background(), func(context.Context) error { return operationErr })
	now = now.Add(time.Minute)
	if err := breaker.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if state := breaker.State(); state != StateClosed {
		t.Fatalf("state = %s, want closed", state)
	}
}

func TestBreakerReopensAfterFailedHalfOpenProbe(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	breaker, err := New(Config{
		FailureThreshold: 1, OpenTimeout: time.Minute, HalfOpenMaxRequests: 1,
	}, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	operationErr := errors.New("upstream failed")
	_ = breaker.Execute(context.Background(), func(context.Context) error { return operationErr })
	now = now.Add(time.Minute)
	_ = breaker.Execute(context.Background(), func(context.Context) error { return operationErr })
	if state := breaker.State(); state != StateOpen {
		t.Fatalf("state = %s, want open", state)
	}
}
