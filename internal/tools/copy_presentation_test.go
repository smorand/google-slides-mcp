package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

func TestCopyPresentation_Success(t *testing.T) {
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			// Verify source ID
			if fileID != "source-presentation-id" {
				t.Errorf("expected source ID 'source-presentation-id', got: %s", fileID)
			}
			// Verify new title
			if file.Name != "My Copied Presentation" {
				t.Errorf("expected title 'My Copied Presentation', got: %s", file.Name)
			}
			// Verify no parents set (root folder)
			if len(file.Parents) != 0 {
				t.Errorf("expected no parents, got: %v", file.Parents)
			}

			return &drive.File{
				Id:   "new-presentation-id",
				Name: "My Copied Presentation",
			}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "source-presentation-id",
		NewTitle: "My Copied Presentation",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.PresentationID != "new-presentation-id" {
		t.Errorf("expected presentation ID 'new-presentation-id', got '%s'", output.PresentationID)
	}
	if output.Title != "My Copied Presentation" {
		t.Errorf("expected title 'My Copied Presentation', got '%s'", output.Title)
	}
	if output.SourceID != "source-presentation-id" {
		t.Errorf("expected source ID 'source-presentation-id', got '%s'", output.SourceID)
	}
	expectedURL := "https://docs.google.com/presentation/d/new-presentation-id/edit"
	if output.URL != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, output.URL)
	}
}

func TestCopyPresentation_WithDestinationFolder(t *testing.T) {
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			// Verify destination folder is set
			if len(file.Parents) != 1 {
				t.Errorf("expected 1 parent, got: %d", len(file.Parents))
			}
			if file.Parents[0] != "destination-folder-id" {
				t.Errorf("expected parent 'destination-folder-id', got: %s", file.Parents[0])
			}

			return &drive.File{
				Id:   "new-presentation-id",
				Name: file.Name,
			}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID:            "source-presentation-id",
		NewTitle:            "Copied to Folder",
		DestinationFolderID: "destination-folder-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.PresentationID != "new-presentation-id" {
		t.Errorf("expected presentation ID 'new-presentation-id', got '%s'", output.PresentationID)
	}
}

func TestCopyPresentation_EmptySourceID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "",
		NewTitle: "Some Title",
	})

	if err == nil {
		t.Fatal("expected error for empty source ID")
	}
	if !errors.Is(err, ErrInvalidSourceID) {
		t.Errorf("expected ErrInvalidSourceID, got: %v", err)
	}
}

func TestCopyPresentation_EmptyTitle(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "some-source-id",
		NewTitle: "",
	})

	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if !errors.Is(err, ErrInvalidTitle) {
		t.Errorf("expected ErrInvalidTitle, got: %v", err)
	}
}

func TestCopyPresentation_SourceNotFound(t *testing.T) {
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			return nil, errors.New("googleapi: Error 404: File not found: xyz123")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "nonexistent-source",
		NewTitle: "Copy Title",
	})

	if err == nil {
		t.Fatal("expected error for source not found")
	}
	if !errors.Is(err, ErrSourceNotFound) {
		t.Errorf("expected ErrSourceNotFound, got: %v", err)
	}
}

func TestCopyPresentation_AccessDenied(t *testing.T) {
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			return nil, errors.New("googleapi: Error 403: The user does not have sufficient permissions")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "restricted-source",
		NewTitle: "Copy Title",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestCopyPresentation_InvalidDestinationFolder(t *testing.T) {
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			return nil, errors.New("googleapi: Error 404: invalid parent specified: invalid-folder-id")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID:            "source-id",
		NewTitle:            "Copy Title",
		DestinationFolderID: "invalid-folder-id",
	})

	if err == nil {
		t.Fatal("expected error for invalid destination folder")
	}
	if !errors.Is(err, ErrDestinationInvalid) {
		t.Errorf("expected ErrDestinationInvalid, got: %v", err)
	}
}

func TestCopyPresentation_DriveServiceFactoryError(t *testing.T) {
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return nil, errors.New("failed to create drive service")
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "source-id",
		NewTitle: "Copy Title",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrDriveAPIError) {
		t.Errorf("expected ErrDriveAPIError, got: %v", err)
	}
}

func TestCopyPresentation_GenericCopyError(t *testing.T) {
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			return nil, errors.New("googleapi: Error 500: Internal Server Error")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "source-id",
		NewTitle: "Copy Title",
	})

	if err == nil {
		t.Fatal("expected error for copy failure")
	}
	if !errors.Is(err, ErrCopyFailed) {
		t.Errorf("expected ErrCopyFailed, got: %v", err)
	}
}

func TestCopyPresentation_PreservesFormattingAndTheme(t *testing.T) {
	// This test verifies that the copy operation uses Drive API's copy functionality
	// which inherently preserves all formatting, themes, and masters.
	// The key verification is that we use the Drive API's Files.Copy method.

	copyCalled := false
	mockService := &mockDriveService{
		CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
			copyCalled = true
			// Drive API's Copy inherently preserves all content, formatting, themes
			return &drive.File{
				Id:       "copied-id",
				Name:     file.Name,
				MimeType: "application/vnd.google-apps.presentation",
			}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
		SourceID: "template-id",
		NewTitle: "New From Template",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !copyCalled {
		t.Error("expected Drive Copy API to be called")
	}
}

func TestCopyPresentation_URLFormat(t *testing.T) {
	testCases := []struct {
		name        string
		newID       string
		expectedURL string
	}{
		{
			name:        "standard ID",
			newID:       "abc123xyz",
			expectedURL: "https://docs.google.com/presentation/d/abc123xyz/edit",
		},
		{
			name:        "ID with dashes",
			newID:       "1abc-def-2ghi",
			expectedURL: "https://docs.google.com/presentation/d/1abc-def-2ghi/edit",
		},
		{
			name:        "ID with underscores",
			newID:       "abc_def_123",
			expectedURL: "https://docs.google.com/presentation/d/abc_def_123/edit",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService := &mockDriveService{
				CopyFileFunc: func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
					return &drive.File{
						Id:   tc.newID,
						Name: "Test",
					}, nil
				},
			}

			driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
				return mockService, nil
			}

			tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
			tokenSource := &mockTokenSource{}

			output, err := tools.CopyPresentation(context.Background(), tokenSource, CopyPresentationInput{
				SourceID: "source",
				NewTitle: "Test",
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.URL != tc.expectedURL {
				t.Errorf("expected URL '%s', got '%s'", tc.expectedURL, output.URL)
			}
		})
	}
}

func TestIsParentNotFoundError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "file not found error (not parent related)",
			err:      errors.New("googleapi: Error 404: File not found"),
			expected: false,
		},
		{
			name:     "invalid parent error",
			err:      errors.New("googleapi: invalid parent specified"),
			expected: true,
		},
		{
			name:     "parent not found error",
			err:      errors.New("parent not found: folder-xyz"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("googleapi: Error 500: Internal Server Error"),
			expected: false,
		},
		{
			name:     "forbidden error",
			err:      errors.New("googleapi: Error 403: forbidden"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isParentNotFoundError(tc.err)
			if result != tc.expected {
				t.Errorf("isParentNotFoundError(%v) = %v, expected %v", tc.err, result, tc.expected)
			}
		})
	}
}
