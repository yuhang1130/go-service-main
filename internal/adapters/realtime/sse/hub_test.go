package sse

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
)

type testStreamWriter struct {
	mu     sync.Mutex
	header http.Header
	body   bytes.Buffer
}

func newTestStreamWriter() *testStreamWriter {
	return &testStreamWriter{header: make(http.Header)}
}

func (w *testStreamWriter) Header() http.Header { return w.header }
func (*testStreamWriter) WriteHeader(int)       {}
func (w *testStreamWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.Write(value)
}
func (*testStreamWriter) Flush() {}
func (w *testStreamWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

type blockingStreamWriter struct {
	header  http.Header
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingStreamWriter() *blockingStreamWriter {
	return &blockingStreamWriter{header: make(http.Header), started: make(chan struct{}), release: make(chan struct{})}
}

func (w *blockingStreamWriter) Header() http.Header { return w.header }
func (*blockingStreamWriter) WriteHeader(int)       {}
func (w *blockingStreamWriter) Write(value []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	return len(value), nil
}
func (*blockingStreamWriter) Flush() {}

func TestHubTargetsEventsAndCountsUsers(t *testing.T) {
	hub := NewHub(nil)
	first := newTestStreamWriter()
	second := newTestStreamWriter()
	clientOne, err := hub.Connect(1, first)
	if err != nil {
		t.Fatal(err)
	}
	clientTwo, err := hub.Connect(2, second)
	if err != nil {
		t.Fatal(err)
	}
	if hub.OnlineCount() != 2 {
		t.Fatalf("online count = %d", hub.OnlineCount())
	}
	hub.PublishNotice(context.Background(), noticeapp.PublishedNotice{ID: 7, Title: "targeted", Type: 1, TargetType: noticedomain.TargetSpecified, TargetUserIDs: []int64{2}, PublishTime: time.Now()})
	eventually(t, func() bool { return strings.Contains(second.String(), `"id":"7"`) })
	if first.String() != "" {
		t.Fatal("non-target user received targeted notice")
	}
	if !strings.Contains(second.String(), "event: notice") {
		t.Fatalf("target event missing: %s", second.String())
	}
	hub.Disconnect(clientOne)
	if hub.OnlineCount() != 1 {
		t.Fatalf("online count after disconnect = %d", hub.OnlineCount())
	}
	hub.Disconnect(clientTwo)
}

func TestDictionaryEventShape(t *testing.T) {
	hub := NewHub(nil)
	recorder := newTestStreamWriter()
	client, err := hub.Connect(1, recorder)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Disconnect(client)
	hub.PublishDictionaryChanged(context.Background(), "gender")
	eventually(t, func() bool { return strings.Contains(recorder.String(), `"dictCode":"gender"`) })
	chunk := recorder.String()
	if !strings.Contains(chunk, "event: dict") || !strings.Contains(chunk, `"dictCode":"gender"`) {
		t.Fatalf("unexpected dictionary event: %s", chunk)
	}
}

func TestSlowClientDoesNotBlockOtherConnections(t *testing.T) {
	hub := NewHub(nil)
	slowWriter := newBlockingStreamWriter()
	slow, err := hub.Connect(1, slowWriter)
	if err != nil {
		t.Fatal(err)
	}
	fastWriter := newTestStreamWriter()
	fast, err := hub.Connect(2, fastWriter)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		close(slowWriter.release)
		hub.Disconnect(slow)
		hub.Disconnect(fast)
	}()

	hub.broadcast("test", map[string]int{"sequence": 0})
	select {
	case <-slowWriter.started:
	case <-time.After(time.Second):
		t.Fatal("slow writer did not start")
	}
	started := time.Now()
	for sequence := 1; sequence <= clientQueueSize+1; sequence++ {
		hub.broadcast("test", map[string]int{"sequence": sequence})
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("broadcast blocked for %s", elapsed)
	}
	if hub.UserOnline(1) {
		t.Fatal("slow client should be disconnected after its queue fills")
	}
	eventually(t, func() bool {
		return strings.Contains(fastWriter.String(), `"sequence":65`)
	})
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
