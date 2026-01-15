package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for create_presentation tool.
var (
	ErrCreateFailed       = errors.New("failed to create presentation")
	ErrInvalidCreateTitle = errors.New("invalid title for presentation")
	ErrFolderNotFound     = errors.New("destination folder not found or inaccessible")
)

// CreatePresentationInput represents the input for the create_presentation tool.
type CreatePresentationInput struct {
	Title    string `json:"title"`
	FolderID string `json:"folder_id,omitempty"`
}

// CreatePresentationOutput represents the output of the create_presentation tool.
type CreatePresentationOutput struct {
	PresentationID string `json:"presentation_id"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	FolderID       string `json:"folder_id,omitempty"`
}

// CreatePresentation creates a new empty Google Slides presentation.
func (t *Tools) CreatePresentation(ctx context.Context, tokenSource oauth2.TokenSource, input CreatePresentationInput) (*CreatePresentationOutput, error) {
	// Validate input
	if input.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidCreateTitle)
	}

	t.config.Logger.Info("creating presentation",
		slog.String("title", input.Title),
		slog.String("folder_id", input.FolderID),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Create the presentation using Slides API
	presentation := &slides.Presentation{
		Title: input.Title,
	}

	createdPresentation, err := slidesService.CreatePresentation(ctx, presentation)
	if err != nil {
		if isForbiddenError(err) {
			return nil, fmt.Errorf("%w: access denied", ErrAccessDenied)
		}
		return nil, fmt.Errorf("%w: %v", ErrCreateFailed, err)
	}

	// If folder is specified, move the presentation to that folder
	if input.FolderID != "" {
		driveService, err := t.driveServiceFactory(ctx, tokenSource)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
		}

		err = driveService.MoveFile(ctx, createdPresentation.PresentationId, input.FolderID)
		if err != nil {
			// Check for folder-related errors
			if isParentNotFoundError(err) || isFolderNotFoundError(err) {
				return nil, fmt.Errorf("%w: %v", ErrFolderNotFound, err)
			}
			if isForbiddenError(err) {
				return nil, fmt.Errorf("%w: access denied to folder", ErrAccessDenied)
			}
			// Presentation was created but move failed - log warning and continue
			t.config.Logger.Warn("failed to move presentation to folder, presentation created in root",
				slog.String("presentation_id", createdPresentation.PresentationId),
				slog.String("folder_id", input.FolderID),
				slog.Any("error", err),
			)
		}
	}

	// Build the presentation URL
	presentationURL := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", createdPresentation.PresentationId)

	output := &CreatePresentationOutput{
		PresentationID: createdPresentation.PresentationId,
		Title:          createdPresentation.Title,
		URL:            presentationURL,
	}

	if input.FolderID != "" {
		output.FolderID = input.FolderID
	}

	t.config.Logger.Info("presentation created successfully",
		slog.String("presentation_id", output.PresentationID),
		slog.String("title", output.Title),
	)

	return output, nil
}

// isFolderNotFoundError checks if an error indicates the folder was not found.
func isFolderNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return isNotFoundError(err) ||
		strings.Contains(errStr, "folder not found") ||
		strings.Contains(errStr, "invalid folder")
}
