package raptor

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/model"
)

func feed(t *testing.T, files map[string]string) *gtfs.Feed {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, b := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(b))
	}
	zw.Close()
	f, err := gtfs.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// A (27.000) --R1--> B (27.020) ; B --walk 200 m--> C ; C --R2--> D (27.040)
// Stops are ~1.7 km apart so the planner cannot just walk to C.
// R3 goes A -> D directly but slowly and later.
var net = map[string]string{
	"stops.txt":  "stop_id,stop_name,stop_lat,stop_lon\nA,A,38.000,27.000\nB,B,38.000,27.020\nC,C,38.0018,27.020\nD,D,38.000,27.040\n",
	"routes.txt": "route_id,route_short_name,route_long_name,route_type\nR1,1,A-B,3\nR2,2,C-D,0\nR3,3,A-D,3\n",
	"trips.txt":  "route_id,service_id,trip_id,trip_headsign\nR1,S,t1,B\nR2,S,t2,D\nR2,S,t2b,D\nR3,S,t3,D\n",
	"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\n" +
		"t1,08:00:00,08:00:00,A,1\nt1,08:10:00,08:10:00,B,2\n" +
		"t2,08:12:00,08:12:00,C,1\nt2,08:20:00,08:20:00,D,2\n" + // too tight: walk B->C takes ~3 min
		"t2b,08:20:00,08:20:00,C,1\nt2b,08:28:00,08:28:00,D,2\n" +
		"t3,08:05:00,08:05:00,A,1\nt3,08:50:00,08:50:00,D,2\n",
	"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nS,1,1,1,1,1,1,1,20260101,20271231\n",
}

func TestPlanWithTransferAndAlternative(t *testing.T) {
	src := gtfs.NewSource("x", feed(t, net), model.SourceInfo{})
	p := New(src)
	at := time.Date(2026, 11, 26, 7, 55, 0, 0, time.UTC)
	its := p.Plan(model.LatLon{38.000, 27.000}, model.LatLon{38.000, 27.040}, at, 3)
	if len(its) < 2 {
		t.Fatalf("want direct and transfer options, got %+v", its)
	}
	first := its[0]
	// Fastest: 1 at 08:00, walk to C, tram 2 at 08:20 (08:12 is missed), arrive 08:28.
	if first.End.Format("15:04") != "08:28" || first.Transfers != 1 {
		t.Fatalf("best = %s transfers=%d legs=%+v", first.End.Format("15:04"), first.Transfers, first.Legs)
	}
	var lines []string
	for _, l := range first.Legs {
		lines = append(lines, l.Mode+":"+l.Line)
	}
	if got := len(first.Legs); got != 3 || first.Legs[0].Line != "1" || first.Legs[1].Mode != "WALK" || first.Legs[2].Mode != "TRAM" {
		t.Fatalf("legs = %v", lines)
	}
	if first.Legs[2].From.ID != "x:C" || first.Legs[2].Start.Format("15:04") != "08:20" {
		t.Fatalf("transfer leg = %+v", first.Legs[2])
	}
	direct := false
	for _, it := range its {
		if it.Transfers == 0 && it.Legs[0].Line == "3" {
			direct = true
		}
	}
	if !direct {
		t.Fatalf("the slower direct line should be offered as an alternative: %+v", its)
	}
}

func TestNoServiceNoRoute(t *testing.T) {
	src := gtfs.NewSource("x", feed(t, net), model.SourceInfo{})
	p := New(src)
	late := time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC)
	if its := p.Plan(model.LatLon{38.000, 27.000}, model.LatLon{38.000, 27.040}, late, 3); len(its) != 0 {
		t.Fatalf("no trips after 23:00, got %+v", its)
	}
}

// TestRealCity plans in Konya when HIBRO_TEST_GTFS_DIR is set.
func TestRealCity(t *testing.T) {
	dir := os.Getenv("HIBRO_TEST_GTFS_DIR")
	if dir == "" {
		t.Skip("HIBRO_TEST_GTFS_DIR not set")
	}
	b, err := os.ReadFile(dir + "/konya.zip")
	if err != nil {
		t.Skip(err)
	}
	f, err := gtfs.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	p := New(gtfs.NewSource("k", f, model.SourceInfo{}))
	st, pats := p.Stats()
	t.Logf("built: %d stops, %d patterns in %s", st, pats, time.Since(start).Round(time.Millisecond))
	loc, _ := time.LoadLocation("Europe/Istanbul")
	at := time.Date(2026, 11, 26, 8, 0, 0, 0, loc)
	// Alaaddin Tepesi -> Selçuk Üniversitesi Kampüs (about 12 km).
	start = time.Now()
	its := p.Plan(model.LatLon{37.8728, 32.4923}, model.LatLon{38.0263, 32.5108}, at, 3)
	t.Logf("query took %s", time.Since(start).Round(time.Millisecond))
	if len(its) == 0 {
		t.Fatal("no itinerary in Konya")
	}
	for _, it := range its {
		var legs []string
		for _, l := range it.Legs {
			if l.Mode == "WALK" {
				legs = append(legs, "yürü "+itoa(l.DistanceM)+"m")
			} else {
				legs = append(legs, l.Line+" "+l.From.Name+"→"+l.To.Name+" "+l.Start.Format("15:04")+"-"+l.End.Format("15:04"))
			}
		}
		t.Logf("%s→%s %d dk, %d aktarma: %v", it.Start.Format("15:04"), it.End.Format("15:04"), it.DurationS/60, it.Transfers, legs)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
