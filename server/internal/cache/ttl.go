// Package cache is a tiny TTL cache that also collapses concurrent
// requests for the same key into one upstream call. When 200 people open
// the same bus stop, the municipality sees a single request.
package cache

import (
	"context"
	"sync"
	"time"
)

type item[V any] struct {
	val     V
	err     error
	expires time.Time
	ready   chan struct{}
}

type TTL[V any] struct {
	mu    sync.Mutex
	items map[string]*item[V]
	ttl   time.Duration
	// errTTL is how long a failure is remembered, so a broken upstream
	// is not hammered on every request.
	errTTL time.Duration
	max    int
}

func New[V any](ttl, errTTL time.Duration, maxItems int) *TTL[V] {
	return &TTL[V]{items: map[string]*item[V]{}, ttl: ttl, errTTL: errTTL, max: maxItems}
}

// Get returns the cached value for key or calls fn once to produce it.
func (c *TTL[V]) Get(ctx context.Context, key string, fn func(context.Context) (V, error)) (V, error) {
	now := time.Now()
	c.mu.Lock()
	if it, ok := c.items[key]; ok {
		select {
		case <-it.ready:
			if now.Before(it.expires) {
				c.mu.Unlock()
				return it.val, it.err
			}
		default:
			c.mu.Unlock()
			select {
			case <-it.ready:
				return it.val, it.err
			case <-ctx.Done():
				var zero V
				return zero, ctx.Err()
			}
		}
	}
	if len(c.items) >= c.max {
		c.evict(now)
	}
	it := &item[V]{ready: make(chan struct{})}
	c.items[key] = it
	c.mu.Unlock()

	// Detach from the caller's cancellation so one impatient client does
	// not poison the shared result for everyone waiting on it.
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	it.val, it.err = fn(fctx)
	cancel()
	if it.err != nil {
		it.expires = time.Now().Add(c.errTTL)
	} else {
		it.expires = time.Now().Add(c.ttl)
	}
	close(it.ready)
	return it.val, it.err
}

func (c *TTL[V]) evict(now time.Time) {
	for k, it := range c.items {
		select {
		case <-it.ready:
			if now.After(it.expires) {
				delete(c.items, k)
			}
		default:
		}
	}
	if len(c.items) >= c.max { // still full: drop everything finished
		for k, it := range c.items {
			select {
			case <-it.ready:
				delete(c.items, k)
			default:
			}
		}
	}
}
