package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiterBackend implements a fixed-window counter using Redis.
// Uses INCR + EXPIRE pipeline so the window resets atomically.
type RedisRateLimiterBackend struct {
	rdb *redis.Client
}

// NewRedisRateLimiterBackend creates a new Redis-backed rate limiter backend.
func NewRedisRateLimiterBackend(rdb *redis.Client) *RedisRateLimiterBackend {
	return &RedisRateLimiterBackend{rdb: rdb}
}

// Allow checks whether a request from the given key should be allowed.
// Returns (allowed, remaining, retryAfter, error).
// The key should encode both the client identifier and the time window (e.g. "ip:1.2.3.4:2024-01-01T12:00").
func (r *RedisRateLimiterBackend) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	windowKey := fmt.Sprintf("rl:%s", key)

	pipe := r.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, 0, fmt.Errorf("rate limiter: pipeline failed for %q: %w", key, err)
	}

	count := int(incrCmd.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	ttl, err := r.rdb.TTL(ctx, windowKey).Result()
	if err != nil || ttl < 0 {
		ttl = window
	}

	if count > limit {
		return false, 0, ttl, nil
	}
	return true, remaining, ttl, nil
}
