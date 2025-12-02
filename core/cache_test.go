package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err)

			got, ok := cache.Get(ctx, tt.key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

func TestMemoryCache_GetNonExistent(t *testing.T) {
	cache := NewMemoryCache()
	ctx := context.Background()

	got, ok := cache.Get(ctx, "non_existent_key")
	assert.False(t, ok, "should return false for non-existent key")
	assert.Empty(t, got, "should return empty string")
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
				require.NoError(t, err)
			}

			_, ok := cache.Get(ctx, tt.key)
			assert.Equal(t, tt.wantOK, ok)
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
			cache := NewMemoryCache()
			ctx := context.Background()

			_ = cache.Set(ctx, "expire_test", "value", tt.ttl)
			time.Sleep(tt.sleepTime)

			_, ok := cache.Get(ctx, "expire_test")
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestMemoryCache_Overwrite(t *testing.T) {
	cache := NewMemoryCache()
	ctx := context.Background()

	_ = cache.Set(ctx, "key", "value1", time.Hour)
	_ = cache.Set(ctx, "key", "value2", time.Hour)

	got, ok := cache.Get(ctx, "key")
	require.True(t, ok)
	assert.Equal(t, "value2", got)
}

func TestMemoryCache_Cleanup(t *testing.T) {
	cache := NewMemoryCache()
	ctx := context.Background()

	_ = cache.Set(ctx, "expired", "value", 1*time.Millisecond)
	_ = cache.Set(ctx, "valid", "value", time.Hour)

	time.Sleep(10 * time.Millisecond)
	cache.Cleanup()

	_, okExpired := cache.Get(ctx, "expired")
	_, okValid := cache.Get(ctx, "valid")

	assert.False(t, okExpired, "expired key should be cleaned up")
	assert.True(t, okValid, "valid key should still exist")
}
