package redisstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
)

type Consumer struct {
	client   *redisclient.Client
	cfg      config.TaskDispatch
	registry *taskdispatch.Registry
	logger   *slog.Logger
	stream   string
	name     string

	ready  atomic.Bool
	cancel context.CancelFunc
	work   chan redisclient.XMessage
	done   chan struct{}
	wg     sync.WaitGroup
}

func NewConsumer(
	client *redisclient.Client,
	cfg config.TaskDispatch,
	registry *taskdispatch.Registry,
	logger *slog.Logger,
) (*Consumer, error) {
	if client == nil || registry == nil {
		return nil, fmt.Errorf("redis client and task handler registry are required")
	}
	if registry.Count() == 0 {
		return nil, fmt.Errorf("no task handlers registered")
	}
	if strings.TrimSpace(cfg.ConsumerGroup) == "" ||
		cfg.Concurrency <= 0 ||
		cfg.ReadBlock <= 0 ||
		cfg.HandlerTimeout <= 0 ||
		cfg.ReclaimInterval <= 0 ||
		cfg.ClaimMinIdle <= 0 ||
		cfg.MaxMessageBytes <= 0 {
		return nil, fmt.Errorf("invalid redis stream consumer configuration")
	}
	stream, err := physicalStream(cfg.StreamPrefix, cfg.Stream)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	name := strings.TrimSpace(cfg.ConsumerName)
	if name == "" {
		hostname, hostErr := os.Hostname()
		if hostErr != nil || strings.TrimSpace(hostname) == "" {
			hostname = "consumer"
		}
		name = hostname + "-" + uuid.NewString()
	}
	return &Consumer{
		client: client, cfg: cfg, registry: registry, logger: logger,
		stream: stream, name: name,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.work = make(chan redisclient.XMessage)
	c.done = make(chan struct{})
	c.ready.Store(true)

	c.wg.Add(c.cfg.Concurrency + 2)
	for range c.cfg.Concurrency {
		go c.worker(runCtx)
	}
	go c.readNew(runCtx)
	go c.reclaimPending(runCtx)
	go func() {
		c.wg.Wait()
		close(c.done)
	}()
	return nil
}

func (c *Consumer) Close(ctx context.Context) error {
	c.ready.Store(false)
	if c.cancel == nil {
		return nil
	}
	c.cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Consumer) Ready(context.Context) error {
	if !c.ready.Load() {
		return fmt.Errorf("redis stream consumer is not ready")
	}
	return nil
}

func (c *Consumer) readNew(ctx context.Context) {
	defer c.wg.Done()
	for {
		streams, err := c.client.XReadGroup(ctx, &redisclient.XReadGroupArgs{
			Group: c.cfg.ConsumerGroup, Consumer: c.name,
			Streams: []string{c.stream, ">"}, Count: 1, Block: c.cfg.ReadBlock,
		}).Result()
		if err != nil {
			if errors.Is(err, redisclient.Nil) {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			if redisclient.HasErrorPrefix(err, "NOGROUP") {
				if groupErr := c.ensureGroup(ctx); groupErr != nil {
					c.logger.Error("redis stream consumer group recovery failed", "stream", c.stream, "error", groupErr)
					if !waitContext(ctx, time.Second) {
						return
					}
				}
				continue
			}
			c.logger.Error("redis stream read failed", "stream", c.stream, "error", err)
			if !waitContext(ctx, time.Second) {
				return
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				if !c.sendWork(ctx, message) {
					return
				}
			}
		}
	}
}

func (c *Consumer) reclaimPending(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.ReclaimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reclaim(ctx)
		}
	}
}

func (c *Consumer) reclaim(ctx context.Context) {
	start := "0-0"
	for {
		messages, next, err := c.client.XAutoClaim(ctx, &redisclient.XAutoClaimArgs{
			Stream: c.stream, Group: c.cfg.ConsumerGroup, Consumer: c.name,
			MinIdle: c.cfg.ClaimMinIdle, Start: start, Count: 1,
		}).Result()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, redisclient.Nil) {
				return
			}
			if redisclient.HasErrorPrefix(err, "NOGROUP") {
				if groupErr := c.ensureGroup(ctx); groupErr != nil {
					c.logger.Error("redis stream consumer group recovery failed", "stream", c.stream, "error", groupErr)
				}
				return
			}
			if ctx.Err() == nil {
				c.logger.Error("redis stream pending reclaim failed", "stream", c.stream, "error", err)
			}
			return
		}
		for _, message := range messages {
			if !c.sendWork(ctx, message) {
				return
			}
		}
		if len(messages) == 0 || next == "0-0" {
			return
		}
		start = next
	}
}

