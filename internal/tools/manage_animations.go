package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
)

// Sentinel errors for manage_animations tool.
var (
	ErrManageAnimationsFailed   = errors.New("failed to manage animations")
	ErrManageAnimationsNotSupported = errors.New("animation management is not supported by the Google Slides API")
	ErrInvalidManageAnimationsAction = errors.New("invalid action for manage_animations")
	ErrInvalidAnimationID       = errors.New("invalid animation_id")
	ErrNoAnimationIDs           = errors.New("animation_ids required for reorder action")
	ErrNoAnimationProperties    = errors.New("properties required for modify action")
)

// Valid actions for manage_animations.
var validManageAnimationsActions = map[string]bool{
	"LIST":    true,
	"REORDER": true,
	"MODIFY":  true,
	"DELETE":  true,
}

// ManageAnimationsInput represents the input for the manage_animations tool.
type ManageAnimationsInput struct {
	PresentationID string                        `json:"presentation_id"`               // Required
	SlideIndex     int                           `json:"slide_index,omitempty"`         // 1-based index (use this OR SlideID)
	SlideID        string                        `json:"slide_id,omitempty"`            // Alternative to SlideIndex
	Action         string                        `json:"action"`                        // Required: list, reorder, modify, delete
	AnimationIDs   []string                      `json:"animation_ids,omitempty"`       // For reorder action: array in new order
	AnimationID    string                        `json:"animation_id,omitempty"`        // For modify/delete actions
	Properties     *AnimationModifyProperties    `json:"properties,omitempty"`          // For modify action
}

// AnimationModifyProperties contains properties that could be modified on an animation.
type AnimationModifyProperties struct {
	AnimationType     string   `json:"animation_type,omitempty"`     // APPEAR, FADE_IN, FLY_IN, etc.
	AnimationCategory string   `json:"animation_category,omitempty"` // entrance, exit, emphasis
	Direction         string   `json:"direction,omitempty"`          // FROM_LEFT, FROM_RIGHT, etc.
	Duration          *float64 `json:"duration,omitempty"`           // Seconds
	Delay             *float64 `json:"delay,omitempty"`              // Seconds
	Trigger           string   `json:"trigger,omitempty"`            // ON_CLICK, AFTER_PREVIOUS, WITH_PREVIOUS
}

// AnimationInfo represents information about a single animation.
type AnimationInfo struct {
	AnimationID       string  `json:"animation_id"`
	ObjectID          string  `json:"object_id"`
	AnimationType     string  `json:"animation_type"`
	AnimationCategory string  `json:"animation_category"`
	Direction         string  `json:"direction,omitempty"`
	Duration          float64 `json:"duration"`
	Delay             float64 `json:"delay"`
	Trigger           string  `json:"trigger"`
	Order             int     `json:"order"` // Position in animation sequence (0-based)
}

// ManageAnimationsOutput represents the output of the manage_animations tool.
type ManageAnimationsOutput struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	Action     string          `json:"action"`
	Animations []AnimationInfo `json:"animations,omitempty"` // For list action
}

