// Package raptor is Hibro's built-in journey planner. It implements
// RAPTOR (Round-bAsed Public Transit Optimized Router, Delling et al.
// 2012) over the GTFS feeds already held in memory, so every GTFS city
// gets "A'dan B'ye" routing without running a separate engine.
//
// Scope: one service day, up to 3 transfers, walking between nearby
// stops, straight-line walking distances with a detour factor.
package raptor

import (
	"sort"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/geo"
	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/model"
)

// Source is a GTFS feed with its public id mapping (gtfs.Source).
type Source interface {
	Feed() *gtfs.Feed
	LineID(route int32) string
	StopID(stop int32) string
}

const (
	inf          = int32(1 << 30)
	walkSpeed    = 1.25     // m/s
	detour       = 1.25     // straight line -> street distance
	accessRadius = 900      // m, origin/destination to stop
	transferMax  = 350      // m, walking transfer between stops
	minChange    = 60       // s, to change vehicles at the same stop
	maxRounds    = 4        // 3 transfers
	horizon      = 4 * 3600 // s: ignore departures more than 4 h after the query

	transferPenalty = 3 * time.Minute
)

func walkSecs(m float64) int32 { return int32(m * detour / walkSpeed) }

type stop struct {
	src      int
	idx      int32
	lat, lon float64
}

type pattern struct {
	src   int
	route int32
	stops []int32 // global stop indexes
	trips []int32 // feed trip indexes, sorted by departure at the first stop
}

type patPos struct{ pat, pos int32 }

type edge struct {
	to   int32
	secs int32
	m    int32
}

// Planner is immutable and safe for concurrent queries.
type Planner struct {
	srcs      []Source
	stops     []stop
	patterns  []pattern
	byStop    [][]patPos
	transfers [][]edge
	index     *geo.Index
}

// New builds the planner (patterns, stop index, walking transfers).
func New(srcs ...Source) *Planner {
	p := &Planner{srcs: srcs}
	type key struct {
		src int
		seq string
	}
	pats := map[key]int32{}
	for si, s := range srcs {
		f := s.Feed()
		base := int32(len(p.stops))
		for i, st := range f.Stops {
			p.stops = append(p.stops, stop{src: si, idx: int32(i), lat: st.Lat, lon: st.Lon})
		}
		for ti, tr := range f.Trips {
			sts := f.TripStops[ti]
			if len(sts) < 2 {
				continue
			}
			var b strings.Builder
			b.WriteString(f.Routes[tr.Route].ID)
			for _, x := range sts {
				b.WriteByte('|')
				b.WriteString(f.Stops[x.Stop].ID)
			}
			k := key{si, b.String()}
			pi, ok := pats[k]
			if !ok {
				gs := make([]int32, len(sts))
				for i, x := range sts {
					gs[i] = base + x.Stop
				}
				pi = int32(len(p.patterns))
				pats[k] = pi
				p.patterns = append(p.patterns, pattern{src: si, route: tr.Route, stops: gs})
			}
			p.patterns[pi].trips = append(p.patterns[pi].trips, int32(ti))
		}
	}
	p.byStop = make([][]patPos, len(p.stops))
	for pi := range p.patterns {
		pt := &p.patterns[pi]
		f := srcs[pt.src].Feed()
		sort.Slice(pt.trips, func(a, b int) bool {
			return f.TripStops[pt.trips[a]][0].Dep < f.TripStops[pt.trips[b]][0].Dep
		})
		for pos, s := range pt.stops {
			p.byStop[s] = append(p.byStop[s], patPos{int32(pi), int32(pos)})
		}
	}
	lat := make([]float64, len(p.stops))
	lon := make([]float64, len(p.stops))
	for i, s := range p.stops {
		lat[i], lon[i] = s.lat, s.lon
	}
	p.index = geo.NewIndex(lat, lon)
	p.transfers = make([][]edge, len(p.stops))
	for i, s := range p.stops {
		if len(p.byStop[i]) == 0 {
			continue
		}
		for _, h := range p.index.Near(s.lat, s.lon, transferMax, 16) {
			if h.Index == i || len(p.byStop[h.Index]) == 0 {
				continue
			}
			p.transfers[i] = append(p.transfers[i], edge{int32(h.Index), walkSecs(h.DistanceM), int32(h.DistanceM)})
		}
	}
	return p
}

// Stats for logs.
func (p *Planner) Stats() (stops, patterns int) { return len(p.stops), len(p.patterns) }

type kind uint8

const (
	none kind = iota
	access
	transit
	transfer
)

type label struct {
	kind       kind
	fromRound  int8  // round of the label at `from` (transit legs)
	from       int32 // previous stop
	pat, trip  int32
	bpos, apos int32
	walkM      int32
}

