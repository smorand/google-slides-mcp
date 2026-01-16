package cache

import (
	"container/list"
	"log/slog"
	"sync"
	"time"
)

// Entry represents a cached entry with expiration.
type Entry struct {
	Key       string
	Value     any
	ExpiresAt time.Time
}

// IsExpired returns true if the entry has expired.
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Metrics tracks cache statistics.
type Metrics struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Expirations int64
}

// Clone returns a copy of the metrics.
func (m *Metrics) Clone() Metrics {
	return Metrics{
		Hits:        m.Hits,
		Misses:      m.Misses,
		Evictions:   m.Evictions,
		Expirations: m.Expirations,
	}
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (m *Metrics) HitRate() float64 {
	total := m.Hits + m.Misses
	if total == 0 {
		return 0
	}
	return float64(m.Hits) / float64(total) * 100
}

// LRUConfig holds configuration for the LRU cache.
type LRUConfig struct {
	MaxEntries int           // Maximum number of entries (0 = unlimited)
	DefaultTTL time.Duration // Default TTL for entries without explicit expiration
	Logger     *slog.Logger
}

// DefaultLRUConfig returns default configuration.
func DefaultLRUConfig() LRUConfig {
	return LRUConfig{
		MaxEntries: 1000,
		DefaultTTL: 5 * time.Minute,
		Logger:     slog.Default(),
	}
}

// LRU implements a thread-safe LRU cache with TTL support.
type LRU struct {
	config  LRUConfig
	cache   map[string]*list.Element
	lruList *list.List
	mu      sync.RWMutex
	metrics Metrics
}

// NewLRU creates a new LRU cache.
func NewLRU(config LRUConfig) *LRU {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 5 * time.Minute
	}

	return &LRU{
		config:  config,
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, nil and false otherwise.
func (c *LRU) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[key]
	if !ok {
		c.metrics.Misses++
		c.config.Logger.Debug("cache miss",
			slog.String("key", key),
		)
		return nil, false
	}

	entry := elem.Value.(*Entry)
	if entry.IsExpired() {
		c.removeElementLocked(elem)
		c.metrics.Misses++
		c.metrics.Expirations++
		c.config.Logger.Debug("cache miss (expired)",
			slog.String("key", key),
		)
		return nil, false
	}

	// Move to front (most recently used)
	c.lruList.MoveToFront(elem)
	c.metrics.Hits++
	c.config.Logger.Debug("cache hit",
		slog.String("key", key),
	)

	return entry.Value, true
}

// Set stores a value in the cache with the default TTL.
func (c *LRU) Set(key string, value any) {
	c.SetWithTTL(key, value, c.config.DefaultTTL)
}

// SetWithTTL stores a value in the cache with a specific TTL.
func (c *LRU) SetWithTTL(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.cache[key]; ok {
		entry := elem.Value.(*Entry)
		entry.Value = value
		entry.ExpiresAt = time.Now().Add(ttl)
		c.lruList.MoveToFront(elem)
		c.config.Logger.Debug("cache update",
			slog.String("key", key),
			slog.Duration("ttl", ttl),
		)
		return
	}

	// Check if we need to evict
	if c.config.MaxEntries > 0 && c.lruList.Len() >= c.config.MaxEntries {
		c.evictOldestLocked()
	}

	// Add new entry
	entry := &Entry{
		Key:       key,
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	elem := c.lruList.PushFront(entry)
	c.cache[key] = elem

	c.config.Logger.Debug("cache set",
		slog.String("key", key),
		slog.Duration("ttl", ttl),
	)
}

// Delete removes a key from the cache.
func (c *LRU) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[key]
	if !ok {
		return false
	}

	c.removeElementLocked(elem)
	c.config.Logger.Debug("cache delete",
		slog.String("key", key),
	)
	return true
}

// DeletePrefix removes all keys with the given prefix.
func (c *LRU) DeletePrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for key, elem := range c.cache {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.removeElementLocked(elem)
			count++
		}
	}

	if count > 0 {
		c.config.Logger.Debug("cache delete by prefix",
			slog.String("prefix", prefix),
			slog.Int("count", count),
		)
	}

	return count
}

// DeleteSuffix removes all keys with the given suffix.
func (c *LRU) DeleteSuffix(suffix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for key, elem := range c.cache {
		if len(key) >= len(suffix) && key[len(key)-len(suffix):] == suffix {
			c.removeElementLocked(elem)
			count++
		}
	}

	if count > 0 {
		c.config.Logger.Debug("cache delete by suffix",
			slog.String("suffix", suffix),
			slog.Int("count", count),
		)
	}

	return count
}

// Clear removes all entries from the cache.
func (c *LRU) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*list.Element)
	c.lruList = list.New()

	c.config.Logger.Debug("cache cleared")
}

// Size returns the number of entries in the cache.
func (c *LRU) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Metrics returns a copy of the current cache metrics.
func (c *LRU) Metrics() Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics.Clone()
}

// ResetMetrics resets all metrics to zero.
func (c *LRU) ResetMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = Metrics{}
}

// Cleanup removes all expired entries.
// This can be called periodically to proactively clean up expired entries.
func (c *LRU) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for key, elem := range c.cache {
		entry := elem.Value.(*Entry)
		if entry.IsExpired() {
			c.removeElementLocked(elem)
			c.metrics.Expirations++
			count++
			c.config.Logger.Debug("cache cleanup expired entry",
				slog.String("key", key),
			)
		}
	}

	return count
}

// Keys returns all non-expired keys in the cache.
func (c *LRU) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.cache))
	for key, elem := range c.cache {
		entry := elem.Value.(*Entry)
		if !entry.IsExpired() {
			keys = append(keys, key)
		}
	}
	return keys
}

// evictOldestLocked removes the least recently used entry.
// Must be called with lock held.
func (c *LRU) evictOldestLocked() {
	elem := c.lruList.Back()
	if elem == nil {
		return
	}

	c.removeElementLocked(elem)
	c.metrics.Evictions++

	entry := elem.Value.(*Entry)
	c.config.Logger.Debug("cache eviction (LRU)",
		slog.String("key", entry.Key),
	)
}

// removeElementLocked removes an element from the cache.
// Must be called with lock held.
func (c *LRU) removeElementLocked(elem *list.Element) {
	entry := elem.Value.(*Entry)
	delete(c.cache, entry.Key)
	c.lruList.Remove(elem)
}
