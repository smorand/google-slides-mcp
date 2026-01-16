package cache

import (
	"log/slog"
	"time"
)

// ManagerConfig holds configuration for the cache manager.
type ManagerConfig struct {
	PresentationConfig PresentationCacheConfig
	TokenConfig        TokenCacheConfig
	PermissionConfig   PermissionCacheConfig
	CleanupInterval    time.Duration // How often to run cleanup (0 = disabled)
	Logger             *slog.Logger
}

// DefaultManagerConfig returns default configuration.
func DefaultManagerConfig() ManagerConfig {
	logger := slog.Default()
	return ManagerConfig{
		PresentationConfig: PresentationCacheConfig{
			MaxEntries: 100,
			TTL:        5 * time.Minute,
			Logger:     logger,
		},
		TokenConfig: TokenCacheConfig{
			MaxEntries: 500,
			TTL:        55 * time.Minute,
			Logger:     logger,
		},
		PermissionConfig: PermissionCacheConfig{
			MaxEntries: 1000,
			TTL:        5 * time.Minute,
			Logger:     logger,
		},
		CleanupInterval: 1 * time.Minute,
		Logger:          logger,
	}
}

// Manager coordinates all caches and handles invalidation.
type Manager struct {
	Presentations *PresentationCache
	Tokens        *TokenCache
	Permissions   *PermissionCache
	config        ManagerConfig
	stopCleanup   chan struct{}
}

// NewManager creates a new cache manager.
func NewManager(config ManagerConfig) *Manager {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	m := &Manager{
		Presentations: NewPresentationCache(config.PresentationConfig),
		Tokens:        NewTokenCache(config.TokenConfig),
		Permissions:   NewPermissionCache(config.PermissionConfig),
		config:        config,
		stopCleanup:   make(chan struct{}),
	}

	// Start background cleanup if interval is set
	if config.CleanupInterval > 0 {
		go m.cleanupLoop()
	}

	return m
}

// cleanupLoop runs periodic cleanup of expired entries.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-m.stopCleanup:
			return
		}
	}
}

// Stop stops the background cleanup goroutine.
func (m *Manager) Stop() {
	close(m.stopCleanup)
}

// Cleanup removes expired entries from all caches.
func (m *Manager) Cleanup() int {
	total := 0
	total += m.Presentations.Cleanup()
	total += m.Tokens.Cleanup()
	total += m.Permissions.Cleanup()

	if total > 0 {
		m.config.Logger.Debug("cache cleanup completed",
			slog.Int("expired_entries", total),
		)
	}

	return total
}

// InvalidatePresentation invalidates all cached data for a presentation.
// This should be called after any write operation on a presentation.
func (m *Manager) InvalidatePresentation(presentationID string) {
	m.Presentations.Invalidate(presentationID)
	m.Permissions.InvalidateByPresentation(presentationID)

	m.config.Logger.Debug("invalidated cache for presentation",
		slog.String("presentation_id", presentationID),
	)
}

// InvalidateUser invalidates all cached data for a user.
// This should be called when user authentication changes.
func (m *Manager) InvalidateUser(userEmail string) {
	m.Permissions.InvalidateByUser(userEmail)

	m.config.Logger.Debug("invalidated cache for user",
		slog.String("user_email", userEmail),
	)
}

// InvalidateAPIKey invalidates cached token for an API key.
func (m *Manager) InvalidateAPIKey(apiKey string) {
	m.Tokens.Invalidate(apiKey)

	m.config.Logger.Debug("invalidated token cache for API key",
		slog.String("api_key", apiKey[:8]+"..."),
	)
}

// Clear removes all entries from all caches.
func (m *Manager) Clear() {
	m.Presentations.Clear()
	m.Tokens.Clear()
	m.Permissions.Clear()

	m.config.Logger.Debug("cleared all caches")
}

// Stats returns statistics for all caches.
type Stats struct {
	Presentations CacheStats
	Tokens        CacheStats
	Permissions   CacheStats
}

// CacheStats holds statistics for a single cache.
type CacheStats struct {
	Size    int
	Metrics Metrics
}

// Stats returns statistics for all caches.
func (m *Manager) Stats() Stats {
	return Stats{
		Presentations: CacheStats{
			Size:    m.Presentations.Size(),
			Metrics: m.Presentations.Metrics(),
		},
		Tokens: CacheStats{
			Size:    m.Tokens.Size(),
			Metrics: m.Tokens.Metrics(),
		},
		Permissions: CacheStats{
			Size:    m.Permissions.Size(),
			Metrics: m.Permissions.Metrics(),
		},
	}
}

// LogStats logs cache statistics.
func (m *Manager) LogStats() {
	stats := m.Stats()

	m.config.Logger.Info("cache statistics",
		slog.Group("presentations",
			slog.Int("size", stats.Presentations.Size),
			slog.Int64("hits", stats.Presentations.Metrics.Hits),
			slog.Int64("misses", stats.Presentations.Metrics.Misses),
			slog.Float64("hit_rate_pct", stats.Presentations.Metrics.HitRate()),
			slog.Int64("evictions", stats.Presentations.Metrics.Evictions),
		),
		slog.Group("tokens",
			slog.Int("size", stats.Tokens.Size),
			slog.Int64("hits", stats.Tokens.Metrics.Hits),
			slog.Int64("misses", stats.Tokens.Metrics.Misses),
			slog.Float64("hit_rate_pct", stats.Tokens.Metrics.HitRate()),
			slog.Int64("evictions", stats.Tokens.Metrics.Evictions),
		),
		slog.Group("permissions",
			slog.Int("size", stats.Permissions.Size),
			slog.Int64("hits", stats.Permissions.Metrics.Hits),
			slog.Int64("misses", stats.Permissions.Metrics.Misses),
			slog.Float64("hit_rate_pct", stats.Permissions.Metrics.HitRate()),
			slog.Int64("evictions", stats.Permissions.Metrics.Evictions),
		),
	)
}

// ResetMetrics resets metrics for all caches.
func (m *Manager) ResetMetrics() {
	m.Presentations.lru.ResetMetrics()
	m.Tokens.lru.ResetMetrics()
	m.Permissions.lru.ResetMetrics()
}
