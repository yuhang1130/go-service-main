package middleware

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
)

type collectingAuditRecorder struct {
	mu      sync.Mutex
	entries []auditdomain.Entry
}

func (r *collectingAuditRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	r.mu.Lock()
	r.entries = append(r.entries, entry)
	r.mu.Unlock()
	return nil
}

func TestAsyncAuditRecorderDrainsOnClose(t *testing.T) {
	delegate := &collectingAuditRecorder{}
	recorder := NewAsyncAuditRecorder(delegate, nil, 4, time.Second)
	for id := int64(1); id <= 3; id++ {
		if err := recorder.Record(context.Background(), auditdomain.Entry{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := recorder.Close(ctx); err != nil {
		t.Fatal(err)
	}
	delegate.mu.Lock()
	defer delegate.mu.Unlock()
	if len(delegate.entries) != 3 {
		t.Fatalf("recorded %d entries, want 3", len(delegate.entries))
	}
	if err := recorder.Record(context.Background(), auditdomain.Entry{}); !errors.Is(err, ErrAuditClosed) {
		t.Fatalf("Record() after Close() = %v, want ErrAuditClosed", err)
	}
}
