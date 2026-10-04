package http

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache keeps small values in Redis, falling back to memory when Redis is absent or down.
type Cache struct {
	rdb *redis.Client
	mu  sync.Mutex
	mem map[string]string
}

func NewCache(redisURL string) *Cache {
	c := &Cache{mem: map[string]string{}}
	if redisURL != "" {
		if opt, err := redis.ParseURL(redisURL); err == nil {
			c.rdb = redis.NewClient(opt)
		}
	}
	return c
}

func (c *Cache) Ping(ctx context.Context) error {
	if c.rdb == nil {
		return nil // optional dependency
	}
	return c.rdb.Ping(ctx).Err()
}

func (c *Cache) Set(ctx context.Context, key string, v any) {
	if c.rdb != nil {
		cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		_ = c.rdb.Set(cctx, key, v, 0).Err()
	}
}

func (c *Cache) SetString(ctx context.Context, key, val string, ttl time.Duration) {
	if c.rdb != nil {
		cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		if c.rdb.Set(cctx, key, val, ttl).Err() == nil {
			return
		}
	}
	c.mu.Lock()
	c.mem[key] = val
	c.mu.Unlock()
}

func (c *Cache) GetString(ctx context.Context, key string) (string, bool) {
	if c.rdb != nil {
		cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		if v, err := c.rdb.Get(cctx, key).Result(); err == nil {
			return v, true
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.mem[key]
	return v, ok
}
