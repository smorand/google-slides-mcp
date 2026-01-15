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

// Sentinel errors for create_bullet_list tool.
var (
	ErrCreateBulletListFailed = errors.New("failed to create bullet list")
	ErrInvalidBulletStyle     = errors.New("invalid bullet style")
)

// Valid bullet styles for bullet lists.
var validBulletStyles = map[string]string{
	// User-friendly names to API preset names
	"DISC":     "BULLET_DISC_CIRCLE_SQUARE",
	"CIRCLE":   "BULLET_DISC_CIRCLE_SQUARE", // Default, starts with disc
	"SQUARE":   "BULLET_DISC_CIRCLE_SQUARE", // Default, includes square at level 3
	"DIAMOND":  "BULLET_DIAMOND_CIRCLE_SQUARE",
	"ARROW":    "BULLET_ARROW_DIAMOND_DISC",
	"STAR":     "BULLET_STAR_CIRCLE_SQUARE",
	"CHECKBOX": "BULLET_CHECKBOX",
	// Full preset names also accepted
	"BULLET_DISC_CIRCLE_SQUARE":               "BULLET_DISC_CIRCLE_SQUARE",
	"BULLET_DIAMONDX_ARROW3D_SQUARE":          "BULLET_DIAMONDX_ARROW3D_SQUARE",
	"BULLET_CHECKBOX":                         "BULLET_CHECKBOX",
	"BULLET_ARROW_DIAMOND_DISC":               "BULLET_ARROW_DIAMOND_DISC",
	"BULLET_STAR_CIRCLE_SQUARE":               "BULLET_STAR_CIRCLE_SQUARE",
	"BULLET_ARROW3D_CIRCLE_SQUARE":            "BULLET_ARROW3D_CIRCLE_SQUARE",
	"BULLET_LEFTTRIANGLE_DIAMOND_DISC":        "BULLET_LEFTTRIANGLE_DIAMOND_DISC",
	"BULLET_DIAMONDX_HOLLOWDIAMOND_SQUARE":    "BULLET_DIAMONDX_HOLLOWDIAMOND_SQUARE",
	"BULLET_DIAMOND_CIRCLE_SQUARE":            "BULLET_DIAMOND_CIRCLE_SQUARE",
}

// CreateBulletListInput represents the input for the create_bullet_list tool.
type CreateBulletListInput struct {
	PresentationID   string  `json:"presentation_id"`
	ObjectID         string  `json:"object_id"`
	ParagraphIndices []int   `json:"paragraph_indices,omitempty"` // Optional, all paragraphs if omitted
	BulletStyle      string  `json:"bullet_style"`                // DISC, CIRCLE, SQUARE, DIAMOND, ARROW, STAR, CHECKBOX or full preset name
	BulletColor      string  `json:"bullet_color,omitempty"`      // Hex color string (e.g., "#FF0000")
}

// CreateBulletListOutput represents the output of the create_bullet_list tool.
type CreateBulletListOutput struct {
	ObjectID       string `json:"object_id"`
	BulletPreset   string `json:"bullet_preset"`    // The actual preset applied
	ParagraphScope string `json:"paragraph_scope"`  // "ALL" or "INDICES [1, 2, 3]"
	BulletColor    string `json:"bullet_color,omitempty"` // The color applied, if any
}

