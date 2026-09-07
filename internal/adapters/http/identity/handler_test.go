package identity

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type handlerSessionsStub struct {
	identityapp.Sessions
	refreshToken string
	tokens       identitydomain.TokenPair
	invalidated  int64
}

func (s *handlerSessionsStub) Refresh(_ context.Context, token string) (identitydomain.TokenPair, error) {
	s.refreshToken = token
	return s.tokens, nil
}

func (s *handlerSessionsStub) InvalidateUser(_ context.Context, accountID int64) error {
	s.invalidated = accountID
	return nil
}

type handlerRepositoryStub struct {
	identityapp.Repository
	passwordID int64
	hash       string
	actorID    int64
	writes     int
}

func (r *handlerRepositoryStub) SetPassword(_ context.Context, id int64, hash string, actorID int64) error {
	r.passwordID = id
	r.hash = hash
	r.actorID = actorID
	r.writes++
	return nil
}

type handlerPasswordStub struct{ identityapp.PasswordHasher }

func (handlerPasswordStub) Hash(string) (string, error) { return "password-hash", nil }

type handlerLoginLimiterStub struct {
	retryAfter time.Duration
	clientID   string
}

func (l *handlerLoginLimiterStub) AllowLogin(_ context.Context, clientID string) (time.Duration, error) {
	l.clientID = clientID
	return l.retryAfter, nil
}

func TestRefreshReadsSensitiveTokenFromJSONBody(t *testing.T) {
	sessions := &handlerSessionsStub{tokens: identitydomain.TokenPair{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 60}}
	handler := NewHandler(identityapp.NewService(nil, sessions, nil, nil, nil, ""), nil)
	router := gin.New()
	router.POST("/refresh", handler.refresh)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(`{"refreshToken":"sensitive-refresh-token"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	if sessions.refreshToken != "sensitive-refresh-token" {
		t.Fatalf("refresh token = %q, want JSON body value", sessions.refreshToken)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/refresh?refreshToken=query-secret", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("query-only status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
	if sessions.refreshToken == "query-secret" {
		t.Fatal("refresh token must not be read from the URL query")
	}
}

func TestResetPasswordReadsSensitiveValueFromJSONBody(t *testing.T) {
	repository := &handlerRepositoryStub{}
	sessions := &handlerSessionsStub{}
	handler := NewHandler(identityapp.NewService(repository, sessions, nil, handlerPasswordStub{}, nil, ""), nil)
	router := gin.New()
	router.PUT("/users/:userId/password/reset", handler.resetPassword)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/users/42/password/reset", bytes.NewBufferString(`{"password":"password123"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(auth.WithPrincipal(request.Context(), auth.Principal{Subject: "7"}))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.passwordID != 42 || repository.hash != "password-hash" || repository.actorID != 7 {
		t.Fatalf("password write = id %d, hash %q, actor %d", repository.passwordID, repository.hash, repository.actorID)
	}
	if sessions.invalidated != 42 {
		t.Fatalf("invalidated account = %d, want 42", sessions.invalidated)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/users/42/password/reset?password=query-secret", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("query-only status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.writes != 1 {
		t.Fatalf("password writes = %d, query parameter must not trigger another write", repository.writes)
	}
}

func TestLoginRateLimitReturnsRetryAfter(t *testing.T) {
	limiter := &handlerLoginLimiterStub{retryAfter: 90 * time.Second}
	handler := NewHandler(nil, limiter)
	router := gin.New()
	router.POST("/login", handler.login)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"admin","password":"password123","captchaId":"captcha","captchaCode":"2468"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Retry-After") != "90" {
		t.Fatalf("Retry-After = %q, want 90", recorder.Header().Get("Retry-After"))
	}
	if limiter.clientID == "" {
		t.Fatal("login limiter did not receive the direct client IP")
	}
}
