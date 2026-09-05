package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	configurationdomain "github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
)

const (
	dataPrefix = "system:config:data:"
	versionKey = "system:config:version"
	cacheTTL   = time.Hour
)

var getVersionedScript = redisclient.NewScript(`
local version = redis.call('GET', KEYS[1]) or '0'
local value = redis.call('GET', ARGV[1] .. version .. ':' .. ARGV[2])
return {version, value or false}
`)

var setIfCurrentVersionScript = redisclient.NewScript(`
local current = redis.call('GET', KEYS[1]) or '0'
if current ~= ARGV[1] then
  return 0
end
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[3])
return 1
`)

type Cache struct{ client *redisclient.Client }

func NewCache(client *redisclient.Client) *Cache { return &Cache{client: client} }

func (c *Cache) Get(ctx context.Context, key string) (configurationdomain.Config, bool, uint64, error) {
	values, err := getVersionedScript.Run(ctx, c.client, []string{versionKey}, dataPrefix, key).Slice()
	if err != nil {
		return configurationdomain.Config{}, false, 0, err
	}
	if len(values) != 2 {
		return configurationdomain.Config{}, false, 0, fmt.Errorf("unexpected configuration cache response")
	}
	version, err := strconv.ParseUint(fmt.Sprint(values[0]), 10, 64)
	if err != nil {
		return configurationdomain.Config{}, false, 0, fmt.Errorf("parse configuration cache version: %w", err)
	}
	if values[1] == nil {
		return configurationdomain.Config{}, false, version, nil
	}
	var item configurationdomain.Config
	if err := json.Unmarshal([]byte(fmt.Sprint(values[1])), &item); err != nil {
		return configurationdomain.Config{}, false, version, err
	}
	return item, true, version, nil
}

func (c *Cache) Set(ctx context.Context, item configurationdomain.Config, version uint64) error {
	value, err := json.Marshal(item)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s%d:%s", dataPrefix, version, item.Key)
	return setIfCurrentVersionScript.Run(ctx, c.client, []string{versionKey, key},
		strconv.FormatUint(version, 10), value, cacheTTL.Milliseconds(),
	).Err()
}

func (c *Cache) Invalidate(ctx context.Context, key string) error {
	return c.InvalidateAll(ctx)
}

func (c *Cache) InvalidateAll(ctx context.Context) error {
	return c.client.Incr(ctx, versionKey).Err()
}
