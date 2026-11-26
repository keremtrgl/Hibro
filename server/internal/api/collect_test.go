package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/keremtrgl/hibro/server/internal/city"
	"github.com/keremtrgl/hibro/server/internal/collect"
)

func newCollectServer(t *testing.T) (*httptest.Server, *Collect, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := collect.NewStore(dir, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	col := &Collect{
		Store: store, Recent: collect.NewRecent(time.Hour), Counters: collect.NewCounters(store),
		Limit: collect.NewLimiter(60, 5), TeleLimit: collect.NewLimiter(60, 5), AdminToken: "secret",
	}
	reg := city.NewRegistry()
	reg.Put(city.New(city.Meta{ID: "test", Name: "Test"}, fakeSource{}))
	ts := httptest.NewServer((&Server{Cities: reg, Collect: col}).Handler())
	t.Cleanup(ts.Close)
	return ts, col, dir
}

func post(t *testing.T, url, body string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestReportFlow(t *testing.T) {
	ts, _, dir := newCollectServer(t)
	if c := post(t, ts.URL+"/v1/cities/test/reports", `{"kind":"crowded","lineId":"x:5","stopId":"x:1"}`); c != 201 {
		t.Fatalf("report -> %d", c)
	}
	if c := post(t, ts.URL+"/v1/cities/test/reports", `{"kind":"crowded","lineId":"x:999"}`); c != 400 {
		t.Fatalf("unknown line -> %d", c)
	}
	if c := post(t, ts.URL+"/v1/cities/test/reports", `{"kind":"crowded","lineId":"x:5","email":"a@b.c"}`); c != 400 {
		t.Fatalf("unknown field must be rejected -> %d", c)
	}
	// The crowded report now shows on arrivals of that line.
	var arr arrivalsResp
	getJSON(t, ts.URL+"/v1/cities/test/stops/x:1/arrivals", &arr)
	if len(arr.Arrivals) != 1 || !arr.Arrivals[0].Crowded {
		t.Fatalf("crowded flag missing: %+v", arr.Arrivals)
	}
	// Stored on disk without any client address.
	b, err := os.ReadFile(dir + "/reports/" + time.Now().UTC().Format("2006-01-02") + ".jsonl")
	if err != nil || !bytes.Contains(b, []byte(`"kind":"crowded"`)) || bytes.Contains(b, []byte("127.0.0.1")) {
		t.Fatalf("stored report: %s %v", b, err)
	}
}

func TestReportRateLimit(t *testing.T) {
	ts, _, _ := newCollectServer(t)
	codes := map[int]int{}
	for i := 0; i < 8; i++ {
		codes[post(t, ts.URL+"/v1/cities/test/reports", `{"kind":"no_show","lineId":"x:5"}`)]++
	}
	if codes[201] != 5 || codes[429] != 3 {
		t.Fatalf("codes = %v", codes)
	}
}

func TestTelemetryAndAdmin(t *testing.T) {
	ts, _, _ := newCollectServer(t)
	if c := post(t, ts.URL+"/v1/telemetry", `{"city":"test","events":{"screen_stop":2,"secret_thing":1}}`); c != 200 {
		t.Fatalf("telemetry -> %d", c)
	}
	post(t, ts.URL+"/v1/cities/test/reports", `{"kind":"wrong_stop","stopId":"x:1","note":"Durak 50 m ileride"}`)

	req, _ := http.NewRequest("GET", ts.URL+"/admin/stats", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Fatalf("admin without token -> %d", resp.StatusCode)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("admin -> %v %d", err, resp.StatusCode)
	}
	var out struct {
		Reports struct {
			Total       int              `json:"total"`
			RecentNotes []collect.Report `json:"recentNotes"`
		} `json:"reports"`
		Telemetry map[string]map[string]int64 `json:"telemetry"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	today := time.Now().UTC().Format("2006-01-02")
	if out.Reports.Total != 1 || len(out.Reports.RecentNotes) != 1 || out.Telemetry[today]["test/screen_stop"] != 2 {
		t.Fatalf("stats = %+v", out)
	}
	if _, leaked := out.Telemetry[today]["test/secret_thing"]; leaked {
		t.Fatal("non-whitelisted event stored")
	}
}

func TestCollectDisabled(t *testing.T) {
	ts := newTestServer(fakeSource{})
	defer ts.Close()
	if c := post(t, ts.URL+"/v1/telemetry", `{"city":"test","events":{"app_open":1}}`); c != 503 {
		t.Fatalf("disabled collect -> %d", c)
	}
}
