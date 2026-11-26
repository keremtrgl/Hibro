package api

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/city"
	"github.com/keremtrgl/hibro/server/internal/collect"
)

// Collect bundles the opt-in data collection pieces.
type Collect struct {
	Store      *collect.Store
	Recent     *collect.Recent
	Counters   *collect.Counters
	Limit      *collect.Limiter // reports and trips
	TeleLimit  *collect.Limiter // telemetry batches
	TrustProxy bool             // read client address from X-Forwarded-For (behind Caddy)
	AdminToken string
}

// client returns an address used only as a rate-limit key in memory.
func (c *Collect) client(r *http.Request) string {
	if c.TrustProxy {
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			return strings.TrimSpace(strings.Split(xf, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) collectOK(w http.ResponseWriter, r *http.Request, lim *collect.Limiter) bool {
	if s.Collect == nil {
		fail(w, http.StatusServiceUnavailable, "veri toplama bu sunucuda kapalı")
		return false
	}
	if lim != nil && !lim.Allow(s.Collect.client(r)) {
		fail(w, http.StatusTooManyRequests, "çok fazla istek, biraz sonra tekrar dene")
		return false
	}
	return true
}

func decode(w http.ResponseWriter, r *http.Request, max int64, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "geçersiz istek gövdesi")
		return false
	}
	return true
}

func (s *Server) reportKinds(w http.ResponseWriter, r *http.Request) {
	writeCached(w, r, collect.ReportKinds, time.Hour)
}

func (s *Server) postReport(w http.ResponseWriter, r *http.Request, c *city.City) {
	if !s.collectOK(w, r, s.Collect.limit()) {
		return
	}
	var rep collect.Report
	if !decode(w, r, 4<<10, &rep) {
		return
	}
	rep.City, rep.At = c.Meta.ID, time.Now().UTC()
	if err := rep.Clean(); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if rep.StopID != "" {
		if _, ok := c.Stop(rep.StopID); !ok {
			fail(w, http.StatusBadRequest, "durak bulunamadı")
			return
		}
	}
	if rep.LineID != "" {
		if _, ok := c.LineInfo(rep.LineID); !ok {
			fail(w, http.StatusBadRequest, "hat bulunamadı")
			return
		}
	}
	if err := s.Collect.Store.Append("reports", rep); err != nil {
		s.Log.Error("report store failed", "err", err)
		fail(w, http.StatusInternalServerError, "kaydedilemedi")
		return
	}
	s.Collect.Recent.Add(rep)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true}, 0)
}

func (c *Collect) limit() *collect.Limiter {
	if c == nil {
		return nil
	}
	return c.Limit
}

func (c *Collect) teleLimit() *collect.Limiter {
	if c == nil {
		return nil
	}
	return c.TeleLimit
}

func (s *Server) postTrip(w http.ResponseWriter, r *http.Request, c *city.City) {
	if !s.collectOK(w, r, s.Collect.limit()) {
		return
	}
	var t collect.TripUpload
	if !decode(w, r, 512<<10, &t) {
		return
	}
	if _, ok := c.LineInfo(t.LineID); !ok {
		fail(w, http.StatusBadRequest, "hat bulunamadı")
		return
	}
	a, b, cc, d := c.Bounds(0.05)
	if err := t.Clean(time.Now(), collect.Bounds{MinLat: a, MinLon: b, MaxLat: cc, MaxLon: d}); err != nil {
		fail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	st := collect.StoredTrip{TripUpload: t, City: c.Meta.ID, Received: time.Now().UTC().Truncate(time.Hour)}
	if err := s.Collect.Store.Append("trips", st); err != nil {
		s.Log.Error("trip store failed", "err", err)
		fail(w, http.StatusInternalServerError, "kaydedilemedi")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "points": len(t.Points)}, 0)
}

type telemetryBody struct {
	City   string         `json:"city"`
	Events map[string]int `json:"events"`
}

func (s *Server) postTelemetry(w http.ResponseWriter, r *http.Request) {
	if !s.collectOK(w, r, s.Collect.teleLimit()) {
		return
	}
	var b telemetryBody
	if !decode(w, r, 8<<10, &b) {
		return
	}
	n := s.Collect.Counters.Add(b.City, b.Events)
	writeJSON(w, http.StatusOK, map[string]any{"accepted": n}, 0)
}

type countRow struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

func top(m map[string]int, n int) []countRow {
	rows := make([]countRow, 0, len(m))
	for k, v := range m {
		rows = append(rows, countRow{k, v})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Count > rows[j].Count || rows[i].Count == rows[j].Count && rows[i].Key < rows[j].Key
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// adminStats summarises collected data for the last N days. Protected by
// HIBRO_ADMIN_TOKEN (Authorization: Bearer ...). Disabled without a token.
func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	if s.Collect == nil || s.Collect.AdminToken == "" {
		fail(w, http.StatusNotFound, "bulunamadı")
		return
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.Collect.AdminToken)) != 1 {
		fail(w, http.StatusUnauthorized, "yetkisiz")
		return
	}
	days := 7
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 {
		days = d
	}
	if days > 90 {
		days = 90
	}
	st := s.Collect.Store
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	byKind, byLine, byStop := map[string]int{}, map[string]int{}, map[string]int{}
	var notes []collect.Report
	reports := 0
	for _, day := range st.Days("reports") {
		if day < since {
			break
		}
		_ = st.Scan("reports", day, func(b []byte) {
			var rep collect.Report
			if json.Unmarshal(b, &rep) != nil {
				return
			}
			reports++
			byKind[rep.Kind]++
			if rep.LineID != "" {
				byLine[rep.City+" "+rep.LineID+" "+rep.Kind]++
			}
			if rep.StopID != "" {
				byStop[rep.City+" "+rep.StopID+" "+rep.Kind]++
			}
			if rep.Note != "" && len(notes) < 50 {
				notes = append(notes, rep)
			}
		})
	}
	trips, points := 0, 0
	tripLines := map[string]int{}
	for _, day := range st.Days("trips") {
		if day < since {
			break
		}
		_ = st.Scan("trips", day, func(b []byte) {
			var t collect.StoredTrip
			if json.Unmarshal(b, &t) != nil {
				return
			}
			trips++
			points += len(t.Points)
			tripLines[t.City+" "+t.LineID]++
		})
	}
	telemetry := map[string]map[string]int64{}
	_ = s.Collect.Counters.Flush()
	for _, day := range st.Days("telemetry") {
		if day < since {
			break
		}
		var m map[string]int64
		if st.ReadSnapshot("telemetry", day, &m) == nil {
			telemetry[day] = m
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days": days,
		"reports": map[string]any{
			"total": reports, "byKind": byKind,
			"topLines": top(byLine, 25), "topStops": top(byStop, 25), "recentNotes": notes,
		},
		"trips":     map[string]any{"total": trips, "points": points, "topLines": top(tripLines, 25)},
		"telemetry": telemetry,
	}, 0)
}