func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, c.stream, c.cfg.ConsumerGroup, "0").Err()
	if err != nil && !redisclient.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("create redis stream consumer group: %w", err)
	}
	return nil
}

func (c *Consumer) sendWork(ctx context.Context, message redisclient.XMessage) bool {
	select {
	case c.work <- message:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Consumer) worker(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-c.work:
			c.handle(ctx, message)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, streamMessage redisclient.XMessage) {
	message, err := decodeMessage(streamMessage.Values, c.cfg.MaxMessageBytes)
	if err != nil {
		c.logger.Error(
			"invalid redis stream task dispatch",
			"message_id", streamMessage.ID,
			"stream", c.stream,
			"error", err,
		)
		return
	}
	handler, exists := c.registry.Handler(message.TaskType, message.Version)
	if !exists {
		c.logger.Error(
			"no handler for redis stream task dispatch",
			"message_id", streamMessage.ID,
			"dispatch_id", message.DispatchID,
			"task_type", message.TaskType,
			"version", message.Version,
		)
		return
	}

	claimTimeout := min(c.cfg.HandlerTimeout, 30*time.Second)
	claimCtx, cancelClaim := context.WithTimeout(ctx, claimTimeout)
	result, claimErr := handler.Claim(claimCtx, message)
	cancelClaim()
	if claimErr != nil {
		c.logger.Error(
			"task dispatch claim failed",
			"message_id", streamMessage.ID,
			"dispatch_id", message.DispatchID,
			"task_type", message.TaskType,
			"error", claimErr,
		)
		return
	}
	if result != taskdispatch.ClaimAcquired && result != taskdispatch.ClaimIgnored {
		c.logger.Error(
			"task dispatch returned invalid claim result",
			"message_id", streamMessage.ID,
			"dispatch_id", message.DispatchID,
			"task_type", message.TaskType,
			"result", result,
		)
		return
	}
	if err := c.ack(ctx, streamMessage.ID); err != nil {
		c.logger.Error(
			"redis stream acknowledgement failed",
			"message_id", streamMessage.ID,
			"dispatch_id", message.DispatchID,
			"task_type", message.TaskType,
			"error", err,
		)
	}
	if result == taskdispatch.ClaimIgnored {
		return
	}

	handlerCtx, cancelHandler := context.WithTimeout(ctx, c.cfg.HandlerTimeout)
	defer cancelHandler()
	if err := handler.Execute(handlerCtx, message); err != nil {
		c.logger.Error(
			"task dispatch execution failed",
			"dispatch_id", message.DispatchID,
			"task_type", message.TaskType,
			"task_id", message.TaskID,
			"error", err,
		)
	}
}

func (c *Consumer) ack(ctx context.Context, messageID string) error {
	acknowledged, err := c.client.XAck(ctx, c.stream, c.cfg.ConsumerGroup, messageID).Result()
	if err != nil {
		return err
	}
	if acknowledged != 1 {
		return fmt.Errorf("acknowledged %d entries, want 1", acknowledged)
	}
	return nil
}

func decodeMessage(values map[string]any, maxBytes int) (taskdispatch.Message, error) {
	size := 0
	for key, value := range values {
		size += len(key) + len(fmt.Sprint(value))
	}
	if size > maxBytes {
		return taskdispatch.Message{}, fmt.Errorf("message exceeds %d bytes", maxBytes)
	}
	version, err := strconv.Atoi(valueString(values["version"]))
	if err != nil {
		return taskdispatch.Message{}, fmt.Errorf("invalid task version")
	}
	message := taskdispatch.Message{
		DispatchID: valueString(values["dispatch_id"]),
		TaskType:   valueString(values["task_type"]),
		TaskID:     valueString(values["task_id"]),
		Version:    version,
	}
	if err := message.Validate(); err != nil {
		return taskdispatch.Message{}, err
	}
	return message, nil
}

func valueString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(value)
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
