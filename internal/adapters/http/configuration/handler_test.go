package configuration

import (
	"testing"

	"github.com/gin-gonic/gin"
	configurationapp "github.com/yuhang1130/go-service-main/internal/features/configuration/application"
)

func TestRouteRegistrationDoesNotConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(configurationapp.NewService(nil, nil)).RegisterProtected(router.Group("/api/v1"))
}
