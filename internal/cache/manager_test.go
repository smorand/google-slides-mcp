package cache

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0 // Disable cleanup for testing

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.Presentations == nil {
		t.Error("expected Presentations cache to be initialized")
	}
	if manager.Tokens == nil {
		t.Error("expected Tokens cache to be initialized")
	}
	if manager.Permissions == nil {
		t.Error("expected Permissions cache to be initialized")
	}
}

func TestManagerCleanup(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0 // Disable automatic cleanup
	config.PresentationConfig.TTL = 50 * time.Millisecond
	config.TokenConfig.TTL = 50 * time.Millisecond
	config.PermissionConfig.TTL = 50 * time.Millisecond

	manager := NewManager(config)

	// Add entries to all caches
	manager.Presentations.Set(&PresentationInfo{ID: "pres1", Title: "Test"})
	manager.Tokens.Set(&CachedToken{APIKey: "key1", UserEmail: "user@example.com"})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user@example.com", PresentationID: "pres1", Level: PermissionRead})

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	total := manager.Cleanup()
	if total != 3 {
		t.Errorf("expected 3 expired entries cleaned up, got %d", total)
	}
}

func TestManagerInvalidatePresentation(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0

	manager := NewManager(config)

	// Add presentation and related permissions
	manager.Presentations.Set(&PresentationInfo{ID: "pres123", Title: "Test"})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user2@example.com", PresentationID: "pres123", Level: PermissionWrite})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres456", Level: PermissionRead})

	// Invalidate presentation
	manager.InvalidatePresentation("pres123")

	// Verify presentation is gone
	_, ok := manager.Presentations.Get("pres123")
	if ok {
		t.Error("expected presentation to be invalidated")
	}

	// Verify permissions for pres123 are gone
	_, ok = manager.Permissions.Get("user1@example.com", "pres123")
	if ok {
		t.Error("expected user1 permission for pres123 to be invalidated")
	}
	_, ok = manager.Permissions.Get("user2@example.com", "pres123")
	if ok {
		t.Error("expected user2 permission for pres123 to be invalidated")
	}

	// Verify permission for pres456 is still there
	_, ok = manager.Permissions.Get("user1@example.com", "pres456")
	if !ok {
		t.Error("expected user1 permission for pres456 to still exist")
	}
}

func TestManagerInvalidateUser(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0

	manager := NewManager(config)

	// Add permissions for different users
	manager.Permissions.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres123", Level: PermissionRead})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user1@example.com", PresentationID: "pres456", Level: PermissionWrite})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user2@example.com", PresentationID: "pres123", Level: PermissionRead})

	// Invalidate user
	manager.InvalidateUser("user1@example.com")

	// Verify user1 permissions are gone
	_, ok := manager.Permissions.Get("user1@example.com", "pres123")
	if ok {
		t.Error("expected user1 pres123 permission to be invalidated")
	}
	_, ok = manager.Permissions.Get("user1@example.com", "pres456")
	if ok {
		t.Error("expected user1 pres456 permission to be invalidated")
	}

	// Verify user2 permission is still there
	_, ok = manager.Permissions.Get("user2@example.com", "pres123")
	if !ok {
		t.Error("expected user2 pres123 permission to still exist")
	}
}

func TestManagerInvalidateAPIKey(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0

	manager := NewManager(config)

	// Add tokens
	manager.Tokens.Set(&CachedToken{APIKey: "api-key-123", UserEmail: "user1@example.com"})
	manager.Tokens.Set(&CachedToken{APIKey: "api-key-456", UserEmail: "user2@example.com"})

	// Invalidate specific API key
	manager.InvalidateAPIKey("api-key-123")

	// Verify api-key-123 is gone
	_, ok := manager.Tokens.Get("api-key-123")
	if ok {
		t.Error("expected api-key-123 to be invalidated")
	}

	// Verify api-key-456 is still there
	_, ok = manager.Tokens.Get("api-key-456")
	if !ok {
		t.Error("expected api-key-456 to still exist")
	}
}

func TestManagerClear(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0

	manager := NewManager(config)

	// Add entries to all caches
	manager.Presentations.Set(&PresentationInfo{ID: "pres1", Title: "Test"})
	manager.Tokens.Set(&CachedToken{APIKey: "key1", UserEmail: "user@example.com"})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user@example.com", PresentationID: "pres1", Level: PermissionRead})

	// Clear all
	manager.Clear()

	// Verify all caches are empty
	if manager.Presentations.Size() != 0 {
		t.Error("expected Presentations cache to be empty")
	}
	if manager.Tokens.Size() != 0 {
		t.Error("expected Tokens cache to be empty")
	}
	if manager.Permissions.Size() != 0 {
		t.Error("expected Permissions cache to be empty")
	}
}

