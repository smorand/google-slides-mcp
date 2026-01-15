package permissions

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

// mockDriveService is a mock implementation of DriveService for testing.
type mockDriveService struct {
	permissions     []*drive.Permission
	permissionsErr  error
	file            *drive.File
	fileErr         error
}

func (m *mockDriveService) GetPermissions(ctx context.Context, fileID string) ([]*drive.Permission, error) {
	return m.permissions, m.permissionsErr
}

func (m *mockDriveService) GetFile(ctx context.Context, fileID string) (*drive.File, error) {
	return m.file, m.fileErr
}

// mockTokenSource is a mock implementation of oauth2.TokenSource.
type mockTokenSource struct{}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "test-access-token",
		TokenType:   "Bearer",
	}, nil
}

// newMockFactory creates a DriveServiceFactory that returns the given mock service.
func newMockFactory(mock DriveService) DriveServiceFactory {
	return func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error) {
		return mock, nil
	}
}

// newErrorFactory creates a DriveServiceFactory that returns an error.
func newErrorFactory(err error) DriveServiceFactory {
	return func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error) {
		return nil, err
	}
}

func TestPermissionLevel_String(t *testing.T) {
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
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("PermissionLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestRoleToPermissionLevel(t *testing.T) {
	tests := []struct {
		role     string
		expected PermissionLevel
	}{
		{"owner", PermissionWrite},
		{"organizer", PermissionWrite},
		{"fileOrganizer", PermissionWrite},
		{"writer", PermissionWrite},
		{"commenter", PermissionRead},
		{"reader", PermissionRead},
		{"unknown", PermissionNone},
		{"", PermissionNone},
	}

	for _, tt := range tests {
		if got := roleToPermissionLevel(tt.role); got != tt.expected {
			t.Errorf("roleToPermissionLevel(%q) = %v, want %v", tt.role, got, tt.expected)
		}
	}
}

func TestChecker_CheckWrite_Success(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckWrite(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("CheckWrite() error = %v, want nil", err)
	}
}

func TestChecker_CheckWrite_NoPermission(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: false,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckWrite(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if !errors.Is(err, ErrNoWritePermission) {
		t.Errorf("CheckWrite() error = %v, want %v", err, ErrNoWritePermission)
	}
}

func TestChecker_CheckRead_Success(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: false,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckRead(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("CheckRead() error = %v, want nil", err)
	}
}

func TestChecker_CheckRead_NoPermission_FileNotFound(t *testing.T) {
	mock := &mockDriveService{
		fileErr: errors.New("404 not found"),
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckRead(context.Background(), &mockTokenSource{}, "user@example.com", "nonexistent-id")
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("CheckRead() error = %v, want %v", err, ErrFileNotFound)
	}
}

func TestChecker_GetPermissionLevel_Owner(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	level, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "owner@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("GetPermissionLevel() error = %v, want nil", err)
	}
	if level != PermissionWrite {
		t.Errorf("GetPermissionLevel() = %v, want %v", level, PermissionWrite)
	}
}

func TestChecker_GetPermissionLevel_Reader(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: false,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	level, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "reader@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("GetPermissionLevel() error = %v, want nil", err)
	}
	if level != PermissionRead {
		t.Errorf("GetPermissionLevel() = %v, want %v", level, PermissionRead)
	}
}

func TestChecker_Caching(t *testing.T) {
	callCount := 0
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	factory := func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error) {
		callCount++
		return mock, nil
	}

	config := DefaultCheckerConfig()
	config.CacheTTL = 10 * time.Minute
	checker := NewChecker(config, factory)

	// First call - should hit Drive API
	_, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("First call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}

	// Second call - should use cache
	_, err = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("Second call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", callCount)
	}

	// Different user - should hit API again
	_, err = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "other@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("Different user call error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls, got %d", callCount)
	}

	// Same user, different file - should hit API again
	_, err = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "different-presentation")
	if err != nil {
		t.Errorf("Different file call error = %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 API calls, got %d", callCount)
	}

	// Verify cache size
	if checker.CacheSize() != 3 {
		t.Errorf("CacheSize() = %d, want 3", checker.CacheSize())
	}
}

func TestChecker_CacheExpiration(t *testing.T) {
	callCount := 0
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	factory := func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error) {
		callCount++
		return mock, nil
	}

	config := DefaultCheckerConfig()
	config.CacheTTL = 1 * time.Millisecond // Very short TTL for testing
	checker := NewChecker(config, factory)

	// First call
	_, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("First call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Second call - cache expired, should hit API again
	_, err = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("Second call error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls after cache expiration, got %d", callCount)
	}
}

func TestChecker_InvalidateCache(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	// Populate cache
	_, _ = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "presentation-1")
	_, _ = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "presentation-2")

	if checker.CacheSize() != 2 {
		t.Errorf("CacheSize() = %d, want 2", checker.CacheSize())
	}

	// Invalidate specific entry
	checker.InvalidateCache("user@example.com", "presentation-1")
	if checker.CacheSize() != 1 {
		t.Errorf("CacheSize() after invalidate = %d, want 1", checker.CacheSize())
	}
}

