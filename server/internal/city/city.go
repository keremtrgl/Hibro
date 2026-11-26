// Package city combines one or more data sources (a GTFS feed, a live
// municipal API...) into a single city the API can serve.
package city

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/keremtrgl/hibro/server/internal/geo"
	"github.com/keremtrgl/hibro/server/internal/model"
	"github.com/keremtrgl/hibro/server/internal/textnorm"
)

var ErrNotFound = errors.New("not found")

// Source is one provider of stops and lines inside a city, such as the
// İzmir Metro GTFS feed or the ESHOT bus API. IDs it returns carry its
// prefix ("m:12"); methods receive the local part ("12").
type Source interface {
	Prefix() string
	Info() model.SourceInfo
	Stops() []model.Stop
	Lines() []model.Line
	Line(ctx context.Context, localID string, now time.Time) (*model.LineDetail, error)
	Arrivals(ctx context.Context, localStopID string, now time.Time) ([]model.Arrival, error)
	Vehicles(ctx context.Context, localLineID string) ([]model.Vehicle, error)
	Alerts(ctx context.Context, now time.Time) []model.Alert
	Features() model.Features
}

type Meta struct {
	ID     string
	Name   string
	Center model.LatLon
	Zoom   float64
}

// City is an immutable snapshot. Reloading builds a new City.
type City struct {
	Meta     Meta
	sources  map[string]Source
	order    []Source
	stops    []model.Stop
	stopByID map[string]int
	lines    []model.Line
	lineByID map[string]int
	index    *geo.Index
	folded   []string // folded stop names for search
	loadedAt time.Time
	packOnce sync.Once
	pack     []byte

	versionOnce sync.Once
	version     string
}

func New(meta Meta, sources ...Source) *City {
	c := &City{
		Meta:     meta,
		sources:  map[string]Source{},
		stopByID: map[string]int{},
		lineByID: map[string]int{},
		loadedAt: time.Now(),
	}
	for _, s := range sources {
		if s == nil {
			continue
		}
		c.sources[s.Prefix()] = s
		c.order = append(c.order, s)
		for _, st := range s.Stops() {
			c.stopByID[st.ID] = len(c.stops)
			c.stops = append(c.stops, st)
		}
		for _, l := range s.Lines() {
			c.lineByID[l.ID] = len(c.lines)
			c.lines = append(c.lines, l)
		}
	}
	lat := make([]float64, len(c.stops))
	lon := make([]float64, len(c.stops))
	c.folded = make([]string, len(c.stops))
	for i, s := range c.stops {
		lat[i], lon[i] = s.Lat, s.Lon
		c.folded[i] = textnorm.Fold(s.Name)
	}
	c.index = geo.NewIndex(lat, lon)
	sort.SliceStable(c.lines, func(a, b int) bool { return lineLess(c.lines[a], c.lines[b]) })
	for i, l := range c.lines {
		c.lineByID[l.ID] = i
	}
	return c
}

// lineLess orders rail before bus, then numerically by short name.
func lineLess(a, b model.Line) bool {
	ra, rb := modeRank(a.Mode), modeRank(b.Mode)
	if ra != rb {
		return ra < rb
	}
	na, nb := leadingInt(a.Short), leadingInt(b.Short)
	if na != nb {
		return na < nb
	}
	return a.Short < b.Short
}

func modeRank(m model.Mode) int {
	switch m {
	case model.ModeSubway:
		return 0
	case model.ModeRail:
		return 1
	case model.ModeTram:
		return 2
	case model.ModeFerry:
		return 3
	default:
		return 4
	}
}

func leadingInt(s string) int {
	n, seen := 0, false
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n, seen = n*10+int(r-'0'), true
	}
	if !seen {
		return 1 << 30
	}
	return n
}

func (c *City) split(id string) (Source, string, error) {
	p, local, ok := strings.Cut(id, ":")
	if !ok {
		return nil, "", ErrNotFound
	}
	s := c.sources[p]
	if s == nil {
		return nil, "", ErrNotFound
	}
	return s, local, nil
}

