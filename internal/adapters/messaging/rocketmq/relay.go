package rocketmq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	mysqlevent "github.com/yuhang1130/go-service-main/internal/adapters/mysql/eventing"
)

type Relay struct {
	store    *mysqlevent.OutboxStore
	producer *Producer
	owner    string
	logger   *slog.Logger
}

func NewRelay(store *mysqlevent.OutboxStore, producer *Producer, logger *slog.Logger) *Relay {
	return &Relay{store: store, producer: producer, owner: uuid.NewString(), logger: logger}
}

func (r *Relay) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		messages, err := r.store.Claim(ctx, r.owner, 100, 30*time.Second)
		if err != nil {
			return err
		}
		for _, message := range messages {
			if err := r.producer.Publish(ctx, message.LogicalTopic, message.Envelope); err != nil {
				retryAt := time.Now().UTC().Add(backoff(message.Attempts))
				dead := message.Attempts >= 20
				if markErr := r.store.MarkRetry(ctx, message.EventID, r.owner, err.Error(), retryAt, dead); markErr != nil {
					return fmt.Errorf("publish %s: %v; mark retry: %w", message.EventID, err, markErr)
				}
				continue
			}
			if err := r.store.MarkSent(ctx, message.EventID, r.owner); err != nil {
				return err
			}
		}
		if len(messages) < 100 {
			return nil
		}
	}
}

func (r *Relay) Cleanup(ctx context.Context) error {
	return r.store.CleanupDelivered(ctx, time.Now().UTC().Add(-30*24*time.Hour), 1000, 20)
}

func backoff(attempt int) time.Duration {
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<attempt) * time.Second
}
