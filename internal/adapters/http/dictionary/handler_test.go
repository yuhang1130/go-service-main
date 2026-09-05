package dictionary

import (
	"testing"

	"github.com/gin-gonic/gin"
	dictionaryapp "github.com/yuhang1130/go-service-main/internal/features/dictionary/application"
)

func TestRoutesRegisterWithoutWildcardConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(dictionaryapp.NewService(nil, nil)).RegisterProtected(router.Group("/api/v1"))
}
