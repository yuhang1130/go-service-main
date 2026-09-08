package bootstrap

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	httpadapter "github.com/yuhang1130/go-service-main/internal/adapters/http"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
)

func serveAPI(
	ctx context.Context,
	cfg config.Role,
	logger *slog.Logger,
	registry *health.Registry,
	components apiComponents,
	startedAt time.Time,
) error {
	router := httpadapter.NewRouter(cfg, logger, components.routes())
	logger.Info("api routes registered", "count", len(router.Routes()))
	applicationServer := server.New("api", cfg.Server.HTTPPort, router, cfg.Server, logger)
	applicationServer.OnShutdown(components.administration.stopRealtime)
	managementServer := server.New("management", cfg.Server.ManagementPort, registry.Handler(), cfg.Server, logger)

	var listeningServers atomic.Int32
	onServerReady := func() {
		if listeningServers.Add(1) != 2 || ctx.Err() != nil {
			return
		}
		registry.SetReady(true)
		logger.Info("service ready", "startup_duration_ms", time.Since(startedAt).Milliseconds())
	}
	applicationServer.OnReady(onServerReady)
	managementServer.OnReady(onServerReady)
	markNotReadyWhenDone(ctx, registry)
	return server.RunAll(ctx, applicationServer, managementServer)
}
