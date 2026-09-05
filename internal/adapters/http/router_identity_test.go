package http

import (
	"context"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	accesshttp "github.com/yuhang1130/go-service-main/internal/adapters/http/accesscontrol"
	configurationhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/configuration"
	dictionaryhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/dictionary"
	identityhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/identity"
	organizationhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/organization"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type fixedVerifier struct{ principal auth.Principal }

func (v fixedVerifier) VerifyAccessToken(context.Context, string) (auth.Principal, error) {
	return v.principal, nil
}

func TestIdentityAccessRoutesRegisterAndRequireAuthentication(t *testing.T) {
	identityHandler := identityhttp.NewHandler(nil)
	router := NewRouter(config.Defaults(), slog.New(slog.NewTextHandler(io.Discard, nil)), RouteSet{
		Verifier:  auth.UnconfiguredVerifier{},
		Public:    []PublicRoutes{identityHandler},
		Protected: []ProtectedRoutes{identityHandler, accesshttp.NewHandler(nil), organizationhttp.NewHandler(nil)},
	})

	wantRoutes := map[string]bool{
		"GET /api/v1/auth/captcha":    false,
		"POST /api/v1/auth/login":     false,
		"GET /api/v1/users/me":        false,
		"GET /api/v1/menus/routes":    false,
		"GET /api/v1/depts/options":   false,
		"PUT /api/v1/roles/:id/menus": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wantRoutes[key]; ok {
			wantRoutes[key] = true
		}
	}
	for route, found := range wantRoutes {
		if !found {
			t.Errorf("route %s was not registered", route)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestAdministrativeReadRoutesRequirePermissions(t *testing.T) {
	router := NewRouter(config.Defaults(), slog.New(slog.NewTextHandler(io.Discard, nil)), RouteSet{
		Verifier: fixedVerifier{principal: auth.Principal{Subject: "42", Permissions: map[string]struct{}{}}},
		Protected: []ProtectedRoutes{
			identityhttp.NewHandler(nil),
			accesshttp.NewHandler(nil),
			organizationhttp.NewHandler(nil),
			configurationhttp.NewHandler(nil),
			dictionaryhttp.NewHandler(nil),
		},
	})

	paths := []string{
		"/api/v1/users",
		"/api/v1/users/template",
		"/api/v1/roles",
		"/api/v1/roles/1/dept-ids",
		"/api/v1/menus",
		"/api/v1/depts",
		"/api/v1/configs/key/internal.setting",
		"/api/v1/configs/1",
		"/api/v1/configs/1/form",
		"/api/v1/dicts/1/form",
		"/api/v1/dicts/system_status/items",
		"/api/v1/dicts/system_status/items/1/form",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer test-token")
			router.ServeHTTP(recorder, request)
			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
