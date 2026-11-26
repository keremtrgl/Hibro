// Package izmir adapts İzmir Büyükşehir Belediyesi's ESHOT open data
// (bus lines, stops, timetables, routes, alerts and live vehicle data)
// to the common model. None of these endpoints need an API key.
package izmir

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/cache"
	"github.com/keremtrgl/hibro/server/internal/fetch"
	"github.com/keremtrgl/hibro/server/internal/geo"
	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/model"
	"github.com/keremtrgl/hibro/server/internal/textnorm"
)

type Endpoints struct {
	Lines      string
	NightLines string
	Stops      string
	Schedule   string
	Shapes     string
	Alerts     string
	Arrivals   string // %s = stop code
	Vehicles   string // %s = line number
	License    string
	PortalURL  string
}

var DefaultEndpoints = Endpoints{
	Lines:      "https://acikveriwapi.izmir.bel.tr/AcikVeriEshot/Hatlar/HatlariGetir",
	NightLines: "https://acikveriwapi.izmir.bel.tr/AcikVeriEshot/Hatlar/BaykusHatlariniGetir",
	Stops:      "https://acikveriwapi.izmir.bel.tr/AcikVeriEshot/Duraklar/DuraklariGetir",
	Schedule:   "https://acikveriwapi.izmir.bel.tr/AcikVeriEshot/Seferler/HareketSaatleriniGetir",
	Shapes:     "https://openfiles.izmir.bel.tr/211488/docs/eshot-otobus-hat-guzergahlari.csv",
	Alerts:     "https://openfiles.izmir.bel.tr/211488/docs/eshot-otobus-hat-duyurulari.csv",
	Arrivals:   "https://acikveriwapi.izmir.bel.tr/AcikVeriEshot/Araclar/DuragaYaklasanOtobusler?durakKodu=%s",
	Vehicles:   "https://openapi.izmir.bel.tr/api/iztek/hatotobuskonumlari/%s",
	License:    "İzmir Büyükşehir Belediyesi Açık Veri Lisansı",
	PortalURL:  "https://acikveri.bizizmir.com",
}

// ---- raw parsing (pure functions, unit tested with real samples) ----

type Line struct {
	No, Name, From, To, Route string
	Cooperative               bool
}

type Stop struct {
	Code, Name, District string
	Lat, Lon             float64
}

// commaFloat parses "38,404575" (Turkish decimal comma) or "38.4".
func commaFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	return f
}

func numString(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	}
	return ""
}

