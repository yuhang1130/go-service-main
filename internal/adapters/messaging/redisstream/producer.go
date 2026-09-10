package redisstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/resilience/circuitbreaker"
	"github.com/yuhang1130/go-service-main/internal/foundation/taskdispatch"
)

type Producer struct {
	client   *redisclient.Client
	prefix   string
	maxBytes int
	logger   *slog.Logger
	breaker  *circuitbreaker.Breaker
	ready    atomic.Bool
}

func NewProducer(
	client *redisclient.Client,
	cfg config.TaskDispatch,
	breakerConfig config.CircuitBreaker,
	logger *slog.Logger,
) (*Producer, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if strings.TrimSpace(cfg.StreamPrefix) == "" || cfg.MaxMessageBytes <= 0 {
		return nil, fmt.Errorf("task dispatch stream prefix and max message bytes are required")
	}
	breaker, err := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold:    breakerConfig.FailureThreshold,
		OpenTimeout:         breakerConfig.OpenTimeout,
		HalfOpenMaxRequests: breakerConfig.HalfOpenMaxRequests,
	})
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	producer := &Producer{
		client: client, prefix: cfg.StreamPrefix, maxBytes: cfg.MaxMessageBytes,
		logger: logger, breaker: breaker,
	}
	producer.ready.Store(true)
	return producer, nil
}

func (p *Producer) Ready(ctx context.Context) error {
	if !p.ready.Load() {
		return fmt.Errorf("redis stream producer is not ready")
	}
	return p.breaker.Ready(ctx)
}

func (p *Producer) Publish(ctx context.Context, logicalStream string, message taskdispatch.Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(body) > p.maxBytes {
		return fmt.Errorf("task dispatch %s exceeds %d bytes", message.DispatchID, p.maxBytes)
	}
	stream, err := physicalStream(p.prefix, logicalStream)
	if err != nil {
		return err
	}
	err = p.breaker.Execute(ctx, func(sendCtx context.Context) error {
		return p.client.XAdd(sendCtx, &redisclient.XAddArgs{
			Stream: stream,
			Values: []string{
				"dispatch_id", message.DispatchID,
				"task_type", message.TaskType,
				"task_id", message.TaskID,
				"version", fmt.Sprint(message.Version),
			},
		}).Err()
	})
	if err != nil {
		p.ready.Store(false)
		return err
	}
	p.ready.Store(true)
	p.logger.Info(
		"task dispatch added to redis stream",
		"dispatch_id", message.DispatchID,
		"task_type", message.TaskType,
		"stream", stream,
	)
	return nil
}

func physicalStream(prefix, logical string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), ":")
	logical = strings.Trim(strings.TrimSpace(logical), ":")
	if prefix == "" || logical == "" {
		return "", fmt.Errorf("stream prefix and logical stream are required")
	}
	return prefix + ":" + logical, nil
}
