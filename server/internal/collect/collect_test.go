package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAppendScanCleanup(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	day1 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return day1 }
	s.Append("reports", Report{Kind: "crowded", City: "izmir", LineID: "b:5", At: day1})
	s.Now = func() time.Time { return day1.AddDate(0, 0, 40) }
	s.Append("reports", Report{Kind: "no_show", City: "izmir", LineID: "b:5", At: day1})
	if days := s.Days("reports"); len(days) != 2 || days[0] != "2026-12-11" {
		t.Fatalf("days = %v", days)
	}
	n := 0
	s.Scan("reports", "2026-12-11", func(b []byte) { n++ })
	if n != 1 {
		t.Fatalf("scan = %d", n)
	}
	if removed := s.Cleanup("reports", 30*24*time.Hour); removed != 1 {
		t.Fatalf("cleanup removed %d", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", "2026-11-01.jsonl")); !os.IsNotExist(err) {
		t.Fatal("old file still present")
	}
}

func TestReportClean(t *testing.T) {
	r := Report{Kind: "crowded", LineID: "b:5", Plate: "35 da 7284", Note: string(make([]rune, 400))}
	if err := r.Clean(); err != nil {
		t.Fatal(err)
	}
	if r.Plate != "35DA7284" || len([]rune(r.Note)) > 280 {
		t.Fatalf("%+v", r)
	}
	bad := Report{Kind: "hack", LineID: "b:5"}
	if bad.Clean() == nil {
		t.Fatal("unknown kind accepted")
	}
	none := Report{Kind: "crowded"}
	if none.Clean() == nil {
		t.Fatal("report without stop or line accepted")
	}
}

func TestRecentCount(t *testing.T) {
	rc := NewRecent(2 * time.Hour)
	now := time.Now()
	rc.Add(Report{Kind: "crowded", City: "izmir", LineID: "b:5", At: now.Add(-50 * time.Minute)})
	rc.Add(Report{Kind: "crowded", City: "izmir", LineID: "b:5", At: now.Add(-5 * time.Minute)})
	if n := rc.Count("izmir", "b:5", "crowded", 20*time.Minute, now); n != 1 {
		t.Fatalf("count = %d", n)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(6, 3)
	ok := 0
	for i := 0; i < 10; i++ {
		if l.Allow("1.2.3.4") {
			ok++
		}
	}
	if ok != 3 {
		t.Fatalf("burst allowed %d", ok)
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("other client blocked")
	}
}

func TestCountersWhitelist(t *testing.T) {
	s, _ := NewStore(t.TempDir(), time.UTC)
	c := NewCounters(s)
	n := c.Add("izmir", map[string]int{"screen_stop": 3, "user_email": 1, "search": 1000})
	if n != 2 {
		t.Fatalf("accepted %d", n)
	}
	_, m := c.Snapshot()
	if m["izmir/screen_stop"] != 3 || m["izmir/search"] != 500 || len(m) != 2 {
		t.Fatalf("%v", m)
	}
	c.Flush()
	c2 := NewCounters(s)
	if _, m2 := c2.Snapshot(); m2["izmir/screen_stop"] != 3 {
		t.Fatalf("snapshot not reloaded: %v", m2)
	}
}

func TestTripCleanTrimsEnds(t *testing.T) {
	now := time.Now()
	area := Bounds{38, 26.5, 39, 27.5}
	var pts [][5]float64
	// 60 points, 50 m apart going north-east: about 3 km.
	for i := 0; i < 60; i++ {
		pts = append(pts, [5]float64{float64(now.Add(time.Duration(i-60) * 10 * time.Second).Unix()), 38.40 + float64(i)*0.00045, 27.10, 10, 5})
	}
	pts = append(pts, [5]float64{float64(now.Unix()) + 1, 0, 0, 10, 5}) // outside area
	tr := TripUpload{Session: "abcdefghijklmnop", LineID: "b:5", Points: pts}
	if err := tr.Clean(now, area); err != nil {
		t.Fatal(err)
	}
	if len(tr.Points) >= 60 || len(tr.Points) < 40 {
		t.Fatalf("trim produced %d points", len(tr.Points))
	}
	b, _ := json.Marshal(tr)
	if len(b) == 0 {
		t.Fatal("marshal")
	}
	short := TripUpload{Session: "abcdefghijklmnop", LineID: "b:5", Points: pts[:12]}
	if short.Clean(now, area) == nil {
		t.Fatal("tiny trip inside trim radius should be rejected")
	}
	badSession := TripUpload{Session: "x", LineID: "b:5", Points: pts}
	if badSession.Clean(now, area) == nil {
		t.Fatal("bad session accepted")
	}
}
