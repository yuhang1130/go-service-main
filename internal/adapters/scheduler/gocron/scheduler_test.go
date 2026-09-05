package gocron

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestStartLogsRegisteredJobs(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	scheduler, err := New(context.Background(), logger, LocalLocker{}, LogRecorder{Logger: logger})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := scheduler.Register(Job{
		Name:     "test-retention",
		Schedule: "0 0 * * *",
		Timeout:  30 * time.Second,
		Lease:    2 * time.Minute,
		Run:      func(context.Context) error { return nil },
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	scheduler.Start()
	t.Cleanup(func() {
		if err := scheduler.Shutdown(); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	logs := output.String()
	checks := []string{
		`msg="scheduler starting" job_count=1 timezone=UTC`,
		`msg="scheduled job configured" job=test-retention`,
		`schedule="0 0 * * *"`,
		`timezone=UTC timeout=30s lease=2m0s`,
	}
	for _, check := range checks {
		if !strings.Contains(logs, check) {
			t.Fatalf("startup logs missing %q: %s", check, logs)
		}
	}
}
