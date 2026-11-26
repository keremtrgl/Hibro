// Package api is the HTTP layer: JSON endpoints under /v1, gzip,
// CORS and cache headers. No cookies, no tracking, no user accounts.
package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keremtrgl/hibro/server/internal/city"
	"github.com/keremtrgl/hibro/server/internal/geocode"
	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/izmir"
	"github.com/keremtrgl/hibro/server/internal/model"
	"github.com/keremtrgl/hibro/server/internal/plan"
	"github.com/keremtrgl/hibro/server/internal/raptor"
)

type Server struct {
	Cities  *city.Registry
	Planner *plan.MOTIS // external planner; nil uses the built-in RAPTOR

	plMu     sync.Mutex
	planners map[string]cityPlanner
	Collect  *Collect           // nil disables contribution endpoints
	Geo      *geocode.Nominatim // place search; nil disables it
	Log      *slog.Logger
	Now      func() time.Time
	Loc      *time.Location
}

func (s *Server) now() time.Time {
	t := time.Now()
	if s.Now != nil {
		t = s.Now()
	}
	if s.Loc != nil {
		t = t.In(s.Loc)
	}
	return t
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /gizlilik", privacyPage)
	mux.HandleFunc("GET /privacy", privacyPage)
	mux.HandleFunc("GET /v1/cities", s.cities)
	mux.HandleFunc("GET /v1/cities/{city}", s.withCity(s.cityInfo))
	mux.HandleFunc("GET /v1/cities/{city}/pack", s.withCity(s.pack))
	mux.HandleFunc("GET /v1/cities/{city}/stops/nearby", s.withCity(s.nearby))
	mux.HandleFunc("GET /v1/cities/{city}/stops/{id}", s.withCity(s.stop))
	mux.HandleFunc("GET /v1/cities/{city}/stops/{id}/arrivals", s.withCity(s.arrivals))
	mux.HandleFunc("GET /v1/cities/{city}/lines", s.withCity(s.lines))
	mux.HandleFunc("GET /v1/cities/{city}/lines/{id}", s.withCity(s.line))
	mux.HandleFunc("GET /v1/cities/{city}/lines/{id}/vehicles", s.withCity(s.vehicles))
	mux.HandleFunc("GET /v1/cities/{city}/alerts", s.withCity(s.alerts))
	mux.HandleFunc("GET /v1/cities/{city}/search", s.withCity(s.search))
	mux.HandleFunc("GET /v1/cities/{city}/plan", s.withCity(s.plan))
	mux.HandleFunc("GET /v1/cities/{city}/places", s.withCity(s.places))
	mux.HandleFunc("GET /v1/cities/{city}/vehicles/nearby", s.withCity(s.nearbyVehicles))
	mux.HandleFunc("GET /v1/report-kinds", s.reportKinds)
	mux.HandleFunc("POST /v1/cities/{city}/reports", s.withCity(s.postReport))
	mux.HandleFunc("POST /v1/cities/{city}/trips", s.withCity(s.postTrip))
	mux.HandleFunc("POST /v1/telemetry", s.postTelemetry)
	mux.HandleFunc("GET /admin/stats", s.adminStats)
	return s.middleware(mux)
}

//go:embed static/gizlilik.html
var privacyHTML []byte

// privacyPage serves the privacy policy (the store listing links here).
func privacyPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(privacyHTML)
}

// ---- middleware ----

// gzipWriter compresses lazily: bodiless responses (304, 204) are sent
// untouched, because a gzip footer there would corrupt the response.
type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	started bool
	plain   bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if code == http.StatusNotModified || code == http.StatusNoContent {
		g.plain = true
	} else if !g.started {
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Add("Vary", "Accept-Encoding")
		g.Header().Del("Content-Length")
		g.started = true
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if g.plain {
		return g.ResponseWriter.Write(b)
	}
	if !g.started {
		g.WriteHeader(http.StatusOK)
	}
	return g.gz.Write(b)
}

func (g *gzipWriter) finish() {
	if g.started {
		g.gz.Close()
	}
}

