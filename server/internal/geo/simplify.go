package geo

import "math"

// Simplify reduces a polyline with the Douglas-Peucker algorithm.
// tolM is the maximum allowed deviation in metres. Points are [lat, lon].
// Municipal shape files are very dense (a point every few metres); this
// typically shrinks them 5-10x with no visible difference on a phone.
func Simplify(pts [][2]float64, tolM float64) [][2]float64 {
	if len(pts) < 3 {
		return pts
	}
	keep := make([]bool, len(pts))
	keep[0], keep[len(pts)-1] = true, true
	type span struct{ a, b int }
	stack := []span{{0, len(pts) - 1}}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		maxD, idx := 0.0, -1
		for i := s.a + 1; i < s.b; i++ {
			if d := perpM(pts[i], pts[s.a], pts[s.b]); d > maxD {
				maxD, idx = d, i
			}
		}
		if idx >= 0 && maxD > tolM {
			keep[idx] = true
			stack = append(stack, span{s.a, idx}, span{idx, s.b})
		}
	}
	out := make([][2]float64, 0, len(pts)/4+2)
	for i, k := range keep {
		if k {
			out = append(out, pts[i])
		}
	}
	return out
}

// perpM is the distance from p to segment ab using a local equirectangular
// projection, accurate enough at city scale.
func perpM(p, a, b [2]float64) float64 {
	k := math.Cos(a[0] * math.Pi / 180)
	ax, ay := a[1]*k*111320, a[0]*111320
	bx, by := b[1]*k*111320, b[0]*111320
	px, py := p[1]*k*111320, p[0]*111320
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / l2
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}
