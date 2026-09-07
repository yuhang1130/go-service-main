package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

func TestOKUsesFrontendCompatibilityEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("request_id", "request-1")
	OK(ctx, gin.H{"value": 1})

	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != CodeSuccess || result["msg"] != "成功" || result["data"] == nil || result["requestId"] != "request-1" {
		t.Fatalf("unexpected envelope: %#v", result)
	}
}

func TestResponseHelpersAlwaysUseTheSameEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		write      func(*gin.Context)
		wantStatus int
		wantCode   string
		wantMsg    string
		wantData   any
	}{
		{name: "message", write: func(ctx *gin.Context) { OKMessage(ctx, "保存成功") }, wantStatus: http.StatusOK, wantCode: CodeSuccess, wantMsg: "保存成功"},
		{name: "page", write: func(ctx *gin.Context) { Page(ctx, []string{"one"}, 1) }, wantStatus: http.StatusOK, wantCode: CodeSuccess, wantMsg: "成功", wantData: map[string]any{"list": []any{"one"}, "total": float64(1)}},
		{name: "error", write: func(ctx *gin.Context) {
			Error(ctx, apperror.InvalidArgument("A0400", "参数无效", errors.New("detail")))
		}, wantStatus: http.StatusBadRequest, wantCode: "A0400", wantMsg: "参数无效"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("request_id", "request-1")
			test.write(ctx)

			var result map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != test.wantStatus || result["code"] != test.wantCode || result["msg"] != test.wantMsg {
				t.Fatalf("status/envelope = %d/%#v", recorder.Code, result)
			}
			if _, exists := result["data"]; !exists {
				t.Fatalf("data field is missing: %#v", result)
			}
			if result["requestId"] != "request-1" {
				t.Fatalf("requestId = %#v, want request-1", result["requestId"])
			}
			if test.wantData != nil {
				page := result["data"].(map[string]any)
				if page["total"] != float64(1) || len(page["list"].([]any)) != 1 {
					t.Fatalf("unexpected page data: %#v", page)
				}
			}
		})
	}
}
