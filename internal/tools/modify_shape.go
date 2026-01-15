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

// Sentinel errors for modify_shape tool.
var (
	ErrModifyShapeFailed = errors.New("failed to modify shape")
	ErrNoProperties      = errors.New("no properties to update")
)

// ModifyShapeInput represents the input for the modify_shape tool.
type ModifyShapeInput struct {
	PresentationID string           `json:"presentation_id"`
	ObjectID       string           `json:"object_id"`
	Properties     *ShapeProperties `json:"properties"`
}

// ShapeProperties defines properties to update.
type ShapeProperties struct {
	FillColor     string  `json:"fill_color,omitempty"`    // Hex string or "transparent"
	OutlineColor  string  `json:"outline_color,omitempty"` // Hex string or "transparent"
	OutlineWeight *float64 `json:"outline_weight,omitempty"` // In points
	OutlineDash   string  `json:"outline_dash,omitempty"`   // Enum: SOLID, DASH, DOT, DASH_DOT
	Shadow        *bool   `json:"shadow,omitempty"`         // Enable/disable shadow
}

// ModifyShapeOutput represents the output of the modify_shape tool.
type ModifyShapeOutput struct {
	ObjectID          string   `json:"object_id"`
	UpdatedProperties []string `json:"updated_properties"`
}

// ModifyShape modifies the properties of a shape.
func (t *Tools) ModifyShape(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyShapeInput) (*ModifyShapeOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}
	if input.Properties == nil {
		return nil, ErrNoProperties
	}

	t.config.Logger.Info("modifying shape",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Verify object exists (and potentially check type, though API handles mismatch usually)
	// For performance, we skip full verification unless needed, but let's check basic existence if we want to return specific errors
	// Assuming batch update will fail if object not found or incompatible.

	requests := buildModifyShapeRequests(input.ObjectID, input.Properties)
	if len(requests) == 0 {
		return nil, ErrNoProperties
	}

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound // Or object not found
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrModifyShapeFailed, err)
	}

	// Collect updated property names for output
	var updatedProps []string
	if input.Properties.FillColor != "" {
		updatedProps = append(updatedProps, "fill_color")
	}
	if input.Properties.OutlineColor != "" {
		updatedProps = append(updatedProps, "outline_color")
	}
	if input.Properties.OutlineWeight != nil {
		updatedProps = append(updatedProps, "outline_weight")
	}
	if input.Properties.OutlineDash != "" {
		updatedProps = append(updatedProps, "outline_dash")
	}
	if input.Properties.Shadow != nil {
		updatedProps = append(updatedProps, "shadow")
	}

	output := &ModifyShapeOutput{
		ObjectID:          input.ObjectID,
		UpdatedProperties: updatedProps,
	}

	t.config.Logger.Info("shape modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Int("updates_count", len(updatedProps)),
	)

	return output, nil
}

func buildModifyShapeRequests(objectID string, props *ShapeProperties) []*slides.Request {
	var requests []*slides.Request

	// updateShapePropertiesRequest is used for Fill, Outline, Shadow, Reflection
	shapeProps := &slides.ShapeProperties{}
	var fields []string

	// Fill
	if props.FillColor != "" {
		if strings.ToLower(props.FillColor) == "transparent" {
			shapeProps.ShapeBackgroundFill = &slides.ShapeBackgroundFill{
				PropertyState: "NOT_RENDERED",
			}
		} else {
			rgb := parseHexColor(props.FillColor)
			if rgb != nil {
				shapeProps.ShapeBackgroundFill = &slides.ShapeBackgroundFill{
					SolidFill: &slides.SolidFill{
						Color: &slides.OpaqueColor{
							RgbColor: rgb,
						},
					},
				}
			}
		}
		if shapeProps.ShapeBackgroundFill != nil {
			fields = append(fields, "shapeBackgroundFill")
		}
	}

	// Outline
	if props.OutlineColor != "" || props.OutlineWeight != nil || props.OutlineDash != "" {
		shapeProps.Outline = &slides.Outline{}
		
		if props.OutlineColor != "" {
			if strings.ToLower(props.OutlineColor) == "transparent" {
				shapeProps.Outline.PropertyState = "NOT_RENDERED"
				fields = append(fields, "outline.propertyState")
			} else {
				rgb := parseHexColor(props.OutlineColor)
				if rgb != nil {
					shapeProps.Outline.OutlineFill = &slides.OutlineFill{
						SolidFill: &slides.SolidFill{
							Color: &slides.OpaqueColor{
								RgbColor: rgb,
							},
						},
					}
					fields = append(fields, "outline.outlineFill")
				}
			}
		}

		if props.OutlineWeight != nil {
			shapeProps.Outline.Weight = &slides.Dimension{
				Magnitude: *props.OutlineWeight,
				Unit:      "PT",
			}
			fields = append(fields, "outline.weight")
		}

		if props.OutlineDash != "" {
			shapeProps.Outline.DashStyle = strings.ToUpper(props.OutlineDash)
			fields = append(fields, "outline.dashStyle")
		}
	}

	// Shadow
	if props.Shadow != nil {
		shapeProps.Shadow = &slides.Shadow{}
		if *props.Shadow {
			// Enable shadow (default assumption if just true)
			// Typically we might want to set a type or ensure it's rendered. 
			// Setting PropertyState to RENDERED might be default but explicit is good?
			// Google Slides API default shadow logic: usually implies setting a type or visible.
			// Let's assume generic OUTER shadow if enabling.
			shapeProps.Shadow.Type = "OUTER"
		} else {
			shapeProps.Shadow.PropertyState = "NOT_RENDERED"
		}
		fields = append(fields, "shadow")
	}

	if len(fields) > 0 {
		requests = append(requests, &slides.Request{
			UpdateShapeProperties: &slides.UpdateShapePropertiesRequest{
				ObjectId:        objectID,
				ShapeProperties: shapeProps,
				Fields:          strings.Join(fields, ","),
			},
		})
	}

	return requests
}
