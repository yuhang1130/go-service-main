package apperror

import "testing"

func TestCodesAreStableAndSemantic(t *testing.T) {
	codes := map[string]string{
		"internal":              CodeInternal,
		"invalid argument":      CodeInvalidArgument,
		"not found":             CodeNotFound,
		"conflict":              CodeConflict,
		"too many requests":     CodeTooManyRequests,
		"invalid credentials":   CodeInvalidCredentials,
		"invalid access token":  CodeInvalidAccessToken,
		"invalid refresh token": CodeInvalidRefreshToken,
		"permission denied":     CodePermissionDenied,
		"forbidden":             CodeForbidden,
		"route not found":       CodeRouteNotFound,
	}
	want := map[string]string{
		"internal":              "INTERNAL_ERROR",
		"invalid argument":      "INVALID_ARGUMENT",
		"not found":             "NOT_FOUND",
		"conflict":              "CONFLICT",
		"too many requests":     "TOO_MANY_REQUESTS",
		"invalid credentials":   "INVALID_CREDENTIALS",
		"invalid access token":  "INVALID_ACCESS_TOKEN",
		"invalid refresh token": "INVALID_REFRESH_TOKEN",
		"permission denied":     "PERMISSION_DENIED",
		"forbidden":             "FORBIDDEN",
		"route not found":       "ROUTE_NOT_FOUND",
	}
	for name, code := range codes {
		if code != want[name] {
			t.Errorf("%s code = %q, want %q", name, code, want[name])
		}
	}
}
