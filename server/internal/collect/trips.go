package collect

import (
	"errors"
	"math"
	"regexp"
	"time"

	"github.com/keremtrgl/hibro/server/internal/geo"
)

// TripUpload is an opt-in GPS trace recorded while a rider was on a
// vehicle ("Bu otobüse bindim" mode). The app already removes the start
// and end of the trip; the server trims again in case it did not.
type TripUpload struct {
	Session   string       `json:"session"` // random, rotates daily on the device
	LineID    string       `json:"lineId"`
	Direction int          `json:"direction"`
	Points    [][5]float64 `json:"points"` // unix seconds, lat, lon, accuracy m, speed m/s
	App       string       `json:"app,omitempty"`
}

type StoredTrip struct {
	TripUpload
	City     string    `json:"city"`
	Received time.Time `json:"received"`
}

var sessionRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// TrimM is how much of each end of a trip is removed so that traces
// never reveal where someone lives or works.
const TrimM = 300

// Bounds is a lat/lon box (city area).
type Bounds struct{ MinLat, MinLon, MaxLat, MaxLon float64 }

func (b Bounds) Contains(lat, lon float64) bool {
	return lat >= b.MinLat && lat <= b.MaxLat && lon >= b.MinLon && lon <= b.MaxLon
}

// Clean validates a trip and drops implausible or private points.
// It returns an error when nothing useful remains.
func (t *TripUpload) Clean(now time.Time, area Bounds) error {
	if !sessionRe.MatchString(t.Session) {
		return errors.New("geçersiz oturum")
	}
	if t.LineID == "" || len(t.LineID) > 64 {
		return errors.New("hat gerekli")
	}
	if len(t.Points) < 10 || len(t.Points) > 5000 {
		return errors.New("nokta sayısı geçersiz")
	}
	minT := float64(now.Add(-48 * time.Hour).Unix())
	maxT := float64(now.Add(5 * time.Minute).Unix())
	pts := t.Points[:0]
	var lastT float64
	for _, p := range t.Points {
		ts, lat, lon, acc, spd := p[0], p[1], p[2], p[3], p[4]
		if ts < minT || ts > maxT || ts <= lastT || !area.Contains(lat, lon) {
			continue
		}
		if acc <= 0 || acc > 80 || spd < 0 || spd > 40 || math.IsNaN(lat+lon) {
			continue
		}
		lastT = ts
		// Round to ~1 m; more precision adds nothing for transit.
		pts = append(pts, [5]float64{ts, round(lat, 5), round(lon, 5), math.Round(acc), round(spd, 1)})
	}
	pts = trimEnds(pts, TrimM)
	if len(pts) < 10 {
		return errors.New("yeterli nokta yok")
	}
	if pts[len(pts)-1][0]-pts[0][0] > 3*3600 {
		return errors.New("yolculuk çok uzun")
	}
	t.Points = pts
	if len(t.App) > 32 {
		t.App = t.App[:32]
	}
	return nil
}

func round(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}

// trimEnds removes points within m metres of the first and last point.
func trimEnds(pts [][5]float64, m float64) [][5]float64 {
	if len(pts) == 0 {
		return pts
	}
	first, last := pts[0], pts[len(pts)-1]
	start := 0
	for start < len(pts) && geo.DistanceM(first[1], first[2], pts[start][1], pts[start][2]) < m {
		start++
	}
	end := len(pts)
	for end > start && geo.DistanceM(last[1], last[2], pts[end-1][1], pts[end-1][2]) < m {
		end--
	}
	return pts[start:end]
}
