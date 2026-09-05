//go:build integration

package scheduler

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestStoreCoordinatesLeasesAcrossInstances(t *testing.T) {
	dsn := os.Getenv("APP_MYSQL_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_MYSQL_DSN is required in CI")
		}
		t.Skip("APP_MYSQL_DSN is not set")
	}
	ctx := context.Background()
	cfg := config.Defaults().MySQL
	cfg.DSN = dsn
	database, err := mysqladapter.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	first := New(database.GORM(), logger)
	second := New(database.GORM(), logger)
	jobName := "integration-lease-" + time.Now().UTC().Format("20060102150405.000000000")
	t.Cleanup(func() { database.GORM().Exec("DELETE FROM scheduler_job_lock WHERE job_name = ?", jobName) })

	lease, acquired, err := first.TryAcquire(ctx, jobName, time.Minute)
	if err != nil || !acquired {
		t.Fatalf("first acquire = %v, %v", acquired, err)
	}
	if _, acquired, err := second.TryAcquire(ctx, jobName, time.Minute); err != nil || acquired {
		t.Fatalf("second acquire while held = %v, %v", acquired, err)
	}
	if err := lease.Renew(ctx, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(ctx); err != nil {
		t.Fatal(err)
	}
	secondLease, acquired, err := second.TryAcquire(ctx, jobName, time.Minute)
	if err != nil || !acquired {
		t.Fatalf("second acquire after release = %v, %v", acquired, err)
	}
	if err := secondLease.Release(ctx); err != nil {
		t.Fatal(err)
	}
}
