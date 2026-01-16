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

// Sentinel errors for create_shape tool.
var (
	ErrCreateShapeFailed  = errors.New("failed to create shape")
	ErrInvalidShapeType   = errors.New("invalid shape type")
	ErrInvalidOutlineWeight = errors.New("outline weight must be positive")
)

// CreateShapeInput represents the input for the create_shape tool.
type CreateShapeInput struct {
	PresentationID string         `json:"presentation_id"`
	SlideIndex     int            `json:"slide_index,omitempty"` // 1-based index
	SlideID        string         `json:"slide_id,omitempty"`    // Alternative to slide_index
	ShapeType      string         `json:"shape_type"`            // RECTANGLE, ELLIPSE, etc.
	Position       *PositionInput `json:"position"`              // Position in points
	Size           *SizeInput     `json:"size"`                  // Size in points
	FillColor      string         `json:"fill_color,omitempty"`  // Hex color string (e.g., "#FF0000") or "transparent"
	OutlineColor   string         `json:"outline_color,omitempty"` // Hex color string or "transparent"
	OutlineWeight  *float64       `json:"outline_weight,omitempty"` // Weight in points
}

// CreateShapeOutput represents the output of the create_shape tool.
type CreateShapeOutput struct {
	ObjectID string `json:"object_id"`
}