func (c *City) Info() model.CityInfo {
	info := model.CityInfo{
		ID: c.Meta.ID, Name: c.Meta.Name, Center: c.Meta.Center, Zoom: c.Meta.Zoom,
		StopCount: len(c.stops), LineCount: len(c.lines), LoadedAt: c.loadedAt,
	}
	if len(c.stops) > 0 {
		a, b, cc, d := c.Bounds(0.02)
		info.Bounds = [4]float64{round4(a), round4(b), round4(cc), round4(d)}
	}
	for _, s := range c.order {
		si := s.Info()
		si.Prefix = s.Prefix()
		info.Sources = append(info.Sources, si)
		f := s.Features()
		info.Features.LiveArrivals = info.Features.LiveArrivals || f.LiveArrivals
		info.Features.LiveVehicles = info.Features.LiveVehicles || f.LiveVehicles
		info.Features.Alerts = info.Features.Alerts || f.Alerts
	}
	return info
}

func (c *City) Stops() []model.Stop { return c.stops }

// Sources returns the city's data sources in load order.
func (c *City) Sources() []Source   { return c.order }
func (c *City) Lines() []model.Line { return c.lines }

func (c *City) Stop(id string) (model.Stop, bool) {
	i, ok := c.stopByID[id]
	if !ok {
		return model.Stop{}, false
	}
	return c.stops[i], true
}

func (c *City) LineInfo(id string) (model.Line, bool) {
	i, ok := c.lineByID[id]
	if !ok {
		return model.Line{}, false
	}
	return c.lines[i], true
}

type NearbyStop struct {
	model.Stop
	DistanceM int `json:"distanceM"`
}

func (c *City) Nearby(lat, lon, radiusM float64, limit int) []NearbyStop {
	hits := c.index.Near(lat, lon, radiusM, limit)
	out := make([]NearbyStop, len(hits))
	for i, h := range hits {
		out[i] = NearbyStop{c.stops[h.Index], int(h.DistanceM)}
	}
	return out
}

type SearchResult struct {
	Stops []model.Stop `json:"stops"`
	Lines []model.Line `json:"lines"`
}

// Search matches lines by short name prefix and stops/lines by folded
// substring. Exact line number matches come first.
func (c *City) Search(q string, limit int) SearchResult {
	fq := textnorm.Fold(q)
	res := SearchResult{Stops: []model.Stop{}, Lines: []model.Line{}}
	if fq == "" {
		return res
	}
	var exact, prefix, other []model.Line
	for _, l := range c.lines {
		fs := textnorm.Fold(l.Short)
		switch {
		case fs == fq:
			exact = append(exact, l)
		case strings.HasPrefix(fs, fq):
			prefix = append(prefix, l)
		case strings.Contains(textnorm.Fold(l.Name), fq):
			other = append(other, l)
		}
	}
	res.Lines = append(append(append(res.Lines, exact...), prefix...), other...)
	if len(res.Lines) > limit {
		res.Lines = res.Lines[:limit]
	}
	type scored struct {
		i     int
		score int
	}
	var hits []scored
	for i, f := range c.folded {
		switch {
		case f == fq:
			hits = append(hits, scored{i, 0})
		case strings.HasPrefix(f, fq):
			hits = append(hits, scored{i, 1})
		case strings.Contains(f, fq):
			hits = append(hits, scored{i, 2})
		case c.stops[i].Code != "" && c.stops[i].Code == q:
			hits = append(hits, scored{i, 0})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score < hits[b].score })
	for _, h := range hits {
		if len(res.Stops) >= limit {
			break
		}
		res.Stops = append(res.Stops, c.stops[h.i])
	}
	return res
}

func (c *City) Line(ctx context.Context, id string, now time.Time) (*model.LineDetail, error) {
	s, local, err := c.split(id)
	if err != nil {
		return nil, err
	}
	return s.Line(ctx, local, now)
}

func (c *City) Arrivals(ctx context.Context, stopID string, now time.Time) ([]model.Arrival, error) {
	s, local, err := c.split(stopID)
	if err != nil {
		return nil, err
	}
	if _, ok := c.stopByID[stopID]; !ok {
		return nil, ErrNotFound
	}
	return s.Arrivals(ctx, local, now)
}

