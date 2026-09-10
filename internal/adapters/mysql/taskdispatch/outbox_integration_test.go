//go:build integration

package taskdispatch

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	dispatch "github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
)

func TestOutboxParticipatesInBusinessTransaction(t *testing.T) {
	database := openIntegrationDatabase(t)
	store := NewOutboxStore(database.GORM())
	dispatchID := uuid.NewString()
	t.Cleanup(func() {
		database.GORM().Exec("DELETE FROM task_dispatch_outbox WHERE dispatch_id = ?", dispatchID)
	})

	transactor := mysqladapter.NewTransactor(database.GORM())
	errRollback := errors.New("rollback")
	err := transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		if err := store.Enqueue(ctx, "material:collection:v1", testMessage(dispatchID)); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction error = %v", err)
	}
	var count int64
	if err := database.GORM().Table("task_dispatch_outbox").
		Where("dispatch_id = ?", dispatchID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled back outbox rows = %d, want 0", count)
	}
}

func TestOutboxClaimAndPublishLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	store := NewOutboxStore(database.GORM())
	dispatchID := uuid.NewString()
	t.Cleanup(func() {
		database.GORM().Exec("DELETE FROM task_dispatch_outbox WHERE dispatch_id = ?", dispatchID)
	})
	if err := store.Enqueue(context.Background(), "material:collection:v1", testMessage(dispatchID)); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), "integration-owner", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Message.DispatchID != dispatchID {
		t.Fatalf("claimed = %#v", claimed)
	}
	if err := store.MarkPublished(context.Background(), dispatchID, "integration-owner"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.GORM().Table("task_dispatch_outbox").
		Select("status").Where("dispatch_id = ?", dispatchID).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != "published" {
		t.Fatalf("status = %q, want published", status)
	}
}

func openIntegrationDatabase(t *testing.T) *mysqladapter.Database {
	t.Helper()
	dsn := os.Getenv("APP_MYSQL_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_MYSQL_DSN is required in CI")
		}
		t.Skip("APP_MYSQL_DSN is not set")
	}
	cfg := config.Defaults().MySQL
	cfg.DSN = dsn
	database, err := mysqladapter.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func testMessage(dispatchID string) dispatch.Message {
	return dispatch.Message{
		DispatchID: dispatchID,
		TaskType:   "material.collect",
		TaskID:     "task-" + dispatchID,
		Version:    1,
	}
}
