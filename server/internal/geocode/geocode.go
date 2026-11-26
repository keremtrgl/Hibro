// Package geocode turns place names ("Kocaeli Üniversitesi", "Kefken")
// into coordinates so riders can plan a route to any place, not only to
// a stop. It uses an OpenStreetMap Nominatim server through Geliyo's own
// backend, which keeps the rider's IP private, caches answers and obeys
// the public server's usage policy (max 1 request per second, identified
// User-Agent). Set HIBRO_GEOCODER_URL to use a self-hosted Nominatim.
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keremtrgl/hibro/server/internal/cache"
	"github.com/keremtrgl/hibro/server/internal/geo"
	"github.com/keremtrgl/hibro/server/internal/textnorm"
)

type Place struct {
	Name   string  `json:"name"`
	Detail string  `json:"detail,omitempty"` // district, city
	Kind   string  `json:"kind,omitempty"`   // university, hospital, village...
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

type Nominatim struct {
	Base      string
	UserAgent string
	HTTP      *http.Client

	mu    sync.Mutex
	last  time.Time
	cache *cache.TTL[[]Place]
}

func NewNominatim(base, userAgent string) *Nominatim {
	if base == "" {
		base = "https://nominatim.openstreetmap.org"
	}
	return &Nominatim{
		Base: strings.TrimRight(base, "/"), UserAgent: userAgent,
		HTTP:  &http.Client{Timeout: 10 * time.Second},
		cache: cache.New[[]Place](7*24*time.Hour, time.Minute, 20000),
	}
}

// wait enforces at most one upstream request per second.
func (n *Nominatim) wait(ctx context.Context) error {
	n.mu.Lock()
	next := n.last.Add(1100 * time.Millisecond)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	n.last = next
	n.mu.Unlock()
	select {
	case <-time.After(time.Until(next)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Search finds places matching q inside bounds [minLat, minLon, maxLat, maxLon].
func (n *Nominatim) Search(ctx context.Context, cityID, q string, bounds [4]float64) ([]Place, error) {
	key := cityID + "|" + textnorm.Fold(q)
	return n.cache.Get(ctx, key, func(ctx context.Context) ([]Place, error) {
		if err := n.wait(ctx); err != nil {
			return nil, err
		}
		v := url.Values{}
		v.Set("q", q)
		v.Set("format", "jsonv2")
		v.Set("limit", "8")
		v.Set("countrycodes", "tr")
		v.Set("accept-language", "tr")
		v.Set("addressdetails", "1")
		if bounds != [4]float64{} {
			v.Set("viewbox", fmt.Sprintf("%.4f,%.4f,%.4f,%.4f", bounds[1], bounds[2], bounds[3], bounds[0]))
			v.Set("bounded", "1")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.Base+"/search?"+v.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", n.UserAgent)
		resp, err := n.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("geocoder: HTTP %d", resp.StatusCode)
		}
		var raw []nominatimResult
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}
		return convert(raw), nil
	})
}

type nominatimResult struct {
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Type        string            `json:"type"`
	Category    string            `json:"category"`
	Address     map[string]string `json:"address"`
}

var kinds = map[string]string{
	"university": "Üniversite", "college": "Yüksekokul", "school": "Okul", "hospital": "Hastane",
	"clinic": "Sağlık merkezi", "mall": "AVM", "stadium": "Stadyum", "park": "Park", "beach": "Plaj",
	"village": "Köy", "town": "İlçe", "suburb": "Mahalle", "neighbourhood": "Mahalle", "quarter": "Mahalle",
	"station": "İstasyon", "bus_station": "Otogar", "aerodrome": "Havalimanı", "ferry_terminal": "İskele",
	"mosque": "Cami", "museum": "Müze", "townhall": "Belediye", "police": "Karakol", "marketplace": "Pazar yeri",
	"residential": "Cadde/sokak", "primary": "Cadde", "secondary": "Cadde", "tertiary": "Cadde",
}

func convert(raw []nominatimResult) []Place {
	out := make([]Place, 0, len(raw))
	seen := map[string]bool{}
	for _, r := range raw {
		lat, e1 := strconv.ParseFloat(r.Lat, 64)
		lon, e2 := strconv.ParseFloat(r.Lon, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		parts := strings.Split(r.DisplayName, ", ")
		name := strings.TrimSpace(r.Name)
		if name == "" && len(parts) > 0 {
			name = parts[0]
		}
		var detail []string
		for _, k := range []string{"suburb", "neighbourhood", "town", "city_district", "county", "city", "province"} {
			if v := r.Address[k]; v != "" && v != name && !contains(detail, v) {
				detail = append(detail, v)
			}
			if len(detail) == 2 {
				break
			}
		}
		key := textnorm.Fold(name + "|" + strings.Join(detail, ","))
		if seen[key] {
			continue
		}
		dup := false
		for _, o := range out { // same place mapped several times (buildings of a campus)
			if textnorm.Fold(o.Name) == textnorm.Fold(name) && geo.DistanceM(o.Lat, o.Lon, lat, lon) < 1000 {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		seen[key] = true
		out = append(out, Place{Name: name, Detail: strings.Join(detail, ", "), Kind: kinds[r.Type], Lat: lat, Lon: lon})
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
