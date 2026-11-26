// Package model holds the city-independent data types the API serves.
// Every city adapter converts its own data into these types, so the
// mobile app never needs to know which municipality a record came from.
package model

import "time"

// Mode is the kind of vehicle that serves a line or stop.
type Mode string

const (
	ModeBus    Mode = "bus"
	ModeTram   Mode = "tram"
	ModeSubway Mode = "subway"
	ModeRail   Mode = "rail"
	ModeFerry  Mode = "ferry"
	ModeCable  Mode = "cable"
)

// ModeFromGTFS maps a GTFS route_type (including extended types) to a Mode.
func ModeFromGTFS(t int) Mode {
	switch {
	case t == 0 || (t >= 900 && t < 1000):
		return ModeTram
	case t == 1 || (t >= 400 && t < 500):
		return ModeSubway
	case t == 2 || (t >= 100 && t < 200):
		return ModeRail
	case t == 4 || (t >= 1000 && t < 1300):
		return ModeFerry
	case t == 5 || t == 6 || t == 7 || (t >= 1300 && t < 1500):
		return ModeCable
	default:
		return ModeBus
	}
}

type LatLon [2]float64 // [lat, lon]

type Stop struct {
	ID       string   `json:"id"`
	Code     string   `json:"code,omitempty"`
	Name     string   `json:"name"`
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	Mode     Mode     `json:"mode"`
	District string   `json:"district,omitempty"`
	Lines    []string `json:"lines,omitempty"` // line IDs serving this stop, when known
}

type Line struct {
	ID        string `json:"id"`
	Short     string `json:"short"`
	Name      string `json:"name"`
	Mode      Mode   `json:"mode"`
	Color     string `json:"color,omitempty"`     // hex without '#'
	TextColor string `json:"textColor,omitempty"` // hex without '#'
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Night     bool   `json:"night,omitempty"`
	HasLive   bool   `json:"hasLive,omitempty"`
}

type StopRef struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// Direction is one way of a line (outbound/inbound).
type Direction struct {
	Index    int       `json:"index"`
	Headsign string    `json:"headsign"`
	Shape    []LatLon  `json:"shape,omitempty"`
	Stops    []StopRef `json:"stops,omitempty"`
	// Departures from the first stop for the requested service day, "HH:MM".
	Departures []string `json:"departures,omitempty"`
}

type LineDetail struct {
	Line
	Directions []Direction `json:"directions"`
	Alerts     []Alert     `json:"alerts,omitempty"`
	DayType    string      `json:"dayType,omitempty"` // weekday | saturday | sunday
}

type Arrival struct {
	LineID     string `json:"lineId"`
	Line       string `json:"line"`
	Headsign   string `json:"headsign,omitempty"`
	Mode       Mode   `json:"mode"`
	Color      string `json:"color,omitempty"`
	Minutes    int    `json:"minutes"`
	Time       string `json:"time"` // HH:MM local
	Realtime   bool   `json:"realtime"`
	Plate      string `json:"plate,omitempty"`
	Accessible *bool  `json:"accessible,omitempty"`
	Bicycle    *bool  `json:"bicycle,omitempty"`
	Electric   *bool  `json:"electric,omitempty"`
	DistanceM  int    `json:"distanceM,omitempty"`
	StopsAway  int    `json:"stopsAway,omitempty"`
	IsLast     bool   `json:"isLast,omitempty"`
	// Live vehicle position, when the source reports it.
	Lat float64 `json:"lat,omitempty"`
	Lon float64 `json:"lon,omitempty"`
	// Crowded is set when riders reported this line as crowded recently.
	Crowded bool `json:"crowded,omitempty"`
}

// NearbyVehicle is a live vehicle near a point, with enough line info
// to draw and label it on the home map.
type NearbyVehicle struct {
	ID        string  `json:"id"`
	LineID    string  `json:"lineId"`
	Line      string  `json:"line"`
	Headsign  string  `json:"headsign,omitempty"`
	Mode      Mode    `json:"mode"`
	Color     string  `json:"color,omitempty"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	DistanceM int     `json:"distanceM"`
}

type Vehicle struct {
	ID        string  `json:"id"`
	LineID    string  `json:"lineId"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Bearing   float64 `json:"bearing,omitempty"`
	Direction int     `json:"direction"`
}

type Alert struct {
	LineIDs []string   `json:"lineIds,omitempty"`
	Lines   []string   `json:"lines,omitempty"`
	Title   string     `json:"title"`
	Start   *time.Time `json:"start,omitempty"`
	End     *time.Time `json:"end,omitempty"`
}

type SourceInfo struct {
	Name      string    `json:"name"`
	License   string    `json:"license"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Stale marks an outdated feed served as a fallback; Note explains it
	// to the user (e.g. "Veri 2018 tarihli...").
	Stale bool   `json:"stale,omitempty"`
	Note  string `json:"note,omitempty"`
	// Unofficial marks community-generated data (not from the operator).
	Unofficial bool `json:"unofficial,omitempty"`
}

type Features struct {
	LiveArrivals bool `json:"liveArrivals"`
	LiveVehicles bool `json:"liveVehicles"`
	Alerts       bool `json:"alerts"`
	Planner      bool `json:"planner"`
}

type CityInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Center LatLon `json:"center"`
	// Bounds is [minLat, minLon, maxLat, maxLon] around all stops; the app
	// uses it to pick the user's city from their location.
	Bounds    [4]float64   `json:"bounds"`
	Zoom      float64      `json:"zoom"`
	StopCount int          `json:"stopCount"`
	LineCount int          `json:"lineCount"`
	Features  Features     `json:"features"`
	Sources   []SourceInfo `json:"sources"`
	LoadedAt  time.Time    `json:"loadedAt"`
}

// Pack is the offline bundle the app downloads once per city.
type Pack struct {
	City  CityInfo `json:"city"`
	Stops []Stop   `json:"stops"`
	Lines []Line   `json:"lines"`
}

// Itinerary is a normalized journey-planner result.
type Itinerary struct {
	DurationS int       `json:"durationS"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Transfers int       `json:"transfers"`
	WalkM     int       `json:"walkM"`
	Legs      []Leg     `json:"legs"`
}

type Leg struct {
	Mode      string    `json:"mode"` // WALK, BUS, TRAM, SUBWAY, RAIL, FERRY...
	From      StopRef   `json:"from"`
	To        StopRef   `json:"to"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Line      string    `json:"line,omitempty"`
	Headsign  string    `json:"headsign,omitempty"`
	Color     string    `json:"color,omitempty"`
	DistanceM int       `json:"distanceM,omitempty"`
	Realtime  bool      `json:"realtime,omitempty"`
	Polyline  string    `json:"polyline,omitempty"`
	Precision int       `json:"precision,omitempty"`
	NumStops  int       `json:"numStops,omitempty"`
}
