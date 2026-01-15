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

// Sentinel errors for style_text tool.
var (
	ErrStyleTextFailed = errors.New("failed to apply text style")
	ErrNoStyleProvided = errors.New("no style properties provided")
)

// StyleTextInput represents the input for the style_text tool.
type StyleTextInput struct {
	PresentationID string              `json:"presentation_id"`
	ObjectID       string              `json:"object_id"`
	StartIndex     *int                `json:"start_index,omitempty"` // Optional, whole text if omitted
	EndIndex       *int                `json:"end_index,omitempty"`   // Optional, whole text if omitted
	Style          *StyleTextStyleSpec `json:"style"`
}

// StyleTextStyleSpec represents the style properties to apply.
type StyleTextStyleSpec struct {
	FontFamily      string `json:"font_family,omitempty"`
	FontSize        int    `json:"font_size,omitempty"`        // In points
	Bold            *bool  `json:"bold,omitempty"`             // Use pointer to distinguish false from unset
	Italic          *bool  `json:"italic,omitempty"`           // Use pointer to distinguish false from unset
	Underline       *bool  `json:"underline,omitempty"`        // Use pointer to distinguish false from unset
	Strikethrough   *bool  `json:"strikethrough,omitempty"`    // Use pointer to distinguish false from unset
	ForegroundColor string `json:"foreground_color,omitempty"` // Hex color (e.g., "#FF0000")
	BackgroundColor string `json:"background_color,omitempty"` // Hex color (e.g., "#FFFF00")
	LinkURL         string `json:"link_url,omitempty"`         // URL for hyperlink
}

// StyleTextOutput represents the output of the style_text tool.
type StyleTextOutput struct {
	ObjectID      string   `json:"object_id"`
	AppliedStyles []string `json:"applied_styles"` // List of style properties that were applied
	TextRange     string   `json:"text_range"`     // "ALL" or "FIXED_RANGE (start-end)"
}

// StyleText applies styling to text in a shape.
func (t *Tools) StyleText(ctx context.Context, tokenSource oauth2.TokenSource, input StyleTextInput) (*StyleTextOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}
	if input.Style == nil {
		return nil, fmt.Errorf("%w: style is required", ErrNoStyleProvided)
	}

	// Validate indices if provided
	if input.StartIndex != nil && *input.StartIndex < 0 {
		return nil, fmt.Errorf("%w: start_index cannot be negative", ErrInvalidTextRange)
	}
	if input.EndIndex != nil && *input.EndIndex < 0 {
		return nil, fmt.Errorf("%w: end_index cannot be negative", ErrInvalidTextRange)
	}
	if input.StartIndex != nil && input.EndIndex != nil && *input.StartIndex > *input.EndIndex {
		return nil, fmt.Errorf("%w: start_index cannot be greater than end_index", ErrInvalidTextRange)
	}

	t.config.Logger.Info("applying text style",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to verify the object exists and get its text length
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
			return nil, fmt.Errorf("%w: tables must be styled cell by cell", ErrNotTextObject)
		}
		return nil, fmt.Errorf("%w: object '%s' does not contain text", ErrNotTextObject, input.ObjectID)
	}

	// Build the style request
	request, appliedStyles := buildStyleTextRequest(input)
	if request == nil || len(appliedStyles) == 0 {
		return nil, ErrNoStyleProvided
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
		return nil, fmt.Errorf("%w: %v", ErrStyleTextFailed, err)
	}

	// Determine text range description
	textRangeDesc := "ALL"
	if input.StartIndex != nil && input.EndIndex != nil {
		textRangeDesc = fmt.Sprintf("FIXED_RANGE (%d-%d)", *input.StartIndex, *input.EndIndex)
	} else if input.StartIndex != nil {
		textRangeDesc = fmt.Sprintf("FROM_START_INDEX (%d)", *input.StartIndex)
	}

	output := &StyleTextOutput{
		ObjectID:      input.ObjectID,
		AppliedStyles: appliedStyles,
		TextRange:     textRangeDesc,
	}

	t.config.Logger.Info("text style applied successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Int("styles_count", len(appliedStyles)),
	)

	return output, nil
}

