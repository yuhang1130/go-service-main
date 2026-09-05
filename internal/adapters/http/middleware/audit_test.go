package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type captureAuditRecorder struct{ entry auditdomain.Entry }

func (r *captureAuditRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	r.entry = entry
	return nil
}

func TestClassifyOperationDoesNotCaptureSensitiveOrReadOnlyEndpoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method string
		path   string
		ok     bool
		action string
	}{
		{http.MethodGet, "/api/v1/users", true, "查询"},
		{http.MethodGet, "/api/v1/users/export", true, "导出"},
		{http.MethodPost, "/api/v1/users/import", true, "导入"},
		{http.MethodPut, "/api/v1/users/7/password/reset", true, "修改"},
		{http.MethodGet, "/api/v1/users/me", false, ""},
		{http.MethodGet, "/api/v1/notices/8/detail", false, ""},
	}
	for _, test := range tests {
		_, action, _, ok := classifyOperation(test.method, test.path)
		if ok != test.ok || action != test.action {
			t.Errorf("classifyOperation(%q, %q) = action %q, ok %v", test.method, test.path, action, ok)
		}
	}
}

func TestOperationAuditSnapshotsPrincipalName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &captureAuditRecorder{}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		principal := auth.Principal{Subject: "7", Name: "当时昵称", System: true}
		ctx.Request = ctx.Request.WithContext(auth.WithPrincipal(ctx.Request.Context(), principal))
		ctx.Next()
	})
	router.Use(OperationAudit(recorder, slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.POST("/api/v1/users", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/users", nil))
	if recorder.entry.OperatorID != 7 || recorder.entry.OperatorName != "当时昵称" {
		t.Fatalf("operator snapshot = %d/%q", recorder.entry.OperatorID, recorder.entry.OperatorName)
	}
}
