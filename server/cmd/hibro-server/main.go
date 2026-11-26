// Command hibro-server serves Hibro's transit API.
//
// Configuration (environment variables):
//
//	HIBRO_ADDR        listen address            (default :8080)
//	HIBRO_DATA_DIR    download cache directory  (default ./data-cache)
//	HIBRO_CITIES      comma separated city ids  (default izmir,konya,gaziantep)
//	HIBRO_MOTIS_URL   MOTIS base URL, enables /plan (optional)
//	HIBRO_REFRESH     static data refresh period (default 24h)
//	HIBRO_LOG_LEVEL   debug|info|warn           (default info)
//
// Opt-in data collection (see docs/VERI_TOPLAMA.md):
//
//	HIBRO_COLLECT              on|off                          (default on)
//	HIBRO_COLLECT_DIR          where contributions are stored  (default ./collected)
//	HIBRO_TRIP_RETENTION_DAYS  raw GPS trace retention         (default 30)
//	HIBRO_REPORT_RETENTION_DAYS rider report retention         (default 365)
//	HIBRO_TRUST_PROXY          1 behind Caddy (rate limiting)  (default 0)
//	HIBRO_ADMIN_TOKEN          enables GET /admin/stats        (optional)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // the container image may have no zoneinfo

	"github.com/keremtrgl/hibro/server/internal/api"
	"github.com/keremtrgl/hibro/server/internal/city"
	"github.com/keremtrgl/hibro/server/internal/fetch"
	"github.com/keremtrgl/hibro/server/internal/plan"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func main() {
	level := slog.LevelInfo
	switch env("HIBRO_LOG_LEVEL", "info") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	fc := fetch.New(env("HIBRO_DATA_DIR", "./data-cache"))
	fc.Log = log
	reg := city.NewRegistry()
	reg.Log = log
	for _, id := range strings.Split(env("HIBRO_CITIES", "izmir,konya,gaziantep"), ",") {
		id = strings.TrimSpace(id)
		l, ok := loaders[id]
		if !ok {
			log.Error("unknown city, skipping", "city", id)
			continue
		}
		reg.Add(id, func(ctx context.Context) (*city.City, error) { return l(ctx, fc) })
	}

	refresh, err := time.ParseDuration(env("HIBRO_REFRESH", "24h"))
	if err != nil || refresh < time.Hour {
		refresh = 24 * time.Hour
	}
	loc, _ := time.LoadLocation("Europe/Istanbul")
	srv := &api.Server{Cities: reg, Planner: plan.NewMOTIS(os.Getenv("HIBRO_MOTIS_URL")), Log: log, Loc: loc}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if env("HIBRO_COLLECT", "on") != "off" {
		col, err := setupCollect(ctx, loc, log)
		if err != nil {
			log.Error("data collection disabled", "err", err)
		} else {
			srv.Collect = col
			defer col.Store.Close()
		}
	}

	// Serve immediately (health reports 503 until a city is loaded) and
	// load cities in the background so a slow municipality never blocks
	// the others.
	go func() {
		reg.LoadAll(ctx)
		reg.RefreshEvery(ctx, refresh)
	}()

	hs := &http.Server{
		Addr:              env("HIBRO_ADDR", ":8080"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	log.Info("hibro-server listening", "addr", hs.Addr, "planner", srv.Planner != nil)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
