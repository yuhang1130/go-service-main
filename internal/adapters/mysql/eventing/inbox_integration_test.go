//go:build integration

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	domainevent "github.com/yuhang1130/go-service-main/internal/foundation/eventing"
)

type countingHandler struct{ calls int }

func (handler *countingHandler) Handle(context.Context, domainevent.Envelope) (domainevent.Result, error) {
	handler.calls++
	return domainevent.Success, nil
}

type failOnceHandler struct{ calls int }

func (handler *failOnceHandler) Handle(context.Context, domainevent.Envelope) (domainevent.Result, error) {
	handler.calls++
	if handler.calls == 1 {
		return domainevent.RetryableFailure, errors.New("temporary failure")
	}
	return domainevent.Success, nil
}

func TestInboxHandlerSuppressesDeliveredDuplicate(t *testing.T) {
	ctx := context.Background()
	database := openIntegrationDatabase(t)
	eventID := uuid.NewString()
	consumerGroup := "integration-inbox"
	t.Cleanup(func() {
		database.GORM().Exec("DELETE FROM event_inbox WHERE consumer_group = ? AND event_id = ?", consumerGroup, eventID)
	})
	next := new(countingHandler)
	handler, err := NewInboxHandler(NewInboxStore(database.GORM()), consumerGroup, next)
	if err != nil {
		t.Fatal(err)
	}
	event := testEnvelope(eventID)
	for attempt := 0; attempt < 2; attempt++ {
		result, err := handler.Handle(ctx, event)
		if err != nil || result != domainevent.Success {
			t.Fatalf("attempt %d result = %q, %v", attempt+1, result, err)
		}
	}
	if next.calls != 1 {
		t.Fatalf("business handler calls = %d, want 1", next.calls)
	}
}

func TestInboxHandlerRollsBackRetryableFailure(t *testing.T) {
	database := openIntegrationDatabase(t)
	eventID := uuid.NewString()
	consumerGroup := "integration-inbox-retry"
	t.Cleanup(func() {
		database.GORM().Exec("DELETE FROM event_inbox WHERE consumer_group = ? AND event_id = ?", consumerGroup, eventID)
	})
	next := new(failOnceHandler)
	handler, err := NewInboxHandler(NewInboxStore(database.GORM()), consumerGroup, next)
	if err != nil {
		t.Fatal(err)
	}
	event := testEnvelope(eventID)

	result, err := handler.Handle(context.Background(), event)
	if err == nil || result != domainevent.RetryableFailure {
		t.Fatalf("first attempt result = %q, %v", result, err)
	}
	result, err = handler.Handle(context.Background(), event)
	if err != nil || result != domainevent.Success {
		t.Fatalf("second attempt result = %q, %v", result, err)
	}
	if next.calls != 2 {
		t.Fatalf("business handler calls = %d, want 2", next.calls)
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

func testEnvelope(eventID string) domainevent.Envelope {
	return domainevent.Envelope{
		EventID:      eventID,
		EventType:    "test.item-created",
		EventVersion: 1,
		OccurredAt:   time.Now().UTC(),
		Producer:     "integration-test",
		AggregateID:  "item-1",
		Payload:      json.RawMessage(`{"item_id":"item-1"}`),
	}
}
