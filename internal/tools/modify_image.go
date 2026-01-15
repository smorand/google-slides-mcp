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

// Sentinel errors for modify_image tool.
var (
	ErrModifyImageFailed      = errors.New("failed to modify image")
	ErrNotImageObject         = errors.New("object is not an image")
	ErrNoImageProperties      = errors.New("no image properties to modify")
	ErrInvalidCropValue       = errors.New("crop values must be between 0 and 1")
	ErrInvalidBrightnessValue = errors.New("brightness must be between -1 and 1")
	ErrInvalidContrastValue   = errors.New("contrast must be between -1 and 1")
	ErrInvalidTransparency    = errors.New("transparency must be between 0 and 1")
)

// ModifyImageInput represents the input for the modify_image tool.
type ModifyImageInput struct {
	PresentationID string                `json:"presentation_id"`
	ObjectID       string                `json:"object_id"`
	Properties     *ImageModifyProperties `json:"properties"`
}

// ImageModifyProperties represents the image properties to modify.
type ImageModifyProperties struct {
	Position     *PositionInput     `json:"position,omitempty"`     // Position in points
	Size         *SizeInput         `json:"size,omitempty"`         // Size in points
	Crop         *CropInput         `json:"crop,omitempty"`         // Crop percentages (0-1)
	Brightness   *float64           `json:"brightness,omitempty"`   // -1 to 1
	Contrast     *float64           `json:"contrast,omitempty"`     // -1 to 1
	Transparency *float64           `json:"transparency,omitempty"` // 0 to 1
	Recolor      *string            `json:"recolor,omitempty"`      // Preset name or "none" to remove
}

// CropInput represents crop values for an image.
type CropInput struct {
	Top    *float64 `json:"top,omitempty"`    // 0-1 percentage from top
	Bottom *float64 `json:"bottom,omitempty"` // 0-1 percentage from bottom
	Left   *float64 `json:"left,omitempty"`   // 0-1 percentage from left
	Right  *float64 `json:"right,omitempty"`  // 0-1 percentage from right
}

// ModifyImageOutput represents the output of the modify_image tool.
type ModifyImageOutput struct {
	ObjectID          string   `json:"object_id"`
	ModifiedProperties []string `json:"modified_properties"`
}

// ModifyImage modifies properties of an existing image.
func (t *Tools) ModifyImage(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyImageInput) (*ModifyImageOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}
	if input.Properties == nil {
		return nil, ErrNoImageProperties
	}

	// Validate properties values
	if err := validateImageProperties(input.Properties); err != nil {
		return nil, err
	}

	// Check if any properties are actually provided
	if !hasImagePropertiesToModify(input.Properties) {
		return nil, ErrNoImageProperties
	}

	t.config.Logger.Info("modifying image properties",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation
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

	// Find the image object
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

	// Verify it's an image
	if targetElement.Image == nil {
		return nil, fmt.Errorf("%w: object '%s' is not an image (type: %s)", ErrNotImageObject, input.ObjectID, determineObjectType(targetElement))
	}

	// Build requests and track modified properties
	requests, modifiedProps := buildModifyImageRequests(input.ObjectID, input.Properties, targetElement)

	if len(requests) == 0 {
		return nil, ErrNoImageProperties
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
		return nil, fmt.Errorf("%w: %v", ErrModifyImageFailed, err)
	}

	output := &ModifyImageOutput{
		ObjectID:          input.ObjectID,
		ModifiedProperties: modifiedProps,
	}

	t.config.Logger.Info("image modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Int("properties_modified", len(modifiedProps)),
	)

	return output, nil
}

