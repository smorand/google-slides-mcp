// Package tools implements MCP tools for Google Slides operations.
package tools

import (
	"context"
	"log/slog"

	"golang.org/x/oauth2"
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
}

// NewTools creates a new Tools instance.
func NewTools(config ToolsConfig, slidesFactory SlidesServiceFactory) *Tools {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if slidesFactory == nil {
		slidesFactory = NewRealSlidesServiceFactory()
	}

	return &Tools{
		config:               config,
		slidesServiceFactory: slidesFactory,
	}
}
