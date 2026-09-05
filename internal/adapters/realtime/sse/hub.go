package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
)

const (
	clientQueueSize = 64
	writeTimeout    = 10 * time.Second
)

var ErrSlowClient = errors.New("SSE client queue is full")

const (
	topicDictionary   = "dict"
	topicOnlineUsers  = "online-users"
	topicNotice       = "notice"
	topicNoticeRevoke = "notice-revoke"
)

type Client struct {
	userID  int64
	writer  http.ResponseWriter
	flusher http.Flusher
	queue   chan []byte
	done    chan struct{}
	close   sync.Once
	closed  atomic.Bool
}

func newClient(userID int64, writer http.ResponseWriter) (*Client, error) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, errors.New("response writer does not support streaming")
	}
	client := &Client{userID: userID, writer: writer, flusher: flusher, queue: make(chan []byte, clientQueueSize), done: make(chan struct{})}
	go client.writeLoop()
	return client, nil
}

func (c *Client) send(event string, payload any) error {
	value, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.enqueue([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, value)))
}

func (c *Client) heartbeat() error {
	return c.enqueue([]byte(": heartbeat\n\n"))
}

func (c *Client) enqueue(message []byte) error {
	if c.closed.Load() {
		return errors.New("SSE connection is closed")
	}
	select {
	case <-c.done:
		return errors.New("SSE connection is closed")
	case c.queue <- message:
		return nil
	default:
		return ErrSlowClient
	}
}

func (c *Client) writeLoop() {
	controller := http.NewResponseController(c.writer)
	for {
		select {
		case <-c.done:
			return
		case message := <-c.queue:
			_ = controller.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := c.writer.Write(message); err != nil {
				c.Close()
				return
			}
			c.flusher.Flush()
		}
	}
}

func (c *Client) Close() {
	c.close.Do(func() {
		c.closed.Store(true)
		close(c.done)
		_ = http.NewResponseController(c.writer).SetWriteDeadline(time.Now())
	})
}
func (c *Client) Done() <-chan struct{} { return c.done }

type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
	logger  *slog.Logger
	closed  bool
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{clients: make(map[int64]map[*Client]struct{}), logger: logger}
}

func (h *Hub) Connect(userID int64, writer http.ResponseWriter) (*Client, error) {
	client, err := newClient(userID, writer)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil, errors.New("SSE hub is closed")
	}
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*Client]struct{})
	}
	h.clients[userID][client] = struct{}{}
	h.mu.Unlock()
	return client, nil
}

func (h *Hub) Disconnect(client *Client) {
	if client == nil {
		return
	}
	client.Close()
	h.mu.Lock()
	connections := h.clients[client.userID]
	delete(connections, client)
	if len(connections) == 0 {
		delete(h.clients, client.userID)
	}
	h.mu.Unlock()
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) UserOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

func (h *Hub) PublishDictionaryChanged(_ context.Context, dictCode string) {
	if dictCode == "" {
		return
	}
	h.broadcast(topicDictionary, map[string]any{"dictCode": dictCode, "timestamp": time.Now().UnixMilli()})
}

func (h *Hub) PublishNotice(_ context.Context, event noticeapp.PublishedNotice) {
	payload := map[string]any{"id": strconv.FormatInt(event.ID, 10), "title": event.Title, "type": event.Type, "publishTime": event.PublishTime}
	if event.TargetType == noticedomain.TargetSpecified {
		h.sendToUsers(event.TargetUserIDs, topicNotice, payload)
		return
	}
	h.broadcast(topicNotice, payload)
}

func (h *Hub) RevokeNotice(_ context.Context, event noticeapp.RevokedNotice) {
	payload := map[string]any{"id": strconv.FormatInt(event.ID, 10)}
	if event.TargetType == noticedomain.TargetSpecified {
		h.sendToUsers(event.TargetUserIDs, topicNoticeRevoke, payload)
		return
	}
	h.broadcast(topicNoticeRevoke, payload)
}

func (h *Hub) broadcast(event string, payload any) {
	h.dispatch(h.snapshot(nil), event, payload)
}

func (h *Hub) sendToUsers(userIDs []int64, event string, payload any) {
	h.dispatch(h.snapshot(userIDs), event, payload)
}

func (h *Hub) snapshot(userIDs []int64) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*Client, 0)
	if len(userIDs) == 0 {
		for _, connections := range h.clients {
			for client := range connections {
				clients = append(clients, client)
			}
		}
		return clients
	}
	for _, userID := range userIDs {
		for client := range h.clients[userID] {
			clients = append(clients, client)
		}
	}
	return clients
}

func (h *Hub) dispatch(clients []*Client, event string, payload any) {
	for _, client := range clients {
		if err := client.send(event, payload); err != nil {
			if h.logger != nil {
				h.logger.Warn("SSE delivery failed", "event", event, "user_id", client.userID, "error", err)
			}
			h.Disconnect(client)
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	clients := make([]*Client, 0)
	for _, connections := range h.clients {
		for client := range connections {
			clients = append(clients, client)
		}
	}
	h.clients = make(map[int64]map[*Client]struct{})
	h.mu.Unlock()
	for _, client := range clients {
		client.Close()
	}
}
