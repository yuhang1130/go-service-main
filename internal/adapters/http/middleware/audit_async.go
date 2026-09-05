package middleware

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
)

var (
	ErrAuditQueueFull = errors.New("operation audit queue is full")
	ErrAuditClosed    = errors.New("operation audit recorder is closed")
)

// AsyncAuditRecorder keeps best-effort audit persistence out of request
// latency. Its bounded queue deliberately applies load shedding instead of
// allowing an unavailable database to grow memory without limit.
type AsyncAuditRecorder struct {
	delegate AuditRecorder
	logger   *slog.Logger
	timeout  time.Duration
	queue    chan auditdomain.Entry
	stopping chan struct{}
	done     chan struct{}
	mu       sync.RWMutex
	closed   bool
	close    sync.Once
}

func NewAsyncAuditRecorder(delegate AuditRecorder, logger *slog.Logger, capacity int, timeout time.Duration) *AsyncAuditRecorder {
	if capacity < 1 {
		capacity = 1
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	recorder := &AsyncAuditRecorder{
		delegate: delegate,
		logger:   logger,
		timeout:  timeout,
		queue:    make(chan auditdomain.Entry, capacity),
		stopping: make(chan struct{}),
		done:     make(chan struct{}),
	}
	go recorder.run()
	return recorder
}

func (r *AsyncAuditRecorder) Record(ctx context.Context, entry auditdomain.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return ErrAuditClosed
	}
	select {
	case r.queue <- entry:
		return nil
	default:
		return ErrAuditQueueFull
	}
}

func (r *AsyncAuditRecorder) Close(ctx context.Context) error {
	r.close.Do(func() {
		r.mu.Lock()
		r.closed = true
		close(r.stopping)
		r.mu.Unlock()
	})
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *AsyncAuditRecorder) run() {
	defer close(r.done)
	for {
		select {
		case entry := <-r.queue:
			r.persist(entry)
		case <-r.stopping:
			for {
				select {
				case entry := <-r.queue:
					r.persist(entry)
				default:
					return
				}
			}
		}
	}
}

func (r *AsyncAuditRecorder) persist(entry auditdomain.Entry) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	if err := r.delegate.Record(ctx, entry); err != nil && r.logger != nil {
		r.logger.Warn("record operation audit failed", "error", err)
	}
}
