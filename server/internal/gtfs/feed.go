// Package gtfs loads a static GTFS feed into a compact in-memory form and
// answers the questions the app asks: what leaves this stop next, which
// stops does a line serve, what is its timetable today.
package gtfs

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/model"
	"github.com/keremtrgl/hibro/server/internal/textnorm"
)

type Stop struct {
	ID, Code, Name string
	Lat, Lon       float64
	LocationType   int
	Parent         string
}

type Route struct {
	ID, Agency, Short, Long, Color, TextColor string
	Type                                      int
}

type Trip struct {
	ID       string
	Route    int32
	Service  int32
	Headsign string
	Dir      int8
	Shape    int32 // -1 when absent
}

type Service struct {
	ID       string
	Start    int // yyyymmdd
	End      int
	Weekdays [7]bool // indexed by time.Weekday (0 = Sunday)
	Added    map[int]bool
	Removed  map[int]bool
}

// StopTime is one call of a trip at a stop. Times are seconds after the
// service day's midnight and may exceed 24h.
type StopTime struct {
	Stop int32
	Arr  int32
	Dep  int32
}

// StopDep is the reverse index: a departure of trip Trip from a stop, at
// position Pos within the trip.
type StopDep struct {
	Trip int32
	Pos  int32
	Dep  int32
}

type Feed struct {
	Agencies  map[string]string
	Stops     []Stop
	StopIdx   map[string]int32
	Routes    []Route
	RouteIdx  map[string]int32
	Trips     []Trip
	Services  []Service
	Shapes    [][]model.LatLon
	TripStops [][]StopTime
	StopDeps  [][]StopDep        // sorted by Dep
	StopLines [][]int32          // route indexes serving each stop
	RepTrip   map[[2]int32]int32 // (route, dir) -> trip with most stops
	FeedStart int
	FeedEnd   int
	Publisher string
	svcIdx    map[string]int32
}

// Parse reads a GTFS zip held in memory.
func Parse(data []byte) (*Feed, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("gtfs zip: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		name := f.Name
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		files[name] = f
	}
	f := &Feed{
		Agencies: map[string]string{},
		StopIdx:  map[string]int32{},
		RouteIdx: map[string]int32{},
		RepTrip:  map[[2]int32]int32{},
	}
	for _, req := range []string{"stops.txt", "routes.txt", "trips.txt", "stop_times.txt"} {
		if files[req] == nil {
			return nil, errors.New("gtfs: missing " + req)
		}
	}
	steps := []struct {
		name string
		fn   func(*table) error
		opt  bool
	}{
		{"agency.txt", f.readAgency, true},
		{"feed_info.txt", f.readFeedInfo, true},
		{"stops.txt", f.readStops, false},
		{"routes.txt", f.readRoutes, false},
		{"calendar.txt", f.readCalendar, true},
		{"calendar_dates.txt", f.readCalendarDates, true},
		{"shapes.txt", nil, true}, // handled with trips
		{"trips.txt", nil, false},
		{"stop_times.txt", f.readStopTimes, false},
	}
	shapeIdx := map[string]int32{}
	for _, s := range steps {
		zf := files[s.name]
		if zf == nil {
			continue
		}
		t, err := openTable(zf)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
		switch s.name {
		case "shapes.txt":
			err = f.readShapes(t, shapeIdx)
		case "trips.txt":
			err = f.readTrips(t, shapeIdx)
		default:
			err = s.fn(t)
		}
		t.close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
	}
	f.buildIndexes()
	return f, nil
}

// ---- CSV helpers ----

type table struct {
	rc   io.ReadCloser
	r    *csv.Reader
	cols map[string]int
	row  []string
}

func openTable(zf *zip.File) (*table, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(rc)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = true
	hdr, err := r.Read()
	if err != nil {
		rc.Close()
		return nil, err
	}
	cols := map[string]int{}
	for i, h := range hdr {
		h = strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))
		cols[h] = i
	}
	return &table{rc: rc, r: r, cols: cols}, nil
}

func (t *table) close() { t.rc.Close() }

func (t *table) next() bool {
	row, err := t.r.Read()
	if err != nil {
		return false
	}
	t.row = row
	return true
}

func (t *table) get(col string) string {
	i, ok := t.cols[col]
	if !ok || i >= len(t.row) {
		return ""
	}
	return strings.TrimSpace(t.row[i])
}

func (t *table) int(col string) int {
	n, _ := strconv.Atoi(t.get(col))
	return n
}

func (t *table) float(col string) float64 {
	n, _ := strconv.ParseFloat(t.get(col), 64)
	return n
}

// ParseTime parses "H:MM:SS" (hours may exceed 23) into seconds.
func ParseTime(s string) (int32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, false
	}
	h, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	sec := 0
	var e3 error
	if len(parts) > 2 {
		sec, e3 = strconv.Atoi(parts[2])
	}
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, false
	}
	return int32(h*3600 + m*60 + sec), true
}