// validShapeTypes contains the allowed shape types for the create_shape tool.
// These map to Google Slides API ShapeType enum.
var validShapeTypes = map[string]bool{
	// Basic shapes
	"RECTANGLE":       true,
	"ROUND_RECTANGLE": true,
	"ELLIPSE":         true,
	"TRIANGLE":        true,
	"DIAMOND":         true,
	"PENTAGON":        true,
	"HEXAGON":         true,
	"HEPTAGON":        true,
	"OCTAGON":         true,
	"DECAGON":         true,
	"DODECAGON":       true,
	"PARALLELOGRAM":   true,
	"TRAPEZOID":       true,

	// Star shapes
	"STAR_4":          true,
	"STAR_5":          true,
	"STAR_6":          true,
	"STAR_7":          true,
	"STAR_8":          true,
	"STAR_10":         true,
	"STAR_12":         true,
	"STAR_16":         true,
	"STAR_24":         true,
	"STAR_32":         true,

	// Arrow shapes
	"ARROW_RIGHT":          true,
	"ARROW_LEFT":           true,
	"ARROW_UP":             true,
	"ARROW_DOWN":           true,
	"ARROW_LEFT_RIGHT":     true,
	"ARROW_UP_DOWN":        true,
	"NOTCHED_RIGHT_ARROW":  true,
	"BENT_ARROW":           true,
	"U_TURN_ARROW":         true,
	"CURVED_RIGHT_ARROW":   true,
	"CURVED_LEFT_ARROW":    true,
	"CURVED_UP_ARROW":      true,
	"CURVED_DOWN_ARROW":    true,
	"STRIPED_RIGHT_ARROW":  true,
	"CHEVRON":              true,
	"HOME_PLATE":           true,

	// Callout shapes
	"RECTANGULAR_CALLOUT":        true,
	"ROUNDED_RECTANGULAR_CALLOUT": true,
	"ELLIPTICAL_CALLOUT":         true,
	"WEDGE_RECTANGLE_CALLOUT":    true,
	"WEDGE_ROUND_RECT_CALLOUT":   true,
	"WEDGE_ELLIPSE_CALLOUT":      true,
	"CLOUD_CALLOUT":              true,

	// Process shapes
	"QUAD_ARROW":            true,
	"LEFT_RIGHT_UP_ARROW":   true,
	"BENT_UP_ARROW":         true,
	"LEFT_UP_ARROW":         true,
	"CIRCULAR_ARROW":        true,

	// Flowchart shapes
	"FLOWCHART_PROCESS":            true,
	"FLOWCHART_DECISION":           true,
	"FLOWCHART_INPUT_OUTPUT":       true,
	"FLOWCHART_PREDEFINED_PROCESS": true,
	"FLOWCHART_INTERNAL_STORAGE":   true,
	"FLOWCHART_DOCUMENT":           true,
	"FLOWCHART_MULTIDOCUMENT":      true,
	"FLOWCHART_TERMINATOR":         true,
	"FLOWCHART_PREPARATION":        true,
	"FLOWCHART_MANUAL_INPUT":       true,
	"FLOWCHART_MANUAL_OPERATION":   true,
	"FLOWCHART_CONNECTOR":          true,
	"FLOWCHART_PUNCHED_CARD":       true,
	"FLOWCHART_PUNCHED_TAPE":       true,
	"FLOWCHART_SUMMING_JUNCTION":   true,
	"FLOWCHART_OR":                 true,
	"FLOWCHART_COLLATE":            true,
	"FLOWCHART_SORT":               true,
	"FLOWCHART_EXTRACT":            true,
	"FLOWCHART_MERGE":              true,
	"FLOWCHART_OFFLINE_STORAGE":    true,
	"FLOWCHART_ONLINE_STORAGE":     true,
	"FLOWCHART_MAGNETIC_TAPE":      true,
	"FLOWCHART_MAGNETIC_DISK":      true,
	"FLOWCHART_MAGNETIC_DRUM":      true,
	"FLOWCHART_DISPLAY":            true,
	"FLOWCHART_DELAY":              true,
	"FLOWCHART_ALTERNATE_PROCESS":  true,
	"FLOWCHART_DATA":               true,

	// Equation shapes
	"PLUS":      true,
	"MINUS":     true,
	"MULTIPLY":  true,
	"DIVIDE":    true,
	"EQUAL":     true,
	"NOT_EQUAL": true,

	// Block shapes
	"CUBE":             true,
	"CAN":              true,
	"BEVEL":            true,
	"FOLDED_CORNER":    true,
	"SMILEY_FACE":      true,
	"DONUT":            true,
	"NO_SMOKING":       true,
	"BLOCK_ARC":        true,
	"HEART":            true,
	"LIGHTNING_BOLT":   true,
	"SUN":              true,
	"MOON":             true,
	"CLOUD":            true,
	"ARC":              true,
	"PLAQUE":           true,
	"FRAME":            true,
	"HALF_FRAME":       true,
	"CORNER":           true,
	"DIAGONAL_STRIPE":  true,
	"CHORD":            true,
	"PIE":              true,
	"L_SHAPE":          true,
	"CORNER_RIBBON":    true,
	"RIBBON":           true,
	"RIBBON_2":         true,
	"WAVE":             true,
	"DOUBLE_WAVE":      true,
	"CROSS":            true,
	"IRREGULAR_SEAL_1": true,
	"IRREGULAR_SEAL_2": true,
	"TEARDROP":         true,
	"SNIP_1_RECTANGLE":      true,
	"SNIP_2_SAME_RECTANGLE": true,
	"SNIP_2_DIAGONAL_RECTANGLE": true,
	"SNIP_ROUND_RECTANGLE":   true,
	"ROUND_1_RECTANGLE":      true,
	"ROUND_2_SAME_RECTANGLE": true,
	"ROUND_2_DIAGONAL_RECTANGLE": true,

	// Bracket shapes
	"LEFT_BRACKET":         true,
	"RIGHT_BRACKET":        true,
	"LEFT_BRACE":           true,
	"RIGHT_BRACE":          true,
	"LEFT_RIGHT_BRACKET":   true,
	"BRACKET_PAIR":         true,
	"BRACE_PAIR":           true,
}

// shapeTimeNowFunc allows overriding the time function for tests.
var shapeTimeNowFunc = time.Now

// generateShapeObjectID generates a unique object ID for a new shape element.
func generateShapeObjectID() string {
	return fmt.Sprintf("shape_%d", shapeTimeNowFunc().UnixNano())
}

