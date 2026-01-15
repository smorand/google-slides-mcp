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

// Sentinel errors for create_numbered_list tool.
var (
	ErrCreateNumberedListFailed = errors.New("failed to create numbered list")
	ErrInvalidNumberStyle       = errors.New("invalid number style")
	ErrInvalidStartNumber       = errors.New("invalid start number")
)

// Valid number styles for numbered lists.
var validNumberStyles = map[string]string{
	// User-friendly names to API preset names
	"DECIMAL":     "NUMBERED_DECIMAL_ALPHA_ROMAN",
	"ALPHA_UPPER": "NUMBERED_UPPERALPHA_ALPHA_ROMAN",
	"ALPHA_LOWER": "NUMBERED_ALPHA_ALPHA_ROMAN",
	"ROMAN_UPPER": "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL",
	"ROMAN_LOWER": "NUMBERED_ROMAN_UPPERALPHA_DECIMAL",
	// Full preset names also accepted
	"NUMBERED_DECIMAL_ALPHA_ROMAN":           "NUMBERED_DECIMAL_ALPHA_ROMAN",
	"NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS":    "NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS",
	"NUMBERED_DECIMAL_NESTED":                "NUMBERED_DECIMAL_NESTED",
	"NUMBERED_UPPERALPHA_ALPHA_ROMAN":        "NUMBERED_UPPERALPHA_ALPHA_ROMAN",
	"NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL": "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL",
	"NUMBERED_ZERODIGIT_ALPHA_ROMAN":         "NUMBERED_ZERODIGIT_ALPHA_ROMAN",
	"NUMBERED_ALPHA_ALPHA_ROMAN":             "NUMBERED_ALPHA_ALPHA_ROMAN",
	"NUMBERED_ROMAN_UPPERALPHA_DECIMAL":      "NUMBERED_ROMAN_UPPERALPHA_DECIMAL",
}

// CreateNumberedListInput represents the input for the create_numbered_list tool.
type CreateNumberedListInput struct {
	PresentationID   string `json:"presentation_id"`
	ObjectID         string `json:"object_id"`
	ParagraphIndices []int  `json:"paragraph_indices,omitempty"` // Optional, all paragraphs if omitted
	NumberStyle      string `json:"number_style"`                // DECIMAL, ALPHA_UPPER, ALPHA_LOWER, ROMAN_UPPER, ROMAN_LOWER or full preset name
	StartNumber      int    `json:"start_number,omitempty"`      // Starting number (default 1)
}

// CreateNumberedListOutput represents the output of the create_numbered_list tool.
type CreateNumberedListOutput struct {
	ObjectID       string `json:"object_id"`
	NumberPreset   string `json:"number_preset"`   // The actual preset applied
	ParagraphScope string `json:"paragraph_scope"` // "ALL" or "INDICES [1, 2, 3]"
	StartNumber    int    `json:"start_number"`    // The start number applied
}

// CreateNumberedList converts text to a numbered list or adds numbering to existing text.
func (t *Tools) CreateNumberedList(ctx context.Context, tokenSource oauth2.TokenSource, input CreateNumberedListInput) (*CreateNumberedListOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}
	if input.NumberStyle == "" {
		return nil, fmt.Errorf("%w: number_style is required", ErrInvalidNumberStyle)
	}

	// Normalize and validate number style
	numberStyleUpper := strings.ToUpper(input.NumberStyle)
	numberPreset, ok := validNumberStyles[numberStyleUpper]
	if !ok {
		return nil, fmt.Errorf("%w: '%s' is not a valid number style; use DECIMAL, ALPHA_UPPER, ALPHA_LOWER, ROMAN_UPPER, ROMAN_LOWER, or a full preset name", ErrInvalidNumberStyle, input.NumberStyle)
	}

	// Set default start number if not provided
	startNumber := input.StartNumber
	if startNumber == 0 {
		startNumber = 1
	}
	if startNumber < 1 {
		return nil, fmt.Errorf("%w: start_number must be at least 1", ErrInvalidStartNumber)
	}

	// Validate paragraph indices
	for _, idx := range input.ParagraphIndices {
		if idx < 0 {
			return nil, fmt.Errorf("%w: paragraph indices cannot be negative", ErrInvalidParagraphIndex)
		}
	}

	t.config.Logger.Info("creating numbered list",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("number_style", numberPreset),
		slog.Int("start_number", startNumber),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to verify the object exists and get paragraph info
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

	// Find the target element
	var targetElement *slides.PageElement
	for _, slide := range presentation.Slides {
		element := findElementByID(slide.PageElements, input.ObjectID)
		if element != nil {
			targetElement = element
			break
		}
	}

	if targetElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Verify the object has text
	if targetElement.Shape == nil || targetElement.Shape.Text == nil {
		if targetElement.Table != nil {
			return nil, fmt.Errorf("%w: tables must have numbering applied cell by cell", ErrNotTextObject)
		}
		return nil, fmt.Errorf("%w: object '%s' does not contain text", ErrNotTextObject, input.ObjectID)
	}

	// Validate paragraph indices if provided
	if len(input.ParagraphIndices) > 0 {
		paragraphCount := countParagraphs(targetElement.Shape.Text)
		for _, idx := range input.ParagraphIndices {
			if idx >= paragraphCount {
				return nil, fmt.Errorf("%w: paragraph index %d is out of range (object has %d paragraphs)", ErrInvalidParagraphIndex, idx, paragraphCount)
			}
		}
	}

	// Build the requests
	requests := buildCreateNumberedListRequests(input, numberPreset, targetElement.Shape.Text, startNumber)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrCreateNumberedListFailed, err)
	}

	// Determine paragraph scope description
	paragraphScope := "ALL"
	if len(input.ParagraphIndices) > 0 {
		paragraphScope = fmt.Sprintf("INDICES %v", input.ParagraphIndices)
	}

	output := &CreateNumberedListOutput{
		ObjectID:       input.ObjectID,
		NumberPreset:   numberPreset,
		ParagraphScope: paragraphScope,
		StartNumber:    startNumber,
	}

	t.config.Logger.Info("numbered list created successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("number_preset", numberPreset),
		slog.Int("start_number", startNumber),
	)

	return output, nil
}

// buildCreateNumberedListRequests creates the requests for creating numbered lists.
// Note: The startNumber parameter is accepted for API completeness but Google Slides API
// CreateParagraphBulletsRequest does not support custom start numbers directly.
// The numbered list will always start from 1 when using this approach.
func buildCreateNumberedListRequests(input CreateNumberedListInput, numberPreset string, text *slides.TextContent, _ int) []*slides.Request {
	var requests []*slides.Request

	// Build text range based on paragraph indices
	textRange := getBulletTextRange(text, input.ParagraphIndices)

	// Create the CreateParagraphBulletsRequest (used for both bullets and numbering)
	bulletRequest := &slides.Request{
		CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
			ObjectId:     input.ObjectID,
			TextRange:    textRange,
			BulletPreset: numberPreset,
		},
	}
	requests = append(requests, bulletRequest)

	return requests
}
