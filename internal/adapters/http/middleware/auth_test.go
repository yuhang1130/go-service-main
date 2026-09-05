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

type verifierStub struct{ err error }

func (v verifierStub) VerifyAccessToken(context.Context, string) (auth.Principal, error) {
	if v.err != nil {
		return auth.Principal{}, v.err
	}
	return auth.Principal{Subject: "1", System: true}, nil
}

func TestAuthenticateUsesAdminAPIExpiredTokenCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Authenticate(verifierStub{err: apperror.Unauthorized("A0230", "expired")}))
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
	if result.Code != "A0230" {
		t.Fatalf("code = %q, want A0230", result.Code)
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
