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
	CreatePresentation(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error)
	BatchUpdate(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error)
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

// CreatePresentation creates a new presentation.
func (s *realSlidesService) CreatePresentation(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
	return s.service.Presentations.Create(presentation).Context(ctx).Do()
}

// BatchUpdate executes batch update requests on a presentation.
func (s *realSlidesService) BatchUpdate(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}
	return s.service.Presentations.BatchUpdate(presentationID, req).Context(ctx).Do()
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
	MoveFile(ctx context.Context, fileID string, folderID string) error
	UploadFile(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error)
	MakeFilePublic(ctx context.Context, fileID string) error
	ListComments(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error)
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

// MoveFile moves a file to a new folder by updating its parents.
func (s *realDriveService) MoveFile(ctx context.Context, fileID string, folderID string) error {
	// Get current file to find existing parents
	file, err := s.service.Files.Get(fileID).
		Fields("parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return err
	}

	// Build list of current parents to remove
	var previousParents string
	if len(file.Parents) > 0 {
		previousParents = file.Parents[0]
		for i := 1; i < len(file.Parents); i++ {
			previousParents += "," + file.Parents[i]
		}
	}

	// Move file to new folder
	_, err = s.service.Files.Update(fileID, nil).
		AddParents(folderID).
		RemoveParents(previousParents).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	return err
}

// UploadFile uploads a file to Drive.
func (s *realDriveService) UploadFile(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
	file := &drive.File{
		Name:     name,
		MimeType: mimeType,
	}
	return s.service.Files.Create(file).Media(content).Context(ctx).Do()
}

// MakeFilePublic makes a file publicly accessible via link.
func (s *realDriveService) MakeFilePublic(ctx context.Context, fileID string) error {
	permission := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}
	_, err := s.service.Permissions.Create(fileID, permission).Context(ctx).Do()
	return err
}

// ListComments lists comments on a file.
func (s *realDriveService) ListComments(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
	call := s.service.Comments.List(fileID).
		Fields("comments(id,kind,content,htmlContent,author,createdTime,modifiedTime,resolved,deleted,anchor,replies,quotedFileContent),nextPageToken").
		IncludeDeleted(includeDeleted).
		Context(ctx)

	if pageSize > 0 {
		call = call.PageSize(pageSize)
	}
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	return call.Do()
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
