package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisTokenBlacklist stores revoked JWT JTIs in Redis with TTL.
type RedisTokenBlacklist struct {
	rdb *redis.Client
}

// NewRedisTokenBlacklist creates a new Redis-backed token blacklist.
func NewRedisTokenBlacklist(rdb *redis.Client) *RedisTokenBlacklist {
	return &RedisTokenBlacklist{rdb: rdb}
}

// Add marks the given JTI as revoked for the specified duration.
func (b *RedisTokenBlacklist) Add(ctx context.Context, jti string, ttl time.Duration) error {
	key := "jti:" + jti
	if err := b.rdb.Set(ctx, key, 1, ttl).Err(); err != nil {
		return fmt.Errorf("blacklist: failed to add JTI %s: %w", jti, err)
	}
	return nil
}

// IsBlacklisted returns true if the JTI has been revoked.
func (b *RedisTokenBlacklist) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := "jti:" + jti
	n, err := b.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("blacklist: failed to check JTI %s: %w", jti, err)
	}
	return n > 0, nil
}
