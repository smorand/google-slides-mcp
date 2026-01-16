package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
)

// Sentinel errors for set_transition tool.
var (
	ErrSetTransitionFailed          = errors.New("failed to set transition")
	ErrTransitionNotSupported       = errors.New("slide transitions are not supported by the Google Slides API")
	ErrInvalidTransitionType        = errors.New("invalid transition type")
	ErrInvalidTransitionDuration    = errors.New("invalid transition duration")
)

// Valid transition types that would be supported if the API allowed it.
// These are documented here for reference and validation purposes.
var validTransitionTypes = map[string]bool{
	"NONE":              true,
	"FADE":              true,
	"SLIDE_FROM_RIGHT":  true,
	"SLIDE_FROM_LEFT":   true,
	"SLIDE_FROM_TOP":    true,
	"SLIDE_FROM_BOTTOM": true,
	"FLIP":              true,
	"CUBE":              true,
	"GALLERY":           true,
	"ZOOM":              true,
	"DISSOLVE":          true,
}

// SetTransitionInput represents the input for the set_transition tool.
type SetTransitionInput struct {
	PresentationID string   `json:"presentation_id"`            // Required
	SlideIndex     int      `json:"slide_index,omitempty"`      // 1-based, optional (use this OR SlideID OR "all")
	SlideID        string   `json:"slide_id,omitempty"`         // Alternative to SlideIndex
	TransitionType string   `json:"transition_type"`            // Required: NONE, FADE, SLIDE_FROM_RIGHT, etc.
	Duration       *float64 `json:"duration,omitempty"`         // Optional: seconds (e.g., 0.5)
}

// SetTransitionOutput represents the output of the set_transition tool.
type SetTransitionOutput struct {
	Success        bool     `json:"success"`
	Message        string   `json:"message"`
	AffectedSlides []string `json:"affected_slides,omitempty"`
}

// SetTransition sets the transition effect for slides.
// IMPORTANT: This tool returns an error because the Google Slides API does not support
// setting slide transitions programmatically. Transitions can only be configured through
// the Google Slides UI (Slide > Transition).
func (t *Tools) SetTransition(ctx context.Context, tokenSource oauth2.TokenSource, input SetTransitionInput) (*SetTransitionOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	// Normalize and validate transition type
	transitionType := strings.ToUpper(strings.TrimSpace(input.TransitionType))
	if transitionType == "" {
		return nil, fmt.Errorf("%w: transition_type is required", ErrInvalidTransitionType)
	}
	if !validTransitionTypes[transitionType] {
		return nil, fmt.Errorf("%w: '%s' is not a valid transition type. Valid types: NONE, FADE, SLIDE_FROM_RIGHT, SLIDE_FROM_LEFT, SLIDE_FROM_TOP, SLIDE_FROM_BOTTOM, FLIP, CUBE, GALLERY, ZOOM, DISSOLVE", ErrInvalidTransitionType, input.TransitionType)
	}

	// Validate duration if provided
	if input.Duration != nil {
		if *input.Duration < 0 {
			return nil, fmt.Errorf("%w: duration cannot be negative", ErrInvalidTransitionDuration)
		}
		if *input.Duration > 10 {
			return nil, fmt.Errorf("%w: duration cannot exceed 10 seconds", ErrInvalidTransitionDuration)
		}
	}

	t.config.Logger.Info("set_transition called",
		slog.String("presentation_id", input.PresentationID),
		slog.String("transition_type", transitionType),
	)

	// The Google Slides API does not support setting slide transitions.
	// The SlideProperties type only contains: IsSkipped, LayoutObjectId, MasterObjectId, NotesPage
	// There is no transition-related property available in the API.
	//
	// This is a known limitation of the Google Slides API. Transitions can only be configured
	// through the Google Slides user interface (Slide > Transition).
	//
	// Reference: https://developers.google.com/slides/api/reference/rest/v1/presentations.pages#SlideProperties
	return nil, fmt.Errorf("%w: the Google Slides API does not provide endpoints for setting slide transitions. "+
		"Slide transitions can only be configured through the Google Slides user interface (Slide > Transition). "+
		"The SlideProperties API only supports: isSkipped, layoutObjectId, masterObjectId, and notesPage. "+
		"Consider using Google Apps Script or the Slides UI for transition management", ErrTransitionNotSupported)
}
