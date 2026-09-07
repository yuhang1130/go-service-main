package http

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	accesshttp "github.com/yuhang1130/go-service-main/internal/adapters/http/accesscontrol"
	audithttp "github.com/yuhang1130/go-service-main/internal/adapters/http/audit"
	configurationhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/configuration"
	dictionaryhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/dictionary"
	filehttp "github.com/yuhang1130/go-service-main/internal/adapters/http/filemanagement"
	identityhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/identity"
	noticehttp "github.com/yuhang1130/go-service-main/internal/adapters/http/notice"
	organizationhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/organization"
	ssehttp "github.com/yuhang1130/go-service-main/internal/adapters/realtime/sse"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestOpenAPICoversEveryApplicationRoute(t *testing.T) {
	t.Parallel()
	document, err := openapi3.NewLoader().LoadFromFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	specification := make(map[string]struct{})
	for path, item := range document.Paths.Map() {
		for method := range item.Operations() {
			specification[strings.ToUpper(method)+" "+normalizeRoutePath(path)] = struct{}{}
		}
	}

	identity := identityhttp.NewHandler(nil, nil)
	files := filehttp.NewHandler(nil, "")
	router := NewRouter(config.Defaults(), slog.New(slog.NewTextHandler(io.Discard, nil)), RouteSet{
		Verifier: auth.UnconfiguredVerifier{},
		Public:   []PublicRoutes{identity, files},
		Protected: []ProtectedRoutes{
			identity,
			accesshttp.NewHandler(nil),
			organizationhttp.NewHandler(nil),
			dictionaryhttp.NewHandler(nil),
			configurationhttp.NewHandler(nil),
			noticehttp.NewHandler(nil),
			audithttp.NewHandler(nil),
			files,
			ssehttp.NewHandler(nil, nil),
		},
	})
	implementation := make(map[string]struct{})
	for _, route := range router.Routes() {
		implementation[route.Method+" "+normalizeRoutePath(route.Path)] = struct{}{}
	}

	for route := range implementation {
		if _, documented := specification[route]; !documented {
			t.Errorf("implemented route is missing from OpenAPI: %s", route)
		}
	}
	for route := range specification {
		if _, implemented := implementation[route]; !implemented {
			t.Errorf("OpenAPI route is not implemented: %s", route)
		}
	}
}

func normalizeRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, ":") ||
			strings.HasPrefix(part, "*") ||
			(strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")) {
			parts[index] = "{parameter}"
		}
	}
	return strings.Join(parts, "/")
}
