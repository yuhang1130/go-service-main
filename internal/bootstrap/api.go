package bootstrap

import (
	"context"
	"fmt"
	"strings"

	httpadapter "github.com/yuhang1130/go-service-main/internal/adapters/http"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	redisadapter "github.com/yuhang1130/go-service-main/internal/adapters/redis"
	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/health"
	"github.com/yuhang1130/go-service-main/internal/foundation/logging"
	"github.com/yuhang1130/go-service-main/internal/foundation/server"
)

func RunAPI(ctx context.Context) error {
	cfg := config.Defaults()
	if err := config.Load(config.Path("api"), "api", &cfg); err != nil {
		return err
	}
	logger := logging.New(cfg.Logging).With("service", "go-service-main", "role", "api")
	registry := health.New(buildinfo.Current())
	database, err := mysqladapter.Open(ctx, cfg.MySQL)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer database.Close()
	registry.Register("mysql", database.Ping)
	redis := redisadapter.Open(cfg.Redis)
	defer redis.Close()
	if err := redis.Ping(ctx); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	registry.Register("redis", redis.Ping)
	identityAccess := wireIdentityAccessAPI(database.GORM(), redis.Inner(), cfg.Identity)
	bootstrapCreated, err := identityAccess.identity.Bootstrap(ctx, cfg.Identity.BootstrapUser, cfg.Identity.BootstrapPass)
	if err != nil {
		return fmt.Errorf("bootstrap identity: %w", err)
	}
	if bootstrapCreated {
		logger.Info("bootstrap administrator created")
	} else if strings.TrimSpace(cfg.Identity.BootstrapUser) != "" {
		logger.Info("bootstrap administrator skipped", "reason", "active user already exists")
	}
	administration, err := wireAdministrationAPI(ctx, database.GORM(), redis.Inner(), cfg.FileStorage, logger)
	if err != nil {
		return fmt.Errorf("wire administration api: %w", err)
	}
	defer administration.close()
	routes := httpadapter.RouteSet{
		Verifier:  identityAccess.verifier,
		Audit:     administration.audit,
		Public:    []httpadapter.PublicRoutes{identityAccess.identityHTTP, administration.files},
		Protected: []httpadapter.ProtectedRoutes{identityAccess.identityHTTP, identityAccess.accessHTTP, identityAccess.organization, administration.dictionary, administration.configuration, administration.notice, administration.auditHTTP, administration.files, administration.realtimeHTTP},
	}
	router := httpadapter.NewRouter(cfg, logger, routes)
	applicationServer := server.New("api", cfg.Server.HTTPPort, router, cfg.Server, logger)
	applicationServer.OnShutdown(administration.stopRealtime)
	managementServer := server.New("management", cfg.Server.ManagementPort, registry.Handler(), cfg.Server, logger)
	registry.SetReady(true)
	defer registry.SetReady(false)
	logger.Info("service ready")
	return server.RunAll(ctx, applicationServer, managementServer)
}
