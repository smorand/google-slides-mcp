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

// Sentinel errors for modify_list tool.
var (
	ErrModifyListFailed = errors.New("failed to modify list")
	ErrInvalidListAction = errors.New("invalid list action")
	ErrNoListProperties  = errors.New("no list properties provided")
)

// Default indentation increment for list nesting (in points).
const defaultIndentIncrement = 18.0

// ModifyListInput represents the input for the modify_list tool.
type ModifyListInput struct {
	PresentationID   string              `json:"presentation_id"`
	ObjectID         string              `json:"object_id"`
	Action           string              `json:"action"` // 'modify' | 'remove' | 'increase_indent' | 'decrease_indent'
	ParagraphIndices []int               `json:"paragraph_indices,omitempty"` // Optional, all paragraphs if omitted
	Properties       *ListModifyProperties `json:"properties,omitempty"` // Required for 'modify' action
}

// ListModifyProperties represents properties to modify on a list.
type ListModifyProperties struct {
	BulletStyle string `json:"bullet_style,omitempty"` // DISC, CIRCLE, SQUARE, DIAMOND, ARROW, STAR, CHECKBOX
	NumberStyle string `json:"number_style,omitempty"` // DECIMAL, ALPHA_UPPER, ALPHA_LOWER, ROMAN_UPPER, ROMAN_LOWER
	Color       string `json:"color,omitempty"`        // Hex color string (e.g., "#FF0000")
}

// ModifyListOutput represents the output of the modify_list tool.
type ModifyListOutput struct {
	ObjectID       string `json:"object_id"`
	Action         string `json:"action"`
	ParagraphScope string `json:"paragraph_scope"` // "ALL" or "INDICES [1, 2, 3]"
	Result         string `json:"result"`          // Description of what was done
}

// ModifyList modifies existing list properties or removes list formatting.
func (t *Tools) ModifyList(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyListInput) (*ModifyListOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Normalize and validate action
	actionLower := strings.ToLower(input.Action)
	if actionLower != "modify" && actionLower != "remove" && actionLower != "increase_indent" && actionLower != "decrease_indent" {
		return nil, fmt.Errorf("%w: action must be 'modify', 'remove', 'increase_indent', or 'decrease_indent'", ErrInvalidListAction)
	}

	// Validate properties for modify action
	if actionLower == "modify" {
		if input.Properties == nil {
			return nil, fmt.Errorf("%w: properties are required for 'modify' action", ErrNoListProperties)
		}
		if input.Properties.BulletStyle == "" && input.Properties.NumberStyle == "" && input.Properties.Color == "" {
			return nil, fmt.Errorf("%w: at least one property (bullet_style, number_style, or color) must be provided", ErrNoListProperties)
		}
		// Validate bullet style if provided
		if input.Properties.BulletStyle != "" {
			bulletStyleUpper := strings.ToUpper(input.Properties.BulletStyle)
			if _, ok := validBulletStyles[bulletStyleUpper]; !ok {
				return nil, fmt.Errorf("%w: '%s' is not a valid bullet style", ErrInvalidBulletStyle, input.Properties.BulletStyle)
			}
		}
		// Validate number style if provided
		if input.Properties.NumberStyle != "" {
			numberStyleUpper := strings.ToUpper(input.Properties.NumberStyle)
			if _, ok := validNumberStyles[numberStyleUpper]; !ok {
				return nil, fmt.Errorf("%w: '%s' is not a valid number style", ErrInvalidNumberStyle, input.Properties.NumberStyle)
			}
		}
	}

	// Validate paragraph indices
	for _, idx := range input.ParagraphIndices {
		if idx < 0 {
			return nil, fmt.Errorf("%w: paragraph indices cannot be negative", ErrInvalidParagraphIndex)
		}
	}

	t.config.Logger.Info("modifying list",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", actionLower),
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
			return nil, fmt.Errorf("%w: tables must have list properties modified cell by cell", ErrNotTextObject)
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

	// Build the requests based on action
	var requests []*slides.Request
	var resultDescription string

	switch actionLower {
	case "modify":
		requests, resultDescription = buildModifyListRequests(input, targetElement.Shape.Text)
	case "remove":
		requests, resultDescription = buildRemoveListRequests(input, targetElement.Shape.Text)
	case "increase_indent":
		requests, resultDescription = buildIndentListRequests(input, targetElement.Shape.Text, true)
	case "decrease_indent":
		requests, resultDescription = buildIndentListRequests(input, targetElement.Shape.Text, false)
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
		return nil, fmt.Errorf("%w: %v", ErrModifyListFailed, err)
	}

	// Determine paragraph scope description
	paragraphScope := "ALL"
	if len(input.ParagraphIndices) > 0 {
		paragraphScope = fmt.Sprintf("INDICES %v", input.ParagraphIndices)
	}

	output := &ModifyListOutput{
		ObjectID:       input.ObjectID,
		Action:         actionLower,
		ParagraphScope: paragraphScope,
		Result:         resultDescription,
	}

	t.config.Logger.Info("list modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", actionLower),
		slog.String("result", resultDescription),
	)

	return output, nil
}

