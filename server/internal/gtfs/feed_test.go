package gtfs

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/keremtrgl/hibro/server/internal/model"
)

func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

var tiny = map[string]string{
	"agency.txt": "agency_id,agency_name\nA,Test Ulaşım\n",
	"stops.txt":  "\ufeffstop_id,stop_name,stop_lat,stop_lon\nS1,Konak,38.41,27.12\nS2,Çankaya,38.42,27.13\nS3,Basmane,38.43,27.14\nX,Kullanılmayan,38.5,27.5\n",
	"routes.txt": "route_id,agency_id,route_short_name,route_long_name,route_type,route_color\nR1,A,M1,Konak - Basmane,1,FF0000\n",
	"trips.txt":  "route_id,service_id,trip_id,trip_headsign,direction_id\nR1,WK,T1,,0\nR1,WK,T2,Basmane,0\nR1,WE,T3,Basmane,0\nR1,WK,T4,Konak,1\n",
	// T1 is listed out of order and has an untimed middle stop.
	"stop_times.txt":     "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nT1,08:10:00,08:10:00,S3,3\nT1,08:00:00,08:00:00,S1,1\nT1,,,S2,2\nT2,24:30:00,24:30:00,S1,1\nT2,24:40:00,24:40:00,S3,2\nT3,09:00:00,09:00:00,S1,1\nT3,09:10:00,09:10:00,S3,2\nT4,10:00:00,10:00:00,S3,1\nT4,10:10:00,10:10:00,S1,2\n",
	"calendar.txt":       "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nWK,1,1,1,1,1,0,0,20260101,20271231\nWE,0,0,0,0,0,1,1,20260101,20271231\n",
	"calendar_dates.txt": "service_id,date,exception_type\nWK,20261127,2\n",
}

func TestParseAndQuery(t *testing.T) {
	f, err := Parse(buildZip(t, tiny))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Stops) != 4 || len(f.Trips) != 4 {
		t.Fatalf("stops=%d trips=%d", len(f.Stops), len(f.Trips))
	}
	// Interpolated middle stop: 08:05.
	if got := f.TripStops[0][1].Dep; got != 8*3600+5*60 {
		t.Fatalf("interpolated dep = %d", got)
	}
	loc, _ := time.LoadLocation("Europe/Istanbul")
	thu := time.Date(2026, 11, 26, 7, 55, 0, 0, loc) // Thursday
	src := NewSource("m", f, model.SourceInfo{Name: "test"})

	arr, err := src.Arrivals(context.Background(), "S1", thu)
	if err != nil || len(arr) == 0 {
		t.Fatalf("arrivals err=%v %+v", err, arr)
	}
	if arr[0].Time != "08:00" || arr[0].Minutes != 5 || arr[0].Headsign != "Basmane" || arr[0].LineID != "m:R1" {
		t.Fatalf("first arrival %+v", arr[0])
	}
	// After midnight: T2 (24:30 on Thursday's service) is found on Friday 00:20.
	fri := time.Date(2026, 11, 27, 0, 20, 0, 0, loc)
	arr, _ = src.Arrivals(context.Background(), "S1", fri)
	if len(arr) != 1 || arr[0].Time != "00:30" {
		t.Fatalf("after-midnight arrivals %+v", arr)
	}
	// Friday 27th WK is removed via calendar_dates, so no 08:00 departure.
	arr, _ = src.Arrivals(context.Background(), "S1", fri.Add(7*time.Hour))
	if len(arr) != 0 {
		t.Fatalf("removed service still listed: %+v", arr)
	}

	stops := src.Stops()
	if len(stops) != 3 {
		t.Fatalf("unused stop should be hidden, got %d stops", len(stops))
	}
	if stops[0].Mode != model.ModeSubway {
		t.Fatalf("mode %s", stops[0].Mode)
	}
	ld, err := src.Line(context.Background(), "R1", thu)
	if err != nil || len(ld.Directions) != 2 {
		t.Fatalf("line detail %v %+v", err, ld)
	}
	if ld.Directions[0].Headsign != "Basmane" || len(ld.Directions[0].Stops) != 3 {
		t.Fatalf("dir0 %+v", ld.Directions[0])
	}
	if got := ld.Directions[0].Departures; len(got) != 2 || got[0] != "08:00" || got[1] != "00:30" {
		t.Fatalf("timetable %v", got)
	}
}

