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

// Sentinel errors for change_z_order tool.
var (
	ErrChangeZOrderFailed = errors.New("failed to change z-order")
	ErrInvalidZOrderAction = errors.New("invalid z-order action")
	ErrObjectInGroup = errors.New("cannot change z-order of grouped objects")
)

// ChangeZOrderInput represents the input for the change_z_order tool.
type ChangeZOrderInput struct {
	PresentationID string `json:"presentation_id"`
	ObjectID       string `json:"object_id"`
	Action         string `json:"action"` // bring_to_front, send_to_back, bring_forward, send_backward
}

// ChangeZOrderOutput represents the output of the change_z_order tool.
type ChangeZOrderOutput struct {
	ObjectID    string `json:"object_id"`
	Action      string `json:"action"`
	NewZOrder   int    `json:"new_z_order"`  // 0-based position (0 = furthest back)
	TotalLayers int    `json:"total_layers"` // Total number of objects on the slide
}

// validZOrderActions maps user-friendly action names to API operations.
var validZOrderActions = map[string]string{
	"BRING_TO_FRONT":  "BRING_TO_FRONT",
	"SEND_TO_BACK":    "SEND_TO_BACK",
	"BRING_FORWARD":   "BRING_FORWARD",
	"SEND_BACKWARD":   "SEND_BACKWARD",
	// User-friendly aliases (lowercase will be normalized)
	"bring_to_front":  "BRING_TO_FRONT",
	"send_to_back":    "SEND_TO_BACK",
	"bring_forward":   "BRING_FORWARD",
	"send_backward":   "SEND_BACKWARD",
}

// ChangeZOrder changes the z-order (layering) of an object on a slide.
func (t *Tools) ChangeZOrder(ctx context.Context, tokenSource oauth2.TokenSource, input ChangeZOrderInput) (*ChangeZOrderOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}
	if input.Action == "" {
		return nil, fmt.Errorf("%w: action is required (bring_to_front, send_to_back, bring_forward, send_backward)", ErrInvalidZOrderAction)
	}

	// Normalize action
	normalizedAction := strings.ToUpper(strings.TrimSpace(input.Action))
	// Handle underscore-separated user input
	normalizedAction = strings.ReplaceAll(normalizedAction, "_", "_")

	apiOperation, validAction := validZOrderActions[normalizedAction]
	if !validAction {
		// Try lowercase version
		apiOperation, validAction = validZOrderActions[strings.ToLower(input.Action)]
	}
	if !validAction {
		return nil, fmt.Errorf("%w: '%s' is not a valid action (use bring_to_front, send_to_back, bring_forward, send_backward)", ErrInvalidZOrderAction, input.Action)
	}

	t.config.Logger.Info("changing z-order",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", apiOperation),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get presentation to find the object and its slide
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

	// Find the object and its containing slide
	var objectSlide *slides.Page
	var objectElement *slides.PageElement
	var isInGroup bool

	for _, slide := range presentation.Slides {
		element, inGroup := findElementAndCheckGroup(slide.PageElements, input.ObjectID)
		if element != nil {
			objectSlide = slide
			objectElement = element
			isInGroup = inGroup
			break
		}
	}

	if objectElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Check if object is in a group (API doesn't allow z-order changes for grouped objects)
	if isInGroup {
		return nil, fmt.Errorf("%w: object '%s' is inside a group", ErrObjectInGroup, input.ObjectID)
	}

	// Execute z-order change
	req := &slides.Request{
		UpdatePageElementsZOrder: &slides.UpdatePageElementsZOrderRequest{
			PageElementObjectIds: []string{input.ObjectID},
			Operation:            apiOperation,
		},
	}

	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, []*slides.Request{req})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrChangeZOrderFailed, err)
	}

	// Fetch updated presentation to determine new z-order position
	updatedPresentation, err := slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		// Z-order change succeeded but we can't get updated position
		t.config.Logger.Warn("failed to fetch updated presentation for z-order position",
			slog.String("error", err.Error()),
		)
		return &ChangeZOrderOutput{
			ObjectID:    input.ObjectID,
			Action:      strings.ToLower(apiOperation),
			NewZOrder:   -1, // Unknown
			TotalLayers: len(objectSlide.PageElements),
		}, nil
	}

	// Find the updated slide and calculate new z-order
	newZOrder := -1
	totalLayers := 0
	for _, slide := range updatedPresentation.Slides {
		if slide.ObjectId == objectSlide.ObjectId {
			totalLayers = len(slide.PageElements)
			for idx, elem := range slide.PageElements {
				if elem.ObjectId == input.ObjectID {
					newZOrder = idx
					break
				}
			}
			break
		}
	}

	return &ChangeZOrderOutput{
		ObjectID:    input.ObjectID,
		Action:      strings.ToLower(apiOperation),
		NewZOrder:   newZOrder,
		TotalLayers: totalLayers,
	}, nil
}

// findElementAndCheckGroup searches for an element by ID and returns whether it's inside a group.
// Returns (element, isInGroup).
func findElementAndCheckGroup(elements []*slides.PageElement, objectID string) (*slides.PageElement, bool) {
	for _, elem := range elements {
		if elem.ObjectId == objectID {
			return elem, false // Found at top level, not in group
		}
		// Check inside groups
		if elem.ElementGroup != nil && elem.ElementGroup.Children != nil {
			if found, _ := findElementAndCheckGroup(elem.ElementGroup.Children, objectID); found != nil {
				return found, true // Found inside a group
			}
		}
	}
	return nil, false
}