// ---- readers ----

func (f *Feed) readAgency(t *table) error {
	for t.next() {
		f.Agencies[t.get("agency_id")] = textnorm.Clean(t.get("agency_name"))
	}
	return nil
}

func (f *Feed) readFeedInfo(t *table) error {
	if t.next() {
		f.Publisher = textnorm.Clean(t.get("feed_publisher_name"))
		f.FeedStart = t.int("feed_start_date")
		f.FeedEnd = t.int("feed_end_date")
	}
	return nil
}

func (f *Feed) readStops(t *table) error {
	for t.next() {
		s := Stop{
			ID:           t.get("stop_id"),
			Code:         t.get("stop_code"),
			Name:         textnorm.Clean(t.get("stop_name")),
			Lat:          t.float("stop_lat"),
			Lon:          t.float("stop_lon"),
			LocationType: t.int("location_type"),
			Parent:       t.get("parent_station"),
		}
		if s.ID == "" {
			continue
		}
		f.StopIdx[s.ID] = int32(len(f.Stops))
		f.Stops = append(f.Stops, s)
	}
	return nil
}

func (f *Feed) readRoutes(t *table) error {
	for t.next() {
		r := Route{
			ID:        t.get("route_id"),
			Agency:    t.get("agency_id"),
			Short:     textnorm.Clean(t.get("route_short_name")),
			Long:      textnorm.Clean(t.get("route_long_name")),
			Color:     strings.TrimPrefix(t.get("route_color"), "#"),
			TextColor: strings.TrimPrefix(t.get("route_text_color"), "#"),
			Type:      t.int("route_type"),
		}
		f.RouteIdx[r.ID] = int32(len(f.Routes))
		f.Routes = append(f.Routes, r)
	}
	return nil
}

func (f *Feed) service(id string) int32 {
	if i, ok := f.svcIdx[id]; ok {
		return i
	}
	if f.svcIdx == nil {
		f.svcIdx = map[string]int32{}
	}
	f.svcIdx[id] = int32(len(f.Services))
	f.Services = append(f.Services, Service{ID: id, Added: map[int]bool{}, Removed: map[int]bool{}})
	return int32(len(f.Services) - 1)
}

func (f *Feed) readCalendar(t *table) error {
	days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	for t.next() {
		s := &f.Services[f.service(t.get("service_id"))]
		for i, d := range days {
			s.Weekdays[i] = t.get(d) == "1"
		}
		s.Start, s.End = t.int("start_date"), t.int("end_date")
	}
	return nil
}

func (f *Feed) readCalendarDates(t *table) error {
	for t.next() {
		s := &f.Services[f.service(t.get("service_id"))]
		d := t.int("date")
		switch t.get("exception_type") {
		case "1":
			s.Added[d] = true
		case "2":
			s.Removed[d] = true
		}
	}
	return nil
}

func (f *Feed) readShapes(t *table, idx map[string]int32) error {
	type pt struct {
		seq int
		p   model.LatLon
	}
	tmp := map[string][]pt{}
	for t.next() {
		id := t.get("shape_id")
		tmp[id] = append(tmp[id], pt{t.int("shape_pt_sequence"), model.LatLon{t.float("shape_pt_lat"), t.float("shape_pt_lon")}})
	}
	for id, pts := range tmp {
		sort.Slice(pts, func(a, b int) bool { return pts[a].seq < pts[b].seq })
		line := make([]model.LatLon, len(pts))
		for i, p := range pts {
			line[i] = p.p
		}
		idx[id] = int32(len(f.Shapes))
		f.Shapes = append(f.Shapes, line)
	}
	return nil
}

func (f *Feed) readTrips(t *table, shapeIdx map[string]int32) error {
	for t.next() {
		ri, ok := f.RouteIdx[t.get("route_id")]
		if !ok {
			continue
		}
		sh := int32(-1)
		if s, ok := shapeIdx[t.get("shape_id")]; ok {
			sh = s
		}
		f.Trips = append(f.Trips, Trip{
			ID:       t.get("trip_id"),
			Route:    ri,
			Service:  f.service(t.get("service_id")),
			Headsign: textnorm.Clean(t.get("trip_headsign")),
			Dir:      int8(t.int("direction_id")),
			Shape:    sh,
		})
	}
	return nil
}

