package geo

import "testing"

func TestEncodePolyline(t *testing.T) {
	// Example from Google's documentation.
	got := EncodePolyline([][2]float64{{38.5, -120.2}, {40.7, -120.95}, {43.252, -126.453}})
	if want := "_p~iF~ps|U_ulLnnqC_mqNvxq`@"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
