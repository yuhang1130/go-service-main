package taskdispatch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type Relay struct {
	store     Outbox
	publisher Publisher
	owner     string
	logger    *slog.Logger
}

func NewRelay(store Outbox, publisher Publisher, logger *slog.Logger) *Relay {
	if logger == nil {
		logger = slog.Default()
	}
	return &Relay{
		store: store, publisher: publisher, owner: uuid.NewString(), logger: logger,
	}
}

func (r *Relay) PublishOne(ctx context.Context, dispatchID string) error {
	outbound, err := r.store.ClaimOne(ctx, r.owner, dispatchID, 30*time.Second)
	if err != nil || outbound == nil {
		return err
	}
	return r.publish(ctx, *outbound)
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
			if err := r.publish(ctx, message); err != nil {
				return err
			}
		}
		if len(messages) < 100 {
			return nil
		}
	}
}

func (r *Relay) Cleanup(ctx context.Context) error {
	return r.store.CleanupPublished(ctx, time.Now().UTC().Add(-30*24*time.Hour), 1000, 20)
}

func (r *Relay) publish(ctx context.Context, outbound Outbound) error {
	err := r.publisher.Publish(ctx, outbound.Stream, outbound.Message)
	if err != nil {
		retryAt := time.Now().UTC().Add(relayBackoff(outbound.Attempts))
		dead := outbound.Attempts >= 20
		if markErr := r.store.MarkRetry(
			ctx,
			outbound.Message.DispatchID,
			r.owner,
			err.Error(),
			retryAt,
			dead,
		); markErr != nil {
			return fmt.Errorf(
				"publish task dispatch %s: %v; mark retry: %w",
				outbound.Message.DispatchID,
				err,
				markErr,
			)
		}
		r.logger.Warn(
			"task dispatch publication failed",
			"dispatch_id", outbound.Message.DispatchID,
			"task_type", outbound.Message.TaskType,
			"stream", outbound.Stream,
			"dead", dead,
			"error", err,
		)
		return nil
	}
	if err := r.store.MarkPublished(ctx, outbound.Message.DispatchID, r.owner); err != nil {
		return err
	}
	r.logger.Info(
		"task dispatch published",
		"dispatch_id", outbound.Message.DispatchID,
		"task_type", outbound.Message.TaskType,
		"stream", outbound.Stream,
	)
	return nil
}

func relayBackoff(attempt int) time.Duration {
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<attempt) * time.Second
}
