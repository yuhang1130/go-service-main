//go:build integration

package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	redisadapter "github.com/yuhang1130/go-service-main/internal/adapters/redis"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func TestStoreRotatesAndRevokesSessionsAndCaptchas(t *testing.T) {
	address := os.Getenv("APP_REDIS_ADDRESS")
	if address == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_REDIS_ADDRESS is required in CI")
		}
		t.Skip("APP_REDIS_ADDRESS is not set")
	}
	ctx := context.Background()
	redisConfig := config.Defaults().Redis
	redisConfig.Address = address
	redisConfig.Password = os.Getenv("APP_REDIS_PASSWORD")
	if rawDatabase := os.Getenv("APP_REDIS_DATABASE"); rawDatabase != "" {
		database, err := strconv.Atoi(rawDatabase)
		if err != nil {
			t.Fatalf("APP_REDIS_DATABASE: %v", err)
		}
		redisConfig.Database = database
	}
	client := redisadapter.Open(redisConfig)
	defer client.Close()
	if err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	identityConfig := config.Defaults().Identity
	identityConfig.AccessTokenTTL = time.Minute
	identityConfig.RefreshTokenTTL = 2 * time.Minute
	identityConfig.LoginRateLimit = 2
	identityConfig.LoginRateWindow = time.Minute
	store := NewStore(client.Inner(), identityConfig)
	accountID, err := strconv.ParseInt(strconv.FormatInt(time.Now().UnixNano(), 10)[8:], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.InvalidateUser(ctx, accountID) })
	loginClientID := "integration-" + strconv.FormatInt(accountID, 10)
	t.Cleanup(func() { _ = client.Inner().Del(ctx, loginRateKey(loginClientID)).Err() })
	for attempt := 0; attempt < identityConfig.LoginRateLimit; attempt++ {
		if retryAfter, err := store.AllowLogin(ctx, loginClientID); err != nil || retryAfter != 0 {
			t.Fatalf("allowed login attempt %d returned retry %s, error %v", attempt+1, retryAfter, err)
		}
	}
	if retryAfter, err := store.AllowLogin(ctx, loginClientID); err != nil || retryAfter <= 0 {
		t.Fatalf("rate-limited login returned retry %s, error %v", retryAfter, err)
	}

	first, err := store.Create(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.AccountID(ctx, first.AccessToken); err != nil || got != accountID {
		t.Fatalf("account = %d, err = %v", got, err)
	}
	second, err := store.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccountID(ctx, first.AccessToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old access token should be revoked, got %v", err)
	}
	if _, err := store.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("refresh token reuse should be detected, got %v", err)
	}
	if _, err := store.AccountID(ctx, second.AccessToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("refresh token reuse should revoke the session family, got %v", err)
	}

	logoutPair, err := store.Create(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeAccess(ctx, logoutPair.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccountID(ctx, logoutPair.AccessToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("logout should revoke access token, got %v", err)
	}

	racing, err := store.Create(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	racingStart := make(chan struct{})
	racingPairs := make([]identityapp.TokenPair, 2)
	racingErrors := make([]error, 2)
	var racingWait sync.WaitGroup
	for index := range racingErrors {
		racingWait.Add(1)
		go func(index int) {
			defer racingWait.Done()
			<-racingStart
			racingPairs[index], racingErrors[index] = store.Refresh(ctx, racing.RefreshToken)
		}(index)
	}
	close(racingStart)
	racingWait.Wait()
	successes := 0
	reuses := 0
	var racedPair identityapp.TokenPair
	for index, refreshErr := range racingErrors {
		switch {
		case refreshErr == nil:
			successes++
			racedPair = racingPairs[index]
		case errors.Is(refreshErr, ErrRefreshTokenReused):
			reuses++
		default:
			t.Fatalf("concurrent refresh returned unexpected error: %v", refreshErr)
		}
	}
	if successes != 1 || reuses != 1 {
		t.Fatalf("concurrent refresh results = %d success, %d reuse; want one of each", successes, reuses)
	}
	if _, err := store.AccountID(ctx, racedPair.AccessToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("concurrent token reuse should revoke the winning session family, got %v", err)
	}

	concurrent, err := store.Create(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	var refreshed identityapp.TokenPair
	var refreshErr, invalidateErr error
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		refreshed, refreshErr = store.Refresh(ctx, concurrent.RefreshToken)
	}()
	go func() {
		defer wait.Done()
		<-start
		invalidateErr = store.InvalidateUser(ctx, accountID)
	}()
	close(start)
	wait.Wait()
	if invalidateErr != nil {
		t.Fatal(invalidateErr)
	}
	if refreshErr != nil && !errors.Is(refreshErr, ErrSessionNotFound) && !errors.Is(refreshErr, ErrRefreshTokenReused) {
		t.Fatalf("concurrent refresh returned unexpected error: %v", refreshErr)
	}
	if refreshErr == nil {
		if _, err := store.AccountID(ctx, refreshed.AccessToken); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("invalidation must revoke a concurrently refreshed session, got %v", err)
		}
	}

	captcha, err := store.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded := strings.TrimPrefix(captcha.Image, "data:image/svg+xml;base64,")
	svg, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`>([0-9]{4})</text>`).FindSubmatch(svg)
	if len(match) != 2 {
		t.Fatalf("captcha code was not present in SVG")
	}
	valid, err := store.Verify(ctx, captcha.ID, string(match[1]))
	if err != nil || !valid {
		t.Fatalf("captcha valid = %v, err = %v", valid, err)
	}
	valid, err = store.Verify(ctx, captcha.ID, string(match[1]))
	if err != nil || valid {
		t.Fatalf("captcha must be one-time: valid = %v, err = %v", valid, err)
	}
}
