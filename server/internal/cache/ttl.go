// Package cache is a tiny TTL cache that also collapses concurrent
// requests for the same key into one upstream call. When 200 people open
// the same bus stop, the municipality sees a single request.
//
// With WithStale, an expired value is still returned immediately while a
// single background refresh fetches a new one ("stale-while-revalidate"),
// so users never wait for a slow municipal API once a key is warm.
package cache

import (
	"context"
	"sync"
	"time"
)

type item[V any] struct {
	val        V
	err        error
	expires    time.Time
	ready      chan struct{}
	refreshing bool
	nextTry    time.Time // after a failed background refresh
}

type TTL[V any] struct {
	mu    sync.Mutex
	items map[string]*item[V]
	ttl   time.Duration
	// errTTL is how long a failure is remembered, so a broken upstream
	// is not hammered on every request.
	errTTL time.Duration
	// stale is how long after expiry a good value may still be served
	// while it is refreshed in the background (0 = never).
	stale time.Duration
	max   int
}

func New[V any](ttl, errTTL time.Duration, maxItems int) *TTL[V] {
	return &TTL[V]{items: map[string]*item[V]{}, ttl: ttl, errTTL: errTTL, max: maxItems}
}

// WithStale enables stale-while-revalidate for up to d after expiry.
func (c *TTL[V]) WithStale(d time.Duration) *TTL[V] {
	c.stale = d
	return c
}

func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	// Detach from the caller's cancellation so one impatient client does
	// not poison the shared result for everyone waiting on it.
	return context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
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
			// Expired but recently good: serve it and refresh once.
			if it.err == nil && c.stale > 0 && now.Before(it.expires.Add(c.stale)) {
				if !it.refreshing && !now.Before(it.nextTry) {
					it.refreshing = true
					go c.refresh(ctx, key, it, fn)
				}
				c.mu.Unlock()
				return it.val, nil
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

	fctx, cancel := detached(ctx)
	it.val, it.err = fn(fctx)
	cancel()
	c.mu.Lock()
	if it.err != nil {
		it.expires = time.Now().Add(c.errTTL)
	} else {
		it.expires = time.Now().Add(c.ttl)
	}
	c.mu.Unlock()
	close(it.ready)
	return it.val, it.err
}

// refresh replaces a stale entry in the background. On failure the old
// value keeps being served (until the stale window ends) and the next
// attempt waits errTTL.
func (c *TTL[V]) refresh(ctx context.Context, key string, old *item[V], fn func(context.Context) (V, error)) {
	fctx, cancel := detached(ctx)
	v, err := fn(fctx)
	cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items[key] != old {
		return
	}
	if err != nil {
		old.refreshing = false
		old.nextTry = time.Now().Add(c.errTTL)
		return
	}
	nit := &item[V]{val: v, expires: time.Now().Add(c.ttl), ready: make(chan struct{})}
	close(nit.ready)
	c.items[key] = nit
}

func (c *TTL[V]) evict(now time.Time) {
	for k, it := range c.items {
		select {
		case <-it.ready:
			if now.After(it.expires.Add(c.stale)) {
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
