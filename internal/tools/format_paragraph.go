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

// Sentinel errors for format_paragraph tool.
var (
	ErrFormatParagraphFailed = errors.New("failed to format paragraph")
	ErrNoFormattingProvided  = errors.New("no formatting properties provided")
	ErrInvalidAlignment      = errors.New("invalid alignment value")
	ErrInvalidParagraphIndex = errors.New("invalid paragraph index")
)

// Valid alignment values for paragraph formatting.
var validAlignments = map[string]bool{
	"START":     true,
	"CENTER":    true,
	"END":       true,
	"JUSTIFIED": true,
}

// FormatParagraphInput represents the input for the format_paragraph tool.
type FormatParagraphInput struct {
	PresentationID string                      `json:"presentation_id"`
	ObjectID       string                      `json:"object_id"`
	ParagraphIndex *int                        `json:"paragraph_index,omitempty"` // Optional, all paragraphs if omitted
	Formatting     *ParagraphFormattingOptions `json:"formatting"`
}

// ParagraphFormattingOptions represents the formatting options to apply.
type ParagraphFormattingOptions struct {
	Alignment       string   `json:"alignment,omitempty"`         // START, CENTER, END, JUSTIFIED
	LineSpacing     *float64 `json:"line_spacing,omitempty"`      // Percentage (100 = normal, 150 = 1.5 lines)
	SpaceAbove      *float64 `json:"space_above,omitempty"`       // Points
	SpaceBelow      *float64 `json:"space_below,omitempty"`       // Points
	IndentFirstLine *float64 `json:"indent_first_line,omitempty"` // Points
	IndentStart     *float64 `json:"indent_start,omitempty"`      // Points
	IndentEnd       *float64 `json:"indent_end,omitempty"`        // Points
}

// FormatParagraphOutput represents the output of the format_paragraph tool.
type FormatParagraphOutput struct {
	ObjectID          string   `json:"object_id"`
	AppliedFormatting []string `json:"applied_formatting"` // List of formatting properties applied
	ParagraphScope    string   `json:"paragraph_scope"`    // "ALL" or "INDEX (N)"
}

// FormatParagraph sets paragraph formatting options.
func (t *Tools) FormatParagraph(ctx context.Context, tokenSource oauth2.TokenSource, input FormatParagraphInput) (*FormatParagraphOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}
	if input.Formatting == nil {
		return nil, fmt.Errorf("%w: formatting is required", ErrNoFormattingProvided)
	}

	// Validate alignment if provided
	if input.Formatting.Alignment != "" {
		alignmentUpper := strings.ToUpper(input.Formatting.Alignment)
		if !validAlignments[alignmentUpper] {
			return nil, fmt.Errorf("%w: must be START, CENTER, END, or JUSTIFIED", ErrInvalidAlignment)
		}
		// Normalize alignment to uppercase
		input.Formatting.Alignment = alignmentUpper
	}

	// Validate paragraph index if provided
	if input.ParagraphIndex != nil && *input.ParagraphIndex < 0 {
		return nil, fmt.Errorf("%w: paragraph_index cannot be negative", ErrInvalidParagraphIndex)
	}

	t.config.Logger.Info("formatting paragraph",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
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
			return nil, fmt.Errorf("%w: tables must be formatted cell by cell", ErrNotTextObject)
		}
		return nil, fmt.Errorf("%w: object '%s' does not contain text", ErrNotTextObject, input.ObjectID)
	}

	// If paragraph_index is specified, validate it exists
	if input.ParagraphIndex != nil {
		paragraphCount := countParagraphs(targetElement.Shape.Text)
		if *input.ParagraphIndex >= paragraphCount {
			return nil, fmt.Errorf("%w: paragraph index %d is out of range (object has %d paragraphs)", ErrInvalidParagraphIndex, *input.ParagraphIndex, paragraphCount)
		}
	}

	// Build the formatting request
	request, appliedFormatting := buildFormatParagraphRequest(input, targetElement.Shape.Text)
	if request == nil || len(appliedFormatting) == 0 {
		return nil, ErrNoFormattingProvided
	}

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, []*slides.Request{request})
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrFormatParagraphFailed, err)
	}

	// Determine paragraph scope description
	paragraphScope := "ALL"
	if input.ParagraphIndex != nil {
		paragraphScope = fmt.Sprintf("INDEX (%d)", *input.ParagraphIndex)
	}

	output := &FormatParagraphOutput{
		ObjectID:          input.ObjectID,
		AppliedFormatting: appliedFormatting,
		ParagraphScope:    paragraphScope,
	}

	t.config.Logger.Info("paragraph formatted successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Int("formatting_count", len(appliedFormatting)),
	)

	return output, nil
}

