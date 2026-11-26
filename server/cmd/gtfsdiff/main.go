// Command gtfsdiff compares two GTFS feeds of the same city and prints how
// much changed: stops (by position), lines (by number), route shapes
// (shared stops) and service levels (trips per weekday).
//
//	go run ./cmd/gtfsdiff eski.zip yeni.zip
//
// Used to answer "how outdated is the old feed?" with numbers, e.g.
// Kocaeli 2018 archive vs. the current veri.kocaeli.bel.tr feed.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/keremtrgl/hibro/server/internal/geo"
	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/textnorm"
)

func load(path string) *gtfs.Feed {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := gtfs.Parse(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, path, err)
		os.Exit(1)
	}
	return f
}

type lineInfo struct {
	name  string
	stops map[int32]bool // every stop any trip of the line serves
	trips int            // weekday trips
}

func lines(f *gtfs.Feed) map[string]*lineInfo {
	out := map[string]*lineInfo{}
	for ti, tr := range f.Trips {
		r := f.Routes[tr.Route]
		key := textnorm.Fold(r.Short)
		if key == "" {
			key = textnorm.Fold(r.Long)
		}
		li := out[key]
		if li == nil {
			li = &lineInfo{name: strings.TrimSpace(r.Short + " " + r.Long), stops: map[int32]bool{}}
			out[key] = li
		}
		svc := f.Services[tr.Service]
		if svc.Weekdays[3] { // Wednesday as a typical weekday
			li.trips++
		}
		for _, st := range f.TripStops[ti] {
			li.stops[st.Stop] = true
		}
	}
	return out
}

func pct(a, b int) string {
	if b == 0 {
		return "-"
	}
	return fmt.Sprintf("%%%d", (a*100+b/2)/b)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("kullanım: gtfsdiff eski.zip yeni.zip")
		os.Exit(2)
	}
	old, cur := load(os.Args[1]), load(os.Args[2])

	// ---- stops: match each old stop to the nearest new stop ----
	lat := make([]float64, len(cur.Stops))
	lon := make([]float64, len(cur.Stops))
	for i, s := range cur.Stops {
		lat[i], lon[i] = s.Lat, s.Lon
	}
	ix := geo.NewIndex(lat, lon)
	match := make([]int32, len(old.Stops)) // old stop -> new stop or -1
	same, moved, gone := 0, 0, 0
	for i, s := range old.Stops {
		match[i] = -1
		if s.LocationType != 0 {
			continue
		}
		if h := ix.Near(s.Lat, s.Lon, 40, 1); len(h) > 0 {
			match[i] = int32(h[0].Index)
			same++
		} else if h := ix.Near(s.Lat, s.Lon, 200, 1); len(h) > 0 {
			match[i] = int32(h[0].Index)
			moved++
		} else {
			gone++
		}
	}
	matchedNew := map[int32]bool{}
	for _, m := range match {
		if m >= 0 {
			matchedNew[m] = true
		}
	}
	newStops := 0
	for i, s := range cur.Stops {
		if s.LocationType == 0 && !matchedNew[int32(i)] {
			newStops++
		}
	}
	total := same + moved + gone
	fmt.Println("== DURAKLAR")
	fmt.Printf("eski: %d  yeni: %d\n", total, len(cur.Stops))
	fmt.Printf("aynı yerde (≤40 m): %d (%s)\n", same, pct(same, total))
	fmt.Printf("yer değiştirmiş (40-200 m): %d (%s)\n", moved, pct(moved, total))
	fmt.Printf("kaldırılmış: %d (%s)\n", gone, pct(gone, total))
	fmt.Printf("yeni eklenmiş: %d\n\n", newStops)

	// ---- lines ----
	ol, nl := lines(old), lines(cur)
	var kept, removed, added []string
	for k := range ol {
		if nl[k] != nil {
			kept = append(kept, k)
		} else {
			removed = append(removed, ol[k].name)
		}
	}
	for k := range nl {
		if ol[k] == nil {
			added = append(added, nl[k].name)
		}
	}
	sort.Strings(kept)
	sort.Strings(removed)
	sort.Strings(added)
	fmt.Println("== HATLAR")
	fmt.Printf("eski: %d  yeni: %d  korunan: %d (%s)  kaldırılan: %d  yeni: %d\n", len(ol), len(nl), len(kept), pct(len(kept), len(ol)), len(removed), len(added))

	// Route similarity: share of an old line's stops that the new line
	// still serves (after stop matching).
	type sim struct {
		key   string
		share int
		trips [2]int
	}
	var sims []sim
	unchanged, changed, rerouted := 0, 0, 0
	for _, k := range kept {
		o, n := ol[k], nl[k]
		hit := 0
		for s := range o.stops {
			if m := match[s]; m >= 0 && n.stops[m] {
				hit++
			}
		}
		share := 0
		if len(o.stops) > 0 {
			share = hit * 100 / len(o.stops)
		}
		switch {
		case share >= 90:
			unchanged++
		case share >= 60:
			changed++
		default:
			rerouted++
		}
		sims = append(sims, sim{k, share, [2]int{o.trips, n.trips}})
	}
	fmt.Printf("korunan hatların güzergâhı: aynı (≥%%90 ortak durak) %d · kısmen değişmiş %d · büyük ölçüde değişmiş %d\n", unchanged, changed, rerouted)
	sort.Slice(sims, func(i, j int) bool { return sims[i].share < sims[j].share })
	fmt.Println("\nen çok değişen 15 hat (ortak durak oranı, hafta içi sefer eski→yeni):")
	for i := 0; i < len(sims) && i < 15; i++ {
		s := sims[i]
		fmt.Printf("  %-40s %%%3d  %d → %d sefer\n", nl[s.key].name, s.share, s.trips[0], s.trips[1])
	}
	show := func(title string, list []string) {
		if len(list) == 0 {
			return
		}
		if len(list) > 25 {
			list = append(list[:25], fmt.Sprintf("… ve %d tane daha", len(list)-25))
		}
		fmt.Printf("\n%s:\n  %s\n", title, strings.Join(list, "\n  "))
	}
	show("kaldırılan hatlar", removed)
	show("yeni hatlar", added)
}
