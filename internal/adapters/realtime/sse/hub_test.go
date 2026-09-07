package sse

import (
	"context"
	"strings"
	"testing"
	"time"

	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
)

func TestHubTargetsEventsAndCountsUsers(t *testing.T) {
	hub := NewHub(nil)
	clientOne, err := hub.Connect(1)
	if err != nil {
		t.Fatal(err)
	}
	clientTwo, err := hub.Connect(2)
	if err != nil {
		t.Fatal(err)
	}
	if hub.OnlineCount() != 2 {
		t.Fatalf("online count = %d", hub.OnlineCount())
	}
	hub.PublishNotice(context.Background(), noticeapp.PublishedNotice{ID: 7, Title: "targeted", Type: 1, TargetType: noticedomain.TargetSpecified, TargetUserIDs: []int64{2}, PublishTime: time.Now()})
	message := receiveMessage(t, clientTwo)
	if !strings.Contains(message, `"id":"7"`) || !strings.Contains(message, "event: notice") {
		t.Fatalf("unexpected target event: %s", message)
	}
	select {
	case message := <-clientOne.messages():
		t.Fatalf("non-target user received targeted notice: %s", message)
	default:
	}
	hub.Disconnect(clientOne)
	if hub.OnlineCount() != 1 {
		t.Fatalf("online count after disconnect = %d", hub.OnlineCount())
	}
	hub.Disconnect(clientTwo)
}

func TestDictionaryEventShape(t *testing.T) {
	hub := NewHub(nil)
	client, err := hub.Connect(1)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Disconnect(client)
	hub.PublishDictionaryChanged(context.Background(), "gender")
	chunk := receiveMessage(t, client)
	if !strings.Contains(chunk, "event: dict") || !strings.Contains(chunk, `"dictCode":"gender"`) {
		t.Fatalf("unexpected dictionary event: %s", chunk)
	}
}

func TestSlowClientDoesNotBlockOtherConnections(t *testing.T) {
	hub := NewHub(nil)
	slow, err := hub.Connect(1)
	if err != nil {
		t.Fatal(err)
	}
	fast, err := hub.Connect(2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		hub.Disconnect(slow)
		hub.Disconnect(fast)
	}()

	started := time.Now()
	for sequence := 0; sequence <= clientQueueSize; sequence++ {
		hub.broadcast("test", map[string]int{"sequence": sequence})
		message := receiveMessage(t, fast)
		if !strings.Contains(message, `"sequence":`) {
			t.Fatalf("unexpected fast-client event: %s", message)
		}
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("broadcast blocked for %s", elapsed)
	}
	if hub.UserOnline(1) {
		t.Fatal("slow client should be disconnected after its queue fills")
	}
}

func receiveMessage(t *testing.T, client *Client) string {
	t.Helper()
	select {
	case message := <-client.messages():
		return string(message)
	case <-time.After(time.Second):
		t.Fatal("message was not delivered before timeout")
		return ""
	}
}