type query struct {
	p      *Planner
	day    time.Time
	active [][]bool // per source, per service
	limit  int32    // latest departure considered (seconds after midnight)
}

func (q *query) dep(pt *pattern, trip int32, pos int32) int32 {
	return q.p.srcs[pt.src].Feed().TripStops[trip][pos].Dep
}

func (q *query) arr(pt *pattern, trip int32, pos int32) int32 {
	return q.p.srcs[pt.src].Feed().TripStops[trip][pos].Arr
}

// earliest returns the first active trip of pt leaving pos at or after t.
func (q *query) earliest(pt *pattern, pos, t int32) int32 {
	f := q.p.srcs[pt.src].Feed()
	trips := pt.trips
	i := sort.Search(len(trips), func(i int) bool { return f.TripStops[trips[i]][pos].Dep >= t })
	for ; i < len(trips); i++ {
		tr := trips[i]
		d := f.TripStops[tr][pos].Dep
		if d > q.limit {
			return -1 // also skips "tomorrow" trips encoded as 24:00+
		}
		if d >= t && q.active[pt.src][f.Trips[tr].Service] {
			return tr
		}
	}
	return -1
}

// Plan returns up to `want` itineraries leaving from `from` at or after
// `at`, ordered by arrival. It retries with later departures to offer
// alternatives (the next connection), like "range" queries.
func (p *Planner) Plan(from, to model.LatLon, at time.Time, want int) []model.Itinerary {
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	q := &query{p: p, day: day, active: make([][]bool, len(p.srcs))}
	for si, s := range p.srcs {
		f := s.Feed()
		q.active[si] = make([]bool, len(f.Services))
		for i := range f.Services {
			q.active[si][i] = f.Active(int32(i), day)
		}
	}
	var out []model.Itinerary
	seen := map[string]bool{}
	t := int32(at.Sub(day).Seconds())
	if d := geo.DistanceM(from[0], from[1], to[0], to[1]); d <= 1500 {
		w := walkSecs(d)
		out = append(out, model.Itinerary{
			DurationS: int(w), Start: day.Add(time.Duration(t) * time.Second), End: day.Add(time.Duration(t+w) * time.Second),
			WalkM: int(d * detour),
			Legs: []model.Leg{{Mode: "WALK", From: model.StopRef{Name: "Başlangıç", Lat: from[0], Lon: from[1]},
				To: model.StopRef{Name: "Varış", Lat: to[0], Lon: to[1]}, DistanceM: int(d * detour),
				Start: day.Add(time.Duration(t) * time.Second), End: day.Add(time.Duration(t+w) * time.Second)}},
		})
	}
	for attempt := 0; attempt < 3 && len(out) < want+1; attempt++ {
		its := q.run(from, to, t)
		next := inf
		for _, it := range its {
			k := signature(it)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, it)
			for _, l := range it.Legs {
				if l.Mode != "WALK" {
					if s := int32(l.Start.Sub(day).Seconds()); s < next {
						next = s
					}
					break
				}
			}
		}
		if next == inf {
			break
		}
		// Ask again just after the first boarding to find the next one.
		walkFirst := int32(0)
		if len(its) > 0 && its[0].Legs[0].Mode == "WALK" {
			walkFirst = int32(its[0].Legs[0].End.Sub(its[0].Legs[0].Start).Seconds())
		}
		t = next - walkFirst + 60
	}
	// Rank by arrival with a 3-minute penalty per transfer: riders prefer
	// fewer changes over arriving a minute earlier.
	score := func(it model.Itinerary) time.Time {
		return it.End.Add(time.Duration(it.Transfers) * transferPenalty)
	}
	sort.SliceStable(out, func(i, j int) bool { return score(out[i]).Before(score(out[j])) })
	// Drop options dominated by one with fewer transfers that arrives at
	// most 3 minutes later (e.g. a 1-minute bus hop instead of a short walk).
	dominated := func(it model.Itinerary) bool {
		for _, o := range out {
			if o.Transfers < it.Transfers && !o.End.After(it.End.Add(transferPenalty)) && !o.Start.Before(it.Start.Add(-10*time.Minute)) {
				return true
			}
		}
		return false
	}
	kept := out[:0:0]
	for _, it := range out {
		if !dominated(it) {
			kept = append(kept, it)
		}
	}
	out = kept
	// Drop options that are much worse than the best one.
	if len(out) > 1 {
		bestEnd, bestDur := out[0].End, out[0].DurationS
		keep := out[:0]
		for _, it := range out {
			late := it.End.Sub(bestEnd) > 45*time.Minute
			slow := it.DurationS > 2*bestDur+600
			if !late && !slow {
				keep = append(keep, it)
			}
		}
		out = keep
	}
	if len(out) > want {
		out = out[:want]
	}
	return out
}

