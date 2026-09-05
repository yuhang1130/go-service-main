package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type runnerFunc func(context.Context) error

func (run runnerFunc) Run(ctx context.Context) error { return run(ctx) }

type routeHandler struct{}

func (routeHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func (routeHandler) Routes() []string {
	return []string{"GET /livez", "GET /readyz", "GET /buildinfo"}
}

func TestHTTPLogsRegisteredRoutes(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	httpServer := New("management", 9091, routeHandler{}, config.Defaults().Server, logger)

	httpServer.logRoutes()

	logs := output.String()
	for _, route := range (routeHandler{}).Routes() {
		if !strings.Contains(logs, "server=management") || !strings.Contains(logs, "route=\""+route+"\"") {
			t.Fatalf("route %q missing from logs: %s", route, logs)
		}
	}
	if got := strings.Count(logs, "msg=\"http route registered\""); got != 3 {
		t.Fatalf("registered route log count = %d, want 3: %s", got, logs)
	}
}

func TestRunAllWaitsForEveryRunner(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	stopped := make(chan struct{})
	want := errors.New("listener failed")

	first := runnerFunc(func(context.Context) error {
		<-started
		return want
	})
	second := runnerFunc(func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil
	})

	if err := RunAll(context.Background(), first, second); !errors.Is(err, want) {
		t.Fatalf("RunAll error = %v, want %v", err, want)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("RunAll returned before every runner stopped")
	}
}
