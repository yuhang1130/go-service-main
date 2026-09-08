package bootstrap

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
)

const startupStartedAtEnv = "GO_SERVICE_MAIN_STARTUP_STARTED_AT_MS"

func RunAPI(ctx context.Context) (runErr error) {
	startedAt := serviceStartupStartedAt(time.Now())
	cfg := config.Defaults()
	if err := config.Load(config.Path("api"), "api", &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", "api")
	registry := health.New(buildinfo.Current())
	logger.Info("api startup stage", "step", "1/4", "stage", "configuration_loaded")

	logger.Info("api startup stage", "step", "2/4", "stage", "starting_infrastructure")
	infrastructure, err := startAPIInfrastructure(ctx, cfg, logger, registry)
	if err != nil {
		return err
	}
	defer func() {
		registry.SetReady(false)
		runErr = errors.Join(runErr, infrastructure.stop(context.Background()))
	}()

	logger.Info("api startup stage", "step", "3/4", "stage", "wiring_components")
	components, err := wireAPIComponents(ctx, infrastructure, cfg, logger)
	if err != nil {
		return err
	}
	defer components.close()

	logger.Info("api startup stage", "step", "4/4", "stage", "starting_http_servers")
	return serveAPI(ctx, cfg, logger, registry, components, startedAt)
}

func serviceStartupStartedAt(now time.Time) time.Time {
	value := os.Getenv(startupStartedAtEnv)
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds <= 0 {
		return now
	}
	startedAt := time.UnixMilli(milliseconds)
	if startedAt.After(now) {
		return now
	}
	return startedAt
}
