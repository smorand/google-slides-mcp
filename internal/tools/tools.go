// Package tools implements MCP tools for Google Slides operations.
package tools

import (
	"context"
	"io"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"
)

// SlidesService abstracts the Google Slides API for testing.
type SlidesService interface {
	GetPresentation(ctx context.Context, presentationID string) (*slides.Presentation, error)
	GetThumbnail(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error)
}

// SlidesServiceFactory creates a Slides service from a token source.
type SlidesServiceFactory func(ctx context.Context, tokenSource oauth2.TokenSource) (SlidesService, error)

// realSlidesService wraps the actual Google Slides API.
type realSlidesService struct {
	service *slides.Service
}

// GetPresentation retrieves a presentation by ID.
func (s *realSlidesService) GetPresentation(ctx context.Context, presentationID string) (*slides.Presentation, error) {
	return s.service.Presentations.Get(presentationID).Context(ctx).Do()
}

// GetThumbnail retrieves a thumbnail for a page.
func (s *realSlidesService) GetThumbnail(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
	return s.service.Presentations.Pages.GetThumbnail(presentationID, pageObjectID).
		ThumbnailPropertiesThumbnailSize("LARGE").
		Context(ctx).
		Do()
}

// NewRealSlidesServiceFactory returns a factory that creates real Slides services.
func NewRealSlidesServiceFactory() SlidesServiceFactory {
	return func(ctx context.Context, tokenSource oauth2.TokenSource) (SlidesService, error) {
		service, err := slides.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			return nil, err
		}
		return &realSlidesService{service: service}, nil
	}
}

// DriveService abstracts the Google Drive API for testing.
type DriveService interface {
	ListFiles(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error)
	CopyFile(ctx context.Context, fileID string, file *drive.File) (*drive.File, error)
	ExportFile(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error)
}

// DriveServiceFactory creates a Drive service from a token source.
type DriveServiceFactory func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error)

// realDriveService wraps the actual Google Drive API.
type realDriveService struct {
	service *drive.Service
}

// ListFiles lists files matching the query.
func (s *realDriveService) ListFiles(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
	call := s.service.Files.List().
		Q(query).
		PageSize(pageSize).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Context(ctx)

	if fields != "" {
		call = call.Fields(fields)
	}

	return call.Do()
}

// CopyFile copies a file to a new location with a new name.
func (s *realDriveService) CopyFile(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
	return s.service.Files.Copy(fileID, file).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
}

// ExportFile exports a Google Workspace file to the specified MIME type.
func (s *realDriveService) ExportFile(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
	resp, err := s.service.Files.Export(fileID, mimeType).
		Context(ctx).
		Download()
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// NewRealDriveServiceFactory returns a factory that creates real Drive services.
func NewRealDriveServiceFactory() DriveServiceFactory {
	return func(ctx context.Context, tokenSource oauth2.TokenSource) (DriveService, error) {
		service, err := drive.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			return nil, err
		}
		return &realDriveService{service: service}, nil
	}
}

// ToolsConfig holds configuration for the tools.
type ToolsConfig struct {
	Logger *slog.Logger
}

// DefaultToolsConfig returns default configuration.
func DefaultToolsConfig() ToolsConfig {
	return ToolsConfig{
		Logger: slog.Default(),
	}
}

// Tools provides MCP tool implementations.
type Tools struct {
	config               ToolsConfig
	slidesServiceFactory SlidesServiceFactory
	driveServiceFactory  DriveServiceFactory
}

// NewTools creates a new Tools instance.
// Deprecated: Use NewToolsWithDrive instead for full functionality.
func NewTools(config ToolsConfig, slidesFactory SlidesServiceFactory) *Tools {
	return NewToolsWithDrive(config, slidesFactory, nil)
}

// NewToolsWithDrive creates a new Tools instance with Drive service support.
func NewToolsWithDrive(config ToolsConfig, slidesFactory SlidesServiceFactory, driveFactory DriveServiceFactory) *Tools {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if slidesFactory == nil {
		slidesFactory = NewRealSlidesServiceFactory()
	}
	if driveFactory == nil {
		driveFactory = NewRealDriveServiceFactory()
	}

	return &Tools{
		config:               config,
		slidesServiceFactory: slidesFactory,
		driveServiceFactory:  driveFactory,
	}
}
