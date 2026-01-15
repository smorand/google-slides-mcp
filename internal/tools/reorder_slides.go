package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for reorder_slides tool.
var (
	ErrReorderSlidesFailed = errors.New("failed to reorder slides")
	ErrNoSlidesToMove      = errors.New("no slides specified to move")
	ErrInvalidInsertAt     = errors.New("invalid insert_at position")
)

// ReorderSlidesInput represents the input for the reorder_slides tool.
type ReorderSlidesInput struct {
	PresentationID string   `json:"presentation_id"`
	SlideIndices   []int    `json:"slide_indices,omitempty"` // 1-based indices (use this OR SlideIDs)
	SlideIDs       []string `json:"slide_ids,omitempty"`     // Slide object IDs (use this OR SlideIndices)
	InsertAt       int      `json:"insert_at"`               // 1-based position to move slides to
}

// ReorderSlidesOutput represents the output of the reorder_slides tool.
type ReorderSlidesOutput struct {
	NewOrder []SlidePosition `json:"new_order"` // New slide order after reordering
}

// SlidePosition represents a slide's position in the presentation.
type SlidePosition struct {
	Index   int    `json:"index"`    // 1-based index in the new order
	SlideID string `json:"slide_id"` // Slide object ID
}

// ReorderSlides moves slides to a new position in the presentation.
func (t *Tools) ReorderSlides(ctx context.Context, tokenSource oauth2.TokenSource, input ReorderSlidesInput) (*ReorderSlidesOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if len(input.SlideIndices) == 0 && len(input.SlideIDs) == 0 {
		return nil, fmt.Errorf("%w: either slide_indices or slide_ids is required", ErrNoSlidesToMove)
	}

	if input.InsertAt < 1 {
		return nil, fmt.Errorf("%w: insert_at must be at least 1", ErrInvalidInsertAt)
	}

	t.config.Logger.Info("reordering slides in presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Any("slide_indices", input.SlideIndices),
		slog.Any("slide_ids", input.SlideIDs),
		slog.Int("insert_at", input.InsertAt),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find slides
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

	numSlides := len(presentation.Slides)

	// Resolve slide IDs to move
	var slideIDsToMove []string
	if len(input.SlideIDs) > 0 {
		// Use provided slide IDs, validate they exist
		slideIDsToMove = make([]string, 0, len(input.SlideIDs))
		existingIDs := make(map[string]bool)
		for _, s := range presentation.Slides {
			existingIDs[s.ObjectId] = true
		}
		for _, id := range input.SlideIDs {
			if !existingIDs[id] {
				return nil, fmt.Errorf("%w: slide with ID '%s' not found", ErrSlideNotFound, id)
			}
			slideIDsToMove = append(slideIDsToMove, id)
		}
	} else {
		// Convert 1-based indices to slide IDs
		slideIDsToMove = make([]string, 0, len(input.SlideIndices))
		for _, idx := range input.SlideIndices {
			if idx < 1 || idx > numSlides {
				return nil, fmt.Errorf("%w: slide index %d out of range (1-%d)", ErrSlideNotFound, idx, numSlides)
			}
			slideIDsToMove = append(slideIDsToMove, presentation.Slides[idx-1].ObjectId)
		}
	}

	// Validate insert_at position
	if input.InsertAt > numSlides {
		// Clamp to the end of the presentation
		input.InsertAt = numSlides
	}

	// Calculate the insertion index for the API (0-based)
	// The API InsertionIndex specifies where the slides should go AFTER the move
	// We need to account for the slides being removed from their current positions
	insertionIndex := input.InsertAt - 1

	// Build the UpdateSlidesPositionRequest
	// This request moves all specified slides to the given position
	requests := []*slides.Request{
		{
			UpdateSlidesPosition: &slides.UpdateSlidesPositionRequest{
				SlideObjectIds: slideIDsToMove,
				InsertionIndex: int64(insertionIndex),
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
		return nil, fmt.Errorf("%w: %v", ErrReorderSlidesFailed, err)
	}

	// Fetch the updated presentation to get new slide order
	updatedPresentation, err := slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		// Even if we can't fetch the updated state, the operation succeeded
		t.config.Logger.Warn("failed to fetch updated presentation after reorder",
			slog.Any("error", err),
		)
		// Return empty result as we can't determine the new order
		return &ReorderSlidesOutput{
			NewOrder: []SlidePosition{},
		}, nil
	}

	// Build the new order output
	newOrder := make([]SlidePosition, len(updatedPresentation.Slides))
	for i, slide := range updatedPresentation.Slides {
		newOrder[i] = SlidePosition{
			Index:   i + 1, // 1-based
			SlideID: slide.ObjectId,
		}
	}

	output := &ReorderSlidesOutput{
		NewOrder: newOrder,
	}

	t.config.Logger.Info("slides reordered successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slides_moved", len(slideIDsToMove)),
		slog.Int("total_slides", len(newOrder)),
	)

	return output, nil
}
