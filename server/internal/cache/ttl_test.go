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
