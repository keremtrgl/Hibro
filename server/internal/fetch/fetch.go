// Package fetch downloads municipal data with an on-disk cache.
//
// Municipal servers are slow and sometimes down. Every static download is
// kept on disk; when a refresh fails the last good copy is used, so the
// app keeps working on yesterday's data instead of showing nothing.
package fetch

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const userAgent = "Hibro/0.1 (+toplu tasima uygulamasi; reklamsiz)"

type Client struct {
	HTTP     *http.Client
	CacheDir string
	Log      *slog.Logger
}

func New(cacheDir string) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 5 * time.Minute},
		CacheDir: cacheDir,
		Log:      slog.Default(),
	}
}

// Result describes where a cached download came from.
type Result struct {
	Data      []byte
	FetchedAt time.Time
	Stale     bool // true when the network failed and an old copy was used
}

func (c *Client) cachePath(url string) string {
	h := sha1.Sum([]byte(url))
	base := filepath.Base(strings.Split(url, "?")[0])
	if len(base) > 40 {
		base = base[:40]
	}
	return filepath.Join(c.CacheDir, hex.EncodeToString(h[:6])+"-"+base)
}

// Cached returns the body of url, using the disk copy if it is younger
// than maxAge. On network failure it falls back to any disk copy.
func (c *Client) Cached(ctx context.Context, url string, maxAge time.Duration) (Result, error) {
	p := c.cachePath(url)
	if st, err := os.Stat(p); err == nil && time.Since(st.ModTime()) < maxAge {
		b, err := os.ReadFile(p)
		if err == nil {
			return Result{Data: b, FetchedAt: st.ModTime()}, nil
		}
	}
	b, err := c.Get(ctx, url, 3)
	if err == nil {
		if mkErr := os.MkdirAll(c.CacheDir, 0o755); mkErr == nil {
			tmp := p + ".tmp"
			if wErr := os.WriteFile(tmp, b, 0o644); wErr == nil {
				_ = os.Rename(tmp, p)
			}
		}
		return Result{Data: b, FetchedAt: time.Now()}, nil
	}
	if st, statErr := os.Stat(p); statErr == nil {
		if old, rErr := os.ReadFile(p); rErr == nil {
			c.Log.Warn("download failed, using cached copy", "url", url, "err", err, "age", time.Since(st.ModTime()).Round(time.Minute))
			return Result{Data: old, FetchedAt: st.ModTime(), Stale: true}, nil
		}
	}
	return Result{}, err
}

// Get downloads url with retries and exponential backoff.
func (c *Client) Get(ctx context.Context, url string, attempts int) ([]byte, error) {
	var last error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<i) * time.Second):
			}
		}
		b, err := c.once(ctx, url)
		if err == nil {
			return b, nil
		}
		last = err
	}
	return nil, fmt.Errorf("get %s: %w", url, last)
}

func (c *Client) once(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<20))
}

// JSON fetches url (no disk cache) and decodes it into v.
func (c *Client) JSON(ctx context.Context, url string, v any) error {
	b, err := c.Get(ctx, url, 2)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// CKANLatest finds the newest resource of a CKAN dataset whose format
// matches (case-insensitive). Konya, for example, publishes a new
// gtfs_11_YYYY.zip every year under a new URL.
func (c *Client) CKANLatest(ctx context.Context, portal, pkg, format string) (string, error) {
	var out struct {
		Success bool `json:"success"`
		Result  struct {
			Resources []struct {
				URL      string `json:"url"`
				Name     string `json:"name"`
				Format   string `json:"format"`
				Created  string `json:"created"`
				Modified string `json:"last_modified"`
			} `json:"resources"`
		} `json:"result"`
	}
	u := strings.TrimRight(portal, "/") + "/api/3/action/package_show?id=" + pkg
	if err := c.JSON(ctx, u, &out); err != nil {
		return "", err
	}
	res := out.Result.Resources
	var cands []int
	for i, r := range res {
		if strings.EqualFold(r.Format, format) {
			cands = append(cands, i)
		}
	}
	if len(cands) == 0 {
		return "", errors.New("ckan: no resource with format " + format)
	}
	sort.SliceStable(cands, func(a, b int) bool {
		ra, rb := res[cands[a]], res[cands[b]]
		ka, kb := ra.Name+ra.Created, rb.Name+rb.Created
		return ka > kb // names embed the year (gtfs_11_2026.zip), then creation time
	})
	return res[cands[0]].URL, nil
}
