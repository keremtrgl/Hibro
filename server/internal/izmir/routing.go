package izmir

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"

	"github.com/keremtrgl/hibro/server/internal/gtfs"
)

// RoutingFeed builds a GTFS timetable for ESHOT buses from two partial
// sources: today's departure times from the terminal (ESHOT API, current)
// and the stop order plus travel time between stops (ESHOT GTFS, older).
// Each departure becomes one trip whose stop times are departure + offset.
// The result is used only for journey planning; live arrivals still come
// from the ESHOT API.
func RoutingFeed(st *Static) (*gtfs.Feed, error) {
	if len(st.StopSeq) == 0 || len(st.Schedule) == 0 {
		return nil, nil
	}
	known := map[string]Stop{}
	for _, s := range st.Stops {
		known[s.Code] = s
	}
	files := map[string]*bytes.Buffer{}
	writers := map[string]*csv.Writer{}
	w := func(name string, header ...string) *csv.Writer {
		b := &bytes.Buffer{}
		cw := csv.NewWriter(b)
		_ = cw.Write(header)
		files[name], writers[name] = b, cw
		return cw
	}
	stops := w("stops.txt", "stop_id", "stop_name", "stop_lat", "stop_lon")
	routes := w("routes.txt", "route_id", "agency_id", "route_short_name", "route_long_name", "route_type")
	trips := w("trips.txt", "route_id", "service_id", "trip_id", "trip_headsign", "direction_id")
	times := w("stop_times.txt", "trip_id", "arrival_time", "departure_time", "stop_id", "stop_sequence")
	cal := w("calendar.txt", "service_id", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "start_date", "end_date")
	_ = cal.Write([]string{"weekday", "1", "1", "1", "1", "1", "0", "0", "20200101", "20991231"})
	_ = cal.Write([]string{"saturday", "0", "0", "0", "0", "0", "1", "0", "20200101", "20991231"})
	_ = cal.Write([]string{"sunday", "0", "0", "0", "0", "0", "0", "1", "20200101", "20991231"})
	w("agency.txt", "agency_id", "agency_name").Write([]string{"ESHOT", "ESHOT"})

	usedStops := map[string]bool{}
	nTrips := 0
	for _, l := range st.Lines {
		routeWritten := false
		for dir := 1; dir <= 2; dir++ {
			key := ShapeKey{l.No, dir}
			seq, offs := st.StopSeq[key], st.StopOffsets[key]
			if len(seq) < 2 || len(offs) != len(seq) {
				continue
			}
			head := l.To
			if dir == 2 {
				head = l.From
			}
			for _, dt := range []string{"weekday", "saturday", "sunday"} {
				for i, dep := range st.Schedule[SchedKey{l.No, dir, dt}] {
					base, ok := gtfs.ParseTime(dep + ":00")
					if !ok {
						continue
					}
					if !routeWritten {
						_ = routes.Write([]string{l.No, "ESHOT", l.No, l.Name, "3"})
						routeWritten = true
					}
					tid := fmt.Sprintf("%s_%d_%s_%d", l.No, dir, dt, i)
					_ = trips.Write([]string{l.No, dt, tid, head, fmt.Sprint(dir - 1)})
					for j, code := range seq {
						if _, ok := known[code]; !ok {
							continue
						}
						usedStops[code] = true
						t := clockHMS(base + offs[j])
						_ = times.Write([]string{tid, t, t, code, fmt.Sprint(j + 1)})
					}
					nTrips++
				}
			}
		}
	}
	if nTrips == 0 {
		return nil, errors.New("no trips")
	}
	for code := range usedStops {
		s := known[code]
		_ = stops.Write([]string{code, s.Name, fmt.Sprintf("%.6f", s.Lat), fmt.Sprintf("%.6f", s.Lon)})
	}
	raw := map[string][]byte{}
	for name, cw := range writers {
		cw.Flush()
		raw[name] = files[name].Bytes()
	}
	z, err := gtfs.ZipFiles(raw)
	if err != nil {
		return nil, err
	}
	return gtfs.Parse(z)
}

func clockHMS(sec int32) string {
	return fmt.Sprintf("%02d:%02d:%02d", sec/3600, sec%3600/60, sec%60)
}
