// Package geo provides distance helpers and a grid index for fast
// "stops near me" lookups without a database.
package geo

import (
	"math"
	"sort"
)

const earthRadiusM = 6371000.0

// DistanceM returns the great-circle distance in metres.
func DistanceM(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * earthRadiusM * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// cellDeg is roughly 550 m of latitude: small enough that a 500 m query
// touches at most a 3x3 block of cells.
const cellDeg = 0.005

type cell struct{ x, y int32 }

// Index is an immutable spatial grid over points identified by int.
type Index struct {
	cells map[cell][]int32
	lat   []float64
	lon   []float64
}

func cellOf(lat, lon float64) cell {
	return cell{int32(math.Floor(lon / cellDeg)), int32(math.Floor(lat / cellDeg))}
}

// NewIndex builds an index over parallel lat/lon slices.
func NewIndex(lat, lon []float64) *Index {
	ix := &Index{cells: make(map[cell][]int32), lat: lat, lon: lon}
	for i := range lat {
		c := cellOf(lat[i], lon[i])
		ix.cells[c] = append(ix.cells[c], int32(i))
	}
	return ix
}

type Hit struct {
	Index     int
	DistanceM float64
}

// Near returns up to limit points within radiusM, closest first.
func (ix *Index) Near(lat, lon, radiusM float64, limit int) []Hit {
	if ix == nil || len(ix.lat) == 0 {
		return nil
	}
	dLat := radiusM / 111320.0
	cosLat := math.Cos(lat * math.Pi / 180)
	if cosLat < 0.01 {
		cosLat = 0.01
	}
	dLon := radiusM / (111320.0 * cosLat)
	lo, hi := cellOf(lat-dLat, lon-dLon), cellOf(lat+dLat, lon+dLon)
	var hits []Hit
	for x := lo.x; x <= hi.x; x++ {
		for y := lo.y; y <= hi.y; y++ {
			for _, i := range ix.cells[cell{x, y}] {
				d := DistanceM(lat, lon, ix.lat[i], ix.lon[i])
				if d <= radiusM {
					hits = append(hits, Hit{int(i), d})
				}
			}
		}
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].DistanceM < hits[b].DistanceM })
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}
