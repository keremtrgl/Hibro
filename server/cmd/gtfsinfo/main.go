// Command gtfsinfo summarises GTFS feeds: size, modes, service dates and
// area, to judge whether a feed is good enough to add a city.
//
//	go run ./cmd/gtfsinfo feed1.zip feed2.zip ...
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/model"
)

func main() {
	fmt.Printf("%-28s %6s %6s %7s %-22s %-19s %-9s %s\n", "dosya", "durak", "hat", "sefer", "türler", "tarife", "parse", "yayıncı")
	for _, p := range os.Args[1:] {
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Println(p, err)
			continue
		}
		start := time.Now()
		f, err := gtfs.Parse(b)
		if err != nil {
			fmt.Printf("%-28s HATA: %v\n", filepath.Base(p), err)
			continue
		}
		took := time.Since(start).Round(10 * time.Millisecond)
		src := gtfs.NewSource("x", f, model.SourceInfo{})
		modes := map[model.Mode]int{}
		for _, l := range src.Lines() {
			modes[l.Mode]++
		}
		var ms []string
		for m, n := range modes {
			ms = append(ms, fmt.Sprintf("%s:%d", m, n))
		}
		sort.Strings(ms)
		minStart := 99999999
		for _, s := range f.Services {
			if s.Start > 0 && s.Start < minStart {
				minStart = s.Start
			}
		}
		if minStart == 99999999 {
			minStart = 0
		}
		pub := f.Publisher
		if pub == "" {
			for _, a := range f.Agencies {
				pub = a
				break
			}
		}
		fmt.Printf("%-28s %6d %6d %7d %-22s %d-%d %-9s %s\n", filepath.Base(p), len(src.Stops()), len(src.Lines()), len(f.Trips),
			strings.Join(ms, ","), minStart, f.ServiceEnd(), took, pub)
	}
}
