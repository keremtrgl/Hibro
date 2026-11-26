package collect

import (
	"regexp"
	"sync"
	"time"
)

// Allowed anonymous usage events. Anything else is dropped, so the app
// cannot accidentally send identifying data through this channel.
var TelemetryEvents = map[string]bool{
	"app_open": true, "screen_map": true, "screen_stop": true, "screen_line": true,
	"screen_search": true, "screen_favorites": true, "screen_alerts": true, "screen_settings": true,
	"search": true, "search_empty": true, "favorite_add": true, "report_sent": true,
	"plan_request": true, "plan_empty": true, "ride_start": true, "ride_end": true,
	"ride_alarm": true, "live_degraded": true, "offline_pack_used": true, "locate_me": true,
	"nearby_vehicles": true,
}

var cityRe = regexp.MustCompile(`^[a-z]{2,20}$`)

// Counters aggregates events per day in memory and snapshots them to
// <dir>/telemetry/<day>.json. Only totals exist: no user or session.
type Counters struct {
	mu    sync.Mutex
	store *Store
	day   string
	m     map[string]int64 // "city/event" -> count
}

func NewCounters(store *Store) *Counters {
	c := &Counters{store: store, m: map[string]int64{}}
	c.day = store.day()
	_ = store.ReadSnapshot("telemetry", c.day, &c.m)
	if c.m == nil {
		c.m = map[string]int64{}
	}
	return c
}

// Add records events; unknown names and silly counts are ignored.
func (c *Counters) Add(city string, events map[string]int) int {
	if !cityRe.MatchString(city) {
		city = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rollover()
	accepted := 0
	for name, n := range events {
		if !TelemetryEvents[name] || n <= 0 {
			continue
		}
		if n > 500 {
			n = 500
		}
		c.m[city+"/"+name] += int64(n)
		accepted++
	}
	return accepted
}

func (c *Counters) rollover() {
	if d := c.store.day(); d != c.day {
		_ = c.store.WriteSnapshot("telemetry", c.day, c.m)
		c.day, c.m = d, map[string]int64{}
	}
}

// Flush writes today's snapshot.
func (c *Counters) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rollover()
	cp := make(map[string]int64, len(c.m))
	for k, v := range c.m {
		cp[k] = v
	}
	return c.store.WriteSnapshot("telemetry", c.day, cp)
}

func (c *Counters) Snapshot() (string, map[string]int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make(map[string]int64, len(c.m))
	for k, v := range c.m {
		cp[k] = v
	}
	return c.day, cp
}

// FlushEvery snapshots periodically until stop is closed.
func (c *Counters) FlushEvery(d time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-stop:
			_ = c.Flush()
			return
		case <-t.C:
			_ = c.Flush()
		}
	}
}
