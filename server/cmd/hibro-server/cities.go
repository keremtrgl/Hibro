package main

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
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
	"istanbul":  loadIstanbul,
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
	// ESHOT GTFS gives the stop order of bus lines (the live API does
	// not). Its timetable is old, so only stop sequences are used.
	if f := eshotGTFS(ctx, fc); f != nil {
		izmir.ApplyGTFS(st, f, "ESHOT GTFS")
	}
	sources := []city.Source{izmir.NewESHOT("b", st, fc, izmir.DefaultEndpoints)}
	for _, f := range []struct{ prefix, url, name string }{
		{"z", "https://www.izban.com.tr/gtfs/rail-izban-gtfs.zip", "İZBAN (banliyö)"},
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
		if !gtfsFiles[base] {
			continue // data dictionaries, PDFs, previews...
		}
		b, err := fc.Cached(ctx, r.URL, staticMaxAge)
		if err != nil {
			if requiredGTFS[base] {
				return nil, time.Time{}, fmt.Errorf("%s: %w", base, err)
			}
			continue
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

var gtfsFiles = map[string]bool{
	"agency.txt": true, "stops.txt": true, "routes.txt": true, "trips.txt": true, "stop_times.txt": true,
	"calendar.txt": true, "calendar_dates.txt": true, "shapes.txt": true, "frequencies.txt": true,
	"transfers.txt": true, "feed_info.txt": true, "fare_attributes.txt": true, "fare_rules.txt": true,
	"translations.txt": true,
}

var requiredGTFS = map[string]bool{"stops.txt": true, "routes.txt": true, "trips.txt": true, "stop_times.txt": true}

// ---- İstanbul: İBB open data (reachable from Türkiye only). Loads
// whatever of İETT and the general public-transport GTFS is available. ----

const ibbPortal = "https://data.ibb.gov.tr"

func loadIstanbul(ctx context.Context, fc *fetch.Client) (*city.City, error) {
	const lic = "İBB Açık Veri Lisansı"
	var sources []city.Source
	var errs []error
	for _, d := range []struct{ prefix, pkg, name string }{
		{"ist", "iett-gtfs-verisi", "İETT (otobüs)"},
		{"isr", "public-transport-gtfs-data", "İBB toplu ulaşım GTFS"},
	} {
		z, at, err := ckanLooseGTFS(ctx, fc, ibbPortal, d.pkg)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.pkg, err))
			continue
		}
		f, err := gtfs.Parse(z)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.pkg, err))
			continue
		}
		info := model.SourceInfo{Name: d.name, License: lic, URL: ibbPortal + "/dataset/" + d.pkg, UpdatedAt: at}
		markIfExpired(f, &info)
		sources = append(sources, gtfs.NewSource(d.prefix, f, info))
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("istanbul: no feed (data.ibb.gov.tr needs a Türkiye IP): %w", errors.Join(errs...))
	}
	return city.New(city.Meta{ID: "istanbul", Name: "İstanbul", Center: model.LatLon{41.0082, 28.9784}, Zoom: 11.5}, sources...), nil
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
	if communityEnabled() {
		if c, err := communityCity(ctx, fc, communityCities["kocaeli"]); err == nil {
			return c, nil
		} else {
			fc.Log.Warn("kocaeli community feed unavailable, using 2018 archive", "err", err)
		}
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

// eshotGTFS downloads the ESHOT GTFS: directly (Türkiye IPs) or from the
// MobilityData mirror. Returns nil when neither works.
func eshotGTFS(ctx context.Context, fc *fetch.Client) *gtfs.Feed {
	for _, u := range []string{
		"https://www.eshot.gov.tr/gtfs/bus-eshot-gtfs.zip",
		"https://storage.googleapis.com/storage/v1/b/mdb-latest/o/tr-kocaeli-eshot-gtfs-1823.zip?alt=media",
	} {
		res, err := fc.Cached(ctx, u, 7*24*time.Hour)
		if err != nil {
			continue
		}
		if f, err := gtfs.Parse(res.Data); err == nil {
			return f
		}
	}
	fc.Log.Warn("eshot gtfs unavailable: bus lines will have no stop list")
	return nil
}

// ---- community feeds (generated from KentKart's public web data by the
// kk-gtfs project, mirrored monthly by MobilityData). Not official:
// enabled with HIBRO_COMMUNITY=on and labelled in the app. Check the
// data licence before a public release (docs/VERI_KAYNAKLARI.md). ----

type communityFeed struct {
	ID, Name, Slug string
	MDB            int
	Zoom           float64
}

var communityCities = map[string]communityFeed{
	"adana":      {"adana", "Adana", "adana", 3407, 12},
	"antalya":    {"antalya", "Antalya", "antalya", 3406, 12},
	"burdur":     {"burdur", "Burdur", "burdur", 3405, 13},
	"canakkale":  {"canakkale", "Çanakkale", "canakkale", 3404, 13},
	"duzce":      {"duzce", "Düzce", "duzce", 3403, 13},
	"erzurum":    {"erzurum", "Erzurum", "erzurum", 3402, 12.5},
	"kastamonu":  {"kastamonu", "Kastamonu", "kastamonu", 3400, 13},
	"kirklareli": {"kirklareli", "Kırklareli", "kirklareli", 3399, 13},
	"kocaeli":    {"kocaeli", "Kocaeli", "kocaeli", 3398, 12},
	"mardin":     {"mardin", "Mardin", "mardin", 3397, 12.5},
	"mugla":      {"mugla", "Muğla", "mugla", 3396, 11},
	"ordu":       {"ordu", "Ordu", "ordu", 3395, 12.5},
	"osmaniye":   {"osmaniye", "Osmaniye", "osmaniye", 3394, 13},
	"safranbolu": {"safranbolu", "Safranbolu", "safranbolu", 3393, 13},
}

func communityEnabled() bool { return env("HIBRO_COMMUNITY", "off") == "on" }

func init() {
	for id, cf := range communityCities {
		if _, taken := loaders[id]; taken {
			continue // official source wins (e.g. Kocaeli)
		}
		cf := cf
		loaders[id] = func(ctx context.Context, fc *fetch.Client) (*city.City, error) {
			if !communityEnabled() {
				return nil, errors.New("community data disabled (HIBRO_COMMUNITY=on to enable)")
			}
			return communityCity(ctx, fc, cf)
		}
	}
}

var mdbZipRe = regexp.MustCompile(`https://files\.mobilitydatabase\.org/mdb-\d+/mdb-\d+-\d+/mdb-\d+-\d+\.zip`)

// mdbLatest finds the newest dataset MobilityData stores for a feed.
func mdbLatest(ctx context.Context, fc *fetch.Client, id int) (string, error) {
	b, err := fc.Get(ctx, fmt.Sprintf("https://mobilitydatabase.org/feeds/gtfs/mdb-%d", id), 2)
	if err != nil {
		return "", err
	}
	all := mdbZipRe.FindAllString(string(b), -1)
	if len(all) == 0 {
		return "", errors.New("mobilitydatabase: no dataset link")
	}
	sort.Strings(all)
	return all[len(all)-1], nil
}

func communityCity(ctx context.Context, fc *fetch.Client, cf communityFeed) (*city.City, error) {
	var data []byte
	var at time.Time
	urls := []string{"https://github.com/Egezenn/kk-gtfs/raw/main/data/" + cf.Slug + ".zip"}
	if u, err := mdbLatest(ctx, fc, cf.MDB); err == nil {
		urls = append(urls, u)
	}
	var lastErr error
	for _, u := range urls {
		res, err := fc.Cached(ctx, u, 7*24*time.Hour)
		if err != nil {
			lastErr = err
			continue
		}
		data, at = res.Data, res.FetchedAt
		break
	}
	if data == nil {
		return nil, fmt.Errorf("%s: %w", cf.ID, lastErr)
	}
	f, err := gtfs.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cf.ID, err)
	}
	info := model.SourceInfo{
		Name:       "Topluluk verisi (KentKart kaynaklı, kk-gtfs)",
		License:    "kk-gtfs veri lisansı",
		URL:        fmt.Sprintf("https://mobilitydatabase.org/feeds/gtfs/mdb-%d", cf.MDB),
		UpdatedAt:  at,
		Unofficial: true,
		Note:       "Bu şehrin verisi belediyeden değil; KentKart'ın herkese açık web verisinden topluluk tarafından üretildi. Küçük hatalar olabilir.",
	}
	markIfExpired(f, &info)
	src := gtfs.NewSource(cf.ID, f, info)
	lat, lon := medianCenter(src.Stops())
	return city.New(city.Meta{ID: cf.ID, Name: cf.Name, Center: model.LatLon{lat, lon}, Zoom: cf.Zoom}, src), nil
}

func medianCenter(stops []model.Stop) (float64, float64) {
	if len(stops) == 0 {
		return 39, 35
	}
	lat := make([]float64, len(stops))
	lon := make([]float64, len(stops))
	for i, s := range stops {
		lat[i], lon[i] = s.Lat, s.Lon
	}
	sort.Float64s(lat)
	sort.Float64s(lon)
	return lat[len(lat)/2], lon[len(lon)/2]
}
