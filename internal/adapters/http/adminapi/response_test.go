package adminapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOKUsesFrontendCompatibilityEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	OK(ctx, gin.H{"value": 1})

	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != CodeSuccess || result["msg"] != "成功" || result["data"] == nil {
		t.Fatalf("unexpected envelope: %#v", result)
	}
}
