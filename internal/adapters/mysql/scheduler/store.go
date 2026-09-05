package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	jobscheduler "github.com/yuhang1130/go-service-main/internal/adapters/scheduler/gocron"
	"gorm.io/gorm"
)

type Store struct {
	database *gorm.DB
	owner    string
	logger   *slog.Logger
}

func New(database *gorm.DB, logger *slog.Logger) *Store {
	return &Store{database: database, owner: uuid.NewString(), logger: logger}
}

func (s *Store) TryAcquire(ctx context.Context, jobName string, duration time.Duration) (jobscheduler.Lease, bool, error) {
	var acquired bool
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT IGNORE INTO scheduler_job_lock
            (job_name, lease_owner, lease_until, created_at, updated_at)
            VALUES (?, '', '1970-01-01 00:00:00.000', UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))`, jobName).Error; err != nil {
			return err
		}
		result := tx.Exec(`UPDATE scheduler_job_lock
            SET lease_owner = ?,
                lease_until = DATE_ADD(UTC_TIMESTAMP(3), INTERVAL ? MICROSECOND),
                updated_at = UTC_TIMESTAMP(3)
            WHERE job_name = ?
              AND (lease_until <= UTC_TIMESTAMP(3) OR lease_owner = ?)`,
			s.owner, duration.Microseconds(), jobName, s.owner)
		if result.Error != nil {
			return result.Error
		}
		acquired = result.RowsAffected == 1
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &lease{store: s, jobName: jobName}, acquired, nil
}

func (s *Store) Started(ctx context.Context, jobName string, scheduledAt time.Time) (string, error) {
	runID := uuid.NewString()
	result := s.database.WithContext(ctx).Exec(`INSERT INTO scheduler_job_run
        (run_id, job_name, scheduled_at, started_at, status, instance_id, created_at)
        VALUES (?, ?, ?, UTC_TIMESTAMP(3), 'running', ?, UTC_TIMESTAMP(3))`,
		runID, jobName, scheduledAt, s.owner)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", fmt.Errorf("start scheduler run %s affected %d rows", runID, result.RowsAffected)
	}
	return runID, nil
}

func (s *Store) Finished(ctx context.Context, runID, status, errorSummary string) error {
	if len(errorSummary) > 1000 {
		errorSummary = errorSummary[:1000]
	}
	result := s.database.WithContext(ctx).Exec(`UPDATE scheduler_job_run
        SET finished_at = UTC_TIMESTAMP(3), status = ?, error_summary = ?
        WHERE run_id = ? AND status = 'running'`, status, strings.TrimSpace(errorSummary), runID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("scheduler run %s was not running", runID)
	}
	return nil
}

func (s *Store) CleanupRuns(ctx context.Context, before time.Time, batchSize, maxBatches int) error {
	for batch := 0; batch < maxBatches; batch++ {
		result := s.database.WithContext(ctx).Exec(`DELETE FROM scheduler_job_run
            WHERE finished_at IS NOT NULL AND finished_at < ? ORDER BY id LIMIT ?`, before, batchSize)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected < int64(batchSize) {
			return nil
		}
	}
	return nil
}

type lease struct {
	store   *Store
	jobName string
}

func (l *lease) Renew(ctx context.Context, duration time.Duration) error {
	result := l.store.database.WithContext(ctx).Exec(`UPDATE scheduler_job_lock
        SET lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL ? MICROSECOND), updated_at=UTC_TIMESTAMP(3)
        WHERE job_name=? AND lease_owner=?`, duration.Microseconds(), l.jobName, l.store.owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("job lease %s was lost", l.jobName)
	}
	return nil
}

func (l *lease) Release(ctx context.Context) error {
	result := l.store.database.WithContext(ctx).Exec(`UPDATE scheduler_job_lock
        SET lease_owner = '', lease_until = UTC_TIMESTAMP(3), updated_at = UTC_TIMESTAMP(3)
        WHERE job_name = ? AND lease_owner = ?`, l.jobName, l.store.owner)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		l.store.logger.Warn("job lease was no longer owned", "job", l.jobName)
	}
	return nil
}
