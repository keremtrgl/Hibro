package gtfs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/keremtrgl/hibro/server/internal/geo"
	"github.com/keremtrgl/hibro/server/internal/model"
)

var ErrNotFound = errors.New("not found")

// Source exposes a parsed Feed through the city.Source interface.
// Arrivals are scheduled (not live) times.
type Source struct {
	prefix  string
	feed    *Feed
	info    model.SourceInfo
	stopIDs map[string]int32 // local stop id -> index (only boardable stops)
	// Some agencies publish each direction of one line as its own route
	// (İzmir Metro: route 1 "Evka 3 – Narlıdere" and route 2 the reverse,
	// both "M1"). Routes sharing agency, short name and type are merged
	// into one line; canon maps every route to the group's first route.
	canon   []int32
	members map[int32][]int32
}

func NewSource(prefix string, f *Feed, info model.SourceInfo) *Source {
	s := &Source{prefix: prefix, feed: f, info: info, stopIDs: map[string]int32{}, members: map[int32][]int32{}}
	for i, st := range f.Stops {
		if st.LocationType == 0 {
			s.stopIDs[st.ID] = int32(i)
		}
	}
	s.canon = make([]int32, len(f.Routes))
	first := map[string]int32{}
	for i, r := range f.Routes {
		ri := int32(i)
		s.canon[ri] = ri
		if r.Short != "" {
			key := r.Agency + "|" + r.Short + "|" + fmt.Sprint(r.Type)
			if c, ok := first[key]; ok {
				s.canon[ri] = c
			} else {
				first[key] = ri
			}
		}
		s.members[s.canon[ri]] = append(s.members[s.canon[ri]], ri)
	}
	return s
}

// lineID returns the public (merged) line id for a route index.
func (s *Source) lineID(ri int32) string { return s.id(s.feed.Routes[s.canon[ri]].ID) }

func (s *Source) Prefix() string           { return s.prefix }
func (s *Source) Info() model.SourceInfo   { return s.info }
func (s *Source) Features() model.Features { return model.Features{} }
func (s *Source) Feed() *Feed              { return s.feed }

func (s *Source) id(local string) string { return s.prefix + ":" + local }

func (s *Source) routeMode(ri int32) model.Mode { return model.ModeFromGTFS(s.feed.Routes[ri].Type) }

func (s *Source) Stops() []model.Stop {
	f := s.feed
	out := make([]model.Stop, 0, len(s.stopIDs))
	for i, st := range f.Stops {
		if st.LocationType != 0 || len(f.StopLines[i]) == 0 {
			continue // skip stations, entrances and stops no trip uses
		}
		mode := model.ModeBus
		lines := make([]string, 0, len(f.StopLines[i]))
		seen := map[string]bool{}
		best := 99
		for _, ri := range f.StopLines[i] {
			if id := s.lineID(ri); !seen[id] {
				seen[id] = true
				lines = append(lines, id)
			}
			m := s.routeMode(ri)
			if r := rank(m); r < best {
				best, mode = r, m
			}
		}
		out = append(out, model.Stop{
			ID: s.id(st.ID), Code: st.Code, Name: st.Name,
			Lat: st.Lat, Lon: st.Lon, Mode: mode, Lines: lines,
		})
	}
	return out
}

func rank(m model.Mode) int {
	switch m {
	case model.ModeSubway, model.ModeRail:
		return 0
	case model.ModeTram:
		return 1
	case model.ModeFerry:
		return 2
	}
	return 3
}

func (s *Source) line(ri int32) model.Line {
	r := s.feed.Routes[ri]
	short := r.Short
	if short == "" {
		short = r.Long
	}
	l := model.Line{
		ID: s.id(r.ID), Short: short, Name: r.Long, Mode: s.routeMode(ri),
		Color: r.Color, TextColor: r.TextColor,
	}
	if rep, ok := s.repTrip(ri); ok {
		sts := s.feed.TripStops[rep]
		if len(sts) > 0 {
			l.From = s.feed.Stops[sts[0].Stop].Name
			l.To = s.feed.Stops[sts[len(sts)-1].Stop].Name
		}
	}
	return l
}

// repTrip finds a representative trip for a (merged) line, preferring
// direction 0 of the first member route that has one.
func (s *Source) repTrip(ri int32) (int32, bool) {
	for _, dir := range []int32{0, 1} {
		for _, m := range s.members[s.canon[ri]] {
			if t, ok := s.feed.RepTrip[[2]int32{m, dir}]; ok {
				return t, true
			}
		}
	}
	return 0, false
}

