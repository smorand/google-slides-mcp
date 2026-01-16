package cache

import (
	"testing"
	"time"
)

func TestNewPresentationCache(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	if cache == nil {
		t.Fatal("expected cache to be created")
	}
	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestPresentationCacheSetAndGet(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	info := &PresentationInfo{
		ID:         "pres123",
		Title:      "Test Presentation",
		SlideCount: 5,
		SlideIDs:   []string{"slide1", "slide2", "slide3", "slide4", "slide5"},
		ObjectIDs: map[string]string{
			"obj1": "slide1",
			"obj2": "slide2",
		},
		UpdatedAt: time.Now(),
	}

	cache.Set(info)

	// Get the presentation
	retrieved, ok := cache.Get("pres123")
	if !ok {
		t.Fatal("expected presentation to be found")
	}
	if retrieved.Title != "Test Presentation" {
		t.Errorf("expected title 'Test Presentation', got '%s'", retrieved.Title)
	}
	if retrieved.SlideCount != 5 {
		t.Errorf("expected 5 slides, got %d", retrieved.SlideCount)
	}

	// Get non-existent presentation
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Error("expected presentation to not be found")
	}
}

func TestPresentationCacheExpiration(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
		Logger:     testLogger(),
	})

	info := &PresentationInfo{
		ID:    "pres123",
		Title: "Test Presentation",
	}

	cache.Set(info)

	// Should be found immediately
	_, ok := cache.Get("pres123")
	if !ok {
		t.Fatal("expected presentation to be found immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("pres123")
	if ok {
		t.Error("expected presentation to be expired")
	}
}

func TestPresentationCacheSetWithTTL(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	info := &PresentationInfo{
		ID:    "pres123",
		Title: "Test Presentation",
	}

	cache.SetWithTTL(info, 50*time.Millisecond)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok := cache.Get("pres123")
	if ok {
		t.Error("expected presentation to be expired")
	}
}

func TestPresentationCacheInvalidate(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	info := &PresentationInfo{
		ID:    "pres123",
		Title: "Test Presentation",
	}

	cache.Set(info)
	cache.Invalidate("pres123")

	_, ok := cache.Get("pres123")
	if ok {
		t.Error("expected presentation to be invalidated")
	}
}

func TestPresentationCacheClear(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set(&PresentationInfo{ID: "pres1", Title: "Test 1"})
	cache.Set(&PresentationInfo{ID: "pres2", Title: "Test 2"})

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestPresentationCacheMetrics(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set(&PresentationInfo{ID: "pres1", Title: "Test 1"})

	// 1 hit
	cache.Get("pres1")

	// 1 miss
	cache.Get("pres2")

	metrics := cache.Metrics()
	if metrics.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.Misses)
	}
}

func TestPresentationCacheCleanup(t *testing.T) {
	cache := NewPresentationCache(PresentationCacheConfig{
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
		Logger:     testLogger(),
	})

	cache.Set(&PresentationInfo{ID: "pres1", Title: "Test 1"})
	cache.Set(&PresentationInfo{ID: "pres2", Title: "Test 2"})

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	count := cache.Cleanup()
	if count != 2 {
		t.Errorf("expected 2 expired entries, got %d", count)
	}

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestDefaultPresentationCacheConfig(t *testing.T) {
	config := DefaultPresentationCacheConfig()

	if config.MaxEntries != 100 {
		t.Errorf("expected max entries 100, got %d", config.MaxEntries)
	}
	if config.TTL != 5*time.Minute {
		t.Errorf("expected TTL 5 minutes, got %v", config.TTL)
	}
}