// validateImageProperties validates the input property values.
func validateImageProperties(props *ImageModifyProperties) error {
	if props.Crop != nil {
		if err := validateCropValues(props.Crop); err != nil {
			return err
		}
	}

	if props.Brightness != nil {
		if *props.Brightness < -1 || *props.Brightness > 1 {
			return ErrInvalidBrightnessValue
		}
	}

	if props.Contrast != nil {
		if *props.Contrast < -1 || *props.Contrast > 1 {
			return ErrInvalidContrastValue
		}
	}

	if props.Transparency != nil {
		if *props.Transparency < 0 || *props.Transparency > 1 {
			return ErrInvalidTransparency
		}
	}

	if props.Size != nil {
		if props.Size.Width <= 0 && props.Size.Height <= 0 {
			return ErrInvalidImageSize
		}
	}

	if props.Position != nil {
		if props.Position.X < 0 || props.Position.Y < 0 {
			return ErrInvalidImagePosition
		}
	}

	return nil
}

// validateCropValues validates crop input values.
func validateCropValues(crop *CropInput) error {
	if crop.Top != nil && (*crop.Top < 0 || *crop.Top > 1) {
		return fmt.Errorf("%w: top crop value %f is invalid", ErrInvalidCropValue, *crop.Top)
	}
	if crop.Bottom != nil && (*crop.Bottom < 0 || *crop.Bottom > 1) {
		return fmt.Errorf("%w: bottom crop value %f is invalid", ErrInvalidCropValue, *crop.Bottom)
	}
	if crop.Left != nil && (*crop.Left < 0 || *crop.Left > 1) {
		return fmt.Errorf("%w: left crop value %f is invalid", ErrInvalidCropValue, *crop.Left)
	}
	if crop.Right != nil && (*crop.Right < 0 || *crop.Right > 1) {
		return fmt.Errorf("%w: right crop value %f is invalid", ErrInvalidCropValue, *crop.Right)
	}
	return nil
}

// hasImagePropertiesToModify checks if any image properties are set.
func hasImagePropertiesToModify(props *ImageModifyProperties) bool {
	if props == nil {
		return false
	}
	return props.Position != nil ||
		props.Size != nil ||
		props.Crop != nil ||
		props.Brightness != nil ||
		props.Contrast != nil ||
		props.Transparency != nil ||
		props.Recolor != nil
}

// buildModifyImageRequests creates batch update requests for image modifications.
func buildModifyImageRequests(objectID string, props *ImageModifyProperties, element *slides.PageElement) ([]*slides.Request, []string) {
	var requests []*slides.Request
	var modifiedProps []string

	// Handle position and/or size changes via UpdatePageElementTransformRequest
	if props.Position != nil || props.Size != nil {
		transformReq := buildImageTransformRequest(objectID, props, element)
		if transformReq != nil {
			requests = append(requests, transformReq)
			if props.Position != nil {
				modifiedProps = append(modifiedProps, "position")
			}
			if props.Size != nil {
				modifiedProps = append(modifiedProps, "size")
			}
		}
	}

	// Handle image properties changes via UpdateImagePropertiesRequest
	imagePropsReq, imageModifiedFields := buildImagePropertiesRequest(objectID, props)
	if imagePropsReq != nil {
		requests = append(requests, imagePropsReq)
		modifiedProps = append(modifiedProps, imageModifiedFields...)
	}

	return requests, modifiedProps
}

