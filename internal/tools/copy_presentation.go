package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

// Sentinel errors for copy_presentation tool.
var (
	ErrSourceNotFound     = errors.New("source presentation not found")
	ErrCopyFailed         = errors.New("failed to copy presentation")
	ErrInvalidSourceID    = errors.New("invalid source presentation ID")
	ErrInvalidTitle       = errors.New("invalid title")
	ErrDestinationInvalid = errors.New("destination folder not found or inaccessible")
)

// CopyPresentationInput represents the input for the copy_presentation tool.
type CopyPresentationInput struct {
	SourceID            string `json:"source_id"`
	NewTitle            string `json:"new_title"`
	DestinationFolderID string `json:"destination_folder_id,omitempty"`
}

// CopyPresentationOutput represents the output of the copy_presentation tool.
type CopyPresentationOutput struct {
	PresentationID string `json:"presentation_id"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	SourceID       string `json:"source_id"`
}

// CopyPresentation copies a Google Slides presentation to a new presentation.
// This is useful for creating presentations from templates.
func (t *Tools) CopyPresentation(ctx context.Context, tokenSource oauth2.TokenSource, input CopyPresentationInput) (*CopyPresentationOutput, error) {
	// Validate input
	if input.SourceID == "" {
		return nil, fmt.Errorf("%w: source_id is required", ErrInvalidSourceID)
	}
	if input.NewTitle == "" {
		return nil, fmt.Errorf("%w: new_title is required", ErrInvalidTitle)
	}

	t.config.Logger.Info("copying presentation",
		slog.String("source_id", input.SourceID),
		slog.String("new_title", input.NewTitle),
		slog.String("destination_folder_id", input.DestinationFolderID),
	)

	// Create Drive service
	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
	}

	// Prepare the copy request
	copyFile := &drive.File{
		Name: input.NewTitle,
	}

	// Set destination folder if specified
	if input.DestinationFolderID != "" {
		copyFile.Parents = []string{input.DestinationFolderID}
	}

	// Copy the presentation
	copiedFile, err := driveService.CopyFile(ctx, input.SourceID, copyFile)
	if err != nil {
		// Check for invalid parent folder first (before generic not found)
		// because parent errors may also contain "not found"
		if isParentNotFoundError(err) {
			return nil, fmt.Errorf("%w: %v", ErrDestinationInvalid, err)
		}
		if isNotFoundError(err) {
			return nil, fmt.Errorf("%w: source presentation not found", ErrSourceNotFound)
		}
		if isForbiddenError(err) {
			return nil, fmt.Errorf("%w: access denied to source presentation", ErrAccessDenied)
		}
		return nil, fmt.Errorf("%w: %v", ErrCopyFailed, err)
	}

	// Build the presentation URL
	presentationURL := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", copiedFile.Id)

	output := &CopyPresentationOutput{
		PresentationID: copiedFile.Id,
		Title:          copiedFile.Name,
		URL:            presentationURL,
		SourceID:       input.SourceID,
	}

	t.config.Logger.Info("presentation copied successfully",
		slog.String("source_id", input.SourceID),
		slog.String("new_id", output.PresentationID),
		slog.String("title", output.Title),
	)

	return output, nil
}

// isParentNotFoundError checks if an error indicates the parent folder was not found.
// This specifically checks for parent-related errors (not general file not found errors).
func isParentNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Only match errors that specifically mention parent
	return strings.Contains(errStr, "invalid parent") ||
		strings.Contains(errStr, "parent not found")
}
