package permissions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
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

// Sentinel errors for permission checks.
var (
	ErrNoWritePermission = errors.New("user does not have write permission on this presentation")
	ErrNoReadPermission  = errors.New("user does not have read permission on this presentation")
	ErrPermissionCheck   = errors.New("failed to check permissions")
	ErrFileNotFound      = errors.New("presentation not found")
)

// CachedPermission holds a cached permission result with expiration.
type CachedPermission struct {
	Level    PermissionLevel
	CachedAt time.Time
}

// CheckerConfig holds configuration for the permission checker.
type CheckerConfig struct {
	CacheTTL time.Duration // Default 5 minutes
	Logger   *slog.Logger
}

// DefaultCheckerConfig returns default configuration.
func DefaultCheckerConfig() CheckerConfig {
	return CheckerConfig{
		CacheTTL: 5 * time.Minute,
		Logger:   slog.Default(),
	}
}

// DriveServiceFactory creates a Drive service from a token source.
// This allows for easy mocking in tests.
type DriveServiceFactory func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error)

// DriveService abstracts the Drive API for testing.
type DriveService interface {
	GetPermissions(ctx context.Context, fileID string) ([]*drive.Permission, error)
	GetFile(ctx context.Context, fileID string) (*drive.File, error)
}

// realDriveService wraps the actual Google Drive API.
type realDriveService struct {
	service *drive.Service
}

// GetPermissions retrieves all permissions for a file.
func (s *realDriveService) GetPermissions(ctx context.Context, fileID string) ([]*drive.Permission, error) {
	var allPermissions []*drive.Permission
	pageToken := ""

	for {
		call := s.service.Permissions.List(fileID).
			Fields("permissions(id,emailAddress,role,type),nextPageToken").
			Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Do()
		if err != nil {
			return nil, err
		}

		allPermissions = append(allPermissions, result.Permissions...)

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return allPermissions, nil
}

// GetFile retrieves file metadata.
func (s *realDriveService) GetFile(ctx context.Context, fileID string) (*drive.File, error) {
	return s.service.Files.Get(fileID).
		Fields("id,name,mimeType,capabilities").
		Context(ctx).
		Do()
}

// NewRealDriveServiceFactory returns a factory that creates real Drive services.
func NewRealDriveServiceFactory() DriveServiceFactory {
	return func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error) {
		service, err := drive.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			return nil, fmt.Errorf("failed to create drive service: %w", err)
		}
		return &realDriveService{service: service}, nil
	}
}

// Checker verifies user permissions on Google Slides presentations.
type Checker struct {
	config              CheckerConfig
	driveServiceFactory DriveServiceFactory
	cache               map[string]*CachedPermission
	mu                  sync.RWMutex
}

// NewChecker creates a new permission checker.
func NewChecker(config CheckerConfig, factory DriveServiceFactory) *Checker {
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if factory == nil {
		factory = NewRealDriveServiceFactory()
	}

	return &Checker{
		config:              config,
		driveServiceFactory: factory,
		cache:               make(map[string]*CachedPermission),
	}
}

// cacheKey generates a cache key for a user/file combination.
func cacheKey(userEmail, fileID string) string {
	return userEmail + ":" + fileID
}

// CheckRead verifies the user has at least read permission on the presentation.
func (c *Checker) CheckRead(ctx context.Context, tokenSource oauth2.TokenSource, userEmail, presentationID string) error {
	level, err := c.GetPermissionLevel(ctx, tokenSource, userEmail, presentationID)
	if err != nil {
		return err
	}

	if level < PermissionRead {
		return ErrNoReadPermission
	}

	return nil
}

// CheckWrite verifies the user has write permission on the presentation.
func (c *Checker) CheckWrite(ctx context.Context, tokenSource oauth2.TokenSource, userEmail, presentationID string) error {
	level, err := c.GetPermissionLevel(ctx, tokenSource, userEmail, presentationID)
	if err != nil {
		return err
	}

	if level < PermissionWrite {
		return ErrNoWritePermission
	}

	return nil
}

