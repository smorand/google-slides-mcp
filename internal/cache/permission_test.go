package cache

import (
	"testing"
	"time"
)

func TestNewPermissionCache(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
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

func TestPermissionCacheSetAndGet(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	perm := &CachedPermission{
		UserEmail:      "user@example.com",
		PresentationID: "pres123",
		Level:          PermissionWrite,
		CachedAt:       time.Now(),
	}

	cache.Set(perm)

	// Get the permission
	retrieved, ok := cache.Get("user@example.com", "pres123")
	if !ok {
		t.Fatal("expected permission to be found")
	}
	if retrieved.Level != PermissionWrite {
		t.Errorf("expected level PermissionWrite, got %v", retrieved.Level)
	}

	// Get non-existent permission
	_, ok = cache.Get("other@example.com", "pres123")
	if ok {
		t.Error("expected permission to not be found")
	}
}

func TestPermissionCacheExpiration(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
		Logger:     testLogger(),
	})

	perm := &CachedPermission{
		UserEmail:      "user@example.com",
		PresentationID: "pres123",
		Level:          PermissionRead,
	}

	cache.Set(perm)

	// Should be found immediately
	_, ok := cache.Get("user@example.com", "pres123")
	if !ok {
		t.Fatal("expected permission to be found immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("user@example.com", "pres123")
	if ok {
		t.Error("expected permission to be expired")
	}
}

func TestPermissionCacheSetWithTTL(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	perm := &CachedPermission{
		UserEmail:      "user@example.com",
		PresentationID: "pres123",
		Level:          PermissionRead,
	}

	cache.SetWithTTL(perm, 50*time.Millisecond)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok := cache.Get("user@example.com", "pres123")
	if ok {
		t.Error("expected permission to be expired")
	}
}

func TestPermissionCacheInvalidate(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	perm := &CachedPermission{
		UserEmail:      "user@example.com",
		PresentationID: "pres123",
		Level:          PermissionRead,
	}

	cache.Set(perm)
	cache.Invalidate("user@example.com", "pres123")

	_, ok := cache.Get("user@example.com", "pres123")
	if ok {
		t.Error("expected permission to be invalidated")
	}
}

func TestPermissionCacheInvalidateByPresentation(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	// Add permissions for different users and presentations
	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})
	cache.Set(&CachedPermission{UserEmail: "user2@example.com", PresentationID: "pres123", Level: PermissionWrite})
	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres456", Level: PermissionRead})

	// Invalidate all permissions for pres123
	count := cache.InvalidateByPresentation("pres123")
	if count != 2 {
		t.Errorf("expected 2 permissions invalidated, got %d", count)
	}

	// Verify pres123 permissions are gone
	_, ok := cache.Get("user1@example.com", "pres123")
	if ok {
		t.Error("expected user1 pres123 permission to be invalidated")
	}
	_, ok = cache.Get("user2@example.com", "pres123")
	if ok {
		t.Error("expected user2 pres123 permission to be invalidated")
	}

	// Verify pres456 permission is still there
	_, ok = cache.Get("user1@example.com", "pres456")
	if !ok {
		t.Error("expected user1 pres456 permission to still exist")
	}
}

func TestPermissionCacheInvalidateByUser(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	// Add permissions for different users and presentations
	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})
	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres456", Level: PermissionWrite})
	cache.Set(&CachedPermission{UserEmail: "user2@example.com", PresentationID: "pres123", Level: PermissionRead})

	// Invalidate all permissions for user1
	count := cache.InvalidateByUser("user1@example.com")
	if count != 2 {
		t.Errorf("expected 2 permissions invalidated, got %d", count)
	}

	// Verify user1 permissions are gone
	_, ok := cache.Get("user1@example.com", "pres123")
	if ok {
		t.Error("expected user1 pres123 permission to be invalidated")
	}
	_, ok = cache.Get("user1@example.com", "pres456")
	if ok {
		t.Error("expected user1 pres456 permission to be invalidated")
	}

	// Verify user2 permission is still there
	_, ok = cache.Get("user2@example.com", "pres123")
	if !ok {
		t.Error("expected user2 pres123 permission to still exist")
	}
}

func TestPermissionCacheClear(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})
	cache.Set(&CachedPermission{UserEmail: "user2@example.com", PresentationID: "pres456", Level: PermissionWrite})

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestPermissionCacheMetrics(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        5 * time.Minute,
		Logger:     testLogger(),
	})

	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})

	// 1 hit
	cache.Get("user1@example.com", "pres123")

	// 1 miss
	cache.Get("user2@example.com", "pres123")

	metrics := cache.Metrics()
	if metrics.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.Misses)
	}
}

func TestPermissionCacheCleanup(t *testing.T) {
	cache := NewPermissionCache(PermissionCacheConfig{
		MaxEntries: 10,
		TTL:        50 * time.Millisecond,
		Logger:     testLogger(),
	})

	cache.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})
	cache.Set(&CachedPermission{UserEmail: "user2@example.com", PresentationID: "pres456", Level: PermissionWrite})

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

func TestDefaultPermissionCacheConfig(t *testing.T) {
	config := DefaultPermissionCacheConfig()

	if config.MaxEntries != 1000 {
		t.Errorf("expected max entries 1000, got %d", config.MaxEntries)
	}
	if config.TTL != 5*time.Minute {
		t.Errorf("expected TTL 5 minutes, got %v", config.TTL)
	}
}

func TestPermissionLevelString(t *testing.T) {
	tests := []struct {
		level    PermissionLevel
		expected string
	}{
		{PermissionNone, "none"},
		{PermissionRead, "read"},
		{PermissionWrite, "write"},
		{PermissionLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("PermissionLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPermissionKey(t *testing.T) {
	tests := []struct {
		userEmail      string
		presentationID string
		expected       string
	}{
		{"user@example.com", "pres123", "user@example.com:pres123"},
		{"test@test.com", "abc", "test@test.com:abc"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := permissionKey(tt.userEmail, tt.presentationID); got != tt.expected {
				t.Errorf("permissionKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}
