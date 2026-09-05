package bootstrap

import (
	"context"
	"fmt"
	rocketmqadapter "github.com/yuhang1130/go-service-main/internal/adapters/messaging/rocketmq"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	mysqlevent "github.com/yuhang1130/go-service-main/internal/adapters/mysql/eventing"
	mysqlscheduler "github.com/yuhang1130/go-service-main/internal/adapters/mysql/scheduler"
	scheduler "github.com/yuhang1130/go-service-main/internal/adapters/scheduler/gocron"
	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
	"time"
)

func RunJob(ctx context.Context) error {
	cfg := config.Defaults()
	if err := config.Load(config.Path("job"), "job", &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", "job")
	registry := health.New(buildinfo.Current())
	database, err := mysqladapter.Open(ctx, cfg.MySQL)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer database.Close()
	registry.Register("mysql", database.Ping)
	store := mysqlscheduler.New(database.GORM(), logger)
	locker, recorder := store, store
	jobScheduler, err := scheduler.New(ctx, logger, locker, recorder)
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}
	if err := jobScheduler.Register(scheduler.Job{
		Name: "scheduler-run-retention", Schedule: "17 3 * * *", Timeout: 30 * time.Second,
		Lease: 2 * time.Minute,
		Run: func(jobCtx context.Context) error {
			return store.CleanupRuns(jobCtx, time.Now().UTC().Add(-30*24*time.Hour), 1000, 20)
		},
	}); err != nil {
		return fmt.Errorf("register scheduler retention: %w", err)
	}
	producer, err := rocketmqadapter.NewProducer(cfg.RocketMQ, logger)
	if err != nil {
		return fmt.Errorf("create rocketmq producer: %w", err)
	}
	if err := producer.Start(); err != nil {
		return fmt.Errorf("start rocketmq producer: %w", err)
	}
	defer producer.Close()
	registry.Register("rocketmq", producer.Ready)
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
	defer jobScheduler.Shutdown()
	managementServer := server.New("management", cfg.Server.ManagementPort, registry.Handler(), cfg.Server, logger)
	registry.SetReady(true)
	defer registry.SetReady(false)
	logger.Info("service ready")
	return managementServer.Run(ctx)
}
