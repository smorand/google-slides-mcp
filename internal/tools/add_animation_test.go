package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
)

func TestAddAnimation(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})

	tools := NewTools(DefaultToolsConfig(), nil)

	tests := []struct {
		name        string
		input       AddAnimationInput
		wantErr     error
		errContains string
	}{
		{
			name: "returns API limitation error for valid entrance animation",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "returns API limitation error for valid exit animation",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_OUT",
				AnimationCategory: "exit",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "returns API limitation error for valid emphasis animation",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "SPIN",
				AnimationCategory: "emphasis",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "returns API limitation error for fly animation with direction",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FLY_IN",
				AnimationCategory: "entrance",
				Direction:         "FROM_LEFT",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "returns API limitation error with duration and delay",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "ZOOM_IN",
				AnimationCategory: "entrance",
				Duration:          ptrFloat64(0.5),
				Delay:             ptrFloat64(0.2),
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "returns API limitation error with trigger",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "APPEAR",
				AnimationCategory: "entrance",
				Trigger:           "ON_CLICK",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "returns API limitation error with all options",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FLY_OUT",
				AnimationCategory: "exit",
				Direction:         "FROM_BOTTOM",
				Duration:          ptrFloat64(1.0),
				Delay:             ptrFloat64(0.5),
				Trigger:           "AFTER_PREVIOUS",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "rejects missing presentation_id",
			input: AddAnimationInput{
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
			},
			wantErr:     ErrInvalidPresentationID,
			errContains: "presentation_id is required",
		},
		{
			name: "rejects missing object_id",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
			},
			wantErr:     ErrInvalidObjectID,
			errContains: "object_id is required",
		},
		{
			name: "rejects missing animation_type",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationCategory: "entrance",
			},
			wantErr:     ErrInvalidAnimationType,
			errContains: "animation_type is required",
		},
		{
			name: "rejects invalid animation_type",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "INVALID_ANIMATION",
				AnimationCategory: "entrance",
			},
			wantErr:     ErrInvalidAnimationType,
			errContains: "not a valid animation type",
		},
		{
			name: "rejects missing animation_category",
			input: AddAnimationInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-123",
				AnimationType:  "FADE_IN",
			},
			wantErr:     ErrInvalidAnimationCategory,
			errContains: "animation_category is required",
		},
		{
			name: "rejects invalid animation_category",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "INVALID_CATEGORY",
			},
			wantErr:     ErrInvalidAnimationCategory,
			errContains: "not a valid animation category",
		},
		{
			name: "rejects invalid direction",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FLY_IN",
				AnimationCategory: "entrance",
				Direction:         "INVALID_DIRECTION",
			},
			wantErr:     ErrInvalidDirection,
			errContains: "not a valid direction",
		},
		{
			name: "rejects invalid trigger",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Trigger:           "INVALID_TRIGGER",
			},
			wantErr:     ErrInvalidAnimationTrigger,
			errContains: "not a valid trigger",
		},
		{
			name: "rejects negative duration",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Duration:          ptrFloat64(-1.0),
			},
			wantErr:     ErrInvalidAnimationDuration,
			errContains: "duration cannot be negative",
		},
		{
			name: "rejects duration exceeding 60 seconds",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Duration:          ptrFloat64(61.0),
			},
			wantErr:     ErrInvalidAnimationDuration,
			errContains: "duration cannot exceed 60 seconds",
		},
		{
			name: "rejects negative delay",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Delay:             ptrFloat64(-1.0),
			},
			wantErr:     ErrInvalidAnimationDelay,
			errContains: "delay cannot be negative",
		},
		{
			name: "rejects delay exceeding 60 seconds",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Delay:             ptrFloat64(61.0),
			},
			wantErr:     ErrInvalidAnimationDelay,
			errContains: "delay cannot exceed 60 seconds",
		},
		{
			name: "normalizes animation_type to uppercase",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "fade_in",
				AnimationCategory: "entrance",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "normalizes animation_category to uppercase",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "ENTRANCE",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "normalizes direction to uppercase",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FLY_IN",
				AnimationCategory: "entrance",
				Direction:         "from_left",
			},
			wantErr: ErrAnimationNotSupported,
		},
		{
			name: "normalizes trigger to uppercase",
			input: AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Trigger:           "on_click",
			},
			wantErr: ErrAnimationNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tools.AddAnimation(ctx, tokenSource, tt.input)

			// Output should always be nil (API not supported)
			if output != nil {
				t.Errorf("expected nil output for all cases, got %+v", output)
			}

			// Check error type
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got '%s'", tt.errContains, err.Error())
				}
			}
		})
	}
}

