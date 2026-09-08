package bootstrap

import (
	"context"
	"log/slog"

	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/lifecycle"
)

func newLifecycleManager(logger *slog.Logger, cfg config.Role, services ...lifecycle.Service) (*lifecycle.Manager, error) {
	return lifecycle.NewManager(logger, lifecycle.Options{
		StartTimeout: cfg.Server.ShutdownTimeout,
		StopTimeout:  cfg.Server.ShutdownTimeout,
	}, services...)
}

func registerLifecycleReadiness(registry *health.Registry, manager *lifecycle.Manager) {
	registry.RegisterAll(manager.ReadinessChecks())
}

func markNotReadyWhenDone(ctx context.Context, registry *health.Registry) {
	go func() {
		<-ctx.Done()
		registry.SetReady(false)
	}()
}
