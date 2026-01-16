package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
)

// Sentinel errors for add_animation tool.
var (
	ErrAddAnimationFailed       = errors.New("failed to add animation")
	ErrAnimationNotSupported    = errors.New("object animations are not supported by the Google Slides API")
	ErrInvalidAnimationType     = errors.New("invalid animation type")
	ErrInvalidAnimationCategory = errors.New("invalid animation category")
	ErrInvalidAnimationTrigger  = errors.New("invalid animation trigger")
	ErrInvalidAnimationDuration = errors.New("invalid animation duration")
	ErrInvalidAnimationDelay    = errors.New("invalid animation delay")
	ErrInvalidDirection         = errors.New("invalid direction")
)

// Valid animation types that would be supported if the API allowed it.
var validAnimationTypes = map[string]bool{
	"APPEAR":   true,
	"FADE_IN":  true,
	"FLY_IN":   true,
	"ZOOM_IN":  true,
	"FADE_OUT": true,
	"FLY_OUT":  true,
	"ZOOM_OUT": true,
	"SPIN":     true,
	"FLOAT":    true,
	"BOUNCE":   true,
}

// Valid animation categories.
var validAnimationCategories = map[string]bool{
	"ENTRANCE": true,
	"EXIT":     true,
	"EMPHASIS": true,
}

// Valid animation triggers.
var validAnimationTriggers = map[string]bool{
	"ON_CLICK":        true,
	"AFTER_PREVIOUS":  true,
	"WITH_PREVIOUS":   true,
}

// Valid directions for fly animations.
var validDirections = map[string]bool{
	"FROM_LEFT":   true,
	"FROM_RIGHT":  true,
	"FROM_TOP":    true,
	"FROM_BOTTOM": true,
}

// AddAnimationInput represents the input for the add_animation tool.
type AddAnimationInput struct {
	PresentationID    string   `json:"presentation_id"`              // Required
	ObjectID          string   `json:"object_id"`                    // Required - ID of object to animate
	AnimationType     string   `json:"animation_type"`               // Required: APPEAR, FADE_IN, FLY_IN, etc.
	AnimationCategory string   `json:"animation_category"`           // Required: entrance, exit, emphasis
	Direction         string   `json:"direction,omitempty"`          // For fly animations: FROM_LEFT, FROM_RIGHT, etc.
	Duration          *float64 `json:"duration,omitempty"`           // Optional: seconds
	Delay             *float64 `json:"delay,omitempty"`              // Optional: seconds before animation starts
	Trigger           string   `json:"trigger,omitempty"`            // Optional: ON_CLICK, AFTER_PREVIOUS, WITH_PREVIOUS
}

// AddAnimationOutput represents the output of the add_animation tool.
type AddAnimationOutput struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	AnimationID string `json:"animation_id,omitempty"`
}

// AddAnimation adds an entrance/exit/emphasis animation to an object.
// IMPORTANT: This tool returns an error because the Google Slides API does not support
// adding object animations programmatically. Animations can only be configured through
// the Google Slides UI (Insert > Animation or View > Motion).
//
// Reference: https://issuetracker.google.com/issues/36761236 - Feature request for animation API support
func (t *Tools) AddAnimation(ctx context.Context, tokenSource oauth2.TokenSource, input AddAnimationInput) (*AddAnimationOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Normalize and validate animation type
	animationType := strings.ToUpper(strings.TrimSpace(input.AnimationType))
	if animationType == "" {
		return nil, fmt.Errorf("%w: animation_type is required", ErrInvalidAnimationType)
	}
	if !validAnimationTypes[animationType] {
		return nil, fmt.Errorf("%w: '%s' is not a valid animation type. Valid types: APPEAR, FADE_IN, FLY_IN, ZOOM_IN, FADE_OUT, FLY_OUT, ZOOM_OUT, SPIN, FLOAT, BOUNCE", ErrInvalidAnimationType, input.AnimationType)
	}

	// Normalize and validate animation category
	animationCategory := strings.ToUpper(strings.TrimSpace(input.AnimationCategory))
	if animationCategory == "" {
		return nil, fmt.Errorf("%w: animation_category is required", ErrInvalidAnimationCategory)
	}
	if !validAnimationCategories[animationCategory] {
		return nil, fmt.Errorf("%w: '%s' is not a valid animation category. Valid categories: entrance, exit, emphasis", ErrInvalidAnimationCategory, input.AnimationCategory)
	}

	// Validate direction if provided (required for FLY_IN and FLY_OUT)
	if input.Direction != "" {
		direction := strings.ToUpper(strings.TrimSpace(input.Direction))
		if !validDirections[direction] {
			return nil, fmt.Errorf("%w: '%s' is not a valid direction. Valid directions: FROM_LEFT, FROM_RIGHT, FROM_TOP, FROM_BOTTOM", ErrInvalidDirection, input.Direction)
		}
	}

	// Validate trigger if provided
	if input.Trigger != "" {
		trigger := strings.ToUpper(strings.TrimSpace(input.Trigger))
		if !validAnimationTriggers[trigger] {
			return nil, fmt.Errorf("%w: '%s' is not a valid trigger. Valid triggers: ON_CLICK, AFTER_PREVIOUS, WITH_PREVIOUS", ErrInvalidAnimationTrigger, input.Trigger)
		}
	}

	// Validate duration if provided
	if input.Duration != nil {
		if *input.Duration < 0 {
			return nil, fmt.Errorf("%w: duration cannot be negative", ErrInvalidAnimationDuration)
		}
		if *input.Duration > 60 {
			return nil, fmt.Errorf("%w: duration cannot exceed 60 seconds", ErrInvalidAnimationDuration)
		}
	}

	// Validate delay if provided
	if input.Delay != nil {
		if *input.Delay < 0 {
			return nil, fmt.Errorf("%w: delay cannot be negative", ErrInvalidAnimationDelay)
		}
		if *input.Delay > 60 {
			return nil, fmt.Errorf("%w: delay cannot exceed 60 seconds", ErrInvalidAnimationDelay)
		}
	}

	t.config.Logger.Info("add_animation called",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("animation_type", animationType),
		slog.String("animation_category", animationCategory),
	)

	// The Google Slides API does not support adding object animations.
	// This is a known limitation documented in Google's issue tracker.
	//
	// The API does not expose any request types for:
	// - Creating animations
	// - Modifying animations
	// - Deleting animations
	// - Reading animation properties
	//
	// Animations can only be configured through:
	// 1. Google Slides UI (Insert > Animation or View > Motion)
	// 2. Google Apps Script (with limitations)
	//
	// Reference: https://issuetracker.google.com/issues/36761236
	return nil, fmt.Errorf("%w: the Google Slides API does not provide endpoints for adding or managing object animations. "+
		"Animations can only be configured through the Google Slides user interface (Insert > Animation or View > Motion). "+
		"This is a known API limitation tracked at https://issuetracker.google.com/issues/36761236. "+
		"Consider using the Slides UI or Google Apps Script for animation management", ErrAnimationNotSupported)
}