func TestAddAnimation_AllAnimationTypes(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test all valid animation types
	animationTypes := []string{
		"APPEAR", "FADE_IN", "FLY_IN", "ZOOM_IN",
		"FADE_OUT", "FLY_OUT", "ZOOM_OUT",
		"SPIN", "FLOAT", "BOUNCE",
	}

	for _, animType := range animationTypes {
		t.Run(animType, func(t *testing.T) {
			input := AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     animType,
				AnimationCategory: "entrance",
			}

			_, err := tools.AddAnimation(ctx, tokenSource, input)
			if !errors.Is(err, ErrAnimationNotSupported) {
				t.Errorf("expected ErrAnimationNotSupported for %s, got %v", animType, err)
			}
		})
	}
}

func TestAddAnimation_AllAnimationCategories(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test all valid animation categories
	categories := []string{"entrance", "exit", "emphasis"}

	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			input := AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: category,
			}

			_, err := tools.AddAnimation(ctx, tokenSource, input)
			if !errors.Is(err, ErrAnimationNotSupported) {
				t.Errorf("expected ErrAnimationNotSupported for %s, got %v", category, err)
			}
		})
	}
}

func TestAddAnimation_AllTriggerTypes(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test all valid trigger types
	triggers := []string{"ON_CLICK", "AFTER_PREVIOUS", "WITH_PREVIOUS"}

	for _, trigger := range triggers {
		t.Run(trigger, func(t *testing.T) {
			input := AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FADE_IN",
				AnimationCategory: "entrance",
				Trigger:           trigger,
			}

			_, err := tools.AddAnimation(ctx, tokenSource, input)
			if !errors.Is(err, ErrAnimationNotSupported) {
				t.Errorf("expected ErrAnimationNotSupported for trigger %s, got %v", trigger, err)
			}
		})
	}
}

func TestAddAnimation_AllDirections(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test all valid directions
	directions := []string{"FROM_LEFT", "FROM_RIGHT", "FROM_TOP", "FROM_BOTTOM"}

	for _, direction := range directions {
		t.Run(direction, func(t *testing.T) {
			input := AddAnimationInput{
				PresentationID:    "test-presentation",
				ObjectID:          "shape-123",
				AnimationType:     "FLY_IN",
				AnimationCategory: "entrance",
				Direction:         direction,
			}

			_, err := tools.AddAnimation(ctx, tokenSource, input)
			if !errors.Is(err, ErrAnimationNotSupported) {
				t.Errorf("expected ErrAnimationNotSupported for direction %s, got %v", direction, err)
			}
		})
	}
}

func TestAddAnimation_ErrorMessageContainsIssueTracker(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	input := AddAnimationInput{
		PresentationID:    "test-presentation",
		ObjectID:          "shape-123",
		AnimationType:     "FADE_IN",
		AnimationCategory: "entrance",
	}

	_, err := tools.AddAnimation(ctx, tokenSource, input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error message contains helpful information
	errMsg := err.Error()
	expectedContents := []string{
		"Google Slides API does not provide endpoints",
		"animations",
		"issuetracker.google.com/issues/36761236",
		"Slides UI",
	}

	for _, expected := range expectedContents {
		if !containsString(errMsg, expected) {
			t.Errorf("expected error message to contain '%s', got '%s'", expected, errMsg)
		}
	}
}

// containsString is defined in replace_text_test.go