// CreateShape creates a new shape on a slide.
func (t *Tools) CreateShape(ctx context.Context, tokenSource oauth2.TokenSource, input CreateShapeInput) (*CreateShapeOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	// Normalize and validate shape type
	shapeType := strings.ToUpper(strings.TrimSpace(input.ShapeType))
	if shapeType == "" {
		return nil, fmt.Errorf("%w: shape_type is required", ErrInvalidShapeType)
	}
	if !validShapeTypes[shapeType] {
		return nil, fmt.Errorf("%w: '%s' is not a valid shape type", ErrInvalidShapeType, input.ShapeType)
	}

	// Validate size
	if input.Size == nil || input.Size.Width <= 0 || input.Size.Height <= 0 {
		return nil, ErrInvalidSize
	}

	// Validate position - default to (0, 0) if not provided
	if input.Position == nil {
		input.Position = &PositionInput{X: 0, Y: 0}
	}

	// Validate outline weight if provided
	if input.OutlineWeight != nil && *input.OutlineWeight <= 0 {
		return nil, ErrInvalidOutlineWeight
	}

	t.config.Logger.Info("creating shape on slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.String("shape_type", shapeType),
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

	// Generate a unique object ID for the shape
	objectID := generateShapeObjectID()

	// Build the requests for creating the shape
	requests := buildCreateShapeRequests(objectID, slideID, shapeType, input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrCreateShapeFailed, err)
	}

	output := &CreateShapeOutput{
		ObjectID: objectID,
	}

	t.config.Logger.Info("shape created successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.String("shape_type", shapeType),
	)

	return output, nil
}

// buildCreateShapeRequests creates the batch update requests to create a shape.
func buildCreateShapeRequests(objectID, slideID, shapeType string, input CreateShapeInput) []*slides.Request {
	requests := []*slides.Request{}

	// Create the shape
	createShapeRequest := &slides.Request{
		CreateShape: &slides.CreateShapeRequest{
			ObjectId:  objectID,
			ShapeType: shapeType,
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

	// Build shape properties update request if fill or outline is specified
	shapePropertiesRequest := buildShapePropertiesRequest(objectID, input)
	if shapePropertiesRequest != nil {
		requests = append(requests, shapePropertiesRequest)
	}

	return requests
}

// buildShapePropertiesRequest creates a request to update shape properties (fill, outline).
func buildShapePropertiesRequest(objectID string, input CreateShapeInput) *slides.Request {
	shapeProperties := &slides.ShapeProperties{}
	var fields []string

	// Handle fill color
	if input.FillColor != "" {
		fillColor := strings.ToLower(strings.TrimSpace(input.FillColor))
		if fillColor == "transparent" {
			// Set transparent fill
			shapeProperties.ShapeBackgroundFill = &slides.ShapeBackgroundFill{
				PropertyState: "NOT_RENDERED",
			}
			fields = append(fields, "shapeBackgroundFill.propertyState")
		} else {
			rgb := parseHexColor(input.FillColor)
			if rgb != nil {
				shapeProperties.ShapeBackgroundFill = &slides.ShapeBackgroundFill{
					PropertyState: "RENDERED",
					SolidFill: &slides.SolidFill{
						Color: &slides.OpaqueColor{
							RgbColor: rgb,
						},
					},
				}
				fields = append(fields, "shapeBackgroundFill")
			}
		}
	}

	// Handle outline
	if input.OutlineColor != "" || input.OutlineWeight != nil {
		outline := &slides.Outline{}
		hasOutline := false

		// Handle outline color
		if input.OutlineColor != "" {
			outlineColor := strings.ToLower(strings.TrimSpace(input.OutlineColor))
			if outlineColor == "transparent" {
				outline.PropertyState = "NOT_RENDERED"
				fields = append(fields, "outline.propertyState")
				hasOutline = true
			} else {
				rgb := parseHexColor(input.OutlineColor)
				if rgb != nil {
					outline.PropertyState = "RENDERED"
					outline.OutlineFill = &slides.OutlineFill{
						SolidFill: &slides.SolidFill{
							Color: &slides.OpaqueColor{
								RgbColor: rgb,
							},
						},
					}
					fields = append(fields, "outline.outlineFill.solidFill.color")
					hasOutline = true
				}
			}
		}

		// Handle outline weight
		if input.OutlineWeight != nil {
			outline.Weight = &slides.Dimension{
				Magnitude: *input.OutlineWeight,
				Unit:      "PT",
			}
			fields = append(fields, "outline.weight")
			hasOutline = true
		}

		if hasOutline {
			shapeProperties.Outline = outline
		}
	}

	if len(fields) == 0 {
		return nil
	}

	return &slides.Request{
		UpdateShapeProperties: &slides.UpdateShapePropertiesRequest{
			ObjectId:        objectID,
			ShapeProperties: shapeProperties,
			Fields:          strings.Join(fields, ","),
		},
	}
}