// buildModifyListRequests creates requests to modify list properties.
func buildModifyListRequests(input ModifyListInput, text *slides.TextContent) ([]*slides.Request, string) {
	var requests []*slides.Request
	var descriptions []string

	textRange := getBulletTextRange(text, input.ParagraphIndices)

	// If a new bullet or number style is provided, we need to recreate the list
	if input.Properties.BulletStyle != "" {
		bulletStyleUpper := strings.ToUpper(input.Properties.BulletStyle)
		bulletPreset := validBulletStyles[bulletStyleUpper]

		requests = append(requests, &slides.Request{
			CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
				ObjectId:     input.ObjectID,
				TextRange:    textRange,
				BulletPreset: bulletPreset,
			},
		})
		descriptions = append(descriptions, fmt.Sprintf("bullet_style=%s", bulletPreset))
	}

	if input.Properties.NumberStyle != "" {
		numberStyleUpper := strings.ToUpper(input.Properties.NumberStyle)
		numberPreset := validNumberStyles[numberStyleUpper]

		requests = append(requests, &slides.Request{
			CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
				ObjectId:     input.ObjectID,
				TextRange:    textRange,
				BulletPreset: numberPreset,
			},
		})
		descriptions = append(descriptions, fmt.Sprintf("number_style=%s", numberPreset))
	}

	// Apply color if provided
	if input.Properties.Color != "" {
		rgb := parseHexColor(input.Properties.Color)
		if rgb != nil {
			requests = append(requests, &slides.Request{
				UpdateTextStyle: &slides.UpdateTextStyleRequest{
					ObjectId:  input.ObjectID,
					TextRange: textRange,
					Style: &slides.TextStyle{
						ForegroundColor: &slides.OptionalColor{
							OpaqueColor: &slides.OpaqueColor{
								RgbColor: rgb,
							},
						},
					},
					Fields: "foregroundColor",
				},
			})
			descriptions = append(descriptions, fmt.Sprintf("color=%s", input.Properties.Color))
		}
	}

	resultDescription := "Modified: " + strings.Join(descriptions, ", ")
	return requests, resultDescription
}

// buildRemoveListRequests creates requests to remove list formatting.
func buildRemoveListRequests(input ModifyListInput, text *slides.TextContent) ([]*slides.Request, string) {
	textRange := getBulletTextRange(text, input.ParagraphIndices)

	requests := []*slides.Request{
		{
			DeleteParagraphBullets: &slides.DeleteParagraphBulletsRequest{
				ObjectId:  input.ObjectID,
				TextRange: textRange,
			},
		},
	}

	return requests, "Removed list formatting (converted to plain text)"
}

// buildIndentListRequests creates requests to increase or decrease list indentation.
func buildIndentListRequests(input ModifyListInput, text *slides.TextContent, increase bool) ([]*slides.Request, string) {
	textRange := getBulletTextRange(text, input.ParagraphIndices)

	// Get current indentation from paragraphs to calculate new value
	currentIndent := getCurrentIndent(text, input.ParagraphIndices)

	var newIndent float64
	var resultDescription string

	if increase {
		newIndent = currentIndent + defaultIndentIncrement
		resultDescription = fmt.Sprintf("Increased indentation to %.0f points", newIndent)
	} else {
		newIndent = currentIndent - defaultIndentIncrement
		if newIndent < 0 {
			newIndent = 0
		}
		resultDescription = fmt.Sprintf("Decreased indentation to %.0f points", newIndent)
	}

	requests := []*slides.Request{
		{
			UpdateParagraphStyle: &slides.UpdateParagraphStyleRequest{
				ObjectId:  input.ObjectID,
				TextRange: textRange,
				Style: &slides.ParagraphStyle{
					IndentStart: &slides.Dimension{
						Magnitude: newIndent,
						Unit:      "PT",
					},
				},
				Fields: "indentStart",
			},
		},
	}

	return requests, resultDescription
}

// getCurrentIndent returns the current indentation of paragraphs in points.
func getCurrentIndent(text *slides.TextContent, paragraphIndices []int) float64 {
	if text == nil || len(text.TextElements) == 0 {
		return 0
	}

	// Find the first relevant paragraph marker
	paragraphRanges := getParagraphRanges(text)
	if len(paragraphRanges) == 0 {
		return 0
	}

	// Determine which paragraphs to check
	indicesToCheck := paragraphIndices
	if len(indicesToCheck) == 0 {
		// All paragraphs - check the first one
		indicesToCheck = []int{0}
	}

	// Look for paragraph marker style in the specified paragraphs
	for _, element := range text.TextElements {
		if element.ParagraphMarker != nil && element.ParagraphMarker.Style != nil {
			style := element.ParagraphMarker.Style
			if style.IndentStart != nil && style.IndentStart.Unit == "PT" {
				return style.IndentStart.Magnitude
			}
		}
	}

	return 0
}
