package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	rocketmqadapter "github.com/yuhang1130/go-service-main/internal/adapters/messaging/rocketmq"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	mysqlevent "github.com/yuhang1130/go-service-main/internal/adapters/mysql/eventing"
	mysqlscheduler "github.com/yuhang1130/go-service-main/internal/adapters/mysql/scheduler"
	scheduler "github.com/yuhang1130/go-service-main/internal/adapters/scheduler/gocron"
	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/lifecycle"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
)

func RunJob(ctx context.Context) (runErr error) {
	cfg := config.Defaults()
	if err := config.Load(config.Path("job"), "job", &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", "job")
	registry := health.New(buildinfo.Current())
	var database *mysqladapter.Database
	var producer *rocketmqadapter.Producer
	var jobScheduler *scheduler.Scheduler
	manager, err := newLifecycleManager(logger, cfg,
		lifecycle.NewService("mysql", nil, func(startCtx context.Context) error {
			opened, openErr := mysqladapter.Open(startCtx, cfg.MySQL)
			if openErr != nil {
				return openErr
			}
			database = opened
			return nil
		}, func(context.Context) error {
			if database == nil {
				return nil
			}
			return database.Close()
		}, func(readyCtx context.Context) error {
			return database.Ping(readyCtx)
		}),
		lifecycle.NewService("rocketmq", nil, func(context.Context) error {
			created, createErr := rocketmqadapter.NewProducer(cfg.RocketMQ, cfg.Resilience.CircuitBreaker, logger)
			if createErr != nil {
				return createErr
			}
			producer = created
			return producer.Start()
		}, func(context.Context) error {
			if producer == nil {
				return nil
			}
			return producer.Close()
		}, func(readyCtx context.Context) error {
			return producer.Ready(readyCtx)
		}),
		lifecycle.NewService("scheduler", []string{"mysql", "rocketmq"}, func(startCtx context.Context) error {
			store := mysqlscheduler.New(database.GORM(), logger)
			created, createErr := scheduler.New(startCtx, logger, store, store)
			if createErr != nil {
				return fmt.Errorf("create scheduler: %w", createErr)
			}
			jobScheduler = created
			if err := jobScheduler.Register(scheduler.Job{
				Name: "scheduler-run-retention", Schedule: "17 3 * * *", Timeout: 30 * time.Second,
				Lease: 2 * time.Minute,
				Run: func(jobCtx context.Context) error {
					return store.CleanupRuns(jobCtx, time.Now().UTC().Add(-30*24*time.Hour), 1000, 20)
				},
			}); err != nil {
				return fmt.Errorf("register scheduler retention: %w", err)
			}
			relay := rocketmqadapter.NewRelay(mysqlevent.NewOutboxStore(database.GORM()), producer, logger)
			if err := jobScheduler.Register(scheduler.Job{
				Name: "outbox-relay", Schedule: "* * * * *", Timeout: 50 * time.Second,
				Lease: 2 * time.Minute, Run: relay.Run,
			}); err != nil {
				return fmt.Errorf("register outbox relay: %w", err)
			}
			if err := jobScheduler.Register(scheduler.Job{
				Name: "event-delivery-retention", Schedule: "43 3 * * *", Timeout: 30 * time.Second,
				Lease: 2 * time.Minute, Run: relay.Cleanup,
			}); err != nil {
				return fmt.Errorf("register event delivery retention: %w", err)
			}
			// Register short, bounded, idempotent discovery and compensation jobs here.
			jobScheduler.Start()
			return nil
		}, func(context.Context) error {
			if jobScheduler == nil {
				return nil
			}
			return jobScheduler.Shutdown()
		}, nil),
	)
	if err != nil {
		return fmt.Errorf("build job lifecycle: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("start job dependencies: %w", err)
	}
	defer func() {
		registry.SetReady(false)
		runErr = errors.Join(runErr, manager.Stop(context.Background()))
	}()
	registerLifecycleReadiness(registry, manager)
	managementServer := server.New("management", cfg.Server.ManagementPort, registry.Handler(), cfg.Server, logger)
	registry.SetReady(true)
	markNotReadyWhenDone(ctx, registry)
	logger.Info("service ready")
	return managementServer.Run(ctx)
}
