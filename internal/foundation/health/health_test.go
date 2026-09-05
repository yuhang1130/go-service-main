package health

import (
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
