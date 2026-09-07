package sse

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type handlerBusStub struct {
	connected    chan int64
	disconnected chan int64
	disconnect   sync.Once
}

func newHandlerBusStub() *handlerBusStub {
	return &handlerBusStub{
		connected:    make(chan int64, 1),
		disconnected: make(chan int64, 1),
	}
}

func (b *handlerBusStub) UserConnected(_ context.Context, userID int64) error {
	b.connected <- userID
	return nil
}

func (b *handlerBusStub) UserDisconnected(_ context.Context, userID int64) error {
	b.disconnect.Do(func() { b.disconnected <- userID })
	return nil
}

func (*handlerBusStub) Heartbeat(context.Context, int64) error { return nil }
func (*handlerBusStub) OnlineCount(context.Context) (int, error) {
	return 0, nil
}

func TestHandlerStreamsAndDisconnectsWhenClientCloses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := NewHub(nil)
	bus := newHandlerBusStub()
	handler := NewHandler(hub, bus)
	router := gin.New()
	router.GET("/sse", func(ctx *gin.Context) {
		requestContext := auth.WithPrincipal(ctx.Request.Context(), auth.Principal{Subject: "42"})
		ctx.Request = ctx.Request.WithContext(requestContext)
		handler.connect(ctx)
	})
	server := httptest.NewServer(router)
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "text/event-stream")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("content type = %q", contentType)
	}
	if userID := receiveUserID(t, bus.connected); userID != 42 {
		t.Fatalf("connected user = %d, want 42", userID)
	}

	hub.PublishDictionaryChanged(context.Background(), "gender")
	reader := bufio.NewReader(response.Body)
	if line := readLine(t, reader); line != "event: dict\n" {
		t.Fatalf("event line = %q", line)
	}
	if line := readLine(t, reader); !strings.Contains(line, `"dictCode":"gender"`) {
		t.Fatalf("data line = %q", line)
	}
	if line := readLine(t, reader); line != "\n" {
		t.Fatalf("event terminator = %q", line)
	}

	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if userID := receiveUserID(t, bus.disconnected); userID != 42 {
		t.Fatalf("disconnected user = %d, want 42", userID)
	}
	eventually(t, func() bool { return hub.OnlineCount() == 0 })
}

func readLine(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	resultChannel := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		resultChannel <- result{line: line, err: err}
	}()
	select {
	case received := <-resultChannel:
		if received.err != nil {
			t.Fatal(received.err)
		}
		return received.line
	case <-time.After(time.Second):
		t.Fatal("SSE line was not received before timeout")
		return ""
	}
}

func receiveUserID(t *testing.T, values <-chan int64) int64 {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(time.Second):
		t.Fatal("SSE lifecycle callback was not received before timeout")
		return 0
	}
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}
