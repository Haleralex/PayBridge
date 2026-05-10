// Package ports defines interfaces for external dependencies.
package ports

import (
	"context"
	"time"
)

// TokenBlacklist manages revoked JWT tokens.
// Used for implementing logout: a token's JTI is added to the blacklist
// and checked on each request.
type TokenBlacklist interface {
	// Add marks a token as revoked. ttl should match the token's remaining lifetime.
	Add(ctx context.Context, jti string, ttl time.Duration) error
	// IsBlacklisted returns true if the token has been revoked.
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// DistributedLock provides mutual exclusion across multiple instances.
// Used to prevent race conditions in idempotency checks.
type DistributedLock interface {
	// Acquire obtains a lock for the given key. Returns a unique token that
	// must be passed to Release. Returns an error if the lock cannot be acquired.
	Acquire(ctx context.Context, key string, ttl time.Duration) (token string, err error)
	// Release releases the lock. The token must match the one returned by Acquire
	// to prevent releasing a lock held by another caller.
	Release(ctx context.Context, key string, token string) error
}
