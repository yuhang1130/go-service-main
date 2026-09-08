package bootstrap

import (
	"context"
	"errors"
	"fmt"

	rocketmqadapter "github.com/yuhang1130/go-service-main/internal/adapters/messaging/rocketmq"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/eventing"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/lifecycle"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
)

func RunConsumer(ctx context.Context) (runErr error) {
	cfg := config.Defaults()
	if err := config.Load(config.Path("consumer"), "consumer", &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", "consumer")
	registry := health.New(buildinfo.Current())
	var database *mysqladapter.Database
	var consumer *rocketmqadapter.Consumer
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
		lifecycle.NewService("rocketmq", []string{"mysql"}, func(context.Context) error {
			eventRegistry := eventing.NewRegistry()
			if err := registerEventHandlers(eventRegistry, database.GORM(), cfg.RocketMQ.ConsumerGroup); err != nil {
				return fmt.Errorf("register event handlers: %w", err)
			}
			if err := requireEventHandlers(eventRegistry); err != nil {
				return err
			}
			created, createErr := rocketmqadapter.NewConsumer(cfg.RocketMQ, eventRegistry, logger)
			if createErr != nil {
				return fmt.Errorf("create rocketmq consumer: %w", createErr)
			}
			consumer = created
			return consumer.Start()
		}, func(context.Context) error {
			if consumer == nil {
				return nil
			}
			return consumer.Close()
		}, func(readyCtx context.Context) error {
			if consumer == nil {
				return nil
			}
			return consumer.Ready(readyCtx)
		}),
	)
	if err != nil {
		return fmt.Errorf("build consumer lifecycle: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("start consumer dependencies: %w", err)
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
