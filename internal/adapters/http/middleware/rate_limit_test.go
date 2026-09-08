package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestRateLimitSeparatesClientsAndReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewClientRateLimiter(config.HTTPRateLimit{
		RequestsPerSecond: 1,
		Burst:             1,
		ClientTTL:         time.Minute,
	})
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(RequestID(), RateLimit(limiter))
	router.GET("/test", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	first := requestFrom(t, router, "192.0.2.1:1234")
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", first.Code)
	}
	second := requestFrom(t, router, "192.0.2.1:1234")
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("second status=%d headers=%v body=%s", second.Code, second.Header(), second.Body.String())
	}
	otherClient := requestFrom(t, router, "192.0.2.2:1234")
	if otherClient.Code != http.StatusNoContent {
		t.Fatalf("other client status = %d", otherClient.Code)
	}
}

func requestFrom(t *testing.T, handler http.Handler, remoteAddress string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.RemoteAddr = remoteAddress
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
