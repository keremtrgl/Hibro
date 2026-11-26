package main

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/keremtrgl/hibro/server/internal/api"
	"github.com/keremtrgl/hibro/server/internal/collect"
)

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(env(key, "")); err == nil && n > 0 {
		return n
	}
	return def
}

func setupCollect(ctx context.Context, loc *time.Location, log *slog.Logger) (*api.Collect, error) {
	store, err := collect.NewStore(env("HIBRO_COLLECT_DIR", "./collected"), loc)
	if err != nil {
		return nil, err
	}
	col := &api.Collect{
		Store:      store,
		Recent:     collect.NewRecent(2 * time.Hour),
		Counters:   collect.NewCounters(store),
		Limit:      collect.NewLimiter(20, 10), // 20/min, bursts of 10
		TeleLimit:  collect.NewLimiter(12, 6),
		TrustProxy: env("HIBRO_TRUST_PROXY", "0") == "1",
		AdminToken: env("HIBRO_ADMIN_TOKEN", ""),
	}
	tripDays := envInt("HIBRO_TRIP_RETENTION_DAYS", 30)
	reportDays := envInt("HIBRO_REPORT_RETENTION_DAYS", 365)
	go col.Counters.FlushEvery(5*time.Minute, ctx.Done())
	go func() {
		// Retention janitor: runs at start and then every 6 hours.
		for {
			t := store.Cleanup("trips", time.Duration(tripDays)*24*time.Hour)
			r := store.Cleanup("reports", time.Duration(reportDays)*24*time.Hour)
			if t+r > 0 {
				log.Info("retention cleanup", "tripFiles", t, "reportFiles", r)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(6 * time.Hour):
			}
		}
	}()
	log.Info("data collection enabled", "tripRetentionDays", tripDays, "reportRetentionDays", reportDays, "admin", col.AdminToken != "")
	return col, nil
}