func TestManagerStats(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0

	manager := NewManager(config)

	// Add entries
	manager.Presentations.Set(&PresentationInfo{ID: "pres1", Title: "Test"})
	manager.Tokens.Set(&CachedToken{APIKey: "key1", UserEmail: "user@example.com"})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user@example.com", PresentationID: "pres1", Level: PermissionRead})

	// Generate some hits and misses
	manager.Presentations.Get("pres1") // hit
	manager.Presentations.Get("pres2") // miss
	manager.Tokens.Get("key1")         // hit
	manager.Tokens.Get("key2")         // miss
	manager.Permissions.Get("user@example.com", "pres1") // hit
	manager.Permissions.Get("other@example.com", "pres1") // miss

	stats := manager.Stats()

	// Check sizes
	if stats.Presentations.Size != 1 {
		t.Errorf("expected Presentations size 1, got %d", stats.Presentations.Size)
	}
	if stats.Tokens.Size != 1 {
		t.Errorf("expected Tokens size 1, got %d", stats.Tokens.Size)
	}
	if stats.Permissions.Size != 1 {
		t.Errorf("expected Permissions size 1, got %d", stats.Permissions.Size)
	}

	// Check hits/misses
	if stats.Presentations.Metrics.Hits != 1 {
		t.Errorf("expected Presentations 1 hit, got %d", stats.Presentations.Metrics.Hits)
	}
	if stats.Presentations.Metrics.Misses != 1 {
		t.Errorf("expected Presentations 1 miss, got %d", stats.Presentations.Metrics.Misses)
	}
	if stats.Tokens.Metrics.Hits != 1 {
		t.Errorf("expected Tokens 1 hit, got %d", stats.Tokens.Metrics.Hits)
	}
	if stats.Tokens.Metrics.Misses != 1 {
		t.Errorf("expected Tokens 1 miss, got %d", stats.Tokens.Metrics.Misses)
	}
	if stats.Permissions.Metrics.Hits != 1 {
		t.Errorf("expected Permissions 1 hit, got %d", stats.Permissions.Metrics.Hits)
	}
	if stats.Permissions.Metrics.Misses != 1 {
		t.Errorf("expected Permissions 1 miss, got %d", stats.Permissions.Metrics.Misses)
	}
}

func TestManagerResetMetrics(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 0

	manager := NewManager(config)

	// Add entries and generate hits/misses
	manager.Presentations.Set(&PresentationInfo{ID: "pres1", Title: "Test"})
	manager.Presentations.Get("pres1") // hit
	manager.Presentations.Get("pres2") // miss

	// Reset metrics
	manager.ResetMetrics()

	// Verify metrics are reset
	stats := manager.Stats()
	if stats.Presentations.Metrics.Hits != 0 {
		t.Errorf("expected 0 hits after reset, got %d", stats.Presentations.Metrics.Hits)
	}
	if stats.Presentations.Metrics.Misses != 0 {
		t.Errorf("expected 0 misses after reset, got %d", stats.Presentations.Metrics.Misses)
	}
}

func TestManagerBackgroundCleanup(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 50 * time.Millisecond
	config.PresentationConfig.TTL = 25 * time.Millisecond
	config.TokenConfig.TTL = 25 * time.Millisecond
	config.PermissionConfig.TTL = 25 * time.Millisecond

	manager := NewManager(config)
	defer manager.Stop()

	// Add entries
	manager.Presentations.Set(&PresentationInfo{ID: "pres1", Title: "Test"})
	manager.Tokens.Set(&CachedToken{APIKey: "key1", UserEmail: "user@example.com"})
	manager.Permissions.Set(&CachedPermission{UserEmail: "user@example.com", PresentationID: "pres1", Level: PermissionRead})

	// Wait for entries to expire and cleanup to run
	time.Sleep(150 * time.Millisecond)

	// Verify entries are cleaned up
	if manager.Presentations.Size() != 0 {
		t.Errorf("expected Presentations cache to be empty after cleanup, got %d", manager.Presentations.Size())
	}
	if manager.Tokens.Size() != 0 {
		t.Errorf("expected Tokens cache to be empty after cleanup, got %d", manager.Tokens.Size())
	}
	if manager.Permissions.Size() != 0 {
		t.Errorf("expected Permissions cache to be empty after cleanup, got %d", manager.Permissions.Size())
	}
}

func TestManagerStop(t *testing.T) {
	config := DefaultManagerConfig()
	config.CleanupInterval = 10 * time.Millisecond

	manager := NewManager(config)

	// Stop should not block
	done := make(chan struct{})
	go func() {
		manager.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Stop() blocked for too long")
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	// Check presentation config
	if config.PresentationConfig.MaxEntries != 100 {
		t.Errorf("expected Presentation MaxEntries 100, got %d", config.PresentationConfig.MaxEntries)
	}
	if config.PresentationConfig.TTL != 5*time.Minute {
		t.Errorf("expected Presentation TTL 5 minutes, got %v", config.PresentationConfig.TTL)
	}

	// Check token config
	if config.TokenConfig.MaxEntries != 500 {
		t.Errorf("expected Token MaxEntries 500, got %d", config.TokenConfig.MaxEntries)
	}
	if config.TokenConfig.TTL != 55*time.Minute {
		t.Errorf("expected Token TTL 55 minutes, got %v", config.TokenConfig.TTL)
	}

	// Check permission config
	if config.PermissionConfig.MaxEntries != 1000 {
		t.Errorf("expected Permission MaxEntries 1000, got %d", config.PermissionConfig.MaxEntries)
	}
	if config.PermissionConfig.TTL != 5*time.Minute {
		t.Errorf("expected Permission TTL 5 minutes, got %v", config.PermissionConfig.TTL)
	}

	// Check cleanup interval
	if config.CleanupInterval != 1*time.Minute {
		t.Errorf("expected CleanupInterval 1 minute, got %v", config.CleanupInterval)
	}
}
