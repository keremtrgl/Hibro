// Package plan talks to a MOTIS 2 journey planner and converts its
// answers into the app's model, so the app never depends on MOTIS'
// wire format and the engine can be swapped later.
package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/model"
)

var ErrDisabled = errors.New("journey planner not configured")

type MOTIS struct {
	BaseURL string // e.g. http://motis:8080
	HTTP    *http.Client
}

func NewMOTIS(base string) *MOTIS {
	if base == "" {
		return nil
	}
	return &MOTIS{BaseURL: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}}
}

type Query struct {
	From, To   model.LatLon
	Time       time.Time
	ArriveBy   bool
	Wheelchair bool
}

type place struct {
	Name   string  `json:"name"`
	StopID string  `json:"stopId"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

type wireLeg struct {
	Mode           string    `json:"mode"`
	From           place     `json:"from"`
	To             place     `json:"to"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
	Distance       float64   `json:"distance"`
	RouteShortName string    `json:"routeShortName"`
	Headsign       string    `json:"headsign"`
	RouteColor     string    `json:"routeColor"`
	RealTime       bool      `json:"realTime"`
	LegGeometry    struct {
		Points    string `json:"points"`
		Precision int    `json:"precision"`
	} `json:"legGeometry"`
	IntermediateStops []place `json:"intermediateStops"`
}

type wire struct {
	Itineraries []struct {
		Duration  int       `json:"duration"`
		StartTime time.Time `json:"startTime"`
		EndTime   time.Time `json:"endTime"`
		Transfers int       `json:"transfers"`
		Legs      []wireLeg `json:"legs"`
	} `json:"itineraries"`
}

func (m *MOTIS) Plan(ctx context.Context, q Query) ([]model.Itinerary, error) {
	if m == nil {
		return nil, ErrDisabled
	}
	v := url.Values{}
	v.Set("fromPlace", fmt.Sprintf("%.6f,%.6f", q.From[0], q.From[1]))
	v.Set("toPlace", fmt.Sprintf("%.6f,%.6f", q.To[0], q.To[1]))
	if !q.Time.IsZero() {
		v.Set("time", q.Time.UTC().Format(time.RFC3339))
	}
	if q.ArriveBy {
		v.Set("arriveBy", "true")
	}
	if q.Wheelchair {
		v.Set("pedestrianProfile", "WHEELCHAIR")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.BaseURL+"/api/v1/plan?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("motis: HTTP %d", resp.StatusCode)
	}
	var w wire
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, err
	}
	return Convert(w), nil
}

// Convert maps MOTIS itineraries to the model (exported for tests).
func Convert(w wire) []model.Itinerary {
	out := make([]model.Itinerary, 0, len(w.Itineraries))
	for _, it := range w.Itineraries {
		mi := model.Itinerary{DurationS: it.Duration, Start: it.StartTime, End: it.EndTime, Transfers: it.Transfers}
		for _, l := range it.Legs {
			leg := model.Leg{
				Mode:  l.Mode,
				From:  model.StopRef{ID: l.From.StopID, Name: l.From.Name, Lat: l.From.Lat, Lon: l.From.Lon},
				To:    model.StopRef{ID: l.To.StopID, Name: l.To.Name, Lat: l.To.Lat, Lon: l.To.Lon},
				Start: l.StartTime, End: l.EndTime, Line: l.RouteShortName, Headsign: l.Headsign,
				Color: strings.TrimPrefix(l.RouteColor, "#"), DistanceM: int(l.Distance), Realtime: l.RealTime,
				Polyline: l.LegGeometry.Points, Precision: l.LegGeometry.Precision,
				NumStops: len(l.IntermediateStops) + 1,
			}
			if l.Mode == "WALK" {
				mi.WalkM += int(l.Distance)
				leg.NumStops = 0
			}
			mi.Legs = append(mi.Legs, leg)
		}
		out = append(out, mi)
	}
	return out
}

// DecodeWire is a helper for tests.
func DecodeWire(b []byte) (wire, error) {
	var w wire
	err := json.Unmarshal(b, &w)
	return w, err
}