// CreateBulletList converts text to a bullet list or adds bullets to existing text.
func (t *Tools) CreateBulletList(ctx context.Context, tokenSource oauth2.TokenSource, input CreateBulletListInput) (*CreateBulletListOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}
	if input.BulletStyle == "" {
		return nil, fmt.Errorf("%w: bullet_style is required", ErrInvalidBulletStyle)
	}

	// Normalize and validate bullet style
	bulletStyleUpper := strings.ToUpper(input.BulletStyle)
	bulletPreset, ok := validBulletStyles[bulletStyleUpper]
	if !ok {
		return nil, fmt.Errorf("%w: '%s' is not a valid bullet style; use DISC, CIRCLE, SQUARE, DIAMOND, ARROW, STAR, CHECKBOX, or a full preset name", ErrInvalidBulletStyle, input.BulletStyle)
	}

	// Validate paragraph indices
	for _, idx := range input.ParagraphIndices {
		if idx < 0 {
			return nil, fmt.Errorf("%w: paragraph indices cannot be negative", ErrInvalidParagraphIndex)
		}
	}

	t.config.Logger.Info("creating bullet list",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("bullet_style", bulletPreset),
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
			return nil, fmt.Errorf("%w: tables must have bullets applied cell by cell", ErrNotTextObject)
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
	requests := buildCreateBulletListRequests(input, bulletPreset, targetElement.Shape.Text)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrCreateBulletListFailed, err)
	}

	// Determine paragraph scope description
	paragraphScope := "ALL"
	if len(input.ParagraphIndices) > 0 {
		paragraphScope = fmt.Sprintf("INDICES %v", input.ParagraphIndices)
	}

	output := &CreateBulletListOutput{
		ObjectID:       input.ObjectID,
		BulletPreset:   bulletPreset,
		ParagraphScope: paragraphScope,
	}

	if input.BulletColor != "" {
		output.BulletColor = input.BulletColor
	}

	t.config.Logger.Info("bullet list created successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("bullet_preset", bulletPreset),
	)

	return output, nil
}

// buildCreateBulletListRequests creates the requests for creating bullet lists.
func buildCreateBulletListRequests(input CreateBulletListInput, bulletPreset string, text *slides.TextContent) []*slides.Request {
	var requests []*slides.Request

	// Build text range based on paragraph indices
	textRange := getBulletTextRange(text, input.ParagraphIndices)

	// Create the CreateParagraphBulletsRequest
	bulletRequest := &slides.Request{
		CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
			ObjectId:     input.ObjectID,
			TextRange:    textRange,
			BulletPreset: bulletPreset,
		},
	}
	requests = append(requests, bulletRequest)

	// If bullet color is specified, apply it using UpdateTextStyle
	// Note: Bullet text styles cannot be modified directly via Slides API.
	// However, using UpdateTextStyleRequest on a paragraph with a bullet
	// will also update the bullet glyph's text style.
	if input.BulletColor != "" {
		rgb := parseHexColor(input.BulletColor)
		if rgb != nil {
			colorRequest := &slides.Request{
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
			}
			requests = append(requests, colorRequest)
		}
	}

	return requests
}

// getBulletTextRange returns the text range for applying bullets.
func getBulletTextRange(text *slides.TextContent, paragraphIndices []int) *slides.Range {
	if len(paragraphIndices) == 0 {
		// Apply to all paragraphs
		return &slides.Range{
			Type: "ALL",
		}
	}

	// Find the range covering all specified paragraphs
	paragraphRanges := getParagraphRanges(text)
	if len(paragraphRanges) == 0 {
		return &slides.Range{Type: "ALL"}
	}

	// Find min and max indices to cover
	var minStart int64 = -1
	var maxEnd int64 = 0

	for _, idx := range paragraphIndices {
		if idx < len(paragraphRanges) {
			pr := paragraphRanges[idx]
			if minStart == -1 || pr.start < minStart {
				minStart = pr.start
			}
			if pr.end > maxEnd {
				maxEnd = pr.end
			}
		}
	}

	if minStart == -1 {
		return &slides.Range{Type: "ALL"}
	}

	return &slides.Range{
		Type:       "FIXED_RANGE",
		StartIndex: &minStart,
		EndIndex:   &maxEnd,
	}
}

// paragraphRange represents the start and end indices of a paragraph.
type paragraphRange struct {
	start int64
	end   int64
}

// getParagraphRanges returns the start and end indices for each paragraph.
func getParagraphRanges(text *slides.TextContent) []paragraphRange {
	if text == nil || len(text.TextElements) == 0 {
		return nil
	}

	var ranges []paragraphRange
	var currentStart int64 = 0

	for _, element := range text.TextElements {
		if element.ParagraphMarker != nil {
			ranges = append(ranges, paragraphRange{
				start: currentStart,
				end:   element.EndIndex,
			})
			currentStart = element.EndIndex
		}
	}

	return ranges
}