// ManageAnimations manages animations on a slide (list, reorder, modify, delete).
// IMPORTANT: This tool returns an error because the Google Slides API does not support
// managing object animations programmatically. Animations can only be configured through
// the Google Slides UI (Insert > Animation or View > Motion).
//
// Reference: https://issuetracker.google.com/issues/36761236 - Feature request for animation API support
func (t *Tools) ManageAnimations(ctx context.Context, tokenSource oauth2.TokenSource, input ManageAnimationsInput) (*ManageAnimationsOutput, error) {
	// Validate presentation_id
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	// Validate slide reference (either SlideIndex or SlideID required)
	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	// Validate slide index if provided
	if input.SlideIndex < 0 {
		return nil, fmt.Errorf("%w: slide_index must be positive (1-based)", ErrInvalidSlideReference)
	}

	// Normalize and validate action
	action := strings.ToUpper(strings.TrimSpace(input.Action))
	if action == "" {
		return nil, fmt.Errorf("%w: action is required", ErrInvalidManageAnimationsAction)
	}
	if !validManageAnimationsActions[action] {
		return nil, fmt.Errorf("%w: '%s' is not a valid action. Valid actions: list, reorder, modify, delete", ErrInvalidManageAnimationsAction, input.Action)
	}

	// Validate action-specific parameters
	switch action {
	case "LIST":
		// No additional parameters required
	case "REORDER":
		if len(input.AnimationIDs) == 0 {
			return nil, fmt.Errorf("%w: animation_ids array is required for reorder action", ErrNoAnimationIDs)
		}
	case "MODIFY":
		if input.AnimationID == "" {
			return nil, fmt.Errorf("%w: animation_id is required for modify action", ErrInvalidAnimationID)
		}
		if input.Properties == nil {
			return nil, fmt.Errorf("%w: properties object is required for modify action", ErrNoAnimationProperties)
		}
		// Validate properties if provided
		if err := t.validateAnimationProperties(input.Properties); err != nil {
			return nil, err
		}
	case "DELETE":
		if input.AnimationID == "" {
			return nil, fmt.Errorf("%w: animation_id is required for delete action", ErrInvalidAnimationID)
		}
	}

	t.config.Logger.Info("manage_animations called",
		slog.String("presentation_id", input.PresentationID),
		slog.String("action", action),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
	)

	// The Google Slides API does not support managing object animations.
	// This is a known limitation documented in Google's issue tracker.
	//
	// The API does not expose any request types for:
	// - Reading/listing animations
	// - Reordering animations
	// - Modifying animation properties
	// - Deleting animations
	//
	// Animations can only be configured through:
	// 1. Google Slides UI (Insert > Animation or View > Motion)
	// 2. Google Apps Script (with limitations)
	//
	// Reference: https://issuetracker.google.com/issues/36761236
	return nil, fmt.Errorf("%w: the Google Slides API does not provide endpoints for listing, reordering, modifying, or deleting object animations. "+
		"Animations can only be managed through the Google Slides user interface (View > Motion or Insert > Animation). "+
		"This is a known API limitation tracked at https://issuetracker.google.com/issues/36761236. "+
		"Consider using the Slides UI or Google Apps Script for animation management", ErrManageAnimationsNotSupported)
}

// validateAnimationProperties validates the animation modification properties.
func (t *Tools) validateAnimationProperties(props *AnimationModifyProperties) error {
	// Validate animation type if provided
	if props.AnimationType != "" {
		animationType := strings.ToUpper(strings.TrimSpace(props.AnimationType))
		if !validAnimationTypes[animationType] {
			return fmt.Errorf("%w: '%s' is not a valid animation type", ErrInvalidAnimationType, props.AnimationType)
		}
	}

	// Validate animation category if provided
	if props.AnimationCategory != "" {
		animationCategory := strings.ToUpper(strings.TrimSpace(props.AnimationCategory))
		if !validAnimationCategories[animationCategory] {
			return fmt.Errorf("%w: '%s' is not a valid animation category", ErrInvalidAnimationCategory, props.AnimationCategory)
		}
	}

	// Validate direction if provided
	if props.Direction != "" {
		direction := strings.ToUpper(strings.TrimSpace(props.Direction))
		if !validDirections[direction] {
			return fmt.Errorf("%w: '%s' is not a valid direction", ErrInvalidDirection, props.Direction)
		}
	}

	// Validate trigger if provided
	if props.Trigger != "" {
		trigger := strings.ToUpper(strings.TrimSpace(props.Trigger))
		if !validAnimationTriggers[trigger] {
			return fmt.Errorf("%w: '%s' is not a valid trigger", ErrInvalidAnimationTrigger, props.Trigger)
		}
	}

	// Validate duration if provided
	if props.Duration != nil {
		if *props.Duration < 0 {
			return fmt.Errorf("%w: duration cannot be negative", ErrInvalidAnimationDuration)
		}
		if *props.Duration > 60 {
			return fmt.Errorf("%w: duration cannot exceed 60 seconds", ErrInvalidAnimationDuration)
		}
	}

	// Validate delay if provided
	if props.Delay != nil {
		if *props.Delay < 0 {
			return fmt.Errorf("%w: delay cannot be negative", ErrInvalidAnimationDelay)
		}
		if *props.Delay > 60 {
			return fmt.Errorf("%w: delay cannot exceed 60 seconds", ErrInvalidAnimationDelay)
		}
	}

	return nil
}