// buildStyleTextRequest creates the UpdateTextStyleRequest.
func buildStyleTextRequest(input StyleTextInput) (*slides.Request, []string) {
	textStyle := &slides.TextStyle{}
	var fields []string
	var appliedStyles []string

	// Font family
	if input.Style.FontFamily != "" {
		textStyle.FontFamily = input.Style.FontFamily
		fields = append(fields, "fontFamily")
		appliedStyles = append(appliedStyles, fmt.Sprintf("font_family=%s", input.Style.FontFamily))
	}

	// Font size
	if input.Style.FontSize > 0 {
		textStyle.FontSize = &slides.Dimension{
			Magnitude: float64(input.Style.FontSize),
			Unit:      "PT",
		}
		fields = append(fields, "fontSize")
		appliedStyles = append(appliedStyles, fmt.Sprintf("font_size=%dpt", input.Style.FontSize))
	}

	// Bold
	if input.Style.Bold != nil {
		textStyle.Bold = *input.Style.Bold
		fields = append(fields, "bold")
		appliedStyles = append(appliedStyles, fmt.Sprintf("bold=%t", *input.Style.Bold))
	}

	// Italic
	if input.Style.Italic != nil {
		textStyle.Italic = *input.Style.Italic
		fields = append(fields, "italic")
		appliedStyles = append(appliedStyles, fmt.Sprintf("italic=%t", *input.Style.Italic))
	}

	// Underline
	if input.Style.Underline != nil {
		textStyle.Underline = *input.Style.Underline
		fields = append(fields, "underline")
		appliedStyles = append(appliedStyles, fmt.Sprintf("underline=%t", *input.Style.Underline))
	}

	// Strikethrough
	if input.Style.Strikethrough != nil {
		textStyle.Strikethrough = *input.Style.Strikethrough
		fields = append(fields, "strikethrough")
		appliedStyles = append(appliedStyles, fmt.Sprintf("strikethrough=%t", *input.Style.Strikethrough))
	}

	// Foreground color
	if input.Style.ForegroundColor != "" {
		rgb := parseHexColor(input.Style.ForegroundColor)
		if rgb != nil {
			textStyle.ForegroundColor = &slides.OptionalColor{
				OpaqueColor: &slides.OpaqueColor{
					RgbColor: rgb,
				},
			}
			fields = append(fields, "foregroundColor")
			appliedStyles = append(appliedStyles, fmt.Sprintf("foreground_color=%s", input.Style.ForegroundColor))
		}
	}

	// Background color
	if input.Style.BackgroundColor != "" {
		rgb := parseHexColor(input.Style.BackgroundColor)
		if rgb != nil {
			textStyle.BackgroundColor = &slides.OptionalColor{
				OpaqueColor: &slides.OpaqueColor{
					RgbColor: rgb,
				},
			}
			fields = append(fields, "backgroundColor")
			appliedStyles = append(appliedStyles, fmt.Sprintf("background_color=%s", input.Style.BackgroundColor))
		}
	}

	// Link URL
	if input.Style.LinkURL != "" {
		textStyle.Link = &slides.Link{
			Url: input.Style.LinkURL,
		}
		fields = append(fields, "link")
		appliedStyles = append(appliedStyles, fmt.Sprintf("link_url=%s", input.Style.LinkURL))
	}

	if len(fields) == 0 {
		return nil, nil
	}

	// Build text range
	var textRange *slides.Range
	if input.StartIndex != nil || input.EndIndex != nil {
		textRange = &slides.Range{
			Type: "FIXED_RANGE",
		}
		if input.StartIndex != nil {
			startIdx64 := int64(*input.StartIndex)
			textRange.StartIndex = &startIdx64
		}
		if input.EndIndex != nil {
			endIdx64 := int64(*input.EndIndex)
			textRange.EndIndex = &endIdx64
		}
	} else {
		textRange = &slides.Range{
			Type: "ALL",
		}
	}

	return &slides.Request{
		UpdateTextStyle: &slides.UpdateTextStyleRequest{
			ObjectId:  input.ObjectID,
			Style:     textStyle,
			TextRange: textRange,
			Fields:    strings.Join(fields, ","),
		},
	}, appliedStyles
}
