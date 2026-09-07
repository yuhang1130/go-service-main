package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type verifierStub struct {
	principal *auth.Principal
	err       error
}

func (v verifierStub) VerifyAccessToken(context.Context, string) (auth.Principal, error) {
	if v.err != nil {
		return auth.Principal{}, v.err
	}
	if v.principal != nil {
		return *v.principal, nil
	}
	return auth.Principal{Subject: "1", System: true}, nil
}

func TestAuthenticateUsesAdminAPIExpiredTokenCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Authenticate(verifierStub{err: apperror.Unauthorized(apperror.CodeInvalidAccessToken, "expired")}))
	router.GET("/protected", func(ctx *gin.Context) { adminapi.OK(ctx, nil) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer expired")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	var result adminapi.Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != apperror.CodeInvalidAccessToken {
		t.Fatalf("code = %q, want %s", result.Code, apperror.CodeInvalidAccessToken)
	}
}

func TestSystemPrincipalBypassesNamedPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Authenticate(verifierStub{}), RequirePermission("sys:anything"))
	router.GET("/protected", func(ctx *gin.Context) { adminapi.OK(ctx, nil) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer valid")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestRequirePermissionUsesPermissionDeniedCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	principal := auth.Principal{Subject: "1", Permissions: map[string]struct{}{}}
	router.Use(Authenticate(verifierStub{principal: &principal}), RequirePermission("sys:missing"))
	router.GET("/protected", func(ctx *gin.Context) { adminapi.OK(ctx, nil) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer valid")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	var result adminapi.Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != apperror.CodePermissionDenied {
		t.Fatalf("code = %q, want %s", result.Code, apperror.CodePermissionDenied)
	}
}
