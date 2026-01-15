package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for delete_slide tool.
var (
	ErrDeleteSlideFailed = errors.New("failed to delete slide")
	ErrLastSlideDelete   = errors.New("cannot delete the last remaining slide")
)

// DeleteSlideInput represents the input for the delete_slide tool.
type DeleteSlideInput struct {
	PresentationID string `json:"presentation_id"`
	SlideIndex     int    `json:"slide_index,omitempty"` // 1-based index (use this OR SlideID)
	SlideID        string `json:"slide_id,omitempty"`    // Slide object ID (use this OR SlideIndex)
}

// DeleteSlideOutput represents the output of the delete_slide tool.
type DeleteSlideOutput struct {
	DeletedSlideID     string `json:"deleted_slide_id"`     // Object ID of the deleted slide
	RemainingSlideCount int    `json:"remaining_slide_count"` // Number of slides after deletion
}

// DeleteSlide deletes a slide from a presentation.
func (t *Tools) DeleteSlide(ctx context.Context, tokenSource oauth2.TokenSource, input DeleteSlideInput) (*DeleteSlideOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	t.config.Logger.Info("deleting slide from presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find the slide and validate
	presentation, err := slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrSlidesAPIError, err)
	}

	// Check if this is the last slide
	if len(presentation.Slides) <= 1 {
		return nil, fmt.Errorf("%w: presentation must have at least one slide", ErrLastSlideDelete)
	}

	// Find the slide to delete
	var slideToDeleteID string
	if input.SlideID != "" {
		// Find by slide ID
		found := false
		for _, slide := range presentation.Slides {
			if slide.ObjectId == input.SlideID {
				slideToDeleteID = input.SlideID
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: slide with ID '%s' not found", ErrSlideNotFound, input.SlideID)
		}
	} else {
		// Find by index (1-based)
		if input.SlideIndex < 1 || input.SlideIndex > len(presentation.Slides) {
			return nil, fmt.Errorf("%w: slide index %d out of range (1-%d)", ErrSlideNotFound, input.SlideIndex, len(presentation.Slides))
		}
		slideToDeleteID = presentation.Slides[input.SlideIndex-1].ObjectId
	}

	// Build the DeleteObjectRequest
	requests := []*slides.Request{
		{
			DeleteObject: &slides.DeleteObjectRequest{
				ObjectId: slideToDeleteID,
			},
		},
	}

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDeleteSlideFailed, err)
	}

	// Calculate remaining slide count
	remainingSlideCount := len(presentation.Slides) - 1

	output := &DeleteSlideOutput{
		DeletedSlideID:     slideToDeleteID,
		RemainingSlideCount: remainingSlideCount,
	}

	t.config.Logger.Info("slide deleted successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("deleted_slide_id", output.DeletedSlideID),
		slog.Int("remaining_slides", output.RemainingSlideCount),
	)

	return output, nil
}