// countParagraphs counts the number of paragraphs in text content.
func countParagraphs(text *slides.TextContent) int {
	if text == nil || len(text.TextElements) == 0 {
		return 0
	}

	count := 0
	for _, element := range text.TextElements {
		if element.ParagraphMarker != nil {
			count++
		}
	}
	return count
}

// getParagraphRange returns the text range for a specific paragraph or all paragraphs.
func getParagraphRange(text *slides.TextContent, paragraphIndex *int) *slides.Range {
	if paragraphIndex == nil {
		// Apply to all paragraphs
		return &slides.Range{
			Type: "ALL",
		}
	}

	// Find the specific paragraph range
	paragraphIdx := *paragraphIndex
	currentParagraph := 0
	var startIndex int64 = 0
	var endIndex int64 = 0

	for _, element := range text.TextElements {
		if element.ParagraphMarker != nil {
			if currentParagraph == paragraphIdx {
				// Found the paragraph, endIndex is at the paragraph marker
				endIndex = element.EndIndex
				break
			}
			currentParagraph++
			// Update startIndex for next paragraph
			startIndex = element.EndIndex
		} else if currentParagraph == paragraphIdx {
			// Update endIndex while in the target paragraph
			endIndex = element.EndIndex
		}
	}

	return &slides.Range{
		Type:       "FIXED_RANGE",
		StartIndex: &startIndex,
		EndIndex:   &endIndex,
	}
}

// buildFormatParagraphRequest creates the UpdateParagraphStyleRequest.
func buildFormatParagraphRequest(input FormatParagraphInput, text *slides.TextContent) (*slides.Request, []string) {
	paragraphStyle := &slides.ParagraphStyle{}
	var fields []string
	var appliedFormatting []string

	// Alignment
	if input.Formatting.Alignment != "" {
		paragraphStyle.Alignment = input.Formatting.Alignment
		fields = append(fields, "alignment")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("alignment=%s", input.Formatting.Alignment))
	}

	// Line spacing (percentage)
	if input.Formatting.LineSpacing != nil {
		paragraphStyle.LineSpacing = *input.Formatting.LineSpacing
		fields = append(fields, "lineSpacing")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("line_spacing=%.1f%%", *input.Formatting.LineSpacing))
	}

	// Space above (points)
	if input.Formatting.SpaceAbove != nil {
		paragraphStyle.SpaceAbove = &slides.Dimension{
			Magnitude: *input.Formatting.SpaceAbove,
			Unit:      "PT",
		}
		fields = append(fields, "spaceAbove")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("space_above=%.1fpt", *input.Formatting.SpaceAbove))
	}

	// Space below (points)
	if input.Formatting.SpaceBelow != nil {
		paragraphStyle.SpaceBelow = &slides.Dimension{
			Magnitude: *input.Formatting.SpaceBelow,
			Unit:      "PT",
		}
		fields = append(fields, "spaceBelow")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("space_below=%.1fpt", *input.Formatting.SpaceBelow))
	}

	// Indent first line (points)
	if input.Formatting.IndentFirstLine != nil {
		paragraphStyle.IndentFirstLine = &slides.Dimension{
			Magnitude: *input.Formatting.IndentFirstLine,
			Unit:      "PT",
		}
		fields = append(fields, "indentFirstLine")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("indent_first_line=%.1fpt", *input.Formatting.IndentFirstLine))
	}

	// Indent start (points)
	if input.Formatting.IndentStart != nil {
		paragraphStyle.IndentStart = &slides.Dimension{
			Magnitude: *input.Formatting.IndentStart,
			Unit:      "PT",
		}
		fields = append(fields, "indentStart")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("indent_start=%.1fpt", *input.Formatting.IndentStart))
	}

	// Indent end (points)
	if input.Formatting.IndentEnd != nil {
		paragraphStyle.IndentEnd = &slides.Dimension{
			Magnitude: *input.Formatting.IndentEnd,
			Unit:      "PT",
		}
		fields = append(fields, "indentEnd")
		appliedFormatting = append(appliedFormatting, fmt.Sprintf("indent_end=%.1fpt", *input.Formatting.IndentEnd))
	}

	if len(fields) == 0 {
		return nil, nil
	}

	// Build text range for paragraph targeting
	textRange := getParagraphRange(text, input.ParagraphIndex)

	return &slides.Request{
		UpdateParagraphStyle: &slides.UpdateParagraphStyleRequest{
			ObjectId:  input.ObjectID,
			Style:     paragraphStyle,
			TextRange: textRange,
			Fields:    strings.Join(fields, ","),
		},
	}, appliedFormatting
}
