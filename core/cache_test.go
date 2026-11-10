package core

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		ttl       time.Duration
		wantValue string
		wantOK    bool
	}{
		{
			name:      "set and get value",
			key:       "test_key",
			value:     "test_value",
			ttl:       time.Hour,
			wantValue: "test_value",
			wantOK:    true,
		},
		{
			name:      "set with zero ttl (never expire)",
			key:       "never_expire",
			value:     "permanent",
			ttl:       0,
			wantValue: "permanent",
			wantOK:    true,
		},
		{
			name:      "empty value",
			key:       "empty",
			value:     "",
			ttl:       time.Hour,
			wantValue: "",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache()
			ctx := context.Background()

			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, ok := cache.Get(ctx, tt.key)
			if ok != tt.wantOK {
				t.Errorf("got %v; want %v", ok, tt.wantOK)
			}
			if got != tt.wantValue {
				t.Errorf("got %v; want %v", got, tt.wantValue)
			}
		})
	}
}

func TestMemoryCache_GetNonExistent(t *testing.T) {
	cache := NewMemoryCache()
	ctx := context.Background()

	got, ok := cache.Get(ctx, "non_existent_key")
	if ok {
		t.Errorf("expected false")
	}
	if got != "" {
		t.Errorf("expected empty"+": %q", got)
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		delete bool
		wantOK bool
	}{
		{
			name:   "delete existing key",
			key:    "to_delete",
			value:  "value",
			delete: true,
			wantOK: false,
		},
		{
			name:   "delete non-existent key (no error)",
			key:    "non_existent",
			value:  "",
			delete: true,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache()
			ctx := context.Background()

			if tt.value != "" {
				_ = cache.Set(ctx, tt.key, tt.value, time.Hour)
			}

			if tt.delete {
				err := cache.Delete(ctx, tt.key)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			_, ok := cache.Get(ctx, tt.key)
			if ok != tt.wantOK {
				t.Errorf("got %v; want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	tests := []struct {
		name      string
		ttl       time.Duration
		sleepTime time.Duration
		wantOK    bool
	}{
		{
			name:      "not expired yet",
			ttl:       100 * time.Millisecond,
			sleepTime: 10 * time.Millisecond,
			wantOK:    true,
		},
		{
			name:      "expired",
			ttl:       10 * time.Millisecond,
			sleepTime: 50 * time.Millisecond,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cache := NewMemoryCache()
				ctx := context.Background()

				_ = cache.Set(ctx, "expire_test", "value", tt.ttl)
				time.Sleep(tt.sleepTime)

				_, ok := cache.Get(ctx, "expire_test")
				if ok != tt.wantOK {
					t.Errorf("got %v; want %v", ok, tt.wantOK)
				}
			})
		})
	}
}

func TestMemoryCache_GetRemovesExpiredItem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := NewMemoryCache()
		ctx := context.Background()

		_ = cache.Set(ctx, "temp", "value", 5*time.Millisecond)
		time.Sleep(10 * time.Millisecond)

		_, ok := cache.Get(ctx, "temp")
		if ok {
			t.Errorf("expected false")
		}

		cache.mu.RLock()
		_, exists := cache.items["temp"]
		cache.mu.RUnlock()

		if exists {
			t.Errorf("expected false")
		}
	})
}

func TestMemoryCache_Overwrite(t *testing.T) {
	cache := NewMemoryCache()
	ctx := context.Background()

	_ = cache.Set(ctx, "key", "value1", time.Hour)
	_ = cache.Set(ctx, "key", "value2", time.Hour)

	got, ok := cache.Get(ctx, "key")
	if !ok {
		t.Fatalf("expected true")
	}
	if got != "value2" {
		t.Errorf("got %v; want %v", got, "value2")
	}
}

func TestMemoryCache_Cleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := NewMemoryCache()
		ctx := context.Background()

		_ = cache.Set(ctx, "expired", "value", 1*time.Millisecond)
		_ = cache.Set(ctx, "valid", "value", time.Hour)

		time.Sleep(10 * time.Millisecond)
		cache.Cleanup()

		_, okExpired := cache.Get(ctx, "expired")
		_, okValid := cache.Get(ctx, "valid")

		if okExpired {
			t.Errorf("expected false")
		}
		if !okValid {
			t.Errorf("expected true")
		}
	})
}
