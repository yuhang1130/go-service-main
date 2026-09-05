package rocketmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"github.com/yuhang1130/go-service-main/internal/foundation/eventing"
)

type Producer struct {
	inner    rmq.Producer
	prefix   string
	maxBytes int
	logger   *slog.Logger
	ready    atomic.Bool
}

func NewProducer(cfg config.RocketMQ, logger *slog.Logger) (*Producer, error) {
	forceConsoleLogging()
	if err := validateCommon(cfg); err != nil {
		return nil, err
	}
	topics := make([]string, 0, len(cfg.Topics))
	for _, topic := range cfg.Topics {
		topics = append(topics, physicalTopic(cfg.TopicPrefix, topic))
	}
	inner, err := rmq.NewProducer(clientConfig(cfg), rmq.WithTopics(topics...))
	if err != nil {
		return nil, err
	}
	return &Producer{inner: inner, prefix: cfg.TopicPrefix, maxBytes: cfg.MaxBodyBytes, logger: logger}, nil
}

func (p *Producer) Start() error {
	if err := p.inner.Start(); err != nil {
		return err
	}
	p.ready.Store(true)
	return nil
}

func (p *Producer) Close() error {
	p.ready.Store(false)
	return p.inner.GracefulStop()
}

func (p *Producer) Ready(context.Context) error {
	if !p.ready.Load() {
		return fmt.Errorf("rocketmq producer is not ready")
	}
	return nil
}

func (p *Producer) Publish(ctx context.Context, logicalTopic string, event eventing.Envelope) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(body) > p.maxBytes {
		return fmt.Errorf("event %s exceeds %d bytes", event.EventID, p.maxBytes)
	}
	message := &rmq.Message{Topic: physicalTopic(p.prefix, logicalTopic), Body: body}
	message.SetKeys(event.EventID)
	if event.AggregateID != "" {
		message.SetMessageGroup(event.AggregateID)
	}
	message.AddProperty("event_type", event.EventType)
	message.AddProperty("event_version", fmt.Sprint(event.EventVersion))
	_, err = p.inner.Send(ctx, message)
	if err == nil {
		p.ready.Store(true)
		p.logger.Info("event published", "event_id", event.EventID, "event_type", event.EventType, "topic", logicalTopic)
	} else {
		p.ready.Store(false)
	}
	return err
}

type Consumer struct {
	inner  rmq.PushConsumer
	logger *slog.Logger
	ready  atomic.Bool
}

func NewConsumer(cfg config.RocketMQ, registry *eventing.Registry, logger *slog.Logger) (*Consumer, error) {
	forceConsoleLogging()
	if err := validateCommon(cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ConsumerGroup) == "" || cfg.Concurrency <= 0 || cfg.AwaitDuration <= 0 || cfg.HandlerTimeout <= 0 {
		return nil, fmt.Errorf("rocketmq consumer_group, concurrency, await_duration, and handler_timeout are required")
	}
	if registry.Count() == 0 {
		return nil, fmt.Errorf("no event handlers registered")
	}
	subscriptions := make(map[string]*rmq.FilterExpression, len(cfg.Topics))
	for _, topic := range cfg.Topics {
		subscriptions[physicalTopic(cfg.TopicPrefix, topic)] = rmq.SUB_ALL
	}
	listener := &rmq.FuncMessageListener{Consume: func(message *rmq.MessageView) rmq.ConsumerResult {
		if len(message.GetBody()) > cfg.MaxBodyBytes {
			logger.Error("invalid message body size", "message_id", message.GetMessageId(), "topic", message.GetTopic())
			return rmq.SUCCESS
		}
		var event eventing.Envelope
		if err := json.Unmarshal(message.GetBody(), &event); err != nil {
			logger.Error("invalid event envelope", "message_id", message.GetMessageId(), "topic", message.GetTopic())
			return rmq.SUCCESS
		}
		handlerCtx, cancel := context.WithTimeout(context.Background(), cfg.HandlerTimeout)
		defer cancel()
		result, err := registry.Handle(handlerCtx, event)
		if err != nil {
			logger.Error("event handling failed", "event_id", event.EventID, "event_type", event.EventType, "result", result, "error", err)
		}
		if result == eventing.RetryableFailure {
			return rmq.NewConsumerResultSuspend(5 * time.Second)
		}
		return rmq.SUCCESS
	}}
	inner, err := rmq.NewPushConsumer(clientConfig(cfg),
		rmq.WithPushSubscriptionExpressions(subscriptions),
		rmq.WithPushMessageListener(listener),
		rmq.WithPushConsumptionThreadCount(cfg.Concurrency),
		rmq.WithPushAwaitDuration(cfg.AwaitDuration),
	)
	if err != nil {
		return nil, err
	}
	return &Consumer{inner: inner, logger: logger}, nil
}

func (c *Consumer) Start() error {
	if err := c.inner.Start(); err != nil {
		return err
	}
	c.ready.Store(true)
	return nil
}

func (c *Consumer) Close() error {
	c.ready.Store(false)
	return c.inner.GracefulStop()
}

func (c *Consumer) Ready(context.Context) error {
	if !c.ready.Load() {
		return fmt.Errorf("rocketmq consumer is not ready")
	}
	return nil
}

func clientConfig(cfg config.RocketMQ) *rmq.Config {
	result := &rmq.Config{
		Endpoint:      cfg.Endpoints,
		ConsumerGroup: cfg.ConsumerGroup,
	}
	if strings.TrimSpace(cfg.AccessKey) != "" {
		result.Credentials = &credentials.SessionCredentials{AccessKey: cfg.AccessKey, AccessSecret: cfg.SecretKey}
	}
	return result
}

func validateCommon(cfg config.RocketMQ) error {
	if strings.TrimSpace(cfg.Endpoints) == "" {
		return fmt.Errorf("rocketmq endpoints are required")
	}
	if (strings.TrimSpace(cfg.AccessKey) == "") != (strings.TrimSpace(cfg.SecretKey) == "") {
		return fmt.Errorf("rocketmq access_key and secret_key must be configured together")
	}
	if strings.TrimSpace(cfg.TopicPrefix) == "" || len(cfg.Topics) == 0 || cfg.MaxBodyBytes <= 0 {
		return fmt.Errorf("rocketmq topic_prefix, topics, and max_body_bytes are required")
	}
	return nil
}

func physicalTopic(prefix, logical string) string {
	return strings.TrimSuffix(prefix, "-") + "-" + strings.TrimPrefix(logical, "-")
}

func forceConsoleLogging() {
	// The upstream SDK otherwise defaults to rotating files. Reset it before any
	// client operation so third-party diagnostics also stay on process stdout.
	_ = os.Setenv(rmq.ENABLE_CONSOLE_APPENDER, "true")
	rmq.ResetLogger()
}
