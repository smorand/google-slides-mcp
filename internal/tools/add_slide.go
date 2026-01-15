package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for add_slide tool.
var (
	ErrAddSlideFailed    = errors.New("failed to add slide")
	ErrInvalidLayout     = errors.New("invalid layout type")
	ErrInvalidPosition   = errors.New("invalid slide position")
)

// Supported layout types for Google Slides.
// These correspond to predefined layout types in the Slides API.
var validLayoutTypes = map[string]bool{
	"BLANK":                 true,
	"CAPTION_ONLY":          true,
	"TITLE":                 true,
	"TITLE_AND_BODY":        true,
	"TITLE_AND_TWO_COLUMNS": true,
	"TITLE_ONLY":            true,
	"ONE_COLUMN_TEXT":       true,
	"MAIN_POINT":            true,
	"BIG_NUMBER":            true,
	"SECTION_HEADER":        true,
	"SECTION_TITLE_AND_DESCRIPTION": true,
}

// AddSlideInput represents the input for the add_slide tool.
type AddSlideInput struct {
	PresentationID string `json:"presentation_id"`
	Position       int    `json:"position,omitempty"` // 1-based position (0 or omitted = end)
	Layout         string `json:"layout"`             // Layout type (BLANK, TITLE, TITLE_AND_BODY, etc.)
}

// AddSlideOutput represents the output of the add_slide tool.
type AddSlideOutput struct {
	SlideIndex int    `json:"slide_index"` // 1-based index of the new slide
	SlideID    string `json:"slide_id"`    // Object ID of the new slide
}

// AddSlide adds a new slide to a presentation.
func (t *Tools) AddSlide(ctx context.Context, tokenSource oauth2.TokenSource, input AddSlideInput) (*AddSlideOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.Layout == "" {
		return nil, fmt.Errorf("%w: layout is required", ErrInvalidLayout)
	}

	// Validate layout type
	if !validLayoutTypes[input.Layout] {
		return nil, fmt.Errorf("%w: unsupported layout '%s'", ErrInvalidLayout, input.Layout)
	}

	t.config.Logger.Info("adding slide to presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("position", input.Position),
		slog.String("layout", input.Layout),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to determine number of slides and find layout
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

	// Determine the insertion index (0-based for the API)
	// Position is 1-based for user input, 0 or omitted means end
	numSlides := len(presentation.Slides)
	var insertionIndex int
	if input.Position <= 0 || input.Position > numSlides {
		// Insert at the end
		insertionIndex = numSlides
	} else {
		// Convert 1-based position to 0-based index
		insertionIndex = input.Position - 1
	}

	// Find the layout object ID that matches the requested layout type
	layoutObjectID := findLayoutByType(presentation.Layouts, input.Layout)
	if layoutObjectID == "" {
		// If no matching layout found, use the first layout as fallback
		// This can happen if the presentation has custom layouts
		if len(presentation.Layouts) > 0 {
			layoutObjectID = presentation.Layouts[0].ObjectId
			t.config.Logger.Warn("requested layout not found, using first available layout",
				slog.String("requested_layout", input.Layout),
				slog.String("fallback_layout_id", layoutObjectID),
			)
		}
	}

	// Build the CreateSlideRequest
	createSlideRequest := &slides.CreateSlideRequest{
		InsertionIndex: int64(insertionIndex),
	}

	// Only set LayoutReference if we found a layout
	if layoutObjectID != "" {
		createSlideRequest.SlideLayoutReference = &slides.LayoutReference{
			LayoutId: layoutObjectID,
		}
	} else {
		// Use predefined layout type
		createSlideRequest.SlideLayoutReference = &slides.LayoutReference{
			PredefinedLayout: input.Layout,
		}
	}

	// Execute batch update
	requests := []*slides.Request{
		{
			CreateSlide: createSlideRequest,
		},
	}

	response, err := slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrAddSlideFailed, err)
	}

	// Extract the new slide ID from the response
	var newSlideID string
	if len(response.Replies) > 0 && response.Replies[0].CreateSlide != nil {
		newSlideID = response.Replies[0].CreateSlide.ObjectId
	}

	// Calculate the 1-based slide index
	newSlideIndex := insertionIndex + 1

	output := &AddSlideOutput{
		SlideIndex: newSlideIndex,
		SlideID:    newSlideID,
	}

	t.config.Logger.Info("slide added successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", output.SlideIndex),
		slog.String("slide_id", output.SlideID),
	)

	return output, nil
}

// findLayoutByType finds a layout object ID by its type name.
func findLayoutByType(layouts []*slides.Page, layoutType string) string {
	for _, layout := range layouts {
		if layout.LayoutProperties != nil {
			// Check Name (e.g., "TITLE_AND_BODY")
			if layout.LayoutProperties.Name == layoutType {
				return layout.ObjectId
			}
		}
	}
	return ""
}
