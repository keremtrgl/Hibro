package geo

import (
	"math"
	"testing"
)

func TestDistance(t *testing.T) {
	// Konak to Alsancak, about 2.2 km.
	d := DistanceM(38.4189, 27.1287, 38.4382, 27.1428)
	if math.Abs(d-2465) > 150 {
		t.Fatalf("distance = %.0f", d)
	}
}

func TestNear(t *testing.T) {
	lat := []float64{38.4189, 38.4195, 38.4382, 37.87}
	lon := []float64{27.1287, 27.1290, 27.1428, 32.48}
	ix := NewIndex(lat, lon)
	hits := ix.Near(38.4190, 27.1288, 500, 10)
	if len(hits) != 2 || hits[0].Index != 0 || hits[1].Index != 1 {
		t.Fatalf("hits = %+v", hits)
	}
	if got := ix.Near(38.4190, 27.1288, 5000, 1); len(got) != 1 {
		t.Fatalf("limit not applied: %+v", got)
	}
}
