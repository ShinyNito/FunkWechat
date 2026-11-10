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
	mu     sync.Mutex
	data   map[string]string
	ttls   map[string]time.Duration
	gets   int
	getErr error
}

func newRecordingCache() *recordingCache {
	return &recordingCache{data: map[string]string{}, ttls: map[string]time.Duration{}}
}

func (c *recordingCache) Get(_ context.Context, key string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	if c.getErr != nil {
		return "", false, c.getErr
	}
	v, ok := c.data[key]
	return v, ok, nil
}

func (c *recordingCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.ttls[key] = ttl
	return nil
}

func TestGetUsesStore(t *testing.T) {
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

func TestGetReadsMemoryWithoutTouchingStore(t *testing.T) {
	cache := newRecordingCache()
	var fetches atomic.Int32
	cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
		fetches.Add(1)
		return "fresh", 2 * time.Hour, nil
	}, nil)

	for range 100 {
		if got, err := cred.Get(context.Background()); err != nil || got != "fresh" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
	if fetches.Load() != 1 || cache.gets != 1 {
		t.Fatalf("fetches = %d, store gets = %d; want 1, 1", fetches.Load(), cache.gets)
	}
	if cache.data["k"] != "fresh" || cache.ttls["k"] != 2*time.Hour-expiryBuffer {
		t.Fatalf("store = %q ttl %v", cache.data["k"], cache.ttls["k"])
	}
}

func TestWorksWithoutStore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var fetches atomic.Int32
		cred := New(nil, "k", func(context.Context) (string, time.Duration, error) {
			fetches.Add(1)
			return "fresh", 2 * time.Hour, nil
		}, nil)

		for range 3 {
			if got, err := cred.Get(context.Background()); err != nil || got != "fresh" {
				t.Fatalf("got %q, %v", got, err)
			}
		}
		if fetches.Load() != 1 {
			t.Fatalf("fetches = %d; want 1", fetches.Load())
		}

		// 扣除提前过期时间后，内存中的值到期，需要重新获取。
		time.Sleep(2*time.Hour - expiryBuffer)
		if _, err := cred.Get(context.Background()); err != nil {
			t.Fatal(err)
		}
		if fetches.Load() != 2 {
			t.Fatalf("fetches after expiry = %d; want 2", fetches.Load())
		}
	})
}

func TestStoreValueIsRechecked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := newRecordingCache()
		_ = cache.Set(context.Background(), "k", "v1", time.Hour)
		cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
			t.Error("fetch should not be called")
			return "", 0, nil
		}, nil)

		if got, _ := cred.Get(context.Background()); got != "v1" {
			t.Fatalf("got %q; want v1", got)
		}
		// 其他进程更新了外部存储，本进程在 storeRecheck 之后读到新值。
		_ = cache.Set(context.Background(), "k", "v2", time.Hour)
		if got, _ := cred.Get(context.Background()); got != "v1" {
			t.Fatalf("got %q; want v1 from memory", got)
		}
		time.Sleep(storeRecheck)
		if got, _ := cred.Get(context.Background()); got != "v2" {
			t.Fatalf("got %q; want v2", got)
		}
	})
}

func TestStoreErrorFallsBackToFetch(t *testing.T) {
	cache := newRecordingCache()
	cache.getErr = errors.New("redis down")
	cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
		return "fresh", 2 * time.Hour, nil
	}, nil)

	if got, err := cred.Get(context.Background()); err != nil || got != "fresh" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRefreshDoesNotReuseOrdinaryFetch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := newRecordingCache()
		var fetches atomic.Int32
		cred := New(cache, "k", func(context.Context) (string, time.Duration, error) {
			n := fetches.Add(1)
			time.Sleep(time.Second)
			if n == 1 {
				return "first", 2 * time.Hour, nil
			}
			return "second", 2 * time.Hour, nil
		}, nil)

		ordinary := make(chan string, 1)
		go func() {
			got, _ := cred.Get(context.Background())
			ordinary <- got
		}()
		synctest.Wait()

		// Refresh 在普通获取进行中发起：等它结束后仍要自己强制获取一次。
		if got, err := cred.Refresh(context.Background()); err != nil || got != "second" {
			t.Fatalf("refresh got %q, %v; want second", got, err)
		}
		if got := <-ordinary; got != "first" {
			t.Fatalf("ordinary got %q; want first", got)
		}
		if fetches.Load() != 2 {
			t.Fatalf("fetches = %d; want 2", fetches.Load())
		}
	})
}

func TestUsableTTL(t *testing.T) {
	tests := []struct {
		ttl, want time.Duration
	}{
		{2 * time.Hour, 2*time.Hour - expiryBuffer},
		{6 * time.Minute, 3 * time.Minute},
		{0, time.Second},
	}
	for _, tt := range tests {
		if got := usableTTL(tt.ttl); got != tt.want {
			t.Errorf("usableTTL(%v) = %v; want %v", tt.ttl, got, tt.want)
		}
	}
}