func ParseLines(b []byte) ([]Line, error) {
	var env struct {
		Data []struct {
			HatNo         any    `json:"HatNo"`
			HatAdi        string `json:"HatAdi"`
			HatBaslangic  string `json:"HatBaslangic"`
			HatBitis      string `json:"HatBitis"`
			Guzergah      string `json:"GuzergahAciklama"`
			HizmetBirimi  string `json:"HizmetBirimi"`
			InterneteAcik int    `json:"InterneteAcik"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	out := make([]Line, 0, len(env.Data))
	for _, d := range env.Data {
		no := numString(d.HatNo)
		if no == "" {
			continue
		}
		out = append(out, Line{
			No: no, Name: textnorm.TitleTR(d.HatAdi),
			From: textnorm.TitleTR(d.HatBaslangic), To: textnorm.TitleTR(d.HatBitis),
			Route:       textnorm.TitleTR(d.Guzergah),
			Cooperative: strings.Contains(strings.ToUpper(d.HizmetBirimi), "KOOPERAT"),
		})
	}
	return out, nil
}

func ParseStops(b []byte) ([]Stop, error) {
	var env struct {
		Data []struct {
			DurakAdi string `json:"DurakAdi"`
			Kod      any    `json:"DurakKodu"`
			X        string `json:"X_Koordinati"` // latitude, despite the name
			Y        string `json:"y_koordinati"` // longitude
			Ilce     string `json:"IlceAdi"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	out := make([]Stop, 0, len(env.Data))
	for _, d := range env.Data {
		lat, lon := commaFloat(d.X), commaFloat(d.Y)
		code := numString(d.Kod)
		if code == "" || lat == 0 || lon == 0 {
			continue
		}
		if lat < lon { // defensive: the feed has swapped axes before
			lat, lon = lon, lat
		}
		out = append(out, Stop{Code: code, Name: textnorm.Clean(d.DurakAdi), District: textnorm.TitleTR(d.Ilce), Lat: lat, Lon: lon})
	}
	return out, nil
}

// SchedKey identifies a timetable column: line, direction (1 = Gidiş,
// 2 = Dönüş) and day type (weekday/saturday/sunday).
type SchedKey struct {
	Line    string
	Dir     int
	DayType string
}

func dayTypeOf(program string) string {
	p := textnorm.Fold(program)
	switch {
	case strings.HasPrefix(p, "cumartesi"):
		return "saturday"
	case strings.HasPrefix(p, "pazar"):
		return "sunday"
	}
	return "weekday"
}

// ParseSchedule returns departure times from the terminal per column.
// When both summer and winter timetables are present, month decides.
func ParseSchedule(b []byte, now time.Time) (map[SchedKey][]string, error) {
	var env struct {
		Data []struct {
			HatNo   any    `json:"HatNo"`
			Yon     int    `json:"Yon"`
			Program string `json:"ProgramAdi"`
			Sezon   string `json:"SezonAdi"`
			Saat    string `json:"Saat"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	summer := now.Month() >= time.June && now.Month() <= time.September
	bySeason := map[string]map[SchedKey][]string{}
	for _, d := range env.Data {
		s := textnorm.Fold(d.Sezon)
		if bySeason[s] == nil {
			bySeason[s] = map[SchedKey][]string{}
		}
		k := SchedKey{numString(d.HatNo), d.Yon, dayTypeOf(d.Program)}
		bySeason[s][k] = append(bySeason[s][k], strings.TrimSpace(d.Saat))
	}
	pick := bySeason["kis"]
	if summer && bySeason["yaz"] != nil {
		pick = bySeason["yaz"]
	}
	if pick == nil {
		for _, v := range bySeason {
			pick = v
			break
		}
	}
	if pick == nil {
		pick = map[SchedKey][]string{}
	}
	for k, v := range pick {
		sort.Strings(v)
		pick[k] = dedupe(v)
	}
	return pick, nil
}

func dedupe(v []string) []string {
	out := v[:0]
	for i, s := range v {
		if i == 0 || s != v[i-1] {
			out = append(out, s)
		}
	}
	return out
}

type ShapeKey struct {
	Line string
	Dir  int
}

// ParseShapes reads HAT_NO;YON;BOYLAM;ENLEM rows (already ordered) and
// simplifies each polyline.
func ParseShapes(r io.Reader) (map[ShapeKey][]model.LatLon, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true
	if _, err := cr.Read(); err != nil {
		return nil, err
	}
	raw := map[ShapeKey][][2]float64{}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(rec) < 4 {
			continue
		}
		dir, _ := strconv.Atoi(strings.TrimSpace(rec[1]))
		k := ShapeKey{strings.TrimSpace(strings.TrimPrefix(rec[0], "\ufeff")), dir}
		lon, lat := commaFloat(rec[2]), commaFloat(rec[3])
		if lat == 0 || lon == 0 {
			continue
		}
		pts := raw[k]
		if n := len(pts); n > 0 && pts[n-1] == [2]float64{lat, lon} {
			continue
		}
		raw[k] = append(pts, [2]float64{lat, lon})
	}
	out := make(map[ShapeKey][]model.LatLon, len(raw))
	for k, pts := range raw {
		s := geo.Simplify(pts, 4)
		line := make([]model.LatLon, len(s))
		for i, p := range s {
			line[i] = p
		}
		out[k] = line
	}
	return out, nil
}

var istanbul = mustLoc("Europe/Istanbul")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("TRT", 3*3600)
	}
	return l
}

// ParseAlerts reads HAT_NO;BASLIK;BASLAMA_TARIHI;BITIS_TARIHI rows and
// merges alerts that share a title across lines.
func ParseAlerts(r io.Reader, prefix string) ([]model.Alert, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	if _, err := cr.Read(); err != nil {
		return nil, err
	}
	byTitle := map[string]*model.Alert{}
	var order []string
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(rec) < 2 {
			continue
		}
		line, title := strings.TrimSpace(rec[0]), textnorm.Clean(rec[1])
		if title == "" {
			continue
		}
		a := byTitle[title]
		if a == nil {
			a = &model.Alert{Title: title}
			if len(rec) > 2 {
				a.Start = parseTime(rec[2])
			}
			if len(rec) > 3 {
				a.End = parseTime(rec[3])
			}
			byTitle[title] = a
			order = append(order, title)
		}
		if line != "" {
			a.Lines = append(a.Lines, line)
			a.LineIDs = append(a.LineIDs, prefix+":"+line)
		}
	}
	out := make([]model.Alert, 0, len(order))
	for _, t := range order {
		out = append(out, *byTitle[t])
	}
	return out, nil
}

func parseTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "02.01.2006 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, istanbul); err == nil {
			return &t
		}
	}
	return nil
}

// ---- live data ----

type latLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type rawArrival struct {
	LineCode       string  `json:"lineCode"`
	Starting       string  `json:"starting"`
	Ending         string  `json:"ending"`
	IsLast         bool    `json:"isLast"`
	Disabled       bool    `json:"disabled"`
	Bicycle        bool    `json:"bicycle"`
	Electric       bool    `json:"electric"`
	Direction      int     `json:"direction"`
	Plate          string  `json:"plate"`
	GPSTime        string  `json:"gpsTime"`
	Current        latLng  `json:"currentLocation"`
	Snapped        latLng  `json:"snappedLocation"`
	StationOrder   int     `json:"stationOrder"`
	TargetOrder    int     `json:"targetStationOrder"`
	DistanceTarget float64 `json:"distanceToTarget"`
	DurationTarget float64 `json:"durationToTarget"` // seconds
}

// MaxGPSAge is how old a vehicle's last GPS fix may be before its
// arrival is dropped. The ESHOT feed keeps reporting vehicles whose
// tracker went silent hours ago (night buses still "3 minutes away" at
// noon); showing those would be worse than showing nothing.
const MaxGPSAge = 5 * time.Minute

// ParseArrivals decodes the DuragaYaklasanOtobusler response. An unknown
// stop yields an empty list, not an error. Stale GPS fixes are skipped.
func ParseArrivals(b []byte, prefix string, now time.Time) ([]model.Arrival, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, errors.New("eshot: empty response")
	}
	var env struct {
		Data struct {
			Data  []rawArrival `json:"data"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	out := make([]model.Arrival, 0, len(env.Data.Data))
	for _, r := range env.Data.Data {
		if gps, err := time.Parse(time.RFC3339, r.GPSTime); err == nil && now.Sub(gps) > MaxGPSAge {
			continue
		}
		secs := int(r.DurationTarget)
		at := now.Add(time.Duration(secs) * time.Second)
		// direction 0 travels from "starting" towards "ending".
		head := r.Ending
		if r.Direction == 1 {
			head = r.Starting
		}
		disabled, bike, elec := r.Disabled, r.Bicycle, r.Electric
		a := model.Arrival{
			LineID: prefix + ":" + r.LineCode, Line: r.LineCode,
			Headsign: textnorm.TitleTR(head), Mode: model.ModeBus,
			Minutes: (secs + 30) / 60, Time: at.In(istanbul).Format("15:04"),
			Realtime: true, Plate: r.Plate, Accessible: &disabled, Bicycle: &bike, Electric: &elec,
			DistanceM: int(r.DistanceTarget), IsLast: r.IsLast,
		}
		// Prefer the position snapped to the route; fall back to raw GPS.
		if r.Snapped.Lat != 0 {
			a.Lat, a.Lon = r.Snapped.Lat, r.Snapped.Lng
		} else {
			a.Lat, a.Lon = r.Current.Lat, r.Current.Lng
		}
		if r.TargetOrder > r.StationOrder {
			a.StopsAway = r.TargetOrder - r.StationOrder
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Minutes < out[j].Minutes })
	return out, nil
}

// ParseVehicles decodes hatotobuskonumlari. KoorX holds latitude and
// KoorY longitude, both with decimal commas; duplicates are removed.
func ParseVehicles(b []byte, lineID string) ([]model.Vehicle, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, errors.New("eshot: empty response")
	}
	var env struct {
		Konum []struct {
			ID  any    `json:"OtobusId"`
			Yon int    `json:"Yon"`
			X   string `json:"KoorX"`
			Y   string `json:"KoorY"`
		} `json:"HatOtobusKonumlari"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	seen := map[string]int{}
	out := []model.Vehicle{}
	for _, k := range env.Konum {
		lat, lon := commaFloat(k.X), commaFloat(k.Y)
		if lat == 0 || lon == 0 {
			continue
		}
		if lat < lon {
			lat, lon = lon, lat
		}
		id := numString(k.ID)
		v := model.Vehicle{ID: id, LineID: lineID, Lat: lat, Lon: lon, Direction: k.Yon}
		if i, ok := seen[id]; ok {
			out[i] = v // keep the latest fix
			continue
		}
		seen[id] = len(out)
		out = append(out, v)
	}
	return out, nil
}

// ---- Source ----

type Static struct {
	Lines     []Line
	Stops     []Stop
	Schedule  map[SchedKey][]string
	Shapes    map[ShapeKey][]model.LatLon
	Alerts    []model.Alert
	Night     map[string]bool
	FetchedAt time.Time
	// From the ESHOT GTFS (optional): ordered stop codes per line and
	// direction, and the lines serving each stop. The live API has
	// neither.
	StopSeq map[ShapeKey][]string
	// Seconds from the first stop to each stop of StopSeq (GTFS travel
	// times), used to build a routable timetable from current departures.
	StopOffsets map[ShapeKey][]int32
	StopLines   map[string][]string
	SeqSource   string // e.g. "ESHOT GTFS (2024)"
}

// ApplyGTFS takes stop order and stop->line lists from an ESHOT GTFS feed
// (stop_id = DurakKodu, route_id = HatNo). GTFS direction 0/1 is mapped to
// ESHOT Yon 1/2, swapped per line when the route shape says otherwise.
func ApplyGTFS(st *Static, f *gtfs.Feed, label string) {
	st.StopSeq = map[ShapeKey][]string{}
	st.StopOffsets = map[ShapeKey][]int32{}
	lines := map[string]map[string]bool{}
	for key, ti := range f.RepTrip {
		route := f.Routes[key[0]].ID
		var seq []string
		var offs []int32
		sts := f.TripStops[ti]
		for _, s := range sts {
			code := f.Stops[s.Stop].ID
			seq = append(seq, code)
			offs = append(offs, s.Arr-sts[0].Dep)
			if lines[code] == nil {
				lines[code] = map[string]bool{}
			}
			lines[code][route] = true
		}
		st.StopSeq[ShapeKey{route, int(key[1]) + 1}] = seq
		st.StopOffsets[ShapeKey{route, int(key[1]) + 1}] = offs
	}
	// Fix direction numbering using the ESHOT route shapes.
	stopPos := map[string][2]float64{}
	for _, s := range f.Stops {
		stopPos[s.ID] = [2]float64{s.Lat, s.Lon}
	}
	for _, l := range st.Lines {
		a, b := st.StopSeq[ShapeKey{l.No, 1}], st.StopSeq[ShapeKey{l.No, 2}]
		sh := st.Shapes[ShapeKey{l.No, 1}]
		if len(a) == 0 || len(sh) == 0 {
			continue
		}
		p := stopPos[a[0]]
		dSame := geo.DistanceM(p[0], p[1], sh[0][0], sh[0][1])
		dOther := geo.DistanceM(p[0], p[1], sh[len(sh)-1][0], sh[len(sh)-1][1])
		if dOther+200 < dSame { // first stop sits at the shape's far end
			st.StopSeq[ShapeKey{l.No, 1}], st.StopSeq[ShapeKey{l.No, 2}] = b, a
			k1, k2 := ShapeKey{l.No, 1}, ShapeKey{l.No, 2}
			st.StopOffsets[k1], st.StopOffsets[k2] = st.StopOffsets[k2], st.StopOffsets[k1]
		}
	}
	st.StopLines = map[string][]string{}
	for code, set := range lines {
		for r := range set {
			st.StopLines[code] = append(st.StopLines[code], r)
		}
		sort.Slice(st.StopLines[code], func(i, j int) bool { return lineLess(st.StopLines[code][i], st.StopLines[code][j]) })
	}
	st.SeqSource = label
}

func lineLess(a, b string) bool {
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)
	if ea == nil && eb == nil {
		return na < nb
	}
	return a < b
}

// LoadStatic downloads (or reuses cached) static ESHOT data. Lines and
// stops are required; the rest degrade gracefully.
func LoadStatic(ctx context.Context, fc *fetch.Client, ep Endpoints, prefix string) (*Static, error) {
	st := &Static{FetchedAt: time.Now(), Night: map[string]bool{}}
	day := 24 * time.Hour
	lr, err := fc.Cached(ctx, ep.Lines, day)
	if err != nil {
		return nil, fmt.Errorf("eshot lines: %w", err)
	}
	if st.Lines, err = ParseLines(lr.Data); err != nil {
		return nil, fmt.Errorf("eshot lines: %w", err)
	}
	sr, err := fc.Cached(ctx, ep.Stops, day)
	if err != nil {
		return nil, fmt.Errorf("eshot stops: %w", err)
	}
	if st.Stops, err = ParseStops(sr.Data); err != nil {
		return nil, fmt.Errorf("eshot stops: %w", err)
	}
	if r, err := fc.Cached(ctx, ep.NightLines, day); err == nil {
		if night, err := ParseLines(r.Data); err == nil {
			known := map[string]bool{}
			for _, l := range st.Lines {
				known[l.No] = true
			}
			for _, l := range night {
				st.Night[l.No] = true
				if !known[l.No] {
					st.Lines = append(st.Lines, l)
				}
			}
		}
	}
	if r, err := fc.Cached(ctx, ep.Schedule, 12*time.Hour); err == nil {
		st.Schedule, _ = ParseSchedule(r.Data, time.Now().In(istanbul))
	} else {
		fc.Log.Warn("eshot schedule unavailable", "err", err)
	}
	if r, err := fc.Cached(ctx, ep.Shapes, day); err == nil {
		st.Shapes, _ = ParseShapes(bytes.NewReader(r.Data))
	} else {
		fc.Log.Warn("eshot shapes unavailable", "err", err)
	}
	if r, err := fc.Cached(ctx, ep.Alerts, time.Hour); err == nil {
		st.Alerts, _ = ParseAlerts(bytes.NewReader(r.Data), prefix)
	}
	return st, nil
}

type ESHOT struct {
	prefix     string
	st         *Static
	fc         *fetch.Client
	ep         Endpoints
	lines      map[string]Line
	stops      map[string]bool
	stopByCode map[string]Stop
	arrivals   *cache.TTL[[]model.Arrival]
	vehicles   *cache.TTL[[]model.Vehicle]
	// Fetch is used for live calls; replaced in tests.
	Fetch func(ctx context.Context, url string) ([]byte, error)
	// routing is a synthetic GTFS (current departures + GTFS travel
	// times) used only by the journey planner.
	routing *gtfs.Source
}

// RoutingSources exposes the synthetic timetable to the planner.
func (e *ESHOT) RoutingSources() []*gtfs.Source {
	if e.routing == nil {
		return nil
	}
	return []*gtfs.Source{e.routing}
}

func NewESHOT(prefix string, st *Static, fc *fetch.Client, ep Endpoints) *ESHOT {
	e := &ESHOT{
		prefix: prefix, st: st, fc: fc, ep: ep,
		lines: map[string]Line{},
		stops: map[string]bool{},
		// Expired entries are served for a while and refreshed in the
		// background, so nobody waits for the (slow) ESHOT API twice.
		arrivals: cache.New[[]model.Arrival](15*time.Second, 5*time.Second, 20000).WithStale(60 * time.Second),
		vehicles: cache.New[[]model.Vehicle](10*time.Second, 5*time.Second, 2000).WithStale(30 * time.Second),
	}
	for _, l := range st.Lines {
		e.lines[l.No] = l
	}
	e.stopByCode = make(map[string]Stop, len(st.Stops))
	for _, s := range st.Stops {
		e.stops[s.Code] = true
		e.stopByCode[s.Code] = s
	}
	if f, err := RoutingFeed(st); err == nil && f != nil {
		e.routing = gtfs.NewSource(prefix, f, model.SourceInfo{Name: "ESHOT (planlama için)", Estimated: true})
	}
	// At most 8 requests to ESHOT at a time: polite to the municipality
	// and keeps a traffic spike from turning into a request storm.
	upstream := make(chan struct{}, 8)
	e.Fetch = func(ctx context.Context, u string) ([]byte, error) {
		select {
		case upstream <- struct{}{}:
			defer func() { <-upstream }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// The ESHOT API intermittently returns empty bodies; retry those.
		var last error
		for i := 0; i < 3; i++ {
			b, err := fc.Get(ctx, u, 1)
			if err == nil && len(bytes.TrimSpace(b)) > 0 {
				return b, nil
			}
			if err == nil {
				err = errors.New("empty response")
			}
			last = err
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(300*(i+1)) * time.Millisecond):
			}
		}
		return nil, last
	}
	return e
}

func (e *ESHOT) Prefix() string { return e.prefix }

func (e *ESHOT) Info() model.SourceInfo {
	return model.SourceInfo{Name: "ESHOT Genel Müdürlüğü (otobüs)", License: e.ep.License, URL: e.ep.PortalURL, UpdatedAt: e.st.FetchedAt}
}

func (e *ESHOT) Features() model.Features {
	return model.Features{LiveArrivals: true, LiveVehicles: true, Alerts: true}
}

func (e *ESHOT) Stops() []model.Stop {
	out := make([]model.Stop, len(e.st.Stops))
	for i, s := range e.st.Stops {
		st := model.Stop{ID: e.prefix + ":" + s.Code, Code: s.Code, Name: s.Name, Lat: s.Lat, Lon: s.Lon, Mode: model.ModeBus, District: s.District}
		for _, no := range e.st.StopLines[s.Code] {
			if _, ok := e.lines[no]; ok {
				st.Lines = append(st.Lines, e.prefix+":"+no)
			}
		}
		out[i] = st
	}
	return out
}

func (e *ESHOT) toLine(l Line) model.Line {
	return model.Line{
		ID: e.prefix + ":" + l.No, Short: l.No, Name: l.Name, Mode: model.ModeBus,
		From: l.From, To: l.To, HasLive: true, Night: e.st.Night[l.No],
		Color: lineColor(l),
	}
}

func lineColor(l Line) string {
	if l.Cooperative {
		return "6D4C41"
	}
	return "1565C0"
}

func (e *ESHOT) Lines() []model.Line {
	out := make([]model.Line, len(e.st.Lines))
	for i, l := range e.st.Lines {
		out[i] = e.toLine(l)
	}
	return out
}

func dayTypeFor(t time.Time) string {
	switch t.Weekday() {
	case time.Saturday:
		return "saturday"
	case time.Sunday:
		return "sunday"
	}
	return "weekday"
}

var ErrNotFound = errors.New("not found")

func (e *ESHOT) Line(_ context.Context, no string, now time.Time) (*model.LineDetail, error) {
	l, ok := e.lines[no]
	if !ok {
		return nil, ErrNotFound
	}
	dt := dayTypeFor(now.In(istanbul))
	d := &model.LineDetail{Line: e.toLine(l), DayType: dt}
	for dir := 1; dir <= 2; dir++ {
		head := l.To
		if dir == 2 {
			head = l.From
		}
		dd := model.Direction{
			Index: dir - 1, Headsign: head,
			Shape:      e.st.Shapes[ShapeKey{no, dir}],
			Departures: e.st.Schedule[SchedKey{no, dir, dt}],
		}
		for _, code := range e.st.StopSeq[ShapeKey{no, dir}] {
			if s, ok := e.stopByCode[code]; ok {
				dd.Stops = append(dd.Stops, model.StopRef{ID: e.prefix + ":" + code, Name: s.Name, Lat: s.Lat, Lon: s.Lon})
			}
		}
		if len(dd.Shape) == 0 && len(dd.Departures) == 0 && len(dd.Stops) == 0 {
			continue
		}
		d.Directions = append(d.Directions, dd)
	}
	for _, a := range e.st.Alerts {
		for _, ln := range a.Lines {
			if ln == no && alertActive(a, now) {
				d.Alerts = append(d.Alerts, a)
				break
			}
		}
	}
	return d, nil
}

func alertActive(a model.Alert, now time.Time) bool {
	if a.End != nil && now.After(*a.End) {
		return false
	}
	if a.Start != nil && now.Before(a.Start.Add(-24*time.Hour)) {
		return false
	}
	return true
}

func (e *ESHOT) Arrivals(ctx context.Context, code string, now time.Time) ([]model.Arrival, error) {
	if !e.stops[code] {
		return nil, ErrNotFound
	}
	live, err := e.arrivals.Get(ctx, code, func(ctx context.Context) ([]model.Arrival, error) {
		b, err := e.Fetch(ctx, fmt.Sprintf(e.ep.Arrivals, url.QueryEscape(code)))
		if err != nil {
			return nil, err
		}
		return ParseArrivals(b, e.prefix, time.Now())
	})
	if len(live) > 0 {
		return live, err
	}
	// Live system empty or down: fall back to the estimated timetable
	// (current terminal departures + GTFS travel times), clearly marked.
	if e.routing != nil {
		if est, serr := e.routing.Arrivals(ctx, code, now); serr == nil && len(est) > 0 {
			cut := est[:0]
			for _, a := range est {
				if a.Minutes <= 60 {
					a.Estimated = true
					a.Color = lineColor(e.lines[strings.TrimPrefix(a.LineID, e.prefix+":")])
					cut = append(cut, a)
				}
			}
			return cut, err
		}
	}
	return live, err
}

func (e *ESHOT) Vehicles(ctx context.Context, no string) ([]model.Vehicle, error) {
	if _, ok := e.lines[no]; !ok {
		return nil, ErrNotFound
	}
	return e.vehicles.Get(ctx, no, func(ctx context.Context) ([]model.Vehicle, error) {
		b, err := e.Fetch(ctx, fmt.Sprintf(e.ep.Vehicles, url.PathEscape(no)))
		if err != nil {
			return nil, err
		}
		return ParseVehicles(b, e.prefix+":"+no)
	})
}

func (e *ESHOT) Alerts(_ context.Context, now time.Time) []model.Alert {
	out := []model.Alert{}
	for _, a := range e.st.Alerts {
		if alertActive(a, now) {
			out = append(out, a)
		}
	}
	return out
}
