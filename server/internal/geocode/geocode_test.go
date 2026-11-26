package geocode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const sample = `[{"lat":"40.8187372","lon":"29.9229080","name":"Kocaeli Üniversitesi","display_name":"Kocaeli Üniversitesi, Umuttepe, İzmit, Kocaeli, Türkiye","category":"amenity","type":"university","address":{"suburb":"Umuttepe","town":"İzmit","province":"Kocaeli"}},
{"lat":"41.1","lon":"30.1","name":"","display_name":"Kefken, Kandıra, Kocaeli, Türkiye","type":"village","address":{"county":"Kandıra","province":"Kocaeli"}}]`

func TestSearchConvertsAndCaches(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if q.Get("countrycodes") != "tr" || q.Get("bounded") != "1" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Geliyo") {
			t.Errorf("bad request: %s ua=%q", r.URL, r.Header.Get("User-Agent"))
		}
		w.Write([]byte(sample))
	}))
	defer srv.Close()
	n := NewNominatim(srv.URL, "Geliyo/test")
	b := [4]float64{40.4, 29.1, 41.2, 30.4}
	ps, err := n.Search(context.Background(), "kocaeli", "Kocaeli Üniversitesi", b)
	if err != nil || len(ps) != 2 {
		t.Fatalf("%v %+v", err, ps)
	}
	if ps[0].Kind != "Üniversite" || ps[0].Detail != "Umuttepe, İzmit" || ps[1].Name != "Kefken" || ps[1].Kind != "Köy" {
		t.Fatalf("%+v", ps)
	}
	n.Search(context.Background(), "kocaeli", "kocaeli universitesi", b) // same after folding
	if calls.Load() != 1 {
		t.Fatalf("not cached: %d calls", calls.Load())
	}
}
