package credential

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type recordingCache struct {
	mu   sync.Mutex
	data map[string]string
	ttls map[string]time.Duration
}

func newRecordingCache() *recordingCache {
	return &recordingCache{data: map[string]string{}, ttls: map[string]time.Duration{}}
}

func (c *recordingCache) Get(_ context.Context, key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *recordingCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.ttls[key] = ttl
	return nil
}

func TestGetUsesCache(t *testing.T) {
	cache := newRecordingCache()
	_ = cache.Set(context.Background(), "k", "cached", time.Hour)
	cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
		t.Error("fetch should not be called")
		return "", 0, nil
	}, nil)

	if got, err := cred.Get(context.Background()); err != nil || got != "cached" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGetMergesConcurrentFetches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var fetches atomic.Int32
		cred := New(newRecordingCache(), "k", func(context.Context) (string, time.Duration, error) {
			fetches.Add(1)
			time.Sleep(time.Second)
			return "fresh", 2 * time.Hour, nil
		}, nil)

		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				if got, err := cred.Get(context.Background()); err != nil || got != "fresh" {
					t.Errorf("got %q, %v", got, err)
				}
			})
		}
		wg.Wait()
		if fetches.Load() != 1 {
			t.Fatalf("fetches = %d; want 1", fetches.Load())
		}
	})
}

func TestRefreshBypassesCache(t *testing.T) {
	cache := newRecordingCache()
	_ = cache.Set(context.Background(), "k", "stale", time.Hour)
	cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
		return "fresh", 2 * time.Hour, nil
	}, nil)

	if got, err := cred.Refresh(context.Background()); err != nil || got != "fresh" {
		t.Fatalf("got %q, %v", got, err)
	}
	if got, _ := cred.Get(context.Background()); got != "fresh" {
		t.Fatalf("cached = %q; want fresh", got)
	}
}

func TestCanceledCallerDoesNotAffectOtherWaiters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cred := New(newRecordingCache(), "k", func(ctx context.Context) (string, time.Duration, error) {
			time.Sleep(time.Second)
			if err := ctx.Err(); err != nil {
				return "", 0, err
			}
			return "fresh", 2 * time.Hour, nil
		}, nil)

		ctx, cancel := context.WithCancel(context.Background())
		first := make(chan error, 1)
		go func() {
			_, err := cred.Get(ctx)
			first <- err
		}()
		synctest.Wait()

		second := make(chan string, 1)
		go func() {
			got, _ := cred.Get(context.Background())
			second <- got
		}()
		synctest.Wait()

		cancel()
		if err := <-first; !errors.Is(err, context.Canceled) {
			t.Fatalf("first caller err = %v; want context.Canceled", err)
		}
		if got := <-second; got != "fresh" {
			t.Fatalf("second caller got %q; want fresh", got)
		}
	})
}

func TestFetchTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cred := New(newRecordingCache(), "k", func(ctx context.Context) (string, time.Duration, error) {
			<-ctx.Done()
			return "", 0, ctx.Err()
		}, nil)

		start := time.Now()
		_, err := cred.Get(context.Background())
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != fetchTimeout {
			t.Fatalf("err = %v after %v", err, time.Since(start))
		}
	})
}

func TestFetchErrorIsNotCached(t *testing.T) {
	cache := newRecordingCache()
	fetchErr := errors.New("boom")
	var fetches atomic.Int32
	cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
		if fetches.Add(1) == 1 {
			return "", 0, fetchErr
		}
		return "fresh", 2 * time.Hour, nil
	}, nil)

	if _, err := cred.Get(context.Background()); !errors.Is(err, fetchErr) {
		t.Fatalf("err = %v", err)
	}
	if got, err := cred.Get(context.Background()); err != nil || got != "fresh" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestEmptyValueIsAnError(t *testing.T) {
	cred := New(newRecordingCache(), "k", func(context.Context) (string, time.Duration, error) {
		return "", time.Hour, nil
	}, nil)
	if _, err := cred.Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCacheTTL(t *testing.T) {
	tests := []struct {
		ttl, want time.Duration
	}{
		{2 * time.Hour, 2*time.Hour - expiryBuffer},
		{6 * time.Minute, 3 * time.Minute},
		{0, time.Second},
	}
	for _, tt := range tests {
		if got := cacheTTL(tt.ttl); got != tt.want {
			t.Errorf("cacheTTL(%v) = %v; want %v", tt.ttl, got, tt.want)
		}
	}
}
