package bootstrap

import (
	"strconv"
	"testing"
	"time"
)

func TestServiceStartupStartedAtUsesRunCommandTimestamp(t *testing.T) {
	now := time.Date(2026, 9, 8, 6, 0, 5, 500_000_000, time.UTC)
	want := now.Add(-5 * time.Second).Truncate(time.Millisecond)
	t.Setenv(startupStartedAtEnv, strconv.FormatInt(want.UnixMilli(), 10))

	if got := serviceStartupStartedAt(now); !got.Equal(want) {
		t.Fatalf("serviceStartupStartedAt() = %s, want %s", got, want)
	}
}

func TestServiceStartupStartedAtFallsBackToProcessTime(t *testing.T) {
	now := time.Date(2026, 9, 8, 6, 0, 0, 0, time.UTC)
	t.Setenv(startupStartedAtEnv, "invalid")

	if got := serviceStartupStartedAt(now); !got.Equal(now) {
		t.Fatalf("serviceStartupStartedAt() = %s, want %s", got, now)
	}
}
