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
	"github.com/yuhang1130/go-service-main/internal/features/identity/domain"
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
	store := NewStore(client.Inner(), identityConfig)
	accountID, err := strconv.ParseInt(strconv.FormatInt(time.Now().UnixNano(), 10)[8:], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.InvalidateUser(ctx, accountID) })

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
	if _, err := store.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("refresh token should be one-time, got %v", err)
	}
	if err := store.RevokeAccess(ctx, second.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccountID(ctx, second.AccessToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("logout should revoke access token, got %v", err)
	}

	concurrent, err := store.Create(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	var refreshed domain.TokenPair
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
	if refreshErr != nil && !errors.Is(refreshErr, ErrSessionNotFound) {
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
