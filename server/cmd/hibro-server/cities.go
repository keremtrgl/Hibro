package main

import (
	"context"
	"errors"
	"fmt"
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
			return gtfs.NewSource(prefix, f, model.SourceInfo{Name: name, License: license, URL: portal, UpdatedAt: res.FetchedAt}), nil
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
