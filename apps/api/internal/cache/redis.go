package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct{ client *redis.Client }

var ErrUnavailable = errors.New("redis is unavailable")

var rateLimitScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("TTL", KEYS[1])}
`)

func New(rawURL string) *Cache {
	options, err := redis.ParseURL(rawURL)
	if err != nil || options.Addr == "" {
		return nil
	}
	return &Cache{client: redis.NewClient(options)}
}

func (cache *Cache) Close() error {
	if cache == nil || cache.client == nil {
		return nil
	}
	return cache.client.Close()
}

// Ping verifies that the configured Redis endpoint is reachable. A nil cache
// means Redis was not configured or its URL could not be parsed.
func (cache *Cache) Ping(ctx context.Context) error {
	if cache == nil || cache.client == nil {
		return ErrUnavailable
	}
	if err := cache.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

// AllowRate performs one atomic fixed-window increment in Redis. Callers can
// fall back to an in-process limiter when this returns an error.
func (cache *Cache) AllowRate(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	if cache == nil || cache.client == nil {
		return false, 0, ErrUnavailable
	}
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	result, err := rateLimitScript.Run(ctx, cache.client, []string{"card-arena:ratelimit:" + key}, int(window/time.Second)).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis rate limit: %w", err)
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("redis rate limit: unexpected response %T", result)
	}
	count, ok := redisInt64(values[0])
	if !ok {
		return false, 0, fmt.Errorf("redis rate limit: invalid count %v", values[0])
	}
	ttl, ok := redisInt64(values[1])
	if !ok || ttl < 1 {
		ttl = int64(window / time.Second)
	}
	return count <= int64(limit), int(ttl), nil
}

func redisInt64(value any) (int64, bool) {
	switch value := value.(type) {
	case int64:
		return value, true
	case uint64:
		return int64(value), true
	case string:
		var parsed int64
		_, err := fmt.Sscan(value, &parsed)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (cache *Cache) GetJSON(ctx context.Context, key string, target any) bool {
	if cache == nil || cache.client == nil {
		return false
	}
	value, err := cache.client.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	return json.Unmarshal([]byte(value), target) == nil
}

func (cache *Cache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if cache == nil || cache.client == nil {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = cache.client.Set(ctx, key, encoded, ttl).Err()
}
