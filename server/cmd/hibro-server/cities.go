package main

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/keremtrgl/hibro/server/internal/city"
	"github.com/keremtrgl/hibro/server/internal/fetch"
	"github.com/keremtrgl/hibro/server/internal/gtfs"
	"github.com/keremtrgl/hibro/server/internal/izmir"
	"github.com/keremtrgl/hibro/server/internal/model"
)

type loader func(ctx context.Context, fc *fetch.Client) (*city.City, error)

var loaders = map[string]loader{
	"izmir":     loadIzmir,
	"konya":     ckanCity(city.Meta{ID: "konya", Name: "Konya", Center: model.LatLon{37.8746, 32.4932}, Zoom: 12.5}, "k", "https://acikveri.konya.bel.tr", "toplu-tasima-gtfs-verileri", "Konya Büyükşehir Belediyesi", "CC BY 4.0"),
	"kocaeli":   loadKocaeli,
	"gaziantep": ckanCity(city.Meta{ID: "gaziantep", Name: "Gaziantep", Center: model.LatLon{37.0662, 37.3833}, Zoom: 12.5}, "g", "https://acikveri.gaziantep.bel.tr", "toplu-ulasim-gtfs-verileri", "Gaziantep Büyükşehir Belediyesi", "Gaziantep Açık Veri Lisansı"),
}

const staticMaxAge = 20 * time.Hour

// gtfsSource downloads and parses one feed. Optional feeds return
// (nil, nil) on failure so the rest of the city still loads.
func gtfsSource(ctx context.Context, fc *fetch.Client, prefix, url, name, license, portal string, optional bool) (city.Source, error) {
	res, err := fc.Cached(ctx, url, staticMaxAge)
	if err == nil {
		var f *gtfs.Feed
		if f, err = gtfs.Parse(res.Data); err == nil {
			info := model.SourceInfo{Name: name, License: license, URL: portal, UpdatedAt: res.FetchedAt}
			markIfExpired(f, &info)
			return gtfs.NewSource(prefix, f, info), nil
		}
	}
	if optional {
		fc.Log.Warn("optional feed skipped", "feed", name, "err", err)
		return nil, nil
	}
	return nil, fmt.Errorf("%s: %w", name, err)
}

func loadIzmir(ctx context.Context, fc *fetch.Client) (*city.City, error) {
	const portal = "https://acikveri.bizizmir.com/dataset/toplu-ulasim-gtfs-verileri"
	lic := izmir.DefaultEndpoints.License
	st, err := izmir.LoadStatic(ctx, fc, izmir.DefaultEndpoints, "b")
	if err != nil {
		return nil, err
	}
	sources := []city.Source{izmir.NewESHOT("b", st, fc, izmir.DefaultEndpoints)}
	for _, f := range []struct{ prefix, url, name string }{
		{"m", "https://www.izmirmetro.com.tr/gtfs/rail-metro-gtfs.zip", "Metro İzmir"},
		{"t", "https://www.tramizmir.com/gtfs/rail-tramizmir-gtfs.zip", "İzmir Tramvay"},
		{"f", "https://www.izdeniz.com.tr/gtfs/ship-izdeniz-gtfs.zip", "İzdeniz (vapur)"},
	} {
		s, err := gtfsSource(ctx, fc, f.prefix, f.url, f.name, lic, portal, true)
		if err != nil {
			return nil, err
		}
		if s != nil {
			sources = append(sources, s)
		}
	}
	return city.New(city.Meta{ID: "izmir", Name: "İzmir", Center: model.LatLon{38.4237, 27.1428}, Zoom: 12.5}, sources...), nil
}

func ckanCity(meta city.Meta, prefix, portal, pkg, name, license string) loader {
	return func(ctx context.Context, fc *fetch.Client) (*city.City, error) {
		url, err := fc.CKANLatest(ctx, portal, pkg, "ZIP")
		if err != nil {
			return nil, errors.Join(errors.New("ckan discovery failed"), err)
		}
		s, err := gtfsSource(ctx, fc, prefix, url, name, license, portal+"/dataset/"+pkg, false)
		if err != nil {
			return nil, err
		}
		return city.New(meta, s), nil
	}
}