// TestRealFeeds parses the real municipal feeds when HIBRO_TEST_GTFS_DIR
// points to a folder holding them (skipped in CI).
func TestRealFeeds(t *testing.T) {
	dir := os.Getenv("HIBRO_TEST_GTFS_DIR")
	if dir == "" {
		t.Skip("HIBRO_TEST_GTFS_DIR not set")
	}
	for _, name := range []string{"konya.zip", "gaziantep.zip", "izmir_metro.zip", "izmir_tram.zip"} {
		b, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			t.Logf("skip %s: %v", name, err)
			continue
		}
		start := time.Now()
		f, err := Parse(b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		src := NewSource("x", f, model.SourceInfo{})
		t.Logf("%s: %d stops (%d served), %d lines, %d trips, parsed in %s",
			name, len(f.Stops), len(src.Stops()), len(src.Lines()), len(f.Trips), time.Since(start).Round(time.Millisecond))
	}
}

func TestMergesRoutesPublishedPerDirection(t *testing.T) {
	files := map[string]string{}
	for k, v := range tiny {
		files[k] = v
	}
	files["routes.txt"] = "route_id,agency_id,route_short_name,route_long_name,route_type\nR1,A,M1,Konak - Basmane,1\nR2,A,M1,Basmane - Konak,1\n"
	files["trips.txt"] = "route_id,service_id,trip_id,trip_headsign,direction_id\nR1,WK,T1,,0\nR1,WK,T2,Basmane,0\nR1,WE,T3,Basmane,0\nR2,WK,T4,Konak,1\n"
	f, err := Parse(buildZip(t, files))
	if err != nil {
		t.Fatal(err)
	}
	src := NewSource("m", f, model.SourceInfo{})
	if ls := src.Lines(); len(ls) != 1 || ls[0].ID != "m:R1" {
		t.Fatalf("lines = %+v", ls)
	}
	for _, st := range src.Stops() {
		if len(st.Lines) != 1 || st.Lines[0] != "m:R1" {
			t.Fatalf("stop %s lines = %v", st.ID, st.Lines)
		}
	}
	ld, err := src.Line(context.Background(), "R2", time.Date(2026, 11, 26, 7, 0, 0, 0, time.UTC))
	if err != nil || len(ld.Directions) != 2 || ld.Directions[1].Headsign != "Konak" {
		t.Fatalf("merged detail: %v %+v", err, ld)
	}
}

func TestStaleCalendarUsesWeekdayPattern(t *testing.T) {
	files := map[string]string{}
	for k, v := range tiny {
		files[k] = v
	}
	// The feed's calendar ended in 2018.
	files["calendar.txt"] = "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nWK,1,1,1,1,1,0,0,20171203,20181231\nWE,0,0,0,0,0,1,1,20171203,20181231\n"
	raw := map[string][]byte{}
	for k, v := range files {
		raw[k] = []byte(v)
	}
	z, err := ZipFiles(raw)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(z)
	if err != nil {
		t.Fatal(err)
	}
	if f.ServiceEnd() != 20181231 {
		t.Fatalf("service end %d", f.ServiceEnd())
	}
	src := NewSource("k", f, model.SourceInfo{})
	thu := time.Date(2026, 11, 26, 7, 55, 0, 0, time.UTC)
	if arr, _ := src.Arrivals(context.Background(), "S1", thu); len(arr) != 0 {
		t.Fatalf("expired calendar should show nothing by default: %+v", arr)
	}
	f.StaleCalendar = true
	arr, _ := src.Arrivals(context.Background(), "S1", thu)
	if len(arr) == 0 || arr[0].Time != "08:00" {
		t.Fatalf("stale mode should use the weekday pattern: %+v", arr)
	}
}
