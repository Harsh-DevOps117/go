// Package ratelimit provides bounded local limits and optional shared Redis limits.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type entry struct {
	count   int
	expires time.Time
}

type Limiter struct {
	mu        sync.Mutex
	entries   map[string]entry
	limit     int
	window    time.Duration
	namespace string
	redis     *redis.Client
	logger    *slog.Logger
	degraded  atomic.Bool
	nextSweep time.Time
}

func New(limit int, window time.Duration, namespace string, client *redis.Client, logger *slog.Logger) *Limiter {
	if limit <= 0 || window <= 0 {
		panic("rate limits must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Limiter{entries: make(map[string]entry), limit: limit, window: window, namespace: namespace, redis: client, logger: logger}
}

var increment = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return {count, redis.call('PTTL', KEYS[1])}
`)

func (l *Limiter) Allow(ctx context.Context, key string) (bool, time.Duration) {
	// Always maintain the local budget, so a Redis outage cannot reset it.
	allowed, retry := l.local(key, time.Now())
	if !allowed || l.redis == nil {
		return allowed, retry
	}
	redisCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	digest := sha256.Sum256([]byte(key))
	result, err := increment.Run(redisCtx, l.redis, []string{fmt.Sprintf("noteflow:limit:%s:%x", l.namespace, digest)}, l.window.Milliseconds()).Int64Slice()
	if err != nil {
		if !l.degraded.Swap(true) {
			l.logger.Warn("Redis rate limiter unavailable; using per-instance limits")
		}
		return allowed, retry
	}
	if l.degraded.Swap(false) {
		l.logger.Info("Redis rate limiter recovered")
	}
	if len(result) != 2 {
		return allowed, retry
	}
	if result[0] > int64(l.limit) {
		return false, max(time.Millisecond, time.Duration(result[1])*time.Millisecond)
	}
	return true, 0
}

func (l *Limiter) local(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !now.Before(l.nextSweep) {
		for k, v := range l.entries {
			if !now.Before(v.expires) {
				delete(l.entries, k)
			}
		}
		l.nextSweep = now.Add(l.window)
	}
	v, exists := l.entries[key]
	if !exists || !now.Before(v.expires) {
		// Bound memory under a high-cardinality attack. Deny new buckets until
		// old ones expire instead of evicting live buckets and bypassing limits.
		if !exists && len(l.entries) >= 10000 {
			return false, l.window
		}
		v = entry{expires: now.Add(l.window)}
	}
	if v.count >= l.limit {
		return false, v.expires.Sub(now)
	}
	v.count++
	l.entries[key] = v
	return true, 0
}