// NearbyVehicles finds live vehicles within radiusM of a point by asking
// the closest live-capable stops what is approaching them. Upstream calls
// are cached per stop, so many users in one area cost one request per
// stop every few seconds.
func (c *City) NearbyVehicles(ctx context.Context, lat, lon, radiusM float64, maxStops int) []model.NearbyVehicle {
	var stops []model.Stop
	for _, h := range c.index.Near(lat, lon, radiusM+400, 200) {
		st := c.stops[h.Index]
		if c.LiveStop(st.ID) {
			stops = append(stops, st)
			if len(stops) >= maxStops {
				break
			}
		}
	}
	type res struct{ arr []model.Arrival }
	out := make(chan res, len(stops))
	sem := make(chan struct{}, 6)
	now := time.Now()
	for _, st := range stops {
		go func(id string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			arr, _ := c.Arrivals(ctx, id, now)
			out <- res{arr}
		}(st.ID)
	}
	best := map[string]model.NearbyVehicle{}
	// Answer within 1.5 s with whatever has arrived; slower stops keep
	// loading into the cache and show up on the next refresh.
	deadline := time.NewTimer(1500 * time.Millisecond)
	defer deadline.Stop()
collect:
	for range stops {
		var r res
		select {
		case r = <-out:
		case <-deadline.C:
			break collect
		case <-ctx.Done():
			break collect
		}
		for _, a := range r.arr {
			if !a.Realtime || a.Lat == 0 || a.Plate == "" {
				continue
			}
			d := geo.DistanceM(lat, lon, a.Lat, a.Lon)
			if d > radiusM {
				continue
			}
			key := a.Plate
			if _, seen := best[key]; seen {
				continue
			}
			color := a.Color
			if color == "" {
				if l, ok := c.LineInfo(a.LineID); ok {
					color = l.Color
				}
			}
			best[key] = model.NearbyVehicle{
				ID: a.Plate, LineID: a.LineID, Line: a.Line, Headsign: a.Headsign, Mode: a.Mode,
				Color: color, Lat: a.Lat, Lon: a.Lon, DistanceM: int(d),
			}
		}
	}
	list := make([]model.NearbyVehicle, 0, len(best))
	for _, v := range best {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].DistanceM < list[j].DistanceM })
	return list
}

// Bounds returns the box around all stops plus a margin (degrees).
func (c *City) Bounds(margin float64) (minLat, minLon, maxLat, maxLon float64) {
	minLat, minLon, maxLat, maxLon = 90, 180, -90, -180
	for _, s := range c.stops {
		minLat, maxLat = min(minLat, s.Lat), max(maxLat, s.Lat)
		minLon, maxLon = min(minLon, s.Lon), max(maxLon, s.Lon)
	}
	return minLat - margin, minLon - margin, maxLat + margin, maxLon + margin
}

func round4(v float64) float64 { return float64(int64(v*10000+0.5*sign(v))) / 10000 }

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// LiveStop reports whether the stop's source provides live arrivals.
func (c *City) LiveStop(stopID string) bool {
	s, _, err := c.split(stopID)
	return err == nil && s.Features().LiveArrivals
}

func (c *City) Vehicles(ctx context.Context, lineID string) ([]model.Vehicle, error) {
	s, local, err := c.split(lineID)
	if err != nil {
		return nil, err
	}
	return s.Vehicles(ctx, local)
}

func (c *City) Alerts(ctx context.Context, now time.Time) []model.Alert {
	out := []model.Alert{}
	for _, s := range c.order {
		out = append(out, s.Alerts(ctx, now)...)
	}
	return out
}

// Pack returns the offline bundle (built once per snapshot).
func (c *City) Pack() model.Pack {
	return model.Pack{City: c.Info(), Stops: c.stops, Lines: c.lines}
}

// PackVersion is a quoted ETag over the content the app keeps offline
// (stops, lines and stable city/source fields), excluding timestamps.
func (c *City) PackVersion() string {
	c.versionOnce.Do(func() {
		h := sha1.New()
		enc := json.NewEncoder(h)
		info := c.Info()
		_ = enc.Encode([]any{info.ID, info.Name, info.Center, info.Bounds, info.Zoom, info.Features})
		for _, src := range info.Sources {
			_ = enc.Encode([]any{src.Name, src.License, src.URL, src.Stale, src.Note})
		}
		_ = enc.Encode(c.stops)
		_ = enc.Encode(c.lines)
		c.version = `"` + hex.EncodeToString(h.Sum(nil)[:8]) + `"`
	})
	return c.version
}

// PackJSON caches the encoded pack; encoding is supplied by the caller
// so this package stays free of HTTP concerns.
func (c *City) PackJSON(encode func(model.Pack) []byte) []byte {
	c.packOnce.Do(func() { c.pack = encode(c.Pack()) })
	return c.pack
}
