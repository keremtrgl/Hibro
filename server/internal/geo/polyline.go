package geo

import "strings"

// EncodePolyline encodes [lat, lon] points in Google's polyline format
// with 5 decimal precision (about 1 m), the format map SDKs understand.
func EncodePolyline(pts [][2]float64) string {
	var b strings.Builder
	var pLat, pLon int64
	enc := func(v int64) {
		v <<= 1
		if v < 0 {
			v = ^v
		}
		for v >= 0x20 {
			b.WriteByte(byte((0x20 | (v & 0x1f)) + 63))
			v >>= 5
		}
		b.WriteByte(byte(v + 63))
	}
	for _, p := range pts {
		lat := int64(round5(p[0]))
		lon := int64(round5(p[1]))
		enc(lat - pLat)
		enc(lon - pLon)
		pLat, pLon = lat, lon
	}
	return b.String()
}

func round5(v float64) float64 {
	if v < 0 {
		return float64(int64(v*1e5 - 0.5))
	}
	return float64(int64(v*1e5 + 0.5))
}
