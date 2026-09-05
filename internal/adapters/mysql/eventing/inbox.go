package eventing

import (
	"context"
	"errors"
	"fmt"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	domainevent "github.com/yuhang1130/go-service-main/internal/foundation/eventing"
	"gorm.io/gorm"
)

var ErrAlreadyProcessing = errors.New("event is already processing")

type InboxStore struct {
	database   *gorm.DB
	transactor *mysqladapter.Transactor
}

func NewInboxStore(database *gorm.DB) *InboxStore {
	return &InboxStore{database: database, transactor: mysqladapter.NewTransactor(database)}
}

// Process inserts Inbox state, runs database-only business changes, and commits
// final succeeded/rejected state in one transaction. Retryable failures roll back.
func (s *InboxStore) Process(ctx context.Context, group string, event domainevent.Envelope, handler domainevent.Handler) (domainevent.Result, error) {
	var result domainevent.Result
	err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		database := mysqladapter.FromContext(txCtx, s.database)
		insert := database.Exec(`INSERT IGNORE INTO event_inbox
            (consumer_group, event_id, event_type, event_version, status, created_at, updated_at)
            VALUES (?, ?, ?, ?, 'processing', UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))`,
			group, event.EventID, event.EventType, event.EventVersion)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 0 {
			var status string
			if err := database.Raw(`SELECT status FROM event_inbox
                WHERE consumer_group=? AND event_id=? FOR UPDATE`, group, event.EventID).Scan(&status).Error; err != nil {
				return err
			}
			if status == "succeeded" || status == "rejected" {
				result = domainevent.Success
				return nil
			}
			return ErrAlreadyProcessing
		}
		var err error
		result, err = handler.Handle(txCtx, event)
		if err != nil || result == domainevent.RetryableFailure {
			if err == nil {
				err = fmt.Errorf("retryable event failure")
			}
			return err
		}
		status := "succeeded"
		if result == domainevent.PermanentRejection || result == domainevent.InvalidMessage {
			status = "rejected"
		}
		update := database.Exec(`UPDATE event_inbox SET status=?, updated_at=UTC_TIMESTAMP(3)
            WHERE consumer_group=? AND event_id=? AND status='processing'`, status, group, event.EventID)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("inbox final state update lost")
		}
		return nil
	})
	if err != nil {
		return domainevent.RetryableFailure, err
	}
	return result, err
}