// GetPermissionLevel returns the user's permission level on a presentation.
func (c *Checker) GetPermissionLevel(ctx context.Context, tokenSource oauth2.TokenSource, userEmail, presentationID string) (PermissionLevel, error) {
	// Check cache first
	key := cacheKey(userEmail, presentationID)
	c.mu.RLock()
	cached, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Since(cached.CachedAt) < c.config.CacheTTL {
		c.config.Logger.Debug("permission cache hit",
			slog.String("user", userEmail),
			slog.String("presentation_id", presentationID),
			slog.String("level", cached.Level.String()),
		)
		return cached.Level, nil
	}

	c.config.Logger.Debug("permission cache miss, checking via Drive API",
		slog.String("user", userEmail),
		slog.String("presentation_id", presentationID),
	)

	// Create Drive service
	driveService, err := c.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return PermissionNone, fmt.Errorf("%w: %v", ErrPermissionCheck, err)
	}

	// Check permission using file capabilities (most reliable method)
	level, err := c.checkPermissionViaCapabilities(ctx, driveService, presentationID)
	if err != nil {
		return PermissionNone, err
	}

	// Cache the result
	c.mu.Lock()
	c.cache[key] = &CachedPermission{
		Level:    level,
		CachedAt: time.Now(),
	}
	c.mu.Unlock()

	c.config.Logger.Debug("permission check complete",
		slog.String("user", userEmail),
		slog.String("presentation_id", presentationID),
		slog.String("level", level.String()),
	)

	return level, nil
}

// checkPermissionViaCapabilities checks permissions using file capabilities.
// This is the most reliable method as it reflects the actual permissions.
func (c *Checker) checkPermissionViaCapabilities(ctx context.Context, driveService DriveService, fileID string) (PermissionLevel, error) {
	file, err := driveService.GetFile(ctx, fileID)
	if err != nil {
		// Check if it's a "not found" error
		if isNotFoundError(err) {
			return PermissionNone, ErrFileNotFound
		}
		return PermissionNone, fmt.Errorf("%w: %v", ErrPermissionCheck, err)
	}

	// Check capabilities to determine permission level
	if file.Capabilities != nil {
		if file.Capabilities.CanEdit {
			return PermissionWrite, nil
		}
		// CanRead or CanDownload or CanView indicates read access
		// In Drive API v3, if you can get the file, you have at least read access
		return PermissionRead, nil
	}

	// If we got the file without error, we have at least read access
	return PermissionRead, nil
}

// checkPermissionViaPermissionsList checks permissions using the permissions list.
// This is an alternative method that explicitly lists permissions.
func (c *Checker) checkPermissionViaPermissionsList(ctx context.Context, driveService DriveService, userEmail, fileID string) (PermissionLevel, error) {
	permissions, err := driveService.GetPermissions(ctx, fileID)
	if err != nil {
		// Check if it's a "not found" error
		if isNotFoundError(err) {
			return PermissionNone, ErrFileNotFound
		}
		return PermissionNone, fmt.Errorf("%w: %v", ErrPermissionCheck, err)
	}

	// Find the user's permission
	for _, perm := range permissions {
		if perm.EmailAddress == userEmail || perm.Type == "anyone" {
			return roleToPermissionLevel(perm.Role), nil
		}
	}

	// If we got here via API without error, user has at least read access
	// (they might have inherited permission from a shared drive or domain)
	return PermissionRead, nil
}

// roleToPermissionLevel converts a Drive API role to a PermissionLevel.
func roleToPermissionLevel(role string) PermissionLevel {
	switch role {
	case "owner", "organizer", "fileOrganizer", "writer":
		return PermissionWrite
	case "commenter", "reader":
		return PermissionRead
	default:
		return PermissionNone
	}
}

// isNotFoundError checks if an error indicates a file was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "404") || contains(errStr, "notFound") || contains(errStr, "not found")
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldBytes(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldBytes(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// InvalidateCache removes cached permissions for a user/presentation combination.
func (c *Checker) InvalidateCache(userEmail, presentationID string) {
	key := cacheKey(userEmail, presentationID)
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

// InvalidateCacheForFile removes all cached permissions for a presentation.
func (c *Checker) InvalidateCacheForFile(presentationID string) {
	c.mu.Lock()
	for key := range c.cache {
		// Key format is "email:presentationID"
		if len(key) > len(presentationID) && key[len(key)-len(presentationID):] == presentationID {
			delete(c.cache, key)
		}
	}
	c.mu.Unlock()
}

// ClearCache removes all cached permissions.
func (c *Checker) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string]*CachedPermission)
	c.mu.Unlock()
}

// CacheSize returns the number of cached permission entries.
func (c *Checker) CacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}
