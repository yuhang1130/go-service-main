package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	httpadapter "github.com/yuhang1130/go-service-main/internal/adapters/http"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type apiComponents struct {
	identityAccess identityAccessAPI
	administration administrationAPI
}

func wireAPIComponents(
	ctx context.Context,
	infrastructure *apiInfrastructure,
	cfg config.Role,
	logger *slog.Logger,
) (apiComponents, error) {
	identityAccess := wireIdentityAccessAPI(infrastructure.database.GORM(), infrastructure.redis.Inner(), cfg.Identity)
	created, err := identityAccess.identity.Bootstrap(ctx, cfg.Identity.BootstrapUser, cfg.Identity.BootstrapPass)
	if err != nil {
		return apiComponents{}, fmt.Errorf("bootstrap identity: %w", err)
	}
	if created {
		logger.Info("bootstrap administrator created")
	} else if strings.TrimSpace(cfg.Identity.BootstrapUser) != "" {
		logger.Info("bootstrap administrator skipped", "reason", "active user already exists")
	}

	administration, err := wireAdministrationAPI(
		ctx,
		infrastructure.database.GORM(),
		infrastructure.redis.Inner(),
		cfg.FileStorage,
		logger,
	)
	if err != nil {
		return apiComponents{}, fmt.Errorf("wire administration api: %w", err)
	}
	return apiComponents{
		identityAccess: identityAccess,
		administration: administration,
	}, nil
}

func (c apiComponents) routes() httpadapter.RouteSet {
	return httpadapter.RouteSet{
		Verifier: c.identityAccess.verifier,
		Audit:    c.administration.audit,
		Public: []httpadapter.PublicRoutes{
			c.identityAccess.identityHTTP,
			c.administration.files,
		},
		Protected: []httpadapter.ProtectedRoutes{
			c.identityAccess.identityHTTP,
			c.identityAccess.accessHTTP,
			c.identityAccess.organization,
			c.administration.dictionary,
			c.administration.configuration,
			c.administration.notice,
			c.administration.auditHTTP,
			c.administration.files,
			c.administration.realtimeHTTP,
		},
	}
}

func (c apiComponents) close() {
	c.administration.close()
}
