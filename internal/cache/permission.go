package cache

import (
	"fmt"
	"log/slog"
	"time"
)

// PermissionLevel represents the user's access level.
type PermissionLevel int

const (
	// PermissionNone means no access.
	PermissionNone PermissionLevel = iota
	// PermissionRead means read-only access (commenter or viewer).
	PermissionRead
	// PermissionWrite means write access (writer or owner).
	PermissionWrite
)

// String returns a human-readable string for the permission level.
func (p PermissionLevel) String() string {
	switch p {
	case PermissionNone:
		return "none"
	case PermissionRead:
		return "read"
	case PermissionWrite:
		return "write"
	default:
		return "unknown"
	}
}

// CachedPermission holds a cached permission result.
type CachedPermission struct {
	UserEmail      string
	PresentationID string
	Level          PermissionLevel
	CachedAt       time.Time
}

// PermissionCacheConfig holds configuration for the permission cache.
type PermissionCacheConfig struct {
	MaxEntries int           // Maximum number of permissions to cache
	TTL        time.Duration // TTL for permission entries
	Logger     *slog.Logger
}

// DefaultPermissionCacheConfig returns default configuration.
func DefaultPermissionCacheConfig() PermissionCacheConfig {
	return PermissionCacheConfig{
		MaxEntries: 1000,
		TTL:        5 * time.Minute,
		Logger:     slog.Default(),
	}
}

// PermissionCache caches permission checks for faster access.
type PermissionCache struct {
	lru    *LRU
	config PermissionCacheConfig
}

// NewPermissionCache creates a new permission cache.
func NewPermissionCache(config PermissionCacheConfig) *PermissionCache {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.TTL == 0 {
		config.TTL = 5 * time.Minute
	}
	if config.MaxEntries == 0 {
		config.MaxEntries = 1000
	}

	return &PermissionCache{
		lru: NewLRU(LRUConfig{
			MaxEntries: config.MaxEntries,
			DefaultTTL: config.TTL,
			Logger:     config.Logger,
		}),
		config: config,
	}
}

// permissionKey generates a cache key for a user/presentation combination.
func permissionKey(userEmail, presentationID string) string {
	return fmt.Sprintf("%s:%s", userEmail, presentationID)
}

// Get retrieves a permission from the cache.
func (c *PermissionCache) Get(userEmail, presentationID string) (*CachedPermission, bool) {
	key := permissionKey(userEmail, presentationID)
	val, ok := c.lru.Get(key)
	if !ok {
		return nil, false
	}
	return val.(*CachedPermission), true
}

// Set stores a permission in the cache.
func (c *PermissionCache) Set(perm *CachedPermission) {
	key := permissionKey(perm.UserEmail, perm.PresentationID)
	c.lru.SetWithTTL(key, perm, c.config.TTL)
}

// SetWithTTL stores a permission in the cache with a specific TTL.
func (c *PermissionCache) SetWithTTL(perm *CachedPermission, ttl time.Duration) {
	key := permissionKey(perm.UserEmail, perm.PresentationID)
	c.lru.SetWithTTL(key, perm, ttl)
}

// Invalidate removes a specific permission from the cache.
func (c *PermissionCache) Invalidate(userEmail, presentationID string) {
	key := permissionKey(userEmail, presentationID)
	c.lru.Delete(key)
}

// InvalidateByPresentation removes all permissions for a specific presentation.
func (c *PermissionCache) InvalidateByPresentation(presentationID string) int {
	return c.lru.DeleteSuffix(":" + presentationID)
}

// InvalidateByUser removes all permissions for a specific user.
func (c *PermissionCache) InvalidateByUser(userEmail string) int {
	return c.lru.DeletePrefix(userEmail + ":")
}

// Clear removes all permissions from the cache.
func (c *PermissionCache) Clear() {
	c.lru.Clear()
}

// Size returns the number of cached permissions.
func (c *PermissionCache) Size() int {
	return c.lru.Size()
}

// Metrics returns cache metrics.
func (c *PermissionCache) Metrics() Metrics {
	return c.lru.Metrics()
}

// Cleanup removes expired entries.
func (c *PermissionCache) Cleanup() int {
	return c.lru.Cleanup()
}
