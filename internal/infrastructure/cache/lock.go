package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// luaRelease atomically releases a lock only if its value matches the token.
// This prevents a caller from releasing a lock that was acquired by another caller
// after the original TTL expired.
var luaRelease = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)

// RedisDistributedLock implements distributed mutual exclusion using Redis SETNX.
type RedisDistributedLock struct {
	rdb *redis.Client
}

// NewRedisDistributedLock creates a new Redis-backed distributed lock.
func NewRedisDistributedLock(rdb *redis.Client) *RedisDistributedLock {
	return &RedisDistributedLock{rdb: rdb}
}

// Acquire attempts to acquire the lock. Returns a unique token to pass to Release.
// Returns an error if the lock is already held by another caller.
func (l *RedisDistributedLock) Acquire(ctx context.Context, key string, ttl time.Duration) (string, error) {
	token := uuid.NewString()
	lockKey := "lock:" + key

	ok, err := l.rdb.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return "", fmt.Errorf("lock: failed to acquire %q: %w", key, err)
	}
	if !ok {
		return "", fmt.Errorf("lock: %q is already held", key)
	}
	return token, nil
}

// Release releases the lock if the token matches. Safe to call even if the lock expired.
func (l *RedisDistributedLock) Release(ctx context.Context, key string, token string) error {
	lockKey := "lock:" + key
	if err := luaRelease.Run(ctx, l.rdb, []string{lockKey}, token).Err(); err != nil && err != redis.Nil {
		return fmt.Errorf("lock: failed to release %q: %w", key, err)
	}
	return nil
}
