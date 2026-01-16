package cache

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewLRU(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	if cache == nil {
		t.Fatal("expected cache to be created")
	}
	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestLRUSetAndGet(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	// Set a value
	cache.Set("key1", "value1")

	// Get the value
	val, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	// Get non-existent key
	_, ok = cache.Get("key2")
	if ok {
		t.Error("expected key2 to not be found")
	}

	// Check size
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}
}

func TestLRUExpiration(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 50 * time.Millisecond,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")

	// Should be found immediately
	_, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestLRUSetWithTTL(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	// Set with short TTL
	cache.SetWithTTL("key1", "value1", 50*time.Millisecond)

	// Should be found immediately
	_, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestLRUEviction(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 3,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	// Fill the cache
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Access key1 to make it recently used
	cache.Get("key1")

	// Add another entry - should evict least recently used (key2)
	cache.Set("key4", "value4")

	// key1 and key3 and key4 should exist
	if _, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist")
	}
	if _, ok := cache.Get("key3"); !ok {
		t.Error("expected key3 to exist")
	}
	if _, ok := cache.Get("key4"); !ok {
		t.Error("expected key4 to exist")
	}

	// key2 should be evicted
	if _, ok := cache.Get("key2"); ok {
		t.Error("expected key2 to be evicted")
	}

	// Check eviction count
	metrics := cache.Metrics()
	if metrics.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", metrics.Evictions)
	}
}

func TestLRUUpdate(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")
	cache.Set("key1", "value2") // Update

	val, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}

	// Size should still be 1
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}
}

func TestLRUDelete(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Delete key1
	deleted := cache.Delete("key1")
	if !deleted {
		t.Error("expected delete to return true")
	}

	// key1 should be gone
	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be deleted")
	}

	// key2 should still exist
	if _, ok := cache.Get("key2"); !ok {
		t.Error("expected key2 to exist")
	}

	// Delete non-existent key
	deleted = cache.Delete("key3")
	if deleted {
		t.Error("expected delete to return false for non-existent key")
	}
}

func TestLRUDeletePrefix(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("user:1:name", "Alice")
	cache.Set("user:1:email", "alice@example.com")
	cache.Set("user:2:name", "Bob")
	cache.Set("other:key", "value")

	// Delete all user:1 entries
	count := cache.DeletePrefix("user:1:")
	if count != 2 {
		t.Errorf("expected 2 deletions, got %d", count)
	}

	// user:1 entries should be gone
	if _, ok := cache.Get("user:1:name"); ok {
		t.Error("expected user:1:name to be deleted")
	}

	// user:2 and other entries should still exist
	if _, ok := cache.Get("user:2:name"); !ok {
		t.Error("expected user:2:name to exist")
	}
	if _, ok := cache.Get("other:key"); !ok {
		t.Error("expected other:key to exist")
	}
}

func TestLRUDeleteSuffix(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("alice@example.com:pres1", "read")
	cache.Set("bob@example.com:pres1", "write")
	cache.Set("alice@example.com:pres2", "write")

	// Delete all entries for pres1
	count := cache.DeleteSuffix(":pres1")
	if count != 2 {
		t.Errorf("expected 2 deletions, got %d", count)
	}

	// pres1 entries should be gone
	if _, ok := cache.Get("alice@example.com:pres1"); ok {
		t.Error("expected alice:pres1 to be deleted")
	}

	// pres2 entry should still exist
	if _, ok := cache.Get("alice@example.com:pres2"); !ok {
		t.Error("expected alice:pres2 to exist")
	}
}

func TestLRUClear(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cache.Size())
	}
}

func TestLRUMetrics(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")

	// 2 hits
	cache.Get("key1")
	cache.Get("key1")

	// 1 miss
	cache.Get("key2")

	metrics := cache.Metrics()
	if metrics.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.Misses)
	}
	if metrics.HitRate() < 66.6 || metrics.HitRate() > 66.7 {
		t.Errorf("expected hit rate ~66.67%%, got %.2f%%", metrics.HitRate())
	}
}

func TestLRUResetMetrics(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")
	cache.Get("key1")
	cache.Get("key2")

	cache.ResetMetrics()

	metrics := cache.Metrics()
	if metrics.Hits != 0 || metrics.Misses != 0 {
		t.Error("expected metrics to be reset")
	}
}

func TestLRUCleanup(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 50 * time.Millisecond,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	count := cache.Cleanup()
	if count != 2 {
		t.Errorf("expected 2 expired entries, got %d", count)
	}

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after cleanup, got %d", cache.Size())
	}
}

func TestLRUKeys(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 10,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	keys := cache.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// Check all keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	if !keyMap["key1"] || !keyMap["key2"] || !keyMap["key3"] {
		t.Error("expected all keys to be present")
	}
}

func TestLRUConcurrency(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 100,
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := "key" + string(rune('0'+n)) + "_" + string(rune('0'+j%10))
				cache.Set(key, n*100+j)
				cache.Get(key)
			}
		}(i)
	}
	wg.Wait()

	// Just verify no panics occurred and cache is in valid state
	if cache.Size() > 100 {
		t.Errorf("cache size %d exceeds max entries 100", cache.Size())
	}
}

func TestLRUZeroMaxEntries(t *testing.T) {
	cache := NewLRU(LRUConfig{
		MaxEntries: 0, // Unlimited
		DefaultTTL: 5 * time.Minute,
		Logger:     testLogger(),
	})

	// Should be able to add many entries without eviction
	for i := 0; i < 1000; i++ {
		cache.Set("key"+string(rune('0'+i%10)), i)
	}

	// All unique keys should be stored (last 10 values only due to key collision)
	// Actually with the key generation above, we only have 10 unique keys
	if cache.Size() != 10 {
		t.Errorf("expected 10 unique keys, got %d", cache.Size())
	}
}

func TestMetricsHitRateZeroTotal(t *testing.T) {
	m := Metrics{}
	if m.HitRate() != 0 {
		t.Errorf("expected 0%% hit rate for zero total, got %.2f%%", m.HitRate())
	}
}

func TestEntryIsExpired(t *testing.T) {
	entry := &Entry{
		Key:       "test",
		Value:     "value",
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}

	if !entry.IsExpired() {
		t.Error("expected entry to be expired")
	}

	entry.ExpiresAt = time.Now().Add(1 * time.Hour)
	if entry.IsExpired() {
		t.Error("expected entry to not be expired")
	}
}
