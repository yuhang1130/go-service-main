package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
)

func TestManagementHandlerReportsRoutes(t *testing.T) {
	t.Parallel()
	handler := New(buildinfo.Info{}).Handler()
	routes, ok := handler.(interface{ Routes() []string })
	if !ok {
		t.Fatal("management handler does not report routes")
	}

	want := []string{"GET /livez", "GET /readyz", "GET /buildinfo"}
	if got := routes.Routes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Routes() = %v, want %v", got, want)
	}
}

func TestReadyRunsDependencyChecksAndReportsStableFailureName(t *testing.T) {
	t.Parallel()
	registry := New(buildinfo.Info{})
	registry.Register("redis", func(context.Context) error { return errors.New("unavailable") })
	registry.Register("mysql", func(context.Context) error { return errors.New("unavailable") })
	registry.SetReady(true)

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, request)

	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusServiceUnavailable || response["dependency"] != "mysql" {
		t.Fatalf("status=%d response=%v, want deterministic mysql failure", recorder.Code, response)
	}
}

func TestReadyDoesNotHoldRegistryLockWhileChecking(t *testing.T) {
	t.Parallel()
	registry := New(buildinfo.Info{})
	registry.Register("mysql", func(context.Context) error {
		registry.Register("redis", func(context.Context) error { return nil })
		return nil
	})
	registry.SetReady(true)

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
