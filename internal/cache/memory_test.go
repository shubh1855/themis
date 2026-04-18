package cache_test

import (
	"testing"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	c := cache.NewMemory(5*time.Minute, 1*time.Minute)
	defer c.Stop()

	c.Set("key1", "value1")

	got, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "value1" {
		t.Errorf("expected 'value1', got %v", got)
	}
}

func TestMemoryCache_MissOnExpired(t *testing.T) {
	c := cache.NewMemory(50*time.Millisecond, 10*time.Millisecond)
	defer c.Stop()

	c.Set("key1", "value1")

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected cache miss after TTL expiration")
	}
}

func TestMemoryCache_CustomTTL(t *testing.T) {
	c := cache.NewMemory(5*time.Minute, 1*time.Minute)
	defer c.Stop()

	c.SetWithTTL("short", "value", 50*time.Millisecond)
	c.SetWithTTL("long", "value", 5*time.Minute)

	time.Sleep(100 * time.Millisecond)

	_, shortOk := c.Get("short")
	_, longOk := c.Get("long")

	if shortOk {
		t.Error("expected short TTL key to expire")
	}
	if !longOk {
		t.Error("expected long TTL key to still exist")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	c := cache.NewMemory(5*time.Minute, 1*time.Minute)
	defer c.Stop()

	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected cache miss after delete")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	c := cache.NewMemory(5*time.Minute, 1*time.Minute)
	defer c.Stop()

	c.Set("k1", "v1")
	c.Set("k2", "v2")
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected empty cache after clear, got %d", c.Len())
	}
}

func TestHashKey_Deterministic(t *testing.T) {
	k1 := cache.HashKey("a", "b", "c")
	k2 := cache.HashKey("a", "b", "c")
	k3 := cache.HashKey("x", "y", "z")

	if k1 != k2 {
		t.Error("same inputs should produce same hash")
	}
	if k1 == k3 {
		t.Error("different inputs should produce different hash")
	}
}
