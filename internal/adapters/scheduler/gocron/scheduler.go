package gocron

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	cron "github.com/go-co-op/gocron/v2"
)

type Job struct {
	Name     string
	Schedule string
	Timeout  time.Duration
	Lease    time.Duration
	Run      func(context.Context) error
}

type Lease interface {
	Renew(context.Context, time.Duration) error
	Release(context.Context) error
}

type Locker interface {
	TryAcquire(context.Context, string, time.Duration) (Lease, bool, error)
}

type Recorder interface {
	Started(context.Context, string, time.Time) (string, error)
	Finished(context.Context, string, string, string) error
}

type Scheduler struct {
	inner      cron.Scheduler
	ctx        context.Context
	logger     *slog.Logger
	locker     Locker
	recorder   Recorder
	jobsMu     sync.RWMutex
	registered []jobDescriptor
}

type jobDescriptor struct {
	name     string
	schedule string
	timeout  time.Duration
	lease    time.Duration
}

const bookkeepingTimeout = 5 * time.Second

func New(ctx context.Context, logger *slog.Logger, locker Locker, recorder Recorder) (*Scheduler, error) {
	inner, err := cron.NewScheduler(cron.WithLocation(time.UTC))
	if err != nil {
		return nil, err
	}
	return &Scheduler{inner: inner, ctx: ctx, logger: logger, locker: locker, recorder: recorder}, nil
}

func (s *Scheduler) Register(job Job) error {
	if job.Name == "" || job.Schedule == "" || job.Run == nil {
		return fmt.Errorf("job name, schedule, and runner are required")
	}
	if job.Timeout <= 0 || job.Lease <= job.Timeout {
		return fmt.Errorf("job timeout must be positive and lease must exceed timeout")
	}
	_, err := s.inner.NewJob(
		cron.CronJob(job.Schedule, false),
		cron.NewTask(func() { s.run(job) }),
		cron.WithName(job.Name),
		cron.WithSingletonMode(cron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	s.jobsMu.Lock()
	s.registered = append(s.registered, jobDescriptor{
		name: job.Name, schedule: job.Schedule, timeout: job.Timeout, lease: job.Lease,
	})
	s.jobsMu.Unlock()
	return nil
}

func (s *Scheduler) Start() {
	s.jobsMu.RLock()
	jobs := append([]jobDescriptor(nil), s.registered...)
	s.jobsMu.RUnlock()
	s.logger.Info("scheduler starting", "job_count", len(jobs), "timezone", "UTC")
	for _, job := range jobs {
		s.logger.Info("scheduled job configured",
			"job", job.name,
			"schedule", job.schedule,
			"timezone", "UTC",
			"timeout", job.timeout,
			"lease", job.lease,
		)
	}
	s.inner.Start()
}

func (s *Scheduler) Shutdown() error { return s.inner.Shutdown() }

func (s *Scheduler) run(job Job) {
	ctx, cancel := context.WithTimeout(s.ctx, job.Timeout)
	defer cancel()
	lease, acquired, err := s.locker.TryAcquire(ctx, job.Name, job.Lease)
	if err != nil {
		s.logger.Error("job lease failed", "job", job.Name, "error", err)
		return
	}
	if !acquired {
		s.logger.Info("job skipped because lease is held", "job", job.Name)
		return
	}
	defer func() {
		releaseCtx, cancelRelease := context.WithTimeout(context.Background(), bookkeepingTimeout)
		defer cancelRelease()
		if err := lease.Release(releaseCtx); err != nil {
			s.logger.Error("job lease release failed", "job", job.Name, "error", err)
		}
	}()
	heartbeatCtx, stopHeartbeat := context.WithCancel(context.Background())
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		interval := job.Lease / 3
		if interval <= 0 {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := lease.Renew(heartbeatCtx, job.Lease); err != nil {
					s.logger.Error("job lease renewal failed", "job", job.Name, "error", err)
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		stopHeartbeat()
		<-heartbeatDone
	}()
	runID, err := s.recorder.Started(ctx, job.Name, time.Now().UTC())
	if err != nil {
		s.logger.Error("job run record failed", "job", job.Name, "error", err)
		return
	}
	status := "succeeded"
	errorSummary := ""
	if err := job.Run(ctx); err != nil {
		status = "failed"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "timed_out"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			status = "cancelled"
		}
		errorSummary = err.Error()
		s.logger.Error("job failed", "job", job.Name, "run_id", runID, "error", err)
	}
	finishCtx, cancelFinish := context.WithTimeout(context.Background(), bookkeepingTimeout)
	defer cancelFinish()
	if err := s.recorder.Finished(finishCtx, runID, status, errorSummary); err != nil {
		s.logger.Error("finish job run record failed", "job", job.Name, "run_id", runID, "error", err)
	}
}

type LocalLocker struct{}
type localLease struct{}

func (LocalLocker) TryAcquire(context.Context, string, time.Duration) (Lease, bool, error) {
	return localLease{}, true, nil
}
func (localLease) Renew(context.Context, time.Duration) error { return nil }
func (localLease) Release(context.Context) error              { return nil }

type LogRecorder struct{ Logger *slog.Logger }

func (r LogRecorder) Started(_ context.Context, name string, scheduled time.Time) (string, error) {
	id := fmt.Sprintf("%s-%d", name, scheduled.UnixNano())
	r.Logger.Info("job started", "job", name, "run_id", id)
	return id, nil
}
func (r LogRecorder) Finished(_ context.Context, id, status, summary string) error {
	r.Logger.Info("job finished", "run_id", id, "status", status, "error_summary", summary)
	return nil
}
