package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for duplicate_slide tool.
var (
	ErrDuplicateSlideFailed = errors.New("failed to duplicate slide")
)

// DuplicateSlideInput represents the input for the duplicate_slide tool.
type DuplicateSlideInput struct {
	PresentationID string `json:"presentation_id"`
	SlideIndex     int    `json:"slide_index,omitempty"` // 1-based index (use this OR SlideID)
	SlideID        string `json:"slide_id,omitempty"`    // Slide object ID (use this OR SlideIndex)
	InsertAt       int    `json:"insert_at,omitempty"`   // 1-based position (0 or omitted = after source slide)
}

// DuplicateSlideOutput represents the output of the duplicate_slide tool.
type DuplicateSlideOutput struct {
	SlideIndex int    `json:"slide_index"` // 1-based index of the new duplicated slide
	SlideID    string `json:"slide_id"`    // Object ID of the new duplicated slide
}

// DuplicateSlide duplicates an existing slide in a presentation.
func (t *Tools) DuplicateSlide(ctx context.Context, tokenSource oauth2.TokenSource, input DuplicateSlideInput) (*DuplicateSlideOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	t.config.Logger.Info("duplicating slide in presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.Int("insert_at", input.InsertAt),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find the slide
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

	// Find the source slide
	var sourceSl *slides.Page
	var sourceSlideIndex int // 0-based index of source slide
	if input.SlideID != "" {
		// Find by slide ID
		for i, slide := range presentation.Slides {
			if slide.ObjectId == input.SlideID {
				sourceSl = slide
				sourceSlideIndex = i
				break
			}
		}
		if sourceSl == nil {
			return nil, fmt.Errorf("%w: slide with ID '%s' not found", ErrSlideNotFound, input.SlideID)
		}
	} else {
		// Find by index (1-based)
		if input.SlideIndex < 1 || input.SlideIndex > len(presentation.Slides) {
			return nil, fmt.Errorf("%w: slide index %d out of range (1-%d)", ErrSlideNotFound, input.SlideIndex, len(presentation.Slides))
		}
		sourceSlideIndex = input.SlideIndex - 1
		sourceSl = presentation.Slides[sourceSlideIndex]
	}

	sourceSlideID := sourceSl.ObjectId

	// Determine the insertion index (0-based for the API)
	// Default is after the source slide (sourceSlideIndex + 1)
	numSlides := len(presentation.Slides)
	var insertionIndex int
	if input.InsertAt <= 0 {
		// Insert after source slide
		insertionIndex = sourceSlideIndex + 1
	} else if input.InsertAt > numSlides+1 {
		// Clamp to end (after duplicating, max valid position is numSlides+1)
		insertionIndex = numSlides
	} else {
		// Convert 1-based position to 0-based index
		insertionIndex = input.InsertAt - 1
	}

	// Build the DuplicateObjectRequest
	requests := []*slides.Request{
		{
			DuplicateObject: &slides.DuplicateObjectRequest{
				ObjectId: sourceSlideID,
			},
		},
	}

	// Execute batch update to duplicate
	response, err := slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDuplicateSlideFailed, err)
	}

	// Extract the new slide ID from the response
	var newSlideID string
	if len(response.Replies) > 0 && response.Replies[0].DuplicateObject != nil {
		newSlideID = response.Replies[0].DuplicateObject.ObjectId
	}

	if newSlideID == "" {
		return nil, fmt.Errorf("%w: no slide ID returned from API", ErrDuplicateSlideFailed)
	}

	// The duplicated slide is inserted immediately after the source slide by default.
	// If user wants a different position, we need to move it.
	// The API places the duplicate right after the source.
	duplicatedSlideCurrentIndex := sourceSlideIndex + 1 // 0-based index where duplicate was placed

	// Check if we need to move the duplicated slide
	if insertionIndex != duplicatedSlideCurrentIndex {
		// Need to move the slide to the requested position
		moveRequests := []*slides.Request{
			{
				UpdateSlidesPosition: &slides.UpdateSlidesPositionRequest{
					SlideObjectIds:  []string{newSlideID},
					InsertionIndex:  int64(insertionIndex),
				},
			},
		}

		_, err = slidesService.BatchUpdate(ctx, input.PresentationID, moveRequests)
		if err != nil {
			// Log warning but don't fail - the slide was duplicated, just not moved
			t.config.Logger.Warn("failed to move duplicated slide to requested position",
				slog.String("slide_id", newSlideID),
				slog.Int("requested_position", input.InsertAt),
				slog.String("error", err.Error()),
			)
			// Return the slide in its current position (after source)
			insertionIndex = duplicatedSlideCurrentIndex
		}
	}

	// Calculate the 1-based slide index
	newSlideIndex := insertionIndex + 1

	output := &DuplicateSlideOutput{
		SlideIndex: newSlideIndex,
		SlideID:    newSlideID,
	}

	t.config.Logger.Info("slide duplicated successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("source_slide_id", sourceSlideID),
		slog.Int("new_slide_index", output.SlideIndex),
		slog.String("new_slide_id", output.SlideID),
	)

	return output, nil
}
