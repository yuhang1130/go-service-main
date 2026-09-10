package taskdispatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	dispatch "github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
	"gorm.io/gorm"
)

type OutboxStore struct {
	database *gorm.DB
}

func NewOutboxStore(database *gorm.DB) *OutboxStore {
	return &OutboxStore{database: database}
}

func (s *OutboxStore) Enqueue(ctx context.Context, stream string, message dispatch.Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return fmt.Errorf("logical stream is required")
	}
	result := mysqladapter.FromContext(ctx, s.database).Exec(`INSERT INTO task_dispatch_outbox
		(dispatch_id, task_type, task_id, task_version, stream_name, status, attempts,
		 next_attempt_at, lease_owner, lease_until, last_error, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', 0, UTC_TIMESTAMP(3), '', NULL, '', UTC_TIMESTAMP(3))`,
		message.DispatchID, message.TaskType, message.TaskID, message.Version, stream)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("enqueue task dispatch %s affected %d rows", message.DispatchID, result.RowsAffected)
	}
	return nil
}

func (s *OutboxStore) Claim(ctx context.Context, owner string, limit int, lease time.Duration) ([]dispatch.Outbound, error) {
	return s.claim(ctx, owner, "", limit, lease)
}

func (s *OutboxStore) ClaimOne(ctx context.Context, owner, dispatchID string, lease time.Duration) (*dispatch.Outbound, error) {
	messages, err := s.claim(ctx, owner, dispatchID, 1, lease)
	if err != nil || len(messages) == 0 {
		return nil, err
	}
	return &messages[0], nil
}

func (s *OutboxStore) claim(ctx context.Context, owner, dispatchID string, limit int, lease time.Duration) ([]dispatch.Outbound, error) {
	if strings.TrimSpace(owner) == "" || limit <= 0 || lease <= 0 {
		return nil, fmt.Errorf("claim owner, positive limit, and positive lease are required")
	}
	claimed := make([]dispatch.Outbound, 0, limit)
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		type row struct {
			DispatchID  string
			TaskType    string
			TaskID      string
			TaskVersion int
			StreamName  string
			Attempts    int
		}
		var rows []row
		query := `SELECT dispatch_id, task_type, task_id, task_version, stream_name, attempts
			FROM task_dispatch_outbox
			WHERE status IN ('pending', 'publishing')
			  AND next_attempt_at <= UTC_TIMESTAMP(3)
			  AND (lease_until IS NULL OR lease_until <= UTC_TIMESTAMP(3))`
		args := make([]any, 0, 2)
		if dispatchID != "" {
			query += " AND dispatch_id = ?"
			args = append(args, dispatchID)
		}
		query += " ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED"
		args = append(args, limit)
		if err := tx.Raw(query, args...).Scan(&rows).Error; err != nil {
			return err
		}
		for _, item := range rows {
			update := tx.Exec(`UPDATE task_dispatch_outbox
				SET status='publishing', lease_owner=?,
				    lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL ? MICROSECOND),
				    attempts=attempts+1
				WHERE dispatch_id=?
				  AND status IN ('pending', 'publishing')
				  AND (lease_until IS NULL OR lease_until <= UTC_TIMESTAMP(3))`,
				owner, lease.Microseconds(), item.DispatchID)
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return fmt.Errorf("claim task dispatch %s affected %d rows", item.DispatchID, update.RowsAffected)
			}
			message := dispatch.Message{
				DispatchID: item.DispatchID,
				TaskType:   item.TaskType,
				TaskID:     item.TaskID,
				Version:    item.TaskVersion,
			}
			if err := message.Validate(); err != nil {
				if markErr := markDead(tx, item.DispatchID, owner, err.Error()); markErr != nil {
					return markErr
				}
				continue
			}
			if strings.TrimSpace(item.StreamName) == "" {
				if markErr := markDead(tx, item.DispatchID, owner, "logical stream is required"); markErr != nil {
					return markErr
				}
				continue
			}
			claimed = append(claimed, dispatch.Outbound{
				Stream: item.StreamName, Message: message, Attempts: item.Attempts + 1,
			})
		}
		return nil
	})
	return claimed, err
}

func (s *OutboxStore) MarkPublished(ctx context.Context, dispatchID, owner string) error {
	result := s.database.WithContext(ctx).Exec(`UPDATE task_dispatch_outbox
		SET status='published', published_at=UTC_TIMESTAMP(3), lease_owner='', lease_until=NULL
		WHERE dispatch_id=? AND status='publishing' AND lease_owner=?`, dispatchID, owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("task dispatch %s lease lost", dispatchID)
	}
	return nil
}

func (s *OutboxStore) MarkRetry(
	ctx context.Context,
	dispatchID, owner, summary string,
	retryAt time.Time,
	dead bool,
) error {
	status := "pending"
	if dead {
		status = "dead"
	}
	summary = truncate(summary, 1000)
	result := s.database.WithContext(ctx).Exec(`UPDATE task_dispatch_outbox
		SET status=?, next_attempt_at=?, lease_owner='', lease_until=NULL, last_error=?
		WHERE dispatch_id=? AND status='publishing' AND lease_owner=?`,
		status, retryAt, summary, dispatchID, owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("task dispatch %s lease lost", dispatchID)
	}
	return nil
}

func (s *OutboxStore) CleanupPublished(ctx context.Context, before time.Time, batchSize, maxBatches int) error {
	for batch := 0; batch < maxBatches; batch++ {
		result := s.database.WithContext(ctx).Exec(`DELETE FROM task_dispatch_outbox
			WHERE status='published' AND published_at < ? ORDER BY id LIMIT ?`, before, batchSize)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected < int64(batchSize) {
			return nil
		}
	}
	return nil
}

func markDead(database *gorm.DB, dispatchID, owner, summary string) error {
	result := database.Exec(`UPDATE task_dispatch_outbox
		SET status='dead', lease_owner='', lease_until=NULL, last_error=?
		WHERE dispatch_id=? AND status='publishing' AND lease_owner=?`,
		truncate(summary, 1000), dispatchID, owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("reject invalid task dispatch %s affected %d rows", dispatchID, result.RowsAffected)
	}
	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