// buildImageTransformRequest creates a request to update position and/or size.
func buildImageTransformRequest(objectID string, props *ImageModifyProperties, element *slides.PageElement) *slides.Request {
	// For position and size, we need to use ABSOLUTE mode to set exact values
	transform := &slides.AffineTransform{
		Unit: "EMU",
	}

	// Get current values from element
	currentScaleX := 1.0
	currentScaleY := 1.0
	currentTranslateX := 0.0
	currentTranslateY := 0.0

	if element.Transform != nil {
		currentScaleX = element.Transform.ScaleX
		currentScaleY = element.Transform.ScaleY
		currentTranslateX = element.Transform.TranslateX
		currentTranslateY = element.Transform.TranslateY
	}

	// Apply position changes
	if props.Position != nil {
		transform.TranslateX = pointsToEMU(props.Position.X)
		transform.TranslateY = pointsToEMU(props.Position.Y)
	} else {
		transform.TranslateX = currentTranslateX
		transform.TranslateY = currentTranslateY
	}

	// Apply size changes by modifying scale
	if props.Size != nil && element.Size != nil {
		// Calculate new scale based on desired size and original element size
		if element.Size.Width != nil && element.Size.Width.Magnitude > 0 {
			originalWidth := element.Size.Width.Magnitude / currentScaleX
			if props.Size.Width > 0 {
				transform.ScaleX = pointsToEMU(props.Size.Width) / originalWidth
			} else {
				transform.ScaleX = currentScaleX
			}
		} else {
			transform.ScaleX = currentScaleX
		}

		if element.Size.Height != nil && element.Size.Height.Magnitude > 0 {
			originalHeight := element.Size.Height.Magnitude / currentScaleY
			if props.Size.Height > 0 {
				transform.ScaleY = pointsToEMU(props.Size.Height) / originalHeight
			} else {
				transform.ScaleY = currentScaleY
			}
		} else {
			transform.ScaleY = currentScaleY
		}
	} else {
		transform.ScaleX = currentScaleX
		transform.ScaleY = currentScaleY
	}

	return &slides.Request{
		UpdatePageElementTransform: &slides.UpdatePageElementTransformRequest{
			ObjectId:  objectID,
			ApplyMode: "ABSOLUTE",
			Transform: transform,
		},
	}
}

// buildImagePropertiesRequest creates a request to update image properties.
func buildImagePropertiesRequest(objectID string, props *ImageModifyProperties) (*slides.Request, []string) {
	imageProps := &slides.ImageProperties{}
	var fields []string
	var modifiedProps []string

	// Handle crop properties
	if props.Crop != nil {
		cropProps := &slides.CropProperties{}
		hasCrop := false

		if props.Crop.Top != nil {
			cropProps.TopOffset = *props.Crop.Top
			fields = append(fields, "cropProperties.topOffset")
			hasCrop = true
		}
		if props.Crop.Bottom != nil {
			cropProps.BottomOffset = *props.Crop.Bottom
			fields = append(fields, "cropProperties.bottomOffset")
			hasCrop = true
		}
		if props.Crop.Left != nil {
			cropProps.LeftOffset = *props.Crop.Left
			fields = append(fields, "cropProperties.leftOffset")
			hasCrop = true
		}
		if props.Crop.Right != nil {
			cropProps.RightOffset = *props.Crop.Right
			fields = append(fields, "cropProperties.rightOffset")
			hasCrop = true
		}

		if hasCrop {
			imageProps.CropProperties = cropProps
			modifiedProps = append(modifiedProps, "crop")
		}
	}

	// Handle brightness
	if props.Brightness != nil {
		imageProps.Brightness = *props.Brightness
		fields = append(fields, "brightness")
		modifiedProps = append(modifiedProps, "brightness")
	}

	// Handle contrast
	if props.Contrast != nil {
		imageProps.Contrast = *props.Contrast
		fields = append(fields, "contrast")
		modifiedProps = append(modifiedProps, "contrast")
	}

	// Handle transparency
	if props.Transparency != nil {
		imageProps.Transparency = *props.Transparency
		fields = append(fields, "transparency")
		modifiedProps = append(modifiedProps, "transparency")
	}

	// Handle recolor
	if props.Recolor != nil {
		recolorValue := strings.ToUpper(strings.TrimSpace(*props.Recolor))
		if recolorValue == "NONE" || recolorValue == "" {
			// To remove recolor, we set an empty Recolor with the field in mask
			// Setting to nil removes the effect
			imageProps.Recolor = nil
			fields = append(fields, "recolor")
			modifiedProps = append(modifiedProps, "recolor")
		} else {
			// Apply recolor preset
			imageProps.Recolor = &slides.Recolor{
				Name: recolorValue,
			}
			fields = append(fields, "recolor")
			modifiedProps = append(modifiedProps, "recolor")
		}
	}

	if len(fields) == 0 {
		return nil, nil
	}

	return &slides.Request{
		UpdateImageProperties: &slides.UpdateImagePropertiesRequest{
			ObjectId:        objectID,
			ImageProperties: imageProps,
			Fields:          strings.Join(fields, ","),
		},
	}, modifiedProps
}
