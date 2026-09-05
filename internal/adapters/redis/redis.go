package redis

import (
	"context"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type Client struct{ inner *redisclient.Client }

func Open(cfg config.Redis) *Client {
	return &Client{inner: redisclient.NewClient(&redisclient.Options{
		Addr: cfg.Address, Password: cfg.Password, DB: cfg.Database,
		DialTimeout: cfg.DialTimeout, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout,
	})}
}

func (c *Client) Ping(ctx context.Context) error { return c.inner.Ping(ctx).Err() }

func (c *Client) Inner() *redisclient.Client { return c.inner }

func (c *Client) Close() error { return c.inner.Close() }
