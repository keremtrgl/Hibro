package izmir

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/keremtrgl/hibro/server/internal/fetch"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseLines(t *testing.T) {
	ls, err := ParseLines(read(t, "hatlar.json"))
	if err != nil || len(ls) != 3 {
		t.Fatalf("%v %d", err, len(ls))
	}
	var five Line
	for _, l := range ls {
		if l.No == "5" {
			five = l
		}
	}
	if five.To != "Üçkuyular İskele" || five.From != "Narlıdere" {
		t.Fatalf("line 5 = %+v", five)
	}
}

func TestParseStopsCoordinates(t *testing.T) {
	ss, err := ParseStops(read(t, "duraklar.json"))
	if err != nil || len(ss) == 0 {
		t.Fatalf("%v %d", err, len(ss))
	}
	for _, s := range ss {
		if s.Lat < 37.5 || s.Lat > 39.5 || s.Lon < 26 || s.Lon > 28.5 {
			t.Fatalf("stop %s outside İzmir: %f,%f", s.Code, s.Lat, s.Lon)
		}
	}
}

func TestParseSchedule(t *testing.T) {
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, istanbul)
	m, err := ParseSchedule(read(t, "hareket.json"), now)
	if err != nil {
		t.Fatal(err)
	}
	wk := m[SchedKey{"5", 1, "weekday"}]
	if len(wk) == 0 || wk[0] != "06:00" {
		t.Fatalf("line 5 weekday = %v", wk)
	}
}

func TestParseShapes(t *testing.T) {
	f, _ := os.Open("testdata/guzergah.csv")
	defer f.Close()
	m, err := ParseShapes(f)
	if err != nil {
		t.Fatal(err)
	}
	sh := m[ShapeKey{"5", 1}]
	if len(sh) < 2 || len(sh) >= 300 {
		t.Fatalf("shape not simplified: %d points", len(sh))
	}
	if sh[0][0] < 38 || sh[0][1] < 26 {
		t.Fatalf("shape axes wrong: %v", sh[0])
	}
}

func TestParseAlerts(t *testing.T) {
	f, _ := os.Open("testdata/duyuru.csv")
	defer f.Close()
	as, err := ParseAlerts(f, "b")
	if err != nil || len(as) == 0 {
		t.Fatalf("%v %d", err, len(as))
	}
	merged := false
	for _, a := range as {
		if strings.Contains(a.Title, "GÜMRÜK") && len(a.Lines) > 1 {
			merged = true
		}
	}
	if !merged {
		t.Fatalf("alerts with the same title should merge: %+v", as)
	}
}

func TestParseArrivals(t *testing.T) {
	now := time.Date(2026, 11, 26, 11, 25, 0, 0, istanbul)
	as, err := ParseArrivals(read(t, "yaklasan.json"), "b", now)
	if err != nil || len(as) != 1 {
		t.Fatalf("%v %+v", err, as)
	}
	a := as[0]
	if a.Line != "19" || a.Minutes != 3 || !a.Realtime || a.StopsAway != 5 || a.Headsign != "Konak" {
		t.Fatalf("arrival = %+v", a)
	}
	if a.Lat < 38.39 || a.Lat > 38.40 || a.Lon < 27.10 || a.Lon > 27.11 {
		t.Fatalf("vehicle position missing: %f,%f", a.Lat, a.Lon)
	}
	if _, err := ParseArrivals([]byte("  "), "b", now); err == nil {
		t.Fatal("empty body must be an error so it is retried")
	}
	// The same arrival 10 minutes later: its GPS fix is stale, so it is dropped.
	stale, err := ParseArrivals(read(t, "yaklasan.json"), "b", now.Add(10*time.Minute))
	if err != nil || len(stale) != 0 {
		t.Fatalf("stale GPS fix should be dropped: %v %+v", err, stale)
	}
	none, err := ParseArrivals([]byte(`{"Data":{"data":null,"error":{"code":1000,"message":"Durak bulunamadı."}}}`), "b", now)
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown stop: %v %v", err, none)
	}
}

func TestParseVehicles(t *testing.T) {
	vs, err := ParseVehicles(read(t, "konum.json"), "b:5")
	if err != nil || len(vs) != 2 {
		t.Fatalf("%v %+v", err, vs)
	}
	if vs[0].Lat != 38.404885 || vs[0].Lon != 27.063175 {
		t.Fatalf("duplicate should keep latest fix, axes lat/lon: %+v", vs[0])
	}
}

func TestSourceWithFakeUpstream(t *testing.T) {
	lines, _ := ParseLines(read(t, "hatlar.json"))
	stops, _ := ParseStops(read(t, "duraklar.json"))
	e := NewESHOT("b", &Static{Lines: lines, Stops: stops, Night: map[string]bool{"910": true}}, fetch.New(t.TempDir()), DefaultEndpoints)
	calls := 0
	e.Fetch = func(ctx context.Context, u string) ([]byte, error) {
		calls++
		return read(t, "yaklasan.json"), nil
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := e.Arrivals(ctx, "12345", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("arrivals not cached: %d upstream calls", calls)
	}
	if _, err := e.Arrivals(ctx, "999999", time.Now()); err != ErrNotFound {
		t.Fatalf("unknown stop err = %v", err)
	}
	night := false
	for _, l := range e.Lines() {
		if l.Short == "910" {
			night = l.Night
		}
	}
	if !night {
		t.Fatal("910 should be a night (Baykuş) line")
	}
}

func TestRoutingFeedFromDeparturesAndOffsets(t *testing.T) {
	lines, _ := ParseLines(read(t, "hatlar.json"))
	stops, _ := ParseStops(read(t, "duraklar.json"))
	st := &Static{
		Lines: lines, Stops: stops,
		Schedule:    map[SchedKey][]string{{"19", 1, "weekday"}: {"07:00", "07:30"}},
		StopSeq:     map[ShapeKey][]string{{"19", 1}: {"12337", "12345"}},
		StopOffsets: map[ShapeKey][]int32{{"19", 1}: {0, 240}},
	}
	f, err := RoutingFeed(st)
	if err != nil || f == nil {
		t.Fatalf("%v %v", err, f)
	}
	if len(f.Trips) != 2 {
		t.Fatalf("trips = %d", len(f.Trips))
	}
	if got := f.TripStops[1][1].Arr; got != 7*3600+30*60+240 {
		t.Fatalf("second trip arrival at 2nd stop = %d", got)
	}
}
