package cache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

// TTLCache is a simple in-process cache with per-key TTL expiration.
// It is safe for concurrent use.
type TTLCache[T any] struct {
	mu      sync.RWMutex
	items   map[string]entry[T]
	ttl     time.Duration
	maxSize int
}

func New[T any](ttl time.Duration, maxSize int) *TTLCache[T] {
	c := &TTLCache[T]{
		items:   make(map[string]entry[T], maxSize),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go c.evictLoop()
	return c
}

func (c *TTLCache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		var zero T
		return zero, false
	}
	return e.value, true
}

func (c *TTLCache[T]) Set(key string, value T) {
	c.mu.Lock()
	// Simple eviction: if at capacity, drop ~25% of entries
	if len(c.items) >= c.maxSize {
		count := 0
		target := c.maxSize / 4
		for k := range c.items {
			delete(c.items, k)
			count++
			if count >= target {
				break
			}
		}
	}
	c.items[key] = entry[T]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *TTLCache[T]) DeletePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}

func (c *TTLCache[T]) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.items {
			if now.After(e.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