func (f *Feed) readStopTimes(t *table) error {
	tripIdx := make(map[string]int32, len(f.Trips))
	for i, tr := range f.Trips {
		tripIdx[tr.ID] = int32(i)
	}
	type raw struct {
		seq int32
		st  StopTime
		ok  bool
	}
	per := make([][]raw, len(f.Trips))
	for t.next() {
		ti, ok := tripIdx[t.get("trip_id")]
		if !ok {
			continue
		}
		si, ok := f.StopIdx[t.get("stop_id")]
		if !ok {
			continue
		}
		arr, okA := ParseTime(t.get("arrival_time"))
		dep, okD := ParseTime(t.get("departure_time"))
		if !okD {
			dep, okD = arr, okA
		}
		if !okA {
			arr = dep
		}
		per[ti] = append(per[ti], raw{int32(t.int("stop_sequence")), StopTime{si, arr, dep}, okD})
	}
	f.TripStops = make([][]StopTime, len(f.Trips))
	for ti, rs := range per {
		sort.Slice(rs, func(a, b int) bool { return rs[a].seq < rs[b].seq })
		out := make([]StopTime, len(rs))
		// Fill missing (non-timepoint) times by linear interpolation.
		lastKnown := -1
		for i, r := range rs {
			out[i] = r.st
			if !r.ok {
				continue
			}
			if lastKnown >= 0 && i-lastKnown > 1 {
				a, b := out[lastKnown].Dep, r.st.Arr
				for j := lastKnown + 1; j < i; j++ {
					v := a + (b-a)*int32(j-lastKnown)/int32(i-lastKnown)
					out[j].Arr, out[j].Dep = v, v
				}
			}
			lastKnown = i
		}
		f.TripStops[ti] = out
	}
	return nil
}

func (f *Feed) buildIndexes() {
	f.StopDeps = make([][]StopDep, len(f.Stops))
	lines := make([]map[int32]bool, len(f.Stops))
	best := map[[2]int32]int{}
	for ti, sts := range f.TripStops {
		tr := f.Trips[ti]
		key := [2]int32{tr.Route, int32(tr.Dir)}
		if n := len(sts); n > best[key] {
			best[key] = n
			f.RepTrip[key] = int32(ti)
		}
		for pos, st := range sts {
			if pos < len(sts)-1 { // the last stop has no departure
				f.StopDeps[st.Stop] = append(f.StopDeps[st.Stop], StopDep{int32(ti), int32(pos), st.Dep})
			}
			if lines[st.Stop] == nil {
				lines[st.Stop] = map[int32]bool{}
			}
			lines[st.Stop][tr.Route] = true
		}
	}
	f.StopLines = make([][]int32, len(f.Stops))
	for si, deps := range f.StopDeps {
		sort.Slice(deps, func(a, b int) bool { return deps[a].Dep < deps[b].Dep })
		for r := range lines[si] {
			f.StopLines[si] = append(f.StopLines[si], r)
		}
		sort.Slice(f.StopLines[si], func(a, b int) bool { return f.StopLines[si][a] < f.StopLines[si][b] })
	}
}

// ---- queries ----

func dateInt(t time.Time) int { return t.Year()*10000 + int(t.Month())*100 + t.Day() }

// Active reports whether service s runs on the service day d.
func (f *Feed) Active(s int32, d time.Time) bool {
	svc := &f.Services[s]
	di := dateInt(d)
	if svc.Removed[di] {
		return false
	}
	if svc.Added[di] {
		return true
	}
	return di >= svc.Start && di <= svc.End && svc.Weekdays[d.Weekday()]
}

// Departure is a scheduled departure from a stop.
type Departure struct {
	Trip int32
	Pos  int32
	At   time.Time
}

// NextDepartures returns departures from stop si between now and
// now+window, at most limit, in time order. It looks at yesterday's
// service day too so that after-midnight trips (25:10:00) are found.
func (f *Feed) NextDepartures(si int32, now time.Time, window time.Duration, limit int) []Departure {
	if int(si) >= len(f.StopDeps) {
		return nil
	}
	deps := f.StopDeps[si]
	var out []Departure
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, day := range []time.Time{today.AddDate(0, 0, -1), today} {
		from := int32(now.Sub(day).Seconds()) - 30
		to := int32(now.Add(window).Sub(day).Seconds())
		i := sort.Search(len(deps), func(i int) bool { return deps[i].Dep >= from })
		for ; i < len(deps) && deps[i].Dep <= to; i++ {
			d := deps[i]
			if !f.Active(f.Trips[d.Trip].Service, day) {
				continue
			}
			out = append(out, Departure{d.Trip, d.Pos, day.Add(time.Duration(d.Dep) * time.Second)})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].At.Before(out[b].At) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Headsign returns the trip's headsign, or the name of its last stop.
func (f *Feed) Headsign(ti int32) string {
	if h := f.Trips[ti].Headsign; h != "" {
		return h
	}
	sts := f.TripStops[ti]
	if len(sts) == 0 {
		return ""
	}
	return f.Stops[sts[len(sts)-1].Stop].Name
}