// markIfExpired keeps an expired feed useful: timetables follow the
// weekday pattern and the app shows a clear "may be outdated" note.
func markIfExpired(f *gtfs.Feed, info *model.SourceInfo) {
	today := time.Now()
	if end := f.ServiceEnd(); end > 0 && end < today.Year()*10000+int(today.Month())*100+today.Day() {
		f.StaleCalendar = true
		info.Stale = true
		if info.Note == "" {
			info.Note = fmt.Sprintf("Bu verinin tarifesi %d.%02d.%d tarihinde sona ermiş. Hatlar ve saatler değişmiş olabilir.", end%100, end/100%100, end/10000)
		}
	}
}

// ckanLooseGTFS downloads a GTFS published as separate .txt resources
// (or as a single zip) on a CKAN portal and returns it as one zip.
func ckanLooseGTFS(ctx context.Context, fc *fetch.Client, portal, pkg string) ([]byte, time.Time, error) {
	res, err := fc.CKANResources(ctx, portal, pkg)
	if err != nil {
		return nil, time.Time{}, err
	}
	files := map[string][]byte{}
	var fetched time.Time
	for _, r := range res {
		base := strings.ToLower(path.Base(strings.Split(r.URL, "?")[0]))
		if strings.HasSuffix(base, ".zip") {
			z, err := fc.Cached(ctx, r.URL, staticMaxAge)
			if err == nil {
				return z.Data, z.FetchedAt, nil
			}
			continue
		}
		if !strings.HasSuffix(base, ".txt") {
			base = strings.ToLower(strings.TrimSpace(r.Name))
			if !strings.HasSuffix(base, ".txt") {
				base += ".txt"
			}
		}
		b, err := fc.Cached(ctx, r.URL, staticMaxAge)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("%s: %w", base, err)
		}
		files[base] = b.Data
		fetched = b.FetchedAt
	}
	for _, need := range []string{"stops.txt", "routes.txt", "trips.txt", "stop_times.txt"} {
		if files[need] == nil {
			return nil, time.Time{}, errors.New("ckan: missing " + need)
		}
	}
	z, err := gtfs.ZipFiles(files)
	return z, fetched, err
}

const (
	kocaeliPortal  = "https://veri.kocaeli.bel.tr"
	kocaeliPackage = "toplu-ulasim-verisi"
	// 2018 feed archived by MobilityData (mdb-1122), used only when the
	// current feed (reachable from Türkiye only) cannot be downloaded.
	kocaeliArchive = "https://files.mobilitydatabase.org/mdb-1122/mdb-1122-202406071521/mdb-1122-202406071521.zip"
)

func loadKocaeli(ctx context.Context, fc *fetch.Client) (*city.City, error) {
	meta := city.Meta{ID: "kocaeli", Name: "Kocaeli", Center: model.LatLon{40.7654, 29.9408}, Zoom: 12}
	const lic = "CC BY (Kocaeli Büyükşehir Belediyesi)"
	if z, at, err := ckanLooseGTFS(ctx, fc, kocaeliPortal, kocaeliPackage); err == nil {
		if f, err := gtfs.Parse(z); err == nil {
			info := model.SourceInfo{Name: "Kocaeli Büyükşehir Belediyesi", License: lic, URL: kocaeliPortal + "/dataset/" + kocaeliPackage, UpdatedAt: at}
			markIfExpired(f, &info)
			return city.New(meta, gtfs.NewSource("kc", f, info)), nil
		} else {
			fc.Log.Warn("kocaeli current feed unparsable, using archive", "err", err)
		}
	} else {
		fc.Log.Warn("kocaeli current feed unavailable (Türkiye IP needed?), using 2018 archive", "err", err)
	}
	res, err := fc.Cached(ctx, kocaeliArchive, 30*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("kocaeli archive: %w", err)
	}
	f, err := gtfs.Parse(res.Data)
	if err != nil {
		return nil, fmt.Errorf("kocaeli archive: %w", err)
	}
	f.StaleCalendar = true
	info := model.SourceInfo{
		Name: "Kocaeli Büyükşehir Belediyesi (2018 arşivi)", License: lic,
		URL: "https://mobilitydatabase.org/feeds/gtfs/mdb-1122", UpdatedAt: res.FetchedAt,
		Stale: true,
		Note:  "Kocaeli'nin güncel verisine şu an ulaşılamadı; 2018 tarihli arşiv gösteriliyor. Duraklar büyük ölçüde doğru olsa da hatlar ve saatler değişmiş olabilir.",
	}
	return city.New(meta, gtfs.NewSource("kc", f, info)), nil
}
