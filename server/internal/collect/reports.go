package collect

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Report kinds riders can send.
var ReportKinds = map[string]string{
	"crowded":    "Araç çok kalabalık",
	"no_show":    "Araç gelmedi",
	"ghost":      "Uygulamada görünen araç yok",
	"late":       "Araç geç geldi",
	"early":      "Araç erken geçti",
	"wrong_stop": "Durak yeri veya adı yanlış",
	"wrong_time": "Sefer saati yanlış",
	"broken":     "Durak/araç arızalı veya kirli",
	"other":      "Diğer",
}

type Report struct {
	Kind   string    `json:"kind"`
	City   string    `json:"city"`
	StopID string    `json:"stopId,omitempty"`
	LineID string    `json:"lineId,omitempty"`
	Plate  string    `json:"plate,omitempty"`
	Note   string    `json:"note,omitempty"`
	App    string    `json:"app,omitempty"`
	At     time.Time `json:"at"`
}

var plateRe = regexp.MustCompile(`^[0-9]{2}[A-Z]{1,3}[0-9]{2,5}$`)

// Clean validates and normalises a report in place.
func (r *Report) Clean() error {
	if _, ok := ReportKinds[r.Kind]; !ok {
		return errors.New("geçersiz bildirim türü")
	}
	if r.StopID == "" && r.LineID == "" {
		return errors.New("durak veya hat gerekli")
	}
	r.Plate = strings.ToUpper(strings.ReplaceAll(r.Plate, " ", ""))
	if r.Plate != "" && !plateRe.MatchString(r.Plate) {
		r.Plate = ""
	}
	r.Note = strings.TrimSpace(r.Note)
	if utf8.RuneCountInString(r.Note) > 280 {
		r.Note = string([]rune(r.Note)[:280])
	}
	if len(r.App) > 32 {
		r.App = r.App[:32]
	}
	if len(r.StopID) > 64 || len(r.LineID) > 64 {
		return errors.New("geçersiz kimlik")
	}
	return nil
}

// Recent keeps the last few hours of reports in memory so the API can
// show "riders reported this bus as crowded" next to an arrival.
type Recent struct {
	mu    sync.Mutex
	items []Report
	keep  time.Duration
}

func NewRecent(keep time.Duration) *Recent { return &Recent{keep: keep} }

func (rc *Recent) Add(r Report) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.prune(r.At)
	rc.items = append(rc.items, r)
}

func (rc *Recent) prune(now time.Time) {
	i := 0
	for i < len(rc.items) && now.Sub(rc.items[i].At) > rc.keep {
		i++
	}
	if i > 0 {
		rc.items = append(rc.items[:0], rc.items[i:]...)
	}
}

// Count returns how many reports of kind for city+line arrived within d.
func (rc *Recent) Count(city, lineID, kind string, d time.Duration, now time.Time) int {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	n := 0
	for _, r := range rc.items {
		if r.City == city && r.LineID == lineID && r.Kind == kind && now.Sub(r.At) <= d {
			n++
		}
	}
	return n
}
