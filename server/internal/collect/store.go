// Package collect stores the data riders choose to contribute: problem
// reports, anonymous usage counters and opt-in trip GPS traces.
//
// Privacy rules enforced here (see docs/VERI_TOPLAMA.md):
//   - No IP addresses, device IDs or accounts are ever written.
//   - Records carry only server-side day/time, never client clocks
//     beyond trip point timestamps.
//   - Raw trip traces are deleted after a retention period.
package collect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store appends JSON records to one file per kind per day:
// <dir>/<kind>/2026-11-26.jsonl
type Store struct {
	dir   string
	mu    sync.Mutex
	files map[string]*os.File
	Now   func() time.Time
	Loc   *time.Location
}

func NewStore(dir string, loc *time.Location) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	if loc == nil {
		loc = time.UTC
	}
	return &Store{dir: dir, files: map[string]*os.File{}, Now: time.Now, Loc: loc}, nil
}

func (s *Store) day() string { return s.Now().In(s.Loc).Format("2006-01-02") }

// Append writes v as one JSON line.
func (s *Store) Append(kind string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := kind + "/" + s.day()
	f := s.files[key]
	if f == nil {
		// Close yesterday's file of the same kind.
		for k, old := range s.files {
			if strings.HasPrefix(k, kind+"/") {
				old.Close()
				delete(s.files, k)
			}
		}
		if err := os.MkdirAll(filepath.Join(s.dir, kind), 0o750); err != nil {
			return err
		}
		f, err = os.OpenFile(filepath.Join(s.dir, key+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		s.files[key] = f
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// WriteSnapshot replaces <dir>/<kind>/<day>.json atomically.
func (s *Store) WriteSnapshot(kind, day string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	d := filepath.Join(s.dir, kind)
	if err := os.MkdirAll(d, 0o750); err != nil {
		return err
	}
	tmp := filepath.Join(d, day+".json.tmp")
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(d, day+".json"))
}

func (s *Store) ReadSnapshot(kind, day string, v any) error {
	b, err := os.ReadFile(filepath.Join(s.dir, kind, day+".json"))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Cleanup deletes files of kind older than keep. It returns the number
// of files removed.
func (s *Store) Cleanup(kind string, keep time.Duration) int {
	cutoff := s.Now().In(s.Loc).Add(-keep).Format("2006-01-02")
	entries, err := os.ReadDir(filepath.Join(s.dir, kind))
	if err != nil {
		return 0
	}
	n := 0
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		day := strings.SplitN(e.Name(), ".", 2)[0]
		if len(day) == 10 && day < cutoff {
			key := kind + "/" + day
			if f := s.files[key]; f != nil {
				f.Close()
				delete(s.files, key)
			}
			if os.Remove(filepath.Join(s.dir, kind, e.Name())) == nil {
				n++
			}
		}
	}
	return n
}

// Days lists the days (newest first) that have files of kind.
func (s *Store) Days(kind string) []string {
	entries, _ := os.ReadDir(filepath.Join(s.dir, kind))
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		day := strings.SplitN(e.Name(), ".", 2)[0]
		if len(day) == 10 && !seen[day] {
			seen[day] = true
			out = append(out, day)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// Scan calls fn for every JSON line of kind on day.
func (s *Store) Scan(kind, day string, fn func([]byte)) error {
	s.mu.Lock()
	if f := s.files[kind+"/"+day]; f != nil {
		_ = f.Sync()
	}
	s.mu.Unlock()
	f, err := os.Open(filepath.Join(s.dir, kind, day+".jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	return sc.Err()
}

func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, f := range s.files {
		f.Close()
		delete(s.files, k)
	}
}

func (s *Store) String() string { return fmt.Sprintf("collect.Store(%s)", s.dir) }