var gzPool = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed); return w }}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "If-None-Match, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gz := gzPool.Get().(*gzip.Writer)
			gz.Reset(w)
			gw := &gzipWriter{ResponseWriter: w, gz: gz}
			next.ServeHTTP(gw, r)
			gw.finish()
			gzPool.Put(gz)
		} else {
			next.ServeHTTP(w, r)
		}
		// Request logs carry no IPs and no query strings (which may
		// contain coordinates): privacy by default.
		if s.Log != nil {
			s.Log.Debug("req", "path", r.URL.Path, "took", time.Since(start).Round(time.Microsecond))
		}
	})
}

// ---- helpers ----

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any, maxAge time.Duration) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	if maxAge > 0 {
		h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int(maxAge.Seconds())))
	} else {
		h.Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeCached writes a cacheable JSON response with an ETag and answers
// 304 Not Modified when the client already has the same body. The app
// keeps ETags in memory, so revisiting a line or the alerts list costs a
// few hundred bytes instead of the whole response.
func writeCached(w http.ResponseWriter, r *http.Request, v any, maxAge time.Duration) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fail(w, http.StatusInternalServerError, "kodlama hatası")
		return
	}
	sum := sha1.Sum(buf.Bytes())
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int(maxAge.Seconds())))
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{msg}, 0)
}

func notFound(w http.ResponseWriter, err error) bool {
	if errors.Is(err, city.ErrNotFound) || errors.Is(err, gtfs.ErrNotFound) || errors.Is(err, izmir.ErrNotFound) {
		fail(w, http.StatusNotFound, "bulunamadı")
		return true
	}
	return false
}

func (s *Server) withCity(fn func(http.ResponseWriter, *http.Request, *city.City)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := s.Cities.Get(r.PathValue("city"))
		if c == nil {
			fail(w, http.StatusNotFound, "şehir bulunamadı veya henüz yükleniyor")
			return
		}
		fn(w, r, c)
	}
}

func floatParam(r *http.Request, name string) (float64, bool) {
	v, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
	return v, err == nil
}

func latLonParam(s string) (model.LatLon, bool) {
	a, b, ok := strings.Cut(s, ",")
	if !ok {
		return model.LatLon{}, false
	}
	lat, e1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	lon, e2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if e1 != nil || e2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return model.LatLon{}, false
	}
	return model.LatLon{lat, lon}, true
}

// ---- handlers ----

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	all := s.Cities.All()
	status := http.StatusOK
	if len(all) == 0 {
		status = http.StatusServiceUnavailable
	}
	ids := make([]string, len(all))
	for i, c := range all {
		ids[i] = c.Meta.ID
	}
	writeJSON(w, status, map[string]any{"ok": len(all) > 0, "cities": ids, "planner": s.Planner != nil}, 0)
}

func (s *Server) cities(w http.ResponseWriter, r *http.Request) {
	out := []model.CityInfo{}
	for _, c := range s.Cities.All() {
		info := c.Info()
		info.Features.Planner = s.hasPlanner(c)
		out = append(out, info)
	}
	writeCached(w, r, out, 5*time.Minute)
}

func (s *Server) cityInfo(w http.ResponseWriter, r *http.Request, c *city.City) {
	info := c.Info()
	info.Features.Planner = s.hasPlanner(c)
	writeCached(w, r, info, 5*time.Minute)
}

// pack serves the full offline bundle with an ETag so the app only
// re-downloads it when the city data actually changed.
func (s *Server) pack(w http.ResponseWriter, r *http.Request, c *city.City) {
	body := c.PackJSON(func(p model.Pack) []byte {
		p.City.Features.Planner = s.hasPlanner(c)
		b, _ := json.Marshal(p)
		return b
	})
	// The ETag covers only what the app stores (stops, lines, names,
	// sources), not load timestamps: a daily reload with unchanged data
	// must not make every phone download the pack again.
	etag := c.PackVersion()
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "public, max-age=3600")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(body)
}

func (s *Server) nearby(w http.ResponseWriter, r *http.Request, c *city.City) {
	lat, ok1 := floatParam(r, "lat")
	lon, ok2 := floatParam(r, "lon")
	if !ok1 || !ok2 {
		fail(w, http.StatusBadRequest, "lat ve lon gerekli")
		return
	}
	radius, ok := floatParam(r, "radius")
	if !ok || radius <= 0 {
		radius = 600
	}
	if radius > 3000 {
		radius = 3000
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	writeJSON(w, http.StatusOK, c.Nearby(lat, lon, radius, limit), 30*time.Second)
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request, c *city.City) {
	st, ok := c.Stop(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "durak bulunamadı")
		return
	}
	writeCached(w, r, st, time.Hour)
}

