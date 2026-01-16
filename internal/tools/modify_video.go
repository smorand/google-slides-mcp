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

// Sentinel errors for modify_video tool.
var (
	ErrModifyVideoFailed  = errors.New("failed to modify video")
	ErrNotVideoObject     = errors.New("object is not a video")
	ErrNoVideoProperties  = errors.New("no video properties to modify")
)

// ModifyVideoInput represents the input for the modify_video tool.
type ModifyVideoInput struct {
	PresentationID string                 `json:"presentation_id"`
	ObjectID       string                 `json:"object_id"`
	Properties     *VideoModifyProperties `json:"properties"`
}

// VideoModifyProperties represents the video properties to modify.
type VideoModifyProperties struct {
	Position  *PositionInput `json:"position,omitempty"`   // Position in points
	Size      *SizeInput     `json:"size,omitempty"`       // Size in points
	StartTime *float64       `json:"start_time,omitempty"` // Start time in seconds
	EndTime   *float64       `json:"end_time,omitempty"`   // End time in seconds
	Autoplay  *bool          `json:"autoplay,omitempty"`   // Auto-play setting
	Mute      *bool          `json:"mute,omitempty"`       // Mute setting
}

// ModifyVideoOutput represents the output of the modify_video tool.
type ModifyVideoOutput struct {
	ObjectID           string   `json:"object_id"`
	ModifiedProperties []string `json:"modified_properties"`
}

// ModifyVideo modifies properties of an existing video.
func (t *Tools) ModifyVideo(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyVideoInput) (*ModifyVideoOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}
	if input.Properties == nil {
		return nil, ErrNoVideoProperties
	}

	// Validate properties values
	if err := validateVideoModifyProperties(input.Properties); err != nil {
		return nil, err
	}

	// Check if any properties are actually provided
	if !hasVideoPropertiesToModify(input.Properties) {
		return nil, ErrNoVideoProperties
	}

	t.config.Logger.Info("modifying video properties",
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

	// Find the video object
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

	// Verify it's a video
	if targetElement.Video == nil {
		return nil, fmt.Errorf("%w: object '%s' is not a video (type: %s)", ErrNotVideoObject, input.ObjectID, determineObjectType(targetElement))
	}

	// Build requests and track modified properties
	requests, modifiedProps := buildModifyVideoRequests(input.ObjectID, input.Properties, targetElement)

	if len(requests) == 0 {
		return nil, ErrNoVideoProperties
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
		return nil, fmt.Errorf("%w: %v", ErrModifyVideoFailed, err)
	}

	output := &ModifyVideoOutput{
		ObjectID:           input.ObjectID,
		ModifiedProperties: modifiedProps,
	}

	t.config.Logger.Info("video modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Int("properties_modified", len(modifiedProps)),
	)

	return output, nil
}

// validateVideoModifyProperties validates the input property values.
func validateVideoModifyProperties(props *VideoModifyProperties) error {
	if props.Size != nil {
		// Width or Height can be 0 (meaning "don't change"), but cannot be negative
		// At least one must be positive if Size is specified
		if props.Size.Width < 0 || props.Size.Height < 0 {
			return ErrInvalidVideoSize
		}
		// At least one dimension must be positive
		if props.Size.Width <= 0 && props.Size.Height <= 0 {
			return ErrInvalidVideoSize
		}
	}

	if props.Position != nil {
		if props.Position.X < 0 || props.Position.Y < 0 {
			return ErrInvalidVideoPosition
		}
	}

	if props.StartTime != nil && *props.StartTime < 0 {
		return fmt.Errorf("%w: start_time cannot be negative", ErrInvalidVideoTime)
	}

	if props.EndTime != nil && *props.EndTime < 0 {
		return fmt.Errorf("%w: end_time cannot be negative", ErrInvalidVideoTime)
	}

	if props.StartTime != nil && props.EndTime != nil && *props.EndTime <= *props.StartTime {
		return ErrInvalidVideoTimeRange
	}

	return nil
}

// hasVideoPropertiesToModify checks if any video properties are set.
func hasVideoPropertiesToModify(props *VideoModifyProperties) bool {
	if props == nil {
		return false
	}
	return props.Position != nil ||
		props.Size != nil ||
		props.StartTime != nil ||
		props.EndTime != nil ||
		props.Autoplay != nil ||
		props.Mute != nil
}

// buildModifyVideoRequests creates batch update requests for video modifications.
func buildModifyVideoRequests(objectID string, props *VideoModifyProperties, element *slides.PageElement) ([]*slides.Request, []string) {
	var requests []*slides.Request
	var modifiedProps []string

	// Handle position and/or size changes via UpdatePageElementTransformRequest
	if props.Position != nil || props.Size != nil {
		transformReq := buildVideoTransformRequest(objectID, props, element)
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

	// Handle video playback properties via UpdateVideoPropertiesRequest
	videoPropsReq, videoModifiedFields := buildModifyVideoPropertiesRequest(objectID, props)
	if videoPropsReq != nil {
		requests = append(requests, videoPropsReq)
		modifiedProps = append(modifiedProps, videoModifiedFields...)
	}

	return requests, modifiedProps
}

// buildVideoTransformRequest creates a request to update position and/or size.
func buildVideoTransformRequest(objectID string, props *VideoModifyProperties, element *slides.PageElement) *slides.Request {
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

// buildModifyVideoPropertiesRequest creates a request to update video properties.
func buildModifyVideoPropertiesRequest(objectID string, props *VideoModifyProperties) (*slides.Request, []string) {
	videoProperties := &slides.VideoProperties{}
	var fields []string
	var modifiedProps []string

	// Handle start time (convert seconds to milliseconds)
	if props.StartTime != nil {
		videoProperties.Start = int64(*props.StartTime * 1000)
		fields = append(fields, "start")
		modifiedProps = append(modifiedProps, "start_time")
	}

	// Handle end time (convert seconds to milliseconds)
	if props.EndTime != nil {
		videoProperties.End = int64(*props.EndTime * 1000)
		fields = append(fields, "end")
		modifiedProps = append(modifiedProps, "end_time")
	}

	// Handle autoplay
	if props.Autoplay != nil {
		videoProperties.AutoPlay = *props.Autoplay
		fields = append(fields, "autoPlay")
		modifiedProps = append(modifiedProps, "autoplay")
	}

	// Handle mute
	if props.Mute != nil {
		videoProperties.Mute = *props.Mute
		fields = append(fields, "mute")
		modifiedProps = append(modifiedProps, "mute")
	}

	if len(fields) == 0 {
		return nil, nil
	}

	return &slides.Request{
		UpdateVideoProperties: &slides.UpdateVideoPropertiesRequest{
			ObjectId:        objectID,
			VideoProperties: videoProperties,
			Fields:          strings.Join(fields, ","),
		},
	}, modifiedProps
}
