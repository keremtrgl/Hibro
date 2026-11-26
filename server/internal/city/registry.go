package city

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Loader builds a fresh City snapshot (downloads, parses, indexes).
type Loader func(ctx context.Context) (*City, error)

type entry struct {
	id     string
	loader Loader
	cur    atomic.Pointer[City]
}

// Registry holds every configured city and swaps snapshots atomically
// when a refresh succeeds, so requests never see half-loaded data.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*entry
	Log     *slog.Logger
}

func NewRegistry() *Registry {
	return &Registry{entries: map[string]*entry{}, Log: slog.Default()}
}

func (r *Registry) Add(id string, l Loader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = &entry{id: id, loader: l}
}

// Put installs a ready snapshot (used by tests).
func (r *Registry) Put(c *City) {
	r.mu.Lock()
	e := r.entries[c.Meta.ID]
	if e == nil {
		e = &entry{id: c.Meta.ID}
		r.entries[c.Meta.ID] = e
	}
	r.mu.Unlock()
	e.cur.Store(c)
}

func (r *Registry) Get(id string) *City {
	r.mu.RLock()
	e := r.entries[id]
	r.mu.RUnlock()
	if e == nil {
		return nil
	}
	return e.cur.Load()
}

// All returns loaded cities sorted by ID.
func (r *Registry) All() []*City {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*City
	for _, e := range r.entries {
		if c := e.cur.Load(); c != nil {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Meta.ID < out[b].Meta.ID })
	return out
}

// LoadAll loads every city concurrently; one failing city does not stop
// the others from serving.
func (r *Registry) LoadAll(ctx context.Context) {
	r.mu.RLock()
	list := make([]*entry, 0, len(r.entries))
	for _, e := range r.entries {
		list = append(list, e)
	}
	r.mu.RUnlock()
	var wg sync.WaitGroup
	for _, e := range list {
		if e.loader == nil {
			continue
		}
		wg.Add(1)
		go func(e *entry) {
			defer wg.Done()
			start := time.Now()
			c, err := e.loader(ctx)
			if err != nil {
				r.Log.Error("city load failed", "city", e.id, "err", err)
				return
			}
			e.cur.Store(c)
			info := c.Info()
			r.Log.Info("city loaded", "city", e.id, "stops", info.StopCount, "lines", info.LineCount, "took", time.Since(start).Round(time.Millisecond))
		}(e)
	}
	wg.Wait()
}

// RefreshEvery reloads all cities on a fixed interval until ctx ends.
func (r *Registry) RefreshEvery(ctx context.Context, d time.Duration) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.LoadAll(ctx)
		}
	}
}
