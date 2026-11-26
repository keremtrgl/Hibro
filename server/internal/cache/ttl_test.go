package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollapsesConcurrentCalls(t *testing.T) {
	c := New[int](time.Minute, time.Second, 100)
	var calls atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.Get(context.Background(), "k", func(context.Context) (int, error) {
				calls.Add(1)
				time.Sleep(50 * time.Millisecond)
				return 42, nil
			})
			if err != nil || v != 42 {
				t.Errorf("got %d %v", v, err)
			}
		}()
	}
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("upstream called %d times", n)
	}
}

func TestErrorsExpireQuickly(t *testing.T) {
	c := New[int](time.Minute, 20*time.Millisecond, 100)
	boom := errors.New("boom")
	n := 0
	fn := func(context.Context) (int, error) { n++; return 0, boom }
	c.Get(context.Background(), "k", fn)
	c.Get(context.Background(), "k", fn)
	if n != 1 {
		t.Fatalf("error not cached briefly: %d", n)
	}
	time.Sleep(30 * time.Millisecond)
	c.Get(context.Background(), "k", fn)
	if n != 2 {
		t.Fatalf("error cached too long: %d", n)
	}
}

func TestStaleWhileRevalidate(t *testing.T) {
	c := New[int](20*time.Millisecond, 20*time.Millisecond, 100).WithStale(time.Second)
	var calls atomic.Int32
	slow := func(context.Context) (int, error) {
		n := calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return int(n), nil
	}
	if v, _ := c.Get(context.Background(), "k", slow); v != 1 {
		t.Fatalf("first = %d", v)
	}
	time.Sleep(30 * time.Millisecond) // expired, inside the stale window
	start := time.Now()
	v, err := c.Get(context.Background(), "k", slow)
	if err != nil || v != 1 || time.Since(start) > 20*time.Millisecond {
		t.Fatalf("stale value should come back instantly: v=%d err=%v took=%s", v, err, time.Since(start))
	}
	time.Sleep(80 * time.Millisecond) // background refresh done
	if v, _ := c.Get(context.Background(), "k", slow); v != 2 {
		t.Fatalf("refreshed value = %d", v)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("upstream calls = %d", n)
	}
}

func TestFailedRefreshKeepsServingStale(t *testing.T) {
	c := New[int](10*time.Millisecond, 200*time.Millisecond, 100).WithStale(time.Second)
	var calls atomic.Int32
	ok := true
	fn := func(context.Context) (int, error) {
		calls.Add(1)
		if !ok {
			return 0, errors.New("down")
		}
		return 7, nil
	}
	c.Get(context.Background(), "k", fn)
	ok = false
	time.Sleep(20 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if v, err := c.Get(context.Background(), "k", fn); v != 7 || err != nil {
			t.Fatalf("should keep serving the last good value: %d %v", v, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := calls.Load(); n > 2 {
		t.Fatalf("failed refresh must not be retried on every request: %d calls", n)
	}
}
