package sse

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
)

const redisChannel = "go-service-main:sse:v1"
const presenceKey = "go-service-main:sse:presence:v1"
const presenceTTL = 45 * time.Second

type envelope struct {
	Origin  string          `json:"origin"`
	Event   string          `json:"event"`
	UserIDs []int64         `json:"userIds,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

type Bus struct {
	client     *redisclient.Client
	hub        *Hub
	pubsub     *redisclient.PubSub
	logger     *slog.Logger
	close      sync.Once
	instanceID string
}

func NewBus(ctx context.Context, client *redisclient.Client, hub *Hub, logger *slog.Logger) (*Bus, error) {
	pubsub := client.Subscribe(ctx, redisChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}
	bus := &Bus{client: client, hub: hub, pubsub: pubsub, logger: logger, instanceID: uuid.NewString()}
	go bus.consume(ctx)
	return bus, nil
}

func (b *Bus) UserConnected(ctx context.Context, userID int64) error {
	if err := b.touch(ctx, userID); err != nil {
		return err
	}
	return b.publishOnlineCount(ctx)
}

func (b *Bus) UserDisconnected(ctx context.Context, userID int64) error {
	if err := b.client.ZRem(ctx, presenceKey, b.member(userID)).Err(); err != nil {
		return err
	}
	return b.publishOnlineCount(ctx)
}

func (b *Bus) Heartbeat(ctx context.Context, userID int64) error { return b.touch(ctx, userID) }

func (b *Bus) OnlineCount(ctx context.Context) (int, error) {
	now := time.Now().UnixMilli()
	pipeline := b.client.TxPipeline()
	pipeline.ZRemRangeByScore(ctx, presenceKey, "-inf", strconv.FormatInt(now, 10))
	members := pipeline.ZRangeByScore(ctx, presenceKey, &redisclient.ZRangeBy{Min: strconv.FormatInt(now+1, 10), Max: "+inf"})
	if _, err := pipeline.Exec(ctx); err != nil {
		return 0, err
	}
	users := make(map[string]struct{})
	for _, member := range members.Val() {
		if separator := strings.LastIndexByte(member, ':'); separator >= 0 && separator+1 < len(member) {
			users[member[separator+1:]] = struct{}{}
		}
	}
	return len(users), nil
}

func (b *Bus) touch(ctx context.Context, userID int64) error {
	expiresAt := time.Now().Add(presenceTTL).UnixMilli()
	return b.client.ZAdd(ctx, presenceKey, redisclient.Z{Score: float64(expiresAt), Member: b.member(userID)}).Err()
}

func (b *Bus) member(userID int64) string { return b.instanceID + ":" + strconv.FormatInt(userID, 10) }

func (b *Bus) publishOnlineCount(ctx context.Context) error {
	count, err := b.OnlineCount(ctx)
	if err != nil {
		return err
	}
	b.publish(ctx, topicOnlineUsers, nil, count)
	return nil
}

func (b *Bus) PublishDictionaryChanged(ctx context.Context, dictCode string) {
	if dictCode == "" {
		return
	}
	b.publish(ctx, topicDictionary, nil, map[string]any{"dictCode": dictCode, "timestamp": time.Now().UnixMilli()})
}

func (b *Bus) PublishNotice(ctx context.Context, event noticeapp.PublishedNotice) {
	var userIDs []int64
	if event.TargetType == noticedomain.TargetSpecified {
		userIDs = event.TargetUserIDs
	}
	b.publish(ctx, topicNotice, userIDs, map[string]any{"id": strconv.FormatInt(event.ID, 10), "title": event.Title, "type": event.Type, "publishTime": event.PublishTime})
}

func (b *Bus) RevokeNotice(ctx context.Context, event noticeapp.RevokedNotice) {
	var userIDs []int64
	if event.TargetType == noticedomain.TargetSpecified {
		userIDs = event.TargetUserIDs
	}
	b.publish(ctx, topicNoticeRevoke, userIDs, map[string]any{"id": strconv.FormatInt(event.ID, 10)})
}

func (b *Bus) publish(ctx context.Context, event string, userIDs []int64, payload any) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return
	}
	localEvent := envelope{Origin: b.instanceID, Event: event, UserIDs: userIDs, Payload: rawPayload}
	b.deliver(localEvent)
	message, err := json.Marshal(localEvent)
	if err != nil {
		return
	}
	if err := b.client.Publish(ctx, redisChannel, message).Err(); err != nil {
		if b.logger != nil {
			b.logger.Warn("SSE event publish failed", "event", event, "error", err)
		}
	}
}

func (b *Bus) consume(ctx context.Context) {
	channel := b.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-channel:
			if !ok {
				return
			}
			var event envelope
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil || event.Event == "" || len(event.Payload) == 0 {
				if err != nil && b.logger != nil {
					b.logger.Warn("invalid SSE bus event", "error", err)
				}
				continue
			}
			if event.Origin != b.instanceID {
				b.deliver(event)
			}
		}
	}
}

func (b *Bus) deliver(event envelope) {
	if len(event.UserIDs) > 0 {
		b.hub.sendToUsers(event.UserIDs, event.Event, event.Payload)
		return
	}
	b.hub.broadcast(event.Event, event.Payload)
}

func (b *Bus) Close() {
	b.close.Do(func() {
		_ = b.pubsub.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var cursor uint64
		values := make([]any, 0)
		for {
			members, next, err := b.client.ZScan(ctx, presenceKey, cursor, b.instanceID+":*", 100).Result()
			if err != nil {
				break
			}
			for index := 0; index+1 < len(members); index += 2 {
				values = append(values, members[index])
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		if len(values) > 0 {
			_ = b.client.ZRem(ctx, presenceKey, values...).Err()
			_ = b.publishOnlineCount(ctx)
		}
	})
}