func TestChecker_ClearCache(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	// Populate cache
	_, _ = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user1@example.com", "presentation-1")
	_, _ = checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user2@example.com", "presentation-2")

	if checker.CacheSize() != 2 {
		t.Errorf("CacheSize() = %d, want 2", checker.CacheSize())
	}

	// Clear cache
	checker.ClearCache()
	if checker.CacheSize() != 0 {
		t.Errorf("CacheSize() after clear = %d, want 0", checker.CacheSize())
	}
}

func TestChecker_DriveServiceFactoryError(t *testing.T) {
	factoryErr := errors.New("failed to create service")
	checker := NewChecker(DefaultCheckerConfig(), newErrorFactory(factoryErr))

	_, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "test-id")
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !errors.Is(err, ErrPermissionCheck) {
		t.Errorf("Expected ErrPermissionCheck, got %v", err)
	}
}

func TestChecker_FileNotFound(t *testing.T) {
	mock := &mockDriveService{
		fileErr: errors.New("404 notFound: File not found"),
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckWrite(context.Background(), &mockTokenSource{}, "user@example.com", "nonexistent")
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("CheckWrite() error = %v, want %v", err, ErrFileNotFound)
	}
}

func TestChecker_ClearErrorMessage(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: false,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckWrite(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")

	// Verify error message is clear and user-friendly
	expectedMsg := "user does not have write permission on this presentation"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestChecker_NilCapabilities(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:           "test-presentation-id",
			Name:         "Test Presentation",
			Capabilities: nil, // No capabilities object
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	// Should return read permission when file is accessible but capabilities is nil
	level, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("GetPermissionLevel() error = %v, want nil", err)
	}
	if level != PermissionRead {
		t.Errorf("GetPermissionLevel() = %v, want %v", level, PermissionRead)
	}
}

func TestChecker_DefaultConfig(t *testing.T) {
	config := DefaultCheckerConfig()

	if config.CacheTTL != 5*time.Minute {
		t.Errorf("Default CacheTTL = %v, want 5m", config.CacheTTL)
	}
	if config.Logger == nil {
		t.Error("Default Logger is nil")
	}
}

func TestChecker_NewChecker_Defaults(t *testing.T) {
	checker := NewChecker(CheckerConfig{}, nil)

	if checker.config.CacheTTL != 5*time.Minute {
		t.Errorf("CacheTTL = %v, want 5m", checker.config.CacheTTL)
	}
	if checker.config.Logger == nil {
		t.Error("Logger is nil")
	}
	if checker.driveServiceFactory == nil {
		t.Error("driveServiceFactory is nil")
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"404 error", errors.New("404 not found"), true},
		{"notFound error", errors.New("notFound: File not found"), true},
		{"not found lowercase", errors.New("file not found"), true},
		{"permission denied", errors.New("403 forbidden"), false},
		{"generic error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFoundError(tt.err); got != tt.expected {
				t.Errorf("isNotFoundError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestReadOnlyOperations_WorkWithoutWritePermission(t *testing.T) {
	// Verify that read operations work when user only has read permission
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: false, // Read-only
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	// Read should succeed
	err := checker.CheckRead(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err != nil {
		t.Errorf("CheckRead() error = %v, want nil for read-only user", err)
	}

	// Write should fail with clear error
	err = checker.CheckWrite(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")
	if err == nil {
		t.Error("CheckWrite() should fail for read-only user")
	}
	if !errors.Is(err, ErrNoWritePermission) {
		t.Errorf("CheckWrite() error = %v, want ErrNoWritePermission", err)
	}
}

func TestWriteOperations_FailWithClearError(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: false,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	err := checker.CheckWrite(context.Background(), &mockTokenSource{}, "user@example.com", "test-presentation-id")

	// Verify error is clear and actionable
	if err == nil {
		t.Fatal("Expected error for write operation without permission")
	}

	// Error should clearly indicate it's a permission issue
	if !errors.Is(err, ErrNoWritePermission) {
		t.Errorf("Expected ErrNoWritePermission, got: %v", err)
	}

	// Error message should be user-friendly
	expectedMsg := "user does not have write permission on this presentation"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestChecker_ConcurrentAccess(t *testing.T) {
	mock := &mockDriveService{
		file: &drive.File{
			Id:   "test-presentation-id",
			Name: "Test Presentation",
			Capabilities: &drive.FileCapabilities{
				CanEdit: true,
			},
		},
	}

	checker := NewChecker(DefaultCheckerConfig(), newMockFactory(mock))

	// Run concurrent permission checks
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(userNum int) {
			email := "user" + string(rune('0'+userNum)) + "@example.com"
			_, err := checker.GetPermissionLevel(context.Background(), &mockTokenSource{}, email, "test-presentation-id")
			if err != nil {
				t.Errorf("Concurrent GetPermissionLevel error: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache integrity
	if checker.CacheSize() != 10 {
		t.Errorf("CacheSize() = %d, want 10", checker.CacheSize())
	}
}
