package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for create_line tool.
var (
	ErrCreateLineFailed = errors.New("failed to create line")
	ErrInvalidPoints    = errors.New("start_point and end_point are required")
)

// CreateLineInput represents the input for the create_line tool.
type CreateLineInput struct {
	PresentationID string  `json:"presentation_id"`
	SlideIndex     int     `json:"slide_index,omitempty"` // 1-based index
	SlideID        string  `json:"slide_id,omitempty"`    // Alternative to slide_index
	StartPoint     *Point  `json:"start_point"`
	EndPoint       *Point  `json:"end_point"`
	LineType       string  `json:"line_type,omitempty"` // STRAIGHT, CURVED, ELBOW
	StartArrow     string  `json:"start_arrow,omitempty"`
	EndArrow       string  `json:"end_arrow,omitempty"`
	LineColor      string  `json:"line_color,omitempty"`
	LineWeight     float64 `json:"line_weight,omitempty"` // in points
	LineDash       string  `json:"line_dash,omitempty"`
}

// Point represents x, y coordinates in points.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CreateLineOutput represents the output of the create_line tool.
type CreateLineOutput struct {
	ObjectID string `json:"object_id"`
}

// CreateLine creates a new line or arrow on a slide.
func (t *Tools) CreateLine(ctx context.Context, tokenSource oauth2.TokenSource, input CreateLineInput) (*CreateLineOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	if input.StartPoint == nil || input.EndPoint == nil {
		return nil, ErrInvalidPoints
	}

	// Default line type
	if input.LineType == "" {
		input.LineType = "STRAIGHT"
	}

	t.config.Logger.Info("creating line on slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.String("line_type", input.LineType),
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

	// Generate a unique object ID
	objectID := generateObjectID()

	// Build the requests
	requests := buildCreateLineRequests(objectID, slideID, input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrCreateLineFailed, err)
	}

	output := &CreateLineOutput{
		ObjectID: objectID,
	}

	t.config.Logger.Info("line created successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
	)

	return output, nil
}

// buildCreateLineRequests creates the batch update requests to create a line.
func buildCreateLineRequests(objectID, slideID string, input CreateLineInput) []*slides.Request {
	requests := []*slides.Request{}

	// Calculate transform and size
	x1, y1 := input.StartPoint.X, input.StartPoint.Y
	x2, y2 := input.EndPoint.X, input.EndPoint.Y

	width := math.Abs(x2 - x1)
	height := math.Abs(y2 - y1)

	scaleX := 1.0
	if x2 < x1 {
		scaleX = -1.0
	}

	scaleY := 1.0
	if y2 < y1 {
		scaleY = -1.0
	}

	// Map line type
	category := "STRAIGHT"
	switch strings.ToUpper(input.LineType) {
	case "CURVED":
		category = "CURVED"
	case "ELBOW", "BENT":
		category = "BENT"
	}

	// Create the line
	createLineRequest := &slides.Request{
		CreateLine: &slides.CreateLineRequest{
			ObjectId: objectID,
			Category: category,
			ElementProperties: &slides.PageElementProperties{
				PageObjectId: slideID,
				Size: &slides.Size{
					Width: &slides.Dimension{
						Magnitude: pointsToEMU(width),
						Unit:      "EMU",
					},
					Height: &slides.Dimension{
						Magnitude: pointsToEMU(height),
						Unit:      "EMU",
					},
				},
				Transform: &slides.AffineTransform{
					ScaleX:     scaleX,
					ScaleY:     scaleY,
					TranslateX: pointsToEMU(x1),
					TranslateY: pointsToEMU(y1),
					Unit:       "EMU",
				},
			},
		},
	}
	requests = append(requests, createLineRequest)

	// Update line properties (styling)
	updateReq := buildUpdateLinePropertiesRequest(objectID, input)
	if updateReq != nil {
		requests = append(requests, updateReq)
	}

	return requests
}

func buildUpdateLinePropertiesRequest(objectID string, input CreateLineInput) *slides.Request {
	lineProps := &slides.LineProperties{}
	var fields []string

	// Color
	if input.LineColor != "" {
		rgb := parseHexColor(input.LineColor)
		if rgb != nil {
			lineProps.LineFill = &slides.LineFill{
				SolidFill: &slides.SolidFill{
					Color: &slides.OpaqueColor{
						RgbColor: rgb,
					},
				},
			}
			fields = append(fields, "lineFill.solidFill.color")
		}
	}

	// Weight
	if input.LineWeight > 0 {
		lineProps.Weight = &slides.Dimension{
			Magnitude: input.LineWeight,
			Unit:      "PT",
		}
		fields = append(fields, "weight")
	}

	// Dash style
	if input.LineDash != "" {
		dash := "SOLID"
		switch strings.ToUpper(input.LineDash) {
		case "DASH":
			dash = "DASH"
		case "DOT":
			dash = "DOT"
		case "DASH_DOT":
			dash = "DASH_DOT"
		case "LONG_DASH":
			dash = "LONG_DASH"
		case "LONG_DASH_DOT":
			dash = "LONG_DASH_DOT"
		}
		lineProps.DashStyle = dash
		fields = append(fields, "dashStyle")
	}

	// Arrows
	if input.StartArrow != "" {
		arrow := mapArrowStyle(input.StartArrow)
		if arrow != "" {
			lineProps.StartArrow = arrow
			fields = append(fields, "startArrow")
		}
	}

	if input.EndArrow != "" {
		arrow := mapArrowStyle(input.EndArrow)
		if arrow != "" {
			lineProps.EndArrow = arrow
			fields = append(fields, "endArrow")
		}
	}

	if len(fields) == 0 {
		return nil
	}

	return &slides.Request{
		UpdateLineProperties: &slides.UpdateLinePropertiesRequest{
			ObjectId:       objectID,
			LineProperties: lineProps,
			Fields:         strings.Join(fields, ","),
		},
	}
}

func mapArrowStyle(style string) string {
	switch strings.ToUpper(style) {
	case "ARROW", "FILL_ARROW":
		return "FILL_ARROW"
	case "DIAMOND", "FILL_DIAMOND":
		return "FILL_DIAMOND"
	case "OVAL", "CIRCLE", "FILL_CIRCLE":
		return "FILL_CIRCLE"
	case "OPEN_ARROW":
		return "OPEN_ARROW"
	case "OPEN_CIRCLE":
		return "OPEN_CIRCLE"
	case "OPEN_DIAMOND":
		return "OPEN_DIAMOND"
	case "STEALTH_ARROW":
		return "STEALTH_ARROW"
	case "NONE":
		return "NONE"
	}
	// Default fallthrough - if user provided valid API name or unknown
	// check if it matches a valid API name roughly or just return it/empty
	return ""
}
