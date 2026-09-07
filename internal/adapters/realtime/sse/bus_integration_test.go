//go:build integration

package sse

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
)

func TestRedisBusDeliversAcrossInstances(t *testing.T) {
	address := os.Getenv("APP_REDIS_ADDRESS")
	if address == "" {
		t.Skip("APP_REDIS_ADDRESS is not configured")
	}
	database, _ := strconv.Atoi(os.Getenv("APP_REDIS_DATABASE"))
	options := &redisclient.Options{Addr: address, Password: os.Getenv("APP_REDIS_PASSWORD"), DB: database}
	firstClient, secondClient := redisclient.NewClient(options), redisclient.NewClient(options)
	defer firstClient.Close()
	defer secondClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstHub, secondHub := NewHub(nil), NewHub(nil)
	firstBus, err := NewBus(ctx, firstClient, firstHub, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer firstBus.Close()
	secondBus, err := NewBus(ctx, secondClient, secondHub, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer secondBus.Close()

	connection, err := secondHub.Connect(991001)
	if err != nil {
		t.Fatal(err)
	}
	defer secondHub.Disconnect(connection)
	if err := secondBus.UserConnected(ctx, 991001); err != nil {
		t.Fatal(err)
	}
	firstBus.PublishDictionaryChanged(ctx, "integration_dict")

	var received bytes.Buffer
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case message := <-connection.messages():
			received.Write(message)
		default:
		}
		if strings.Contains(received.String(), `"dictCode":"integration_dict"`) {
			count, err := firstBus.OnlineCount(ctx)
			if err != nil || count < 1 {
				t.Fatalf("online count=%d err=%v", count, err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("cross-instance event not received: %s", received.String())
}