func (s *Source) Lines() []model.Line {
	used := map[int32]bool{}
	for key := range s.feed.RepTrip {
		used[s.canon[key[0]]] = true
	}
	out := make([]model.Line, 0, len(used))
	for ri := range s.feed.Routes {
		if used[int32(ri)] {
			out = append(out, s.line(int32(ri)))
		}
	}
	return out
}

// DayType names the timetable column a date falls into.
func DayType(d time.Time) string {
	switch d.Weekday() {
	case time.Saturday:
		return "saturday"
	case time.Sunday:
		return "sunday"
	}
	return "weekday"
}

func clock(sec int32) string {
	h := (sec / 3600) % 24
	m := (sec % 3600) / 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

func (s *Source) Line(_ context.Context, local string, now time.Time) (*model.LineDetail, error) {
	f := s.feed
	ri, ok := f.RouteIdx[local]
	if !ok {
		return nil, ErrNotFound
	}
	ri = s.canon[ri]
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	d := &model.LineDetail{Line: s.line(ri), DayType: DayType(day)}
	type variant struct{ route, dir int32 }
	var variants []variant
	for _, dir := range []int32{0, 1} {
		for _, m := range s.members[ri] {
			if _, ok := f.RepTrip[[2]int32{m, dir}]; ok {
				variants = append(variants, variant{m, dir})
			}
		}
	}
	if len(variants) > 3 {
		variants = variants[:3]
	}
	for vi, v := range variants {
		rep := f.RepTrip[[2]int32{v.route, v.dir}]
		dd := model.Direction{Index: vi, Headsign: f.Headsign(rep)}
		for _, st := range f.TripStops[rep] {
			stop := f.Stops[st.Stop]
			dd.Stops = append(dd.Stops, model.StopRef{ID: s.id(stop.ID), Name: stop.Name, Lat: stop.Lat, Lon: stop.Lon})
		}
		if sh := f.Trips[rep].Shape; sh >= 0 {
			dd.Shape = simplify(f.Shapes[sh])
		} else {
			for _, st := range dd.Stops {
				dd.Shape = append(dd.Shape, model.LatLon{st.Lat, st.Lon})
			}
		}
		var deps []int32
		for ti, tr := range f.Trips {
			if tr.Route != v.route || int32(tr.Dir) != v.dir || len(f.TripStops[ti]) == 0 {
				continue
			}
			if f.Active(tr.Service, day) {
				deps = append(deps, f.TripStops[ti][0].Dep)
			}
		}
		sort.Slice(deps, func(a, b int) bool { return deps[a] < deps[b] })
		seen := map[string]bool{}
		for _, t := range deps {
			c := clock(t)
			if !seen[c] {
				seen[c] = true
				dd.Departures = append(dd.Departures, c)
			}
		}
		d.Directions = append(d.Directions, dd)
	}
	return d, nil
}

func simplify(pts []model.LatLon) []model.LatLon {
	in := make([][2]float64, len(pts))
	for i, p := range pts {
		in[i] = p
	}
	out := geo.Simplify(in, 4)
	res := make([]model.LatLon, len(out))
	for i, p := range out {
		res[i] = p
	}
	return res
}

func (s *Source) Arrivals(_ context.Context, local string, now time.Time) ([]model.Arrival, error) {
	f := s.feed
	si, ok := s.stopIDs[local]
	if !ok {
		return nil, ErrNotFound
	}
	deps := f.NextDepartures(si, now, 2*time.Hour, 40)
	out := make([]model.Arrival, 0, len(deps))
	// Keep at most 3 departures per line+headsign so one frequent line
	// does not push every other line off the screen.
	perKey := map[string]int{}
	for _, d := range deps {
		tr := f.Trips[d.Trip]
		r := f.Routes[tr.Route]
		hs := f.Headsign(d.Trip)
		key := r.ID + "|" + hs
		if perKey[key] >= 3 {
			continue
		}
		perKey[key]++
		short := r.Short
		if short == "" {
			short = r.Long
		}
		mins := int(d.At.Sub(now).Minutes())
		if mins < 0 {
			mins = 0
		}
		out = append(out, model.Arrival{
			LineID: s.lineID(tr.Route), Line: short, Headsign: hs, Mode: s.routeMode(tr.Route),
			Color: r.Color, Minutes: mins, Time: d.At.Format("15:04"),
			StopsAway: 0,
		})
	}
	return out, nil
}

func (s *Source) Vehicles(context.Context, string) ([]model.Vehicle, error) {
	return []model.Vehicle{}, nil
}
func (s *Source) Alerts(context.Context, time.Time) []model.Alert { return nil }
