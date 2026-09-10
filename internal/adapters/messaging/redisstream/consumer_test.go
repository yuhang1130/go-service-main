package redisstream

import (
	"testing"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
)

func TestDecodeMessage(t *testing.T) {
	t.Parallel()
	message, err := decodeMessage(map[string]any{
		"dispatch_id": "dispatch-1",
		"task_type":   "material.collect",
		"task_id":     "task-1",
		"version":     "1",
	}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if message.DispatchID != "dispatch-1" || message.TaskID != "task-1" || message.Version != 1 {
		t.Fatalf("message = %#v", message)
	}
}

func TestDecodeMessageRejectsOversizedPayload(t *testing.T) {
	t.Parallel()
	_, err := decodeMessage(map[string]any{
		"dispatch_id": "dispatch-1",
		"task_type":   "material.collect",
		"task_id":     "task-1",
		"version":     "1",
		"unexpected":  "large-value",
	}, 8)
	if err == nil {
		t.Fatal("oversized message error = nil")
	}
}

func TestPhysicalStream(t *testing.T) {
	t.Parallel()
	stream, err := physicalStream("go-service-main:", ":material:collection:v1")
	if err != nil {
		t.Fatal(err)
	}
	if stream != "go-service-main:material:collection:v1" {
		t.Fatalf("stream = %q", stream)
	}
}

func TestConsumerRejectsEmptyRegistry(t *testing.T) {
	t.Parallel()
	client := redisclient.NewClient(&redisclient.Options{Addr: "127.0.0.1:6379"})
	t.Cleanup(func() { _ = client.Close() })
	_, err := NewConsumer(
		client,
		testConsumerConfig(),
		taskdispatch.NewRegistry(),
		nil,
	)
	if err == nil {
		t.Fatal("empty registry error = nil")
	}
}