func signature(it model.Itinerary) string {
	var b strings.Builder
	for _, l := range it.Legs {
		if l.Mode != "WALK" {
			b.WriteString(l.Line + "@" + l.Start.Format("15:04") + "/")
		}
	}
	return b.String()
}

func (q *query) run(from, to model.LatLon, t0 int32) []model.Itinerary {
	p := q.p
	q.limit = t0 + horizon
	n := len(p.stops)
	tau := make([][]int32, maxRounds+1)
	labels := make([][]label, maxRounds+1)
	for k := range tau {
		tau[k] = make([]int32, n)
		labels[k] = make([]label, n)
		for i := range tau[k] {
			tau[k][i] = inf
		}
	}
	best := make([]int32, n)
	roundOf := make([]int8, n) // round in which best[s] was reached
	for i := range best {
		best[i] = inf
	}
	prevArr := make([]int32, n)
	prevRound := make([]int8, n)
	marked := make([]bool, n)
	var markedList []int32
	mark := func(s int32) {
		if !marked[s] {
			marked[s] = true
			markedList = append(markedList, s)
		}
	}
	for _, h := range p.index.Near(from[0], from[1], accessRadius, 60) {
		s := int32(h.Index)
		if len(p.byStop[s]) == 0 {
			continue
		}
		a := t0 + walkSecs(h.DistanceM)
		tau[0][s], best[s], roundOf[s] = a, a, 0
		labels[0][s] = label{kind: access, from: -1, walkM: int32(h.DistanceM * detour)}
		mark(s)
	}
	egress := p.index.Near(to[0], to[1], accessRadius, 60)
	targetBest := inf
	type result struct {
		k, stop int32
		arr     int32
		walkM   int32
	}
	var results []result

	for k := 1; k <= maxRounds && len(markedList) > 0; k++ {
		// Boarding uses the best arrival from any earlier round.
		copy(prevArr, best)
		copy(prevRound, roundOf)
		// Patterns to scan, from their earliest marked position.
		queue := map[int32]int32{}
		for _, s := range markedList {
			for _, pp := range p.byStop[s] {
				if cur, ok := queue[pp.pat]; !ok || pp.pos < cur {
					queue[pp.pat] = pp.pos
				}
			}
			marked[s] = false
		}
		markedList = markedList[:0]
		for pi, pos0 := range queue {
			pt := &p.patterns[pi]
			trip, bpos := int32(-1), int32(-1)
			for i := pos0; i < int32(len(pt.stops)); i++ {
				s := pt.stops[i]
				if trip >= 0 {
					a := q.arr(pt, trip, i)
					if a < best[s] && a < targetBest {
						tau[k][s], best[s], roundOf[s] = a, a, int8(k)
						labels[k][s] = label{kind: transit, from: pt.stops[bpos], fromRound: prevRound[pt.stops[bpos]], pat: pi, trip: trip, bpos: bpos, apos: i}
						mark(s)
					}
				}
				prev := prevArr[s]
				if prev == inf {
					continue
				}
				if labels[prevRound[s]][s].kind == transit {
					prev += minChange
				}
				if trip < 0 || prev <= q.dep(pt, trip, i) {
					if nt := q.earliest(pt, i, prev); nt >= 0 && (trip < 0 || q.dep(pt, nt, i) < q.dep(pt, trip, i)) {
						trip, bpos = nt, i
					}
				}
			}
		}
		// Walking transfers from stops improved in this round.
		improved := append([]int32(nil), markedList...)
		for _, s := range improved {
			for _, e := range p.transfers[s] {
				a := tau[k][s] + e.secs
				if a < best[e.to] && a < targetBest {
					tau[k][e.to], best[e.to], roundOf[e.to] = a, a, int8(k)
					labels[k][e.to] = label{kind: transfer, from: s, walkM: int32(float64(e.m) * detour)}
					mark(e.to)
				}
			}
		}
		// Destination check.
		for _, h := range egress {
			s := int32(h.Index)
			if tau[k][s] == inf {
				continue
			}
			a := tau[k][s] + walkSecs(h.DistanceM)
			if a < targetBest {
				targetBest = a
				if len(results) > 0 && results[len(results)-1].k == int32(k) {
					results[len(results)-1] = result{int32(k), s, a, int32(h.DistanceM * detour)}
				} else {
					results = append(results, result{int32(k), s, a, int32(h.DistanceM * detour)})
				}
			}
		}
	}
	var its []model.Itinerary
	for _, r := range results {
		if it, ok := q.build(labels, int(r.k), r.stop, r.arr, r.walkM, from, to); ok {
			its = append(its, it)
		}
	}
	return its
}

func (q *query) at(s int32) time.Time { return q.day.Add(time.Duration(s) * time.Second) }

