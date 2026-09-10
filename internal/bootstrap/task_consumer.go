package bootstrap

import (
	"context"
	"errors"
	"fmt"

	redisstream "github.com/yuhang1130/go-service-main/internal/adapters/messaging/redisstream"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	redisadapter "github.com/yuhang1130/go-service-main/internal/adapters/redis"
	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/lifecycle"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
	"github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
	"gorm.io/gorm"
)

func RunCollectionConsumer(ctx context.Context) error {
	return runTaskConsumer(ctx, "collection-consumer", collectionTaskHandlerRegistrations)
}

func RunTransformationConsumer(ctx context.Context) error {
	return runTaskConsumer(ctx, "transformation-consumer", transformationTaskHandlerRegistrations)
}

func RunUploadConsumer(ctx context.Context) error {
	return runTaskConsumer(ctx, "upload-consumer", uploadTaskHandlerRegistrations)
}

func RunInfrastructureConsumer(ctx context.Context) error {
	return runTaskConsumer(ctx, "infrastructure-consumer", infrastructureTaskHandlerRegistrations)
}

func runTaskConsumer(
	ctx context.Context,
	role string,
	registrations []taskHandlerRegistration,
) (runErr error) {
	cfg := config.Defaults()
	if err := config.Load(config.Path(role), role, &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", role)
	registry := health.New(buildinfo.Current())
	var database *mysqladapter.Database
	var redis *redisadapter.Client
	var consumer *redisstream.Consumer
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
		lifecycle.NewService("redis", nil, func(startCtx context.Context) error {
			opened := redisadapter.Open(cfg.Redis)
			if err := opened.Ping(startCtx); err != nil {
				_ = opened.Close()
				return err
			}
			redis = opened
			return nil
		}, func(context.Context) error {
			if redis == nil {
				return nil
			}
			return redis.Close()
		}, func(readyCtx context.Context) error {
			return redis.Ping(readyCtx)
		}),
		lifecycle.NewService("task-dispatch", []string{"mysql", "redis"}, func(startCtx context.Context) error {
			taskRegistry := taskdispatch.NewRegistry()
			if err := registerTaskHandlers(taskRegistry, database.GORM(), registrations); err != nil {
				return fmt.Errorf("register task handlers: %w", err)
			}
			if err := requireTaskHandlers(taskRegistry); err != nil {
				return err
			}
			created, createErr := redisstream.NewConsumer(redis.Inner(), cfg.TaskDispatch, taskRegistry, logger)
			if createErr != nil {
				return fmt.Errorf("create redis stream consumer: %w", createErr)
			}
			consumer = created
			return consumer.Start(startCtx)
		}, func(stopCtx context.Context) error {
			if consumer == nil {
				return nil
			}
			return consumer.Close(stopCtx)
		}, func(readyCtx context.Context) error {
			if consumer == nil {
				return fmt.Errorf("redis stream consumer is not initialized")
			}
			return consumer.Ready(readyCtx)
		}),
	)
	if err != nil {
		return fmt.Errorf("build %s lifecycle: %w", role, err)
	}
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("start %s dependencies: %w", role, err)
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

type taskHandlerRegistration struct {
	feature  string
	register func(*taskdispatch.Registry, *gorm.DB) error
}

var (
	collectionTaskHandlerRegistrations     []taskHandlerRegistration
	transformationTaskHandlerRegistrations []taskHandlerRegistration
	uploadTaskHandlerRegistrations         []taskHandlerRegistration
	infrastructureTaskHandlerRegistrations []taskHandlerRegistration
)

func registerTaskHandlers(
	registry *taskdispatch.Registry,
	database *gorm.DB,
	registrations []taskHandlerRegistration,
) error {
	for _, registration := range registrations {
		if err := registration.register(registry, database); err != nil {
			return fmt.Errorf("register %s task handlers: %w", registration.feature, err)
		}
	}
	return nil
}

func requireTaskHandlers(registry *taskdispatch.Registry) error {
	if registry.Count() == 0 {
		return fmt.Errorf("consumer has no registered task handlers")
	}
	return nil
}
