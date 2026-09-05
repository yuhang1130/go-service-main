package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	domainevent "github.com/yuhang1130/go-service-main/internal/foundation/eventing"
	"gorm.io/gorm"
)

type OutboxMessage struct {
	EventID      string
	LogicalTopic string
	Envelope     domainevent.Envelope
	Attempts     int
}

type OutboxStore struct{ database *gorm.DB }

func NewOutboxStore(database *gorm.DB) *OutboxStore { return &OutboxStore{database: database} }

func (s *OutboxStore) Enqueue(ctx context.Context, logicalTopic string, event domainevent.Envelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	result := mysqladapter.FromContext(ctx, s.database).Exec(`INSERT INTO event_outbox
        (event_id, event_type, event_version, aggregate_id, logical_topic, payload, status,
         attempts, next_attempt_at, lease_owner, lease_until, last_error, created_at)
        VALUES (?, ?, ?, ?, ?, ?, 'pending', 0, UTC_TIMESTAMP(3), '', NULL, '', UTC_TIMESTAMP(3))`,
		event.EventID, event.EventType, event.EventVersion, event.AggregateID, logicalTopic, payload)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("enqueue outbox event %s affected %d rows", event.EventID, result.RowsAffected)
	}
	return nil
}

func (s *OutboxStore) Claim(ctx context.Context, owner string, limit int, lease time.Duration) ([]OutboxMessage, error) {
	claimed := make([]OutboxMessage, 0, limit)
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		type row struct {
			EventID, LogicalTopic string
			Payload               []byte
			Attempts              int
		}
		var rows []row
		if err := tx.Raw(`SELECT event_id, logical_topic, payload, attempts FROM event_outbox
            WHERE status IN ('pending', 'publishing')
              AND next_attempt_at <= UTC_TIMESTAMP(3)
              AND (lease_until IS NULL OR lease_until <= UTC_TIMESTAMP(3))
            ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED`, limit).Scan(&rows).Error; err != nil {
			return err
		}
		for _, item := range rows {
			update := tx.Exec(`UPDATE event_outbox SET status='publishing', lease_owner=?,
				lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL ? MICROSECOND), attempts=attempts+1
				WHERE event_id=?`, owner, lease.Microseconds(), item.EventID)
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return fmt.Errorf("claim outbox event %s affected %d rows", item.EventID, update.RowsAffected)
			}
			var envelope domainevent.Envelope
			if err := json.Unmarshal(item.Payload, &envelope); err != nil {
				summary := fmt.Sprintf("decode outbox event: %v", err)
				if len(summary) > 1000 {
					summary = summary[:1000]
				}
				dead := tx.Exec(`UPDATE event_outbox SET status='dead', lease_owner='', lease_until=NULL,
					last_error=? WHERE event_id=? AND status='publishing' AND lease_owner=?`, summary, item.EventID, owner)
				if dead.Error != nil {
					return dead.Error
				}
				if dead.RowsAffected != 1 {
					return fmt.Errorf("reject corrupt outbox event %s affected %d rows", item.EventID, dead.RowsAffected)
				}
				continue
			}
			claimed = append(claimed, OutboxMessage{EventID: item.EventID, LogicalTopic: item.LogicalTopic, Envelope: envelope, Attempts: item.Attempts + 1})
		}
		return nil
	})
	return claimed, err
}

func (s *OutboxStore) MarkSent(ctx context.Context, eventID, owner string) error {
	result := s.database.WithContext(ctx).Exec(`UPDATE event_outbox SET status='sent', sent_at=UTC_TIMESTAMP(3),
        lease_owner='', lease_until=NULL WHERE event_id=? AND status='publishing' AND lease_owner=?`, eventID, owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox event %s lease lost", eventID)
	}
	return nil
}

func (s *OutboxStore) MarkRetry(ctx context.Context, eventID, owner, summary string, retryAt time.Time, dead bool) error {
	status := "pending"
	if dead {
		status = "dead"
	}
	if len(summary) > 1000 {
		summary = summary[:1000]
	}
	result := s.database.WithContext(ctx).Exec(`UPDATE event_outbox SET status=?, next_attempt_at=?,
        lease_owner='', lease_until=NULL, last_error=?
        WHERE event_id=? AND status='publishing' AND lease_owner=?`, status, retryAt, summary, eventID, owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox event %s lease lost", eventID)
	}
	return nil
}

func (s *OutboxStore) CleanupDelivered(ctx context.Context, before time.Time, batchSize, maxBatches int) error {
	for batch := 0; batch < maxBatches; batch++ {
		outbox := s.database.WithContext(ctx).Exec(`DELETE FROM event_outbox
            WHERE status='sent' AND sent_at < ? ORDER BY id LIMIT ?`, before, batchSize)
		if outbox.Error != nil {
			return outbox.Error
		}
		inbox := s.database.WithContext(ctx).Exec(`DELETE FROM event_inbox
            WHERE status IN ('succeeded', 'rejected') AND updated_at < ? ORDER BY id LIMIT ?`, before, batchSize)
		if inbox.Error != nil {
			return inbox.Error
		}
		if outbox.RowsAffected < int64(batchSize) && inbox.RowsAffected < int64(batchSize) {
			return nil
		}
	}
	return nil
}
