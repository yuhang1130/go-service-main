//go:build integration

package redisstream

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
)

type claimOnceHandler struct {
	mu       sync.Mutex
	claimed  map[string]struct{}
	executed chan string
}

func newClaimOnceHandler() *claimOnceHandler {
	return &claimOnceHandler{claimed: make(map[string]struct{}), executed: make(chan string, 10)}
}

func (h *claimOnceHandler) Claim(_ context.Context, message taskdispatch.Message) (taskdispatch.ClaimResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.claimed[message.TaskID]; exists {
		return taskdispatch.ClaimIgnored, nil
	}
	h.claimed[message.TaskID] = struct{}{}
	return taskdispatch.ClaimAcquired, nil
}

func (h *claimOnceHandler) Execute(_ context.Context, message taskdispatch.Message) error {
	h.executed <- message.TaskID
	return nil
}

func TestConsumerExecutesDuplicateTaskOnlyOnce(t *testing.T) {
	client := openIntegrationRedis(t)
	cfg := integrationConsumerConfig("duplicate")
	registry, handler := integrationRegistry(t)
	consumer, err := NewConsumer(client, cfg, registry, integrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = consumer.Close(ctx)
	})
	producer, err := NewProducer(
		client,
		cfg,
		config.Defaults().Resilience.CircuitBreaker,
		integrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	taskID := "task-" + uuid.NewString()
	for attempt := 0; attempt < 2; attempt++ {
		message := taskdispatch.Message{
			DispatchID: uuid.NewString(), TaskType: "test.task",
			TaskID: taskID, Version: 1,
		}
		if err := producer.Publish(context.Background(), cfg.Stream, message); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case got := <-handler.executed:
		if got != taskID {
			t.Fatalf("executed task = %q, want %q", got, taskID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("task was not executed")
	}
	waitPendingEmpty(t, client, cfg)
	select {
	case duplicate := <-handler.executed:
		t.Fatalf("duplicate execution = %q", duplicate)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestConsumerReclaimsPendingMessage(t *testing.T) {
	client := openIntegrationRedis(t)
	cfg := integrationConsumerConfig("reclaim")
	stream, err := physicalStream(cfg.StreamPrefix, cfg.Stream)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.XGroupCreateMkStream(context.Background(), stream, cfg.ConsumerGroup, "0").Err(); err != nil {
		t.Fatal(err)
	}
	messageID, err := client.XAdd(context.Background(), &redisclient.XAddArgs{
		Stream: stream,
		Values: []string{
			"dispatch_id", uuid.NewString(),
			"task_type", "test.task",
			"task_id", "task-" + uuid.NewString(),
			"version", "1",
		},
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	streams, err := client.XReadGroup(context.Background(), &redisclient.XReadGroupArgs{
		Group: cfg.ConsumerGroup, Consumer: "dead-consumer",
		Streams: []string{stream, ">"}, Count: 1,
	}).Result()
	if err != nil || len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("seed pending message = %#v, %v", streams, err)
	}
	if streams[0].Messages[0].ID != messageID {
		t.Fatalf("pending message id = %q, want %q", streams[0].Messages[0].ID, messageID)
	}
	time.Sleep(25 * time.Millisecond)

	registry, handler := integrationRegistry(t)
	consumer, err := NewConsumer(client, cfg, registry, integrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = consumer.Close(ctx)
	})
	select {
	case <-handler.executed:
	case <-time.After(3 * time.Second):
		t.Fatal("pending task was not reclaimed")
	}
	waitPendingEmpty(t, client, cfg)
}

func TestConsumerRecreatesGroupAfterStreamLoss(t *testing.T) {
	client := openIntegrationRedis(t)
	cfg := integrationConsumerConfig("stream-loss")
	registry, handler := integrationRegistry(t)
	consumer, err := NewConsumer(client, cfg, registry, integrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = consumer.Close(ctx)
	})
	stream, err := physicalStream(cfg.StreamPrefix, cfg.Stream)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Del(context.Background(), stream).Err(); err != nil {
		t.Fatal(err)
	}
	message := taskdispatch.Message{
		DispatchID: uuid.NewString(), TaskType: "test.task",
		TaskID: "task-" + uuid.NewString(), Version: 1,
	}
	producer, err := NewProducer(
		client,
		cfg,
		config.Defaults().Resilience.CircuitBreaker,
		integrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := producer.Publish(context.Background(), cfg.Stream, message); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-handler.executed:
		if got != message.TaskID {
			t.Fatalf("executed task = %q, want %q", got, message.TaskID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("task was not consumed after stream recreation")
	}
	waitPendingEmpty(t, client, cfg)
}

func integrationRegistry(t *testing.T) (*taskdispatch.Registry, *claimOnceHandler) {
	t.Helper()
	registry := taskdispatch.NewRegistry()
	handler := newClaimOnceHandler()
	if err := registry.Register("test.task", 1, handler); err != nil {
		t.Fatal(err)
	}
	return registry, handler
}

func integrationConsumerConfig(suffix string) config.TaskDispatch {
	return config.TaskDispatch{
		StreamPrefix:  "integration-" + uuid.NewString(),
		Stream:        "tasks:" + suffix,
		ConsumerGroup: "integration-" + suffix,
		ReadBlock:     50 * time.Millisecond, HandlerTimeout: time.Second,
		ReclaimInterval: 10 * time.Millisecond, ClaimMinIdle: 10 * time.Millisecond,
		Concurrency: 1, MaxMessageBytes: 4096,
	}
}

func openIntegrationRedis(t *testing.T) *redisclient.Client {
	t.Helper()
	address := os.Getenv("APP_REDIS_ADDRESS")
	if address == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_REDIS_ADDRESS is required in CI")
		}
		t.Skip("APP_REDIS_ADDRESS is not set")
	}
	database, _ := strconv.Atoi(os.Getenv("APP_REDIS_DATABASE"))
	client := redisclient.NewClient(&redisclient.Options{
		Addr: address, Password: os.Getenv("APP_REDIS_PASSWORD"), DB: database,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func waitPendingEmpty(t *testing.T, client *redisclient.Client, cfg config.TaskDispatch) {
	t.Helper()
	stream, err := physicalStream(cfg.StreamPrefix, cfg.Stream)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Del(context.Background(), stream) })
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		pending, err := client.XPending(context.Background(), stream, cfg.ConsumerGroup).Result()
		if err == nil && pending.Count == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("consumer group still has pending messages")
}

func integrationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
