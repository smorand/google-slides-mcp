package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestCreatePresentation_Success(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			if presentation.Title != "New Presentation" {
				t.Errorf("expected title 'New Presentation', got '%s'", presentation.Title)
			}
			return &slides.Presentation{
				PresentationId: "new-presentation-id",
				Title:          presentation.Title,
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	output, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "New Presentation",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.PresentationID != "new-presentation-id" {
		t.Errorf("expected presentation ID 'new-presentation-id', got '%s'", output.PresentationID)
	}
	if output.Title != "New Presentation" {
		t.Errorf("expected title 'New Presentation', got '%s'", output.Title)
	}
	if output.URL != "https://docs.google.com/presentation/d/new-presentation-id/edit" {
		t.Errorf("expected URL 'https://docs.google.com/presentation/d/new-presentation-id/edit', got '%s'", output.URL)
	}
	if output.FolderID != "" {
		t.Errorf("expected empty folder ID, got '%s'", output.FolderID)
	}
}

func TestCreatePresentation_WithFolder(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "new-presentation-id",
				Title:          presentation.Title,
			}, nil
		},
	}

	moveFileCalled := false
	mockDriveService := &mockDriveService{
		MoveFileFunc: func(ctx context.Context, fileID string, folderID string) error {
			moveFileCalled = true
			if fileID != "new-presentation-id" {
				t.Errorf("expected file ID 'new-presentation-id', got '%s'", fileID)
			}
			if folderID != "destination-folder" {
				t.Errorf("expected folder ID 'destination-folder', got '%s'", folderID)
			}
			return nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDriveService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)

	output, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title:    "New Presentation",
		FolderID: "destination-folder",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !moveFileCalled {
		t.Error("expected MoveFile to be called")
	}
	if output.FolderID != "destination-folder" {
		t.Errorf("expected folder ID 'destination-folder', got '%s'", output.FolderID)
	}
}

func TestCreatePresentation_EmptyTitle(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "",
	})

	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if !errors.Is(err, ErrInvalidCreateTitle) {
		t.Errorf("expected ErrInvalidCreateTitle, got: %v", err)
	}
}

func TestCreatePresentation_SlidesAPIError(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return nil, errors.New("slides API error")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "New Presentation",
	})

	if err == nil {
		t.Fatal("expected error from slides API")
	}
	if !errors.Is(err, ErrCreateFailed) {
		t.Errorf("expected ErrCreateFailed, got: %v", err)
	}
}

func TestCreatePresentation_SlidesServiceFactoryError(t *testing.T) {
	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create slides service")
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "New Presentation",
	})

	if err == nil {
		t.Fatal("expected error from slides factory")
	}
	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got: %v", err)
	}
}

func TestCreatePresentation_AccessDenied(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return nil, errors.New("403 forbidden")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "New Presentation",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestCreatePresentation_DriveServiceFactoryError(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "new-presentation-id",
				Title:          presentation.Title,
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return nil, errors.New("failed to create drive service")
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title:    "New Presentation",
		FolderID: "some-folder",
	})

	if err == nil {
		t.Fatal("expected error from drive factory")
	}
	if !errors.Is(err, ErrDriveAPIError) {
		t.Errorf("expected ErrDriveAPIError, got: %v", err)
	}
}

func TestCreatePresentation_FolderNotFound(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "new-presentation-id",
				Title:          presentation.Title,
			}, nil
		},
	}

	mockDriveService := &mockDriveService{
		MoveFileFunc: func(ctx context.Context, fileID string, folderID string) error {
			return errors.New("404 not found")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDriveService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title:    "New Presentation",
		FolderID: "nonexistent-folder",
	})

	if err == nil {
		t.Fatal("expected error for folder not found")
	}
	if !errors.Is(err, ErrFolderNotFound) {
		t.Errorf("expected ErrFolderNotFound, got: %v", err)
	}
}

func TestCreatePresentation_FolderAccessDenied(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "new-presentation-id",
				Title:          presentation.Title,
			}, nil
		},
	}

	mockDriveService := &mockDriveService{
		MoveFileFunc: func(ctx context.Context, fileID string, folderID string) error {
			return errors.New("403 forbidden")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDriveService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)

	_, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title:    "New Presentation",
		FolderID: "restricted-folder",
	})

	if err == nil {
		t.Fatal("expected error for folder access denied")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestCreatePresentation_ReturnsURL(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "abc123xyz",
				Title:          presentation.Title,
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	output, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "My Presentation",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedURL := "https://docs.google.com/presentation/d/abc123xyz/edit"
	if output.URL != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, output.URL)
	}
}

func TestCreatePresentation_TitleIsCorrectlySet(t *testing.T) {
	var receivedTitle string
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			receivedTitle = presentation.Title
			return &slides.Presentation{
				PresentationId: "new-id",
				Title:          presentation.Title,
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	output, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "Q1 2024 Report",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedTitle != "Q1 2024 Report" {
		t.Errorf("expected title 'Q1 2024 Report' to be passed to API, got '%s'", receivedTitle)
	}
	if output.Title != "Q1 2024 Report" {
		t.Errorf("expected output title 'Q1 2024 Report', got '%s'", output.Title)
	}
}

func TestCreatePresentation_FolderPlacementWorks(t *testing.T) {
	var movedToFolder string
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "pres-id",
				Title:          presentation.Title,
			}, nil
		},
	}
	mockDriveService := &mockDriveService{
		MoveFileFunc: func(ctx context.Context, fileID string, folderID string) error {
			movedToFolder = folderID
			return nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDriveService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)

	output, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title:    "Test",
		FolderID: "my-folder-id",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if movedToFolder != "my-folder-id" {
		t.Errorf("expected file to be moved to 'my-folder-id', got '%s'", movedToFolder)
	}
	if output.FolderID != "my-folder-id" {
		t.Errorf("expected output folder ID 'my-folder-id', got '%s'", output.FolderID)
	}
}

func TestCreatePresentation_ReturnsPresentationID(t *testing.T) {
	mockSlidesService := &mockSlidesService{
		CreatePresentationFunc: func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "unique-pres-12345",
				Title:          presentation.Title,
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlidesService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)

	output, err := tools.CreatePresentation(context.Background(), &mockTokenSource{}, CreatePresentationInput{
		Title: "Test Presentation",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.PresentationID != "unique-pres-12345" {
		t.Errorf("expected presentation ID 'unique-pres-12345', got '%s'", output.PresentationID)
	}
}
