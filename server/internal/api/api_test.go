package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/keremtrgl/hibro/server/internal/city"
	"github.com/keremtrgl/hibro/server/internal/model"
)

type fakeSource struct{ arrErr error }

func (fakeSource) Prefix() string           { return "x" }
func (fakeSource) Info() model.SourceInfo   { return model.SourceInfo{Name: "fake"} }
func (fakeSource) Features() model.Features { return model.Features{LiveArrivals: true} }
func (fakeSource) Stops() []model.Stop {
	return []model.Stop{
		{ID: "x:1", Name: "Konak", Lat: 38.4189, Lon: 27.1287, Mode: model.ModeBus},
		{ID: "x:2", Name: "Üçyol", Lat: 38.4062, Lon: 27.1209, Mode: model.ModeSubway},
	}
}
func (fakeSource) Lines() []model.Line {
	return []model.Line{{ID: "x:12", Short: "12", Name: "Konak - Üçyol"}, {ID: "x:5", Short: "5", Name: "Narlıdere"}}
}
func (fakeSource) Line(_ context.Context, id string, _ time.Time) (*model.LineDetail, error) {
	if id != "5" {
		return nil, city.ErrNotFound
	}
	return &model.LineDetail{Line: model.Line{ID: "x:5", Short: "5"}}, nil
}
func (f fakeSource) Arrivals(context.Context, string, time.Time) ([]model.Arrival, error) {
	if f.arrErr != nil {
		return nil, f.arrErr
	}
	return []model.Arrival{{LineID: "x:5", Line: "5", Minutes: 2, Realtime: true}}, nil
}
func (fakeSource) Vehicles(context.Context, string) ([]model.Vehicle, error) { return nil, nil }
func (fakeSource) Alerts(context.Context, time.Time) []model.Alert           { return nil }

func newTestServer(src city.Source) *httptest.Server {
	reg := city.NewRegistry()
	reg.Put(city.New(city.Meta{ID: "test", Name: "Test"}, src))
	return httptest.NewServer((&Server{Cities: reg}).Handler())
}

func getJSON(t *testing.T, url string, v any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func TestEndpoints(t *testing.T) {
	ts := newTestServer(fakeSource{})
	defer ts.Close()

	var near []city.NearbyStop
	if code := getJSON(t, ts.URL+"/v1/cities/test/stops/nearby?lat=38.4189&lon=27.1287&radius=500", &near); code != 200 || len(near) != 1 || near[0].ID != "x:1" {
		t.Fatalf("nearby %d %+v", code, near)
	}
	var arr arrivalsResp
	if code := getJSON(t, ts.URL+"/v1/cities/test/stops/x:1/arrivals", &arr); code != 200 || !arr.Live || len(arr.Arrivals) != 1 {
		t.Fatalf("arrivals %d %+v", code, arr)
	}
	if code := getJSON(t, ts.URL+"/v1/cities/test/stops/x:99/arrivals", nil); code != 404 {
		t.Fatalf("unknown stop -> %d", code)
	}
	if code := getJSON(t, ts.URL+"/v1/cities/nope/lines", nil); code != 404 {
		t.Fatalf("unknown city -> %d", code)
	}
	var cities []model.CityInfo
	getJSON(t, ts.URL+"/v1/cities", &cities)
	if len(cities) != 1 || cities[0].Bounds[0] > 38.4062 || cities[0].Bounds[2] < 38.4189 {
		t.Fatalf("city bounds should cover all stops: %+v", cities)
	}
	var lines []model.Line
	getJSON(t, ts.URL+"/v1/cities/test/lines", &lines)
	if len(lines) != 2 || lines[0].Short != "5" {
		t.Fatalf("lines should be sorted numerically: %+v", lines)
	}
	var sr city.SearchResult
	getJSON(t, ts.URL+"/v1/cities/test/search?q=UCYOL", &sr)
	if len(sr.Stops) != 1 || len(sr.Lines) != 1 {
		t.Fatalf("search %+v", sr)
	}
	if code := getJSON(t, ts.URL+"/v1/cities/test/plan?from=1,2&to=3,4", nil); code != http.StatusNotImplemented {
		t.Fatalf("plan without MOTIS -> %d", code)
	}
}

func TestDegradedArrivals(t *testing.T) {
	ts := newTestServer(fakeSource{arrErr: io.ErrUnexpectedEOF})
	defer ts.Close()
	var arr arrivalsResp
	if code := getJSON(t, ts.URL+"/v1/cities/test/stops/x:1/arrivals", &arr); code != 200 || !arr.Degraded {
		t.Fatalf("upstream failure should be reported as degraded: %d %+v", code, arr)
	}
}

func TestPackGzipAndETag(t *testing.T) {
	ts := newTestServer(fakeSource{})
	defer ts.Close()
	req, _ := http.NewRequest("GET", ts.URL+"/v1/cities/test/pack", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("pack not gzipped")
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var p model.Pack
	if err := json.NewDecoder(zr).Decode(&p); err != nil || len(p.Stops) != 2 {
		t.Fatalf("pack decode %v %d", err, len(p.Stops))
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	req.Header.Set("If-None-Match", etag)
	resp, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 304 || len(body) != 0 || resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("304: code=%d body=%d enc=%q", resp.StatusCode, len(body), resp.Header.Get("Content-Encoding"))
	}
}
