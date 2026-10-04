// Package ratelimit is a fixed-window per-key limiter backed by Redis, falling back to
// process memory when Redis is not configured or unreachable, so the demo never depends on it.
package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb *redis.Client
	mu  sync.Mutex
	mem map[string]*bucket
}

type bucket struct {
	n   int
	end time.Time
}

func New(redisURL string) *Limiter {
	l := &Limiter{mem: map[string]*bucket{}}
	if redisURL != "" {
		if opt, err := redis.ParseURL(redisURL); err == nil {
			l.rdb = redis.NewClient(opt)
		}
	}
	return l
}

// Allow reports whether key may proceed: at most limit calls per window.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	if l.rdb != nil {
		cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		k := fmt.Sprintf("rl:%s:%d", key, time.Now().Unix()/int64(window.Seconds()+1))
		n, err := l.rdb.Incr(cctx, k).Result()
		if err == nil {
			if n == 1 {
				l.rdb.Expire(cctx, k, window+time.Second)
			}
			return int(n) <= limit
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b := l.mem[key]
	if b == nil || now.After(b.end) {
		b = &bucket{end: now.Add(window)}
		l.mem[key] = b
	}
	b.n++
	return b.n <= limit
}
