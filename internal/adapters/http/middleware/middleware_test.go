package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
)

func TestWriteErrorUsesAdminAPIEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(requestIDKey, "request-1")

	WriteError(ctx, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")

	var result adminapi.Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusNotFound || result.Code != "ROUTE_NOT_FOUND" || result.Msg != "route not found" || result.Data != nil || result.RequestID != "request-1" {
		t.Fatalf("unexpected response: status=%d result=%#v", recorder.Code, result)
	}
}