func (q *query) ref(gs int32) model.StopRef {
	st := q.p.stops[gs]
	src := q.p.srcs[st.src]
	fs := src.Feed().Stops[st.idx]
	return model.StopRef{ID: src.StopID(st.idx), Name: fs.Name, Lat: fs.Lat, Lon: fs.Lon}
}

var modeNames = map[model.Mode]string{
	model.ModeBus: "BUS", model.ModeTram: "TRAM", model.ModeSubway: "SUBWAY",
	model.ModeRail: "RAIL", model.ModeFerry: "FERRY", model.ModeCable: "CABLE",
}

// build walks the labels back from the egress stop to the origin.
func (q *query) build(labels [][]label, k int, s int32, arrive, egressM int32, from, to model.LatLon) (model.Itinerary, bool) {
	var legs []model.Leg
	cur, r := s, k
	for guard := 0; guard < 20; guard++ {
		l := labels[r][cur]
		switch l.kind {
		case transfer:
			legs = append(legs, model.Leg{Mode: "WALK", From: q.ref(l.from), To: q.ref(cur), DistanceM: int(l.walkM)})
			cur = l.from
		case transit:
			pt := &q.p.patterns[l.pat]
			src := q.p.srcs[pt.src]
			f := src.Feed()
			rt := f.Routes[pt.route]
			short := rt.Short
			if short == "" {
				short = rt.Long
			}
			note := ""
			if in, ok := src.(interface{ Info() model.SourceInfo }); ok {
				switch info := in.Info(); {
				case info.Stale:
					note = "eski tarife"
				case info.Estimated:
					note = "tahmini saat"
				}
			}
			legs = append(legs, model.Leg{
				Note: note,
				Mode: modeNames[model.ModeFromGTFS(rt.Type)], From: q.ref(pt.stops[l.bpos]), To: q.ref(pt.stops[l.apos]),
				Start: q.at(q.dep(pt, l.trip, l.bpos)), End: q.at(q.arr(pt, l.trip, l.apos)),
				Line: short, Headsign: f.Headsign(l.trip), Color: rt.Color, NumStops: int(l.apos - l.bpos),
			})
			cur = l.from
			r = int(l.fromRound)
		case access:
			legs = append(legs, model.Leg{Mode: "WALK", From: model.StopRef{Name: "Başlangıç", Lat: from[0], Lon: from[1]}, To: q.ref(cur), DistanceM: int(l.walkM)})
			guard = 99
		default:
			return model.Itinerary{}, false
		}
		if guard == 99 {
			break
		}
	}
	// Reverse to travel order.
	for i, j := 0, len(legs)-1; i < j; i, j = i+1, j-1 {
		legs[i], legs[j] = legs[j], legs[i]
	}
	if egressM > 0 {
		legs = append(legs, model.Leg{Mode: "WALK", From: q.ref(s), To: model.StopRef{Name: "Varış", Lat: to[0], Lon: to[1]}, DistanceM: int(egressM)})
	}
	// Give walking legs times: leave just in time for the first vehicle,
	// walk transfers right after arriving.
	walk := func(m int) time.Duration { return time.Duration(float64(m)/walkSpeed) * time.Second }
	for i := range legs {
		if legs[i].Mode != "WALK" {
			continue
		}
		if i == 0 && len(legs) > 1 {
			legs[i].End = legs[1].Start
			legs[i].Start = legs[i].End.Add(-walk(legs[i].DistanceM))
		} else if i > 0 {
			legs[i].Start = legs[i-1].End
			legs[i].End = legs[i].Start.Add(walk(legs[i].DistanceM))
		}
	}
	// Merge consecutive walks and drop zero-length ones.
	clean := legs[:0]
	for _, l := range legs {
		if n := len(clean); n > 0 && l.Mode == "WALK" && clean[n-1].Mode == "WALK" {
			clean[n-1].To = l.To
			clean[n-1].End = l.End
			clean[n-1].DistanceM += l.DistanceM
			continue
		}
		clean = append(clean, l)
	}
	legs = clean[:0]
	for _, l := range clean {
		if l.Mode == "WALK" && l.DistanceM < 15 {
			continue
		}
		legs = append(legs, l)
	}
	if len(legs) == 0 {
		return model.Itinerary{}, false
	}
	it := model.Itinerary{Start: legs[0].Start, End: legs[len(legs)-1].End, Legs: legs}
	it.DurationS = int(it.End.Sub(it.Start).Seconds())
	rides := 0
	for _, l := range legs {
		if l.Mode == "WALK" {
			it.WalkM += l.DistanceM
		} else {
			rides++
		}
	}
	if rides == 0 || !it.End.After(it.Start) {
		return model.Itinerary{}, false
	}
	it.Transfers = rides - 1
	_ = arrive
	return it, true
}