type arrivalsResp struct {
	Stop      model.Stop      `json:"stop"`
	Arrivals  []model.Arrival `json:"arrivals"`
	Live      bool            `json:"live"`
	Degraded  bool            `json:"degraded,omitempty"` // live source failed
	FetchedAt time.Time       `json:"fetchedAt"`
}

func (s *Server) arrivals(w http.ResponseWriter, r *http.Request, c *city.City) {
	id := r.PathValue("id")
	st, ok := c.Stop(id)
	if !ok {
		fail(w, http.StatusNotFound, "durak bulunamadı")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	arr, err := c.Arrivals(ctx, id, s.now())
	resp := arrivalsResp{Stop: st, Arrivals: arr, FetchedAt: time.Now()}
	if resp.Arrivals == nil {
		resp.Arrivals = []model.Arrival{}
	}
	resp.Live = c.LiveStop(id)
	for i, a := range arr {
		resp.Live = resp.Live || a.Realtime
		if s.Collect != nil && s.Collect.Recent.Count(c.Meta.ID, a.LineID, "crowded", 20*time.Minute, time.Now()) > 0 {
			arr[i].Crowded = true
		}
	}
	if err != nil {
		if notFound(w, err) {
			return
		}
		// Upstream down: tell the app so it can show a clear message
		// instead of an empty list that looks like "no buses".
		resp.Degraded = true
		writeJSON(w, http.StatusOK, resp, 0)
		return
	}
	writeJSON(w, http.StatusOK, resp, 10*time.Second)
}

func (s *Server) lines(w http.ResponseWriter, r *http.Request, c *city.City) {
	writeCached(w, r, c.Lines(), time.Hour)
}

func (s *Server) line(w http.ResponseWriter, r *http.Request, c *city.City) {
	now := s.now()
	if d := r.URL.Query().Get("date"); d != "" {
		if t, err := time.ParseInLocation("2006-01-02", d, now.Location()); err == nil {
			now = t.Add(12 * time.Hour)
		}
	}
	ld, err := c.Line(r.Context(), r.PathValue("id"), now)
	if err != nil {
		if !notFound(w, err) {
			fail(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	writeCached(w, r, ld, 10*time.Minute)
}

func (s *Server) vehicles(w http.ResponseWriter, r *http.Request, c *city.City) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	vs, err := c.Vehicles(ctx, r.PathValue("id"))
	if err != nil {
		if !notFound(w, err) {
			writeJSON(w, http.StatusOK, map[string]any{"vehicles": []model.Vehicle{}, "degraded": true}, 0)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicles": vs, "fetchedAt": time.Now()}, 5*time.Second)
}

func (s *Server) alerts(w http.ResponseWriter, r *http.Request, c *city.City) {
	writeCached(w, r, c.Alerts(r.Context(), s.now()), 5*time.Minute)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request, c *city.City) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) > 64 {
		q = string([]rune(q)[:64])
	}
	writeCached(w, r, c.Search(q, 25), time.Minute)
}

func (s *Server) nearbyVehicles(w http.ResponseWriter, r *http.Request, c *city.City) {
	lat, ok1 := floatParam(r, "lat")
	lon, ok2 := floatParam(r, "lon")
	if !ok1 || !ok2 {
		fail(w, http.StatusBadRequest, "lat ve lon gerekli")
		return
	}
	radius, ok := floatParam(r, "radius")
	if !ok || radius <= 0 || radius > 1500 {
		radius = 900
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	vs := c.NearbyVehicles(ctx, lat, lon, radius, 16)
	writeJSON(w, http.StatusOK, map[string]any{"vehicles": vs, "fetchedAt": time.Now()}, 5*time.Second)
}

type cityPlanner struct {
	c *city.City
	p *raptor.Planner
}

// gtfsSources returns the city's timetable sources usable for routing.
func gtfsSources(c *city.City) []raptor.Source {
	var out []raptor.Source
	for _, src := range c.Sources() {
		if rs, ok := src.(raptor.Source); ok {
			out = append(out, rs)
		}
		// Sources without a GTFS of their own (ESHOT) may offer a
		// timetable built for routing.
		if rf, ok := src.(interface{ RoutingSources() []*gtfs.Source }); ok {
			for _, g := range rf.RoutingSources() {
				out = append(out, g)
			}
		}
	}
	return out
}

func (s *Server) hasPlanner(c *city.City) bool {
	return s.Planner != nil || len(gtfsSources(c)) > 0
}

// builtinPlanner returns (building once per city snapshot) the RAPTOR
// planner over the city's GTFS feeds.
func (s *Server) builtinPlanner(c *city.City) *raptor.Planner {
	srcs := gtfsSources(c)
	if len(srcs) == 0 {
		return nil
	}
	s.plMu.Lock()
	defer s.plMu.Unlock()
	if s.planners == nil {
		s.planners = map[string]cityPlanner{}
	}
	if cp, ok := s.planners[c.Meta.ID]; ok && cp.c == c {
		return cp.p
	}
	start := time.Now()
	p := raptor.New(srcs...)
	if s.Log != nil {
		st, pats := p.Stats()
		s.Log.Info("planner built", "city", c.Meta.ID, "stops", st, "patterns", pats, "took", time.Since(start).Round(time.Millisecond))
	}
	s.planners[c.Meta.ID] = cityPlanner{c, p}
	return p
}

// places searches addresses and points of interest inside the city
// ("Kocaeli Üniversitesi", "Kefken") so a route can be planned to them.
func (s *Server) places(w http.ResponseWriter, r *http.Request, c *city.City) {
	if s.Geo == nil {
		fail(w, http.StatusNotImplemented, "yer araması kapalı")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 3 {
		writeJSON(w, http.StatusOK, map[string]any{"places": []geocode.Place{}}, 0)
		return
	}
	if len([]rune(q)) > 80 {
		q = string([]rune(q)[:80])
	}
	if s.Collect != nil && s.Collect.TeleLimit != nil && !s.Collect.TeleLimit.Allow(s.Collect.client(r)) {
		fail(w, http.StatusTooManyRequests, "çok fazla arama, biraz bekle")
		return
	}
	a, b, cc, d := c.Bounds(0.15)
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	ps, err := s.Geo.Search(ctx, c.Meta.ID, q, [4]float64{a, b, cc, d})
	if err != nil {
		if s.Log != nil {
			s.Log.Warn("place search failed", "err", err)
		}
		fail(w, http.StatusBadGateway, "yer araması şu an yapılamıyor")
		return
	}
	writeCached(w, r, map[string]any{"places": ps}, time.Hour)
}

func (s *Server) plan(w http.ResponseWriter, r *http.Request, c *city.City) {
	if !s.hasPlanner(c) {
		fail(w, http.StatusNotImplemented, "bu şehirde rota planlama henüz yok")
		return
	}
	q := r.URL.Query()
	from, ok1 := latLonParam(q.Get("from"))
	to, ok2 := latLonParam(q.Get("to"))
	if !ok1 || !ok2 {
		fail(w, http.StatusBadRequest, "from ve to 'enlem,boylam' biçiminde gerekli")
		return
	}
	pq := plan.Query{From: from, To: to, ArriveBy: q.Get("arriveBy") == "true", Wheelchair: q.Get("wheelchair") == "true"}
	if t := q.Get("time"); t != "" {
		if tt, err := time.Parse(time.RFC3339, t); err == nil {
			pq.Time = tt
		}
	}
	if s.Planner == nil {
		at := pq.Time
		if at.IsZero() {
			at = s.now()
		} else if s.Loc != nil {
			at = at.In(s.Loc)
		}
		its := s.builtinPlanner(c).Plan(pq.From, pq.To, at, 4)
		writeJSON(w, http.StatusOK, map[string]any{"itineraries": its, "engine": "hibro-raptor"}, 0)
		return
	}
	its, err := s.Planner.Plan(r.Context(), pq)
	if err != nil {
		s.Log.Warn("plan failed", "err", err)
		fail(w, http.StatusBadGateway, "rota hesaplanamadı")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"itineraries": its}, 0)
}
