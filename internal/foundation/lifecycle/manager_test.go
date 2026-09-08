package lifecycle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
)

func TestManagerStartsByLayerAndStopsInReverse(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	started := map[string]bool{}
	stopped := map[string]bool{}
	service := func(name string, dependencies []string) Service {
		return NewService(name, dependencies, func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			for _, dependency := range dependencies {
				if !started[dependency] {
					t.Errorf("%s started before dependency %s", name, dependency)
				}
			}
			started[name] = true
			return nil
		}, func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			for candidate, candidateStarted := range started {
				if !candidateStarted || stopped[candidate] {
					continue
				}
				for _, dependency := range []string{"database", "cache"} {
					if name == dependency && candidate == "application" {
						t.Errorf("%s stopped before application", name)
					}
				}
			}
			stopped[name] = true
			return nil
		}, nil)
	}

	manager, err := NewManager(testLogger(),
		Options{},
		service("database", nil),
		service("cache", nil),
		service("application", []string{"database", "cache"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRollsBackAfterStartupFailure(t *testing.T) {
	t.Parallel()
	var dependencyStopped bool
	var failedServiceStopped bool
	want := errors.New("cannot start")
	manager, err := NewManager(testLogger(), Options{},
		NewService("dependency", nil, func(context.Context) error { return nil }, func(context.Context) error {
			dependencyStopped = true
			return nil
		}, nil),
		NewService("application", []string{"dependency"}, func(context.Context) error { return want }, func(context.Context) error {
			failedServiceStopped = true
			return nil
		}, nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Start() error = %v, want %v", err, want)
	}
	if !dependencyStopped {
		t.Fatal("started dependency was not rolled back")
	}
	if !failedServiceStopped {
		t.Fatal("failed service was not given a cleanup opportunity")
	}
}

func TestManagerReadinessIncludesLifecycleStateAndServiceCheck(t *testing.T) {
	t.Parallel()
	checkErr := errors.New("dependency unavailable")
	manager, err := NewManager(testLogger(), Options{},
		NewService("database", nil, func(context.Context) error { return nil }, nil, func(context.Context) error {
			return checkErr
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	check := manager.ReadinessChecks()["database"]
	if err := check(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("check before start = %v, want ErrNotReady", err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := check(context.Background()); !errors.Is(err, checkErr) {
		t.Fatalf("check after start = %v, want %v", err, checkErr)
	}
}

func TestManagerRejectsInvalidDependencyGraphs(t *testing.T) {
	t.Parallel()
	if _, err := NewManager(testLogger(), Options{},
		NewService("application", []string{"missing"}, nil, nil, nil),
	); err == nil {
		t.Fatal("unknown dependency should fail")
	}
	if _, err := NewManager(testLogger(), Options{},
		NewService("a", []string{"b"}, nil, nil, nil),
		NewService("b", []string{"a"}, nil, nil, nil),
	); err == nil {
		t.Fatal("dependency cycle should fail")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
