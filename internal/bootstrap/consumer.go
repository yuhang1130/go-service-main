package bootstrap

import (
	"context"
	"fmt"
	rocketmqadapter "github.com/yuhang1130/go-service-main/internal/adapters/messaging/rocketmq"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/eventing"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
)

func RunConsumer(ctx context.Context) error {
	cfg := config.Defaults()
	if err := config.Load(config.Path("consumer"), "consumer", &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", "consumer")
	registry := health.New(buildinfo.Current())
	database, err := mysqladapter.Open(ctx, cfg.MySQL)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer database.Close()
	registry.Register("mysql", database.Ping)
	eventRegistry := eventing.NewRegistry()
	if err := registerEventHandlers(eventRegistry, database.GORM(), cfg.RocketMQ.ConsumerGroup); err != nil {
		return fmt.Errorf("register event handlers: %w", err)
	}
	if eventRegistry.Count() == 0 {
		logger.Info("no event handlers registered; consumer running idle")
	} else {
		consumer, err := rocketmqadapter.NewConsumer(cfg.RocketMQ, eventRegistry, logger)
		if err != nil {
			return fmt.Errorf("create rocketmq consumer: %w", err)
		}
		if err := consumer.Start(); err != nil {
			return fmt.Errorf("start rocketmq consumer: %w", err)
		}
		defer consumer.Close()
		registry.Register("rocketmq", consumer.Ready)
	}
	managementServer := server.New("management", cfg.Server.ManagementPort, registry.Handler(), cfg.Server, logger)
	registry.SetReady(true)
	defer registry.SetReady(false)
	logger.Info("service ready")
	return managementServer.Run(ctx)
}
