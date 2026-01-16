package cache

import (
	"log/slog"
	"time"
)

// PresentationInfo holds cached presentation structure information.
type PresentationInfo struct {
	ID          string
	Title       string
	SlideCount  int
	SlideIDs    []string
	ObjectIDs   map[string]string // objectID -> slideID mapping
	UpdatedAt   time.Time
}

// PresentationCacheConfig holds configuration for the presentation cache.
type PresentationCacheConfig struct {
	MaxEntries int           // Maximum number of presentations to cache
	TTL        time.Duration // TTL for presentation entries
	Logger     *slog.Logger
}

// DefaultPresentationCacheConfig returns default configuration.
func DefaultPresentationCacheConfig() PresentationCacheConfig {
	return PresentationCacheConfig{
		MaxEntries: 100,
		TTL:        5 * time.Minute,
		Logger:     slog.Default(),
	}
}

// PresentationCache caches presentation structure for faster access.
type PresentationCache struct {
	lru    *LRU
	config PresentationCacheConfig
}

// NewPresentationCache creates a new presentation cache.
func NewPresentationCache(config PresentationCacheConfig) *PresentationCache {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.TTL == 0 {
		config.TTL = 5 * time.Minute
	}
	if config.MaxEntries == 0 {
		config.MaxEntries = 100
	}

	return &PresentationCache{
		lru: NewLRU(LRUConfig{
			MaxEntries: config.MaxEntries,
			DefaultTTL: config.TTL,
			Logger:     config.Logger,
		}),
		config: config,
	}
}

// Get retrieves a presentation from the cache.
func (c *PresentationCache) Get(presentationID string) (*PresentationInfo, bool) {
	val, ok := c.lru.Get(presentationID)
	if !ok {
		return nil, false
	}
	return val.(*PresentationInfo), true
}

// Set stores a presentation in the cache.
func (c *PresentationCache) Set(info *PresentationInfo) {
	c.lru.SetWithTTL(info.ID, info, c.config.TTL)
}

// SetWithTTL stores a presentation in the cache with a specific TTL.
func (c *PresentationCache) SetWithTTL(info *PresentationInfo, ttl time.Duration) {
	c.lru.SetWithTTL(info.ID, info, ttl)
}

// Invalidate removes a presentation from the cache.
func (c *PresentationCache) Invalidate(presentationID string) {
	c.lru.Delete(presentationID)
}

// Clear removes all presentations from the cache.
func (c *PresentationCache) Clear() {
	c.lru.Clear()
}

// Size returns the number of cached presentations.
func (c *PresentationCache) Size() int {
	return c.lru.Size()
}

// Metrics returns cache metrics.
func (c *PresentationCache) Metrics() Metrics {
	return c.lru.Metrics()
}

// Cleanup removes expired entries.
func (c *PresentationCache) Cleanup() int {
	return c.lru.Cleanup()
}
