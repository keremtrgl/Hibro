package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/keremtrgl/hibro/server/internal/fetch"
	"github.com/keremtrgl/hibro/server/internal/gtfs"
)

// A CKAN portal that publishes GTFS as loose files plus unrelated
// resources (like veri.kocaeli.bel.tr and data.ibb.gov.tr).
func TestCKANLooseGTFS(t *testing.T) {
	files := map[string]string{
		"/f/stops.txt":      "stop_id,stop_name,stop_lat,stop_lon\nA,A,41.0,29.0\nB,B,41.01,29.01\n",
		"/f/routes.txt":     "route_id,route_short_name,route_long_name,route_type\nR,500T,Test,3\n",
		"/f/trips.txt":      "route_id,service_id,trip_id\nR,S,T1\n",
		"/f/stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nT1,08:00:00,08:00:00,A,1\nT1,08:10:00,08:10:00,B,2\n",
		"/f/calendar.txt":   "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nS,1,1,1,1,1,1,1,20260101,20301231\n",
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/3/action/package_show" {
			res := []map[string]string{
				{"name": "Duraklar", "url": srv.URL + "/f/stops.txt", "format": "TXT"},
				{"name": "routes", "url": srv.URL + "/f/routes.txt", "format": "TXT"},
				{"name": "trips", "url": srv.URL + "/f/trips.txt", "format": "TXT"},
				{"name": "stop_times", "url": srv.URL + "/f/stop_times.txt", "format": "TXT"},
				{"name": "calendar", "url": srv.URL + "/f/calendar.txt", "format": "TXT"},
				{"name": "Veri sözlüğü", "url": srv.URL + "/f/sozluk.pdf", "format": "PDF"},
				{"name": "shapes", "url": srv.URL + "/f/missing-shapes.txt", "format": "TXT"},
			}
			json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{"resources": res}})
			return
		}
		if b, ok := files[r.URL.Path]; ok {
			w.Write([]byte(b))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	fc := fetch.New(t.TempDir())
	z, _, err := ckanLooseGTFS(context.Background(), fc, srv.URL, "x")
	if err != nil {
		t.Fatalf("loose GTFS: %v", err)
	}
	f, err := gtfs.Parse(z)
	if err != nil || len(f.Stops) != 2 || len(f.Trips) != 1 {
		t.Fatalf("parse: %v stops=%d", err, len(f.Stops))
	}
}

func TestCommunityAndOfficialLoaders(t *testing.T) {
	for _, id := range []string{"izmir", "konya", "gaziantep", "kocaeli", "istanbul", "antalya", "erzurum"} {
		if loaders[id] == nil {
			t.Errorf("no loader for %s", id)
		}
	}
}
