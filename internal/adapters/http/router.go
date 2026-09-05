package http

import (
	"log/slog"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type PublicRoutes interface {
	RegisterPublic(*gin.RouterGroup)
}

type ProtectedRoutes interface {
	RegisterProtected(*gin.RouterGroup)
}

type RouteSet struct {
	Verifier  auth.Verifier
	Audit     middleware.AuditRecorder
	Public    []PublicRoutes
	Protected []ProtectedRoutes
}

func NewRouter(cfg config.Role, logger *slog.Logger, routes RouteSet) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(
		middleware.RequestID(),
		middleware.Recovery(logger),
		middleware.AccessLog(logger),
		middleware.BodySize(cfg.Server.MaxBodyBytes),
	)
	router.NoRoute(func(ctx *gin.Context) {
		middleware.WriteError(ctx, stdhttp.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	api := router.Group("/api/v1")
	for _, registrar := range routes.Public {
		registrar.RegisterPublic(api)
	}
	protected := api.Group("")
	protected.Use(middleware.Authenticate(routes.Verifier))
	if routes.Audit != nil {
		protected.Use(middleware.OperationAudit(routes.Audit, logger))
	}
	for _, registrar := range routes.Protected {
		registrar.RegisterProtected(protected)
	}
	return router
}
