package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	redisadapter "github.com/yuhang1130/go-service-main/internal/adapters/redis"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/lifecycle"
)

type apiInfrastructure struct {
	database *mysqladapter.Database
	redis    *redisadapter.Client
	manager  *lifecycle.Manager
}

func startAPIInfrastructure(
	ctx context.Context,
	cfg config.Role,
	logger *slog.Logger,
	registry *health.Registry,
) (*apiInfrastructure, error) {
	infrastructure := &apiInfrastructure{}
	manager, err := newLifecycleManager(logger, cfg,
		infrastructure.mysqlService(cfg.MySQL),
		infrastructure.redisService(cfg.Redis),
	)
	if err != nil {
		return nil, fmt.Errorf("build api lifecycle: %w", err)
	}
	infrastructure.manager = manager
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("start api dependencies: %w", err)
	}
	registerLifecycleReadiness(registry, manager)
	return infrastructure, nil
}

func (i *apiInfrastructure) mysqlService(cfg config.MySQL) lifecycle.Service {
	return lifecycle.NewService("mysql", nil,
		func(ctx context.Context) error {
			database, err := mysqladapter.Open(ctx, cfg)
			if err != nil {
				return err
			}
			i.database = database
			return nil
		},
		func(context.Context) error {
			if i.database == nil {
				return nil
			}
			return i.database.Close()
		},
		func(ctx context.Context) error {
			return i.database.Ping(ctx)
		},
	)
}

func (i *apiInfrastructure) redisService(cfg config.Redis) lifecycle.Service {
	return lifecycle.NewService("redis", nil,
		func(ctx context.Context) error {
			client := redisadapter.Open(cfg)
			if err := client.Ping(ctx); err != nil {
				_ = client.Close()
				return err
			}
			i.redis = client
			return nil
		},
		func(context.Context) error {
			if i.redis == nil {
				return nil
			}
			return i.redis.Close()
		},
		func(ctx context.Context) error {
			return i.redis.Ping(ctx)
		},
	)
}

func (i *apiInfrastructure) stop(ctx context.Context) error {
	if i == nil || i.manager == nil {
		return nil
	}
	return i.manager.Stop(ctx)
}
