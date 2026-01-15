package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for add_text_box tool.
var (
	ErrAddTextBoxFailed = errors.New("failed to add text box")
	ErrInvalidText      = errors.New("text content is required")
	ErrInvalidSize      = errors.New("size (width and height) is required")
)

// AddTextBoxInput represents the input for the add_text_box tool.
type AddTextBoxInput struct {
	PresentationID string          `json:"presentation_id"`
	SlideIndex     int             `json:"slide_index,omitempty"` // 1-based index
	SlideID        string          `json:"slide_id,omitempty"`    // Alternative to slide_index
	Text           string          `json:"text"`
	Position       *PositionInput  `json:"position"` // Position in points
	Size           *SizeInput      `json:"size"`     // Size in points
	Style          *TextStyleInput `json:"style,omitempty"`
}

// PositionInput represents x, y coordinates in points.
type PositionInput struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SizeInput represents width and height in points.
type SizeInput struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// TextStyleInput represents optional text styling.
type TextStyleInput struct {
	FontFamily string `json:"font_family,omitempty"`
	FontSize   int    `json:"font_size,omitempty"` // In points
	Bold       bool   `json:"bold,omitempty"`
	Italic     bool   `json:"italic,omitempty"`
	Color      string `json:"color,omitempty"` // Hex color string (e.g., "#FF0000")
}

// AddTextBoxOutput represents the output of the add_text_box tool.
type AddTextBoxOutput struct {
	ObjectID string `json:"object_id"`
}

// AddTextBox adds a new text box to a slide.
func (t *Tools) AddTextBox(ctx context.Context, tokenSource oauth2.TokenSource, input AddTextBoxInput) (*AddTextBoxOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	if input.Text == "" {
		return nil, ErrInvalidText
	}

	if input.Position == nil {
		input.Position = &PositionInput{X: 0, Y: 0}
	}

	if input.Size == nil || input.Size.Width <= 0 || input.Size.Height <= 0 {
		return nil, ErrInvalidSize
	}

	t.config.Logger.Info("adding text box to slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.Int("text_length", len(input.Text)),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find the target slide
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

	// Find the target slide
	slideID, _, err := findSlide(presentation, input.SlideIndex, input.SlideID)
	if err != nil {
		return nil, err
	}

	// Generate a unique object ID for the text box
	objectID := generateObjectID()

	// Build the requests for creating the text box
	requests := buildTextBoxRequests(objectID, slideID, input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrAddTextBoxFailed, err)
	}

	output := &AddTextBoxOutput{
		ObjectID: objectID,
	}

	t.config.Logger.Info("text box added successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
	)

	return output, nil
}

// findSlide locates a slide by index or ID and returns the slide ID and index.
func findSlide(presentation *slides.Presentation, slideIndex int, slideID string) (string, int, error) {
	if slideID != "" {
		// Find by slide_id
		for i, slide := range presentation.Slides {
			if slide.ObjectId == slideID {
				return slideID, i + 1, nil
			}
		}
		return "", 0, fmt.Errorf("%w: slide_id '%s' not found", ErrSlideNotFound, slideID)
	}

	// Find by slide_index (1-based)
	if slideIndex < 1 || slideIndex > len(presentation.Slides) {
		return "", 0, fmt.Errorf("%w: slide index %d out of range (1-%d)", ErrSlideNotFound, slideIndex, len(presentation.Slides))
	}
	return presentation.Slides[slideIndex-1].ObjectId, slideIndex, nil
}

// timeNowFunc allows overriding the time function for tests.
var timeNowFunc = time.Now

// generateObjectID generates a unique object ID for a new element.
// Google Slides accepts client-generated IDs that start with a letter.
func generateObjectID() string {
	// Use a simple timestamp-based format with a prefix
	return fmt.Sprintf("textbox_%d", timeNowFunc().UnixNano())
}

// pointsToEMU converts points to EMU (English Metric Units).
// 1 point = 12700 EMU
const pointsPerEMU = 12700.0

func pointsToEMU(points float64) float64 {
	return points * pointsPerEMU
}

// buildTextBoxRequests creates the batch update requests to add a text box.
func buildTextBoxRequests(objectID, slideID string, input AddTextBoxInput) []*slides.Request {
	requests := []*slides.Request{}

	// Create the shape (text box)
	createShapeRequest := &slides.Request{
		CreateShape: &slides.CreateShapeRequest{
			ObjectId:  objectID,
			ShapeType: "TEXT_BOX",
			ElementProperties: &slides.PageElementProperties{
				PageObjectId: slideID,
				Size: &slides.Size{
					Width: &slides.Dimension{
						Magnitude: pointsToEMU(input.Size.Width),
						Unit:      "EMU",
					},
					Height: &slides.Dimension{
						Magnitude: pointsToEMU(input.Size.Height),
						Unit:      "EMU",
					},
				},
				Transform: &slides.AffineTransform{
					ScaleX:     1,
					ScaleY:     1,
					TranslateX: pointsToEMU(input.Position.X),
					TranslateY: pointsToEMU(input.Position.Y),
					Unit:       "EMU",
				},
			},
		},
	}
	requests = append(requests, createShapeRequest)

	// Insert text into the shape
	insertTextRequest := &slides.Request{
		InsertText: &slides.InsertTextRequest{
			ObjectId:       objectID,
			InsertionIndex: 0,
			Text:           input.Text,
		},
	}
	requests = append(requests, insertTextRequest)

	// Apply text style if provided
	if input.Style != nil {
		styleRequest := buildTextStyleRequest(objectID, input.Style)
		if styleRequest != nil {
			requests = append(requests, styleRequest)
		}
	}

	return requests
}

// buildTextStyleRequest creates a request to update text style.
func buildTextStyleRequest(objectID string, style *TextStyleInput) *slides.Request {
	if style == nil {
		return nil
	}

	textStyle := &slides.TextStyle{}
	var fields []string

	if style.FontFamily != "" {
		textStyle.FontFamily = style.FontFamily
		fields = append(fields, "fontFamily")
	}

	if style.FontSize > 0 {
		textStyle.FontSize = &slides.Dimension{
			Magnitude: float64(style.FontSize),
			Unit:      "PT",
		}
		fields = append(fields, "fontSize")
	}

	if style.Bold {
		textStyle.Bold = true
		fields = append(fields, "bold")
	}

	if style.Italic {
		textStyle.Italic = true
		fields = append(fields, "italic")
	}

	if style.Color != "" {
		rgb := parseHexColor(style.Color)
		if rgb != nil {
			textStyle.ForegroundColor = &slides.OptionalColor{
				OpaqueColor: &slides.OpaqueColor{
					RgbColor: rgb,
				},
			}
			fields = append(fields, "foregroundColor")
		}
	}

	if len(fields) == 0 {
		return nil
	}

	return &slides.Request{
		UpdateTextStyle: &slides.UpdateTextStyleRequest{
			ObjectId: objectID,
			Style:    textStyle,
			TextRange: &slides.Range{
				Type: "ALL",
			},
			Fields: strings.Join(fields, ","),
		},
	}
}

// parseHexColor parses a hex color string (e.g., "#FF0000") into RGB components.
func parseHexColor(hex string) *slides.RgbColor {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return nil
	}

	r, g, b := 0, 0, 0
	_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	if err != nil {
		return nil
	}

	return &slides.RgbColor{
		Red:   float64(r) / 255.0,
		Green: float64(g) / 255.0,
		Blue:  float64(b) / 255.0,
	}
}
