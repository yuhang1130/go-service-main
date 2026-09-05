//go:build integration

package configuration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	configurationdomain "github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
)

func TestVersionedCacheRejectsStaleRefill(t *testing.T) {
	address := os.Getenv("APP_REDIS_ADDRESS")
	if address == "" {
		t.Skip("APP_REDIS_ADDRESS is not configured")
	}
	database, _ := strconv.Atoi(os.Getenv("APP_REDIS_DATABASE"))
	client := redisclient.NewClient(&redisclient.Options{Addr: address, Password: os.Getenv("APP_REDIS_PASSWORD"), DB: database})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cache := NewCache(client)
	key := "integration.versioned." + strconv.FormatInt(time.Now().UnixNano(), 10)
	item := configurationdomain.Config{ID: 7, Name: "versioned", Key: key, Value: "new"}

	_, found, staleVersion, err := cache.Get(ctx, key)
	if err != nil || found {
		t.Fatalf("initial Get() found=%v error=%v", found, err)
	}
	if err := cache.InvalidateAll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, item, staleVersion); err != nil {
		t.Fatal(err)
	}
	_, found, currentVersion, err := cache.Get(ctx, key)
	if err != nil || found {
		t.Fatalf("stale refill found=%v error=%v", found, err)
	}
	if currentVersion == staleVersion {
		t.Fatalf("cache version did not advance: %d", currentVersion)
	}

	if err := cache.Set(ctx, item, currentVersion); err != nil {
		t.Fatal(err)
	}
	got, found, _, err := cache.Get(ctx, key)
	if err != nil || !found || got.Value != item.Value {
		t.Fatalf("current refill item=%#v found=%v error=%v", got, found, err)
	}
	if err := cache.Invalidate(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, found, _, err := cache.Get(ctx, key); err != nil || found {
		t.Fatalf("invalidated Get() found=%v error=%v", found, err)
	}
}
