package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
)

func TestManageAnimations(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})

	tools := NewTools(DefaultToolsConfig(), nil)

	tests := []struct {
		name        string
		input       ManageAnimationsInput
		wantErr     error
		errContains string
	}{
		{
			name: "returns API limitation error for list action by slide index",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "list",
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
		{
			name: "returns API limitation error for list action by slide ID",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideID:        "slide-123",
				Action:         "list",
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
		{
			name: "returns API limitation error for reorder action",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "reorder",
				AnimationIDs:   []string{"anim-1", "anim-2", "anim-3"},
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
		{
			name: "returns API limitation error for modify action",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Duration: ptrFloat64(0.5),
				},
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
		{
			name: "returns API limitation error for delete action",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "delete",
				AnimationID:    "anim-1",
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
		{
			name: "rejects missing presentation_id",
			input: ManageAnimationsInput{
				SlideIndex: 1,
				Action:     "list",
			},
			wantErr:     ErrInvalidPresentationID,
			errContains: "presentation_id is required",
		},
		{
			name: "rejects missing slide reference",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				Action:         "list",
			},
			wantErr:     ErrInvalidSlideReference,
			errContains: "either slide_index or slide_id is required",
		},
		{
			name: "rejects negative slide index",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     -1,
				Action:         "list",
			},
			wantErr:     ErrInvalidSlideReference,
			errContains: "slide_index must be positive",
		},
		{
			name: "rejects missing action",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
			},
			wantErr:     ErrInvalidManageAnimationsAction,
			errContains: "action is required",
		},
		{
			name: "rejects invalid action",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "INVALID_ACTION",
			},
			wantErr:     ErrInvalidManageAnimationsAction,
			errContains: "not a valid action",
		},
		{
			name: "rejects reorder without animation_ids",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "reorder",
			},
			wantErr:     ErrNoAnimationIDs,
			errContains: "animation_ids array is required",
		},
		{
			name: "rejects reorder with empty animation_ids",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "reorder",
				AnimationIDs:   []string{},
			},
			wantErr:     ErrNoAnimationIDs,
			errContains: "animation_ids array is required",
		},
		{
			name: "rejects modify without animation_id",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				Properties: &AnimationModifyProperties{
					Duration: ptrFloat64(0.5),
				},
			},
			wantErr:     ErrInvalidAnimationID,
			errContains: "animation_id is required",
		},
		{
			name: "rejects modify without properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
			},
			wantErr:     ErrNoAnimationProperties,
			errContains: "properties object is required",
		},
		{
			name: "rejects delete without animation_id",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "delete",
			},
			wantErr:     ErrInvalidAnimationID,
			errContains: "animation_id is required",
		},
		{
			name: "rejects invalid animation_type in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					AnimationType: "INVALID_TYPE",
				},
			},
			wantErr:     ErrInvalidAnimationType,
			errContains: "not a valid animation type",
		},
		{
			name: "rejects invalid animation_category in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					AnimationCategory: "INVALID_CATEGORY",
				},
			},
			wantErr:     ErrInvalidAnimationCategory,
			errContains: "not a valid animation category",
		},
		{
			name: "rejects invalid direction in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Direction: "INVALID_DIRECTION",
				},
			},
			wantErr:     ErrInvalidDirection,
			errContains: "not a valid direction",
		},
		{
			name: "rejects invalid trigger in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Trigger: "INVALID_TRIGGER",
				},
			},
			wantErr:     ErrInvalidAnimationTrigger,
			errContains: "not a valid trigger",
		},
		{
			name: "rejects negative duration in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Duration: ptrFloat64(-1.0),
				},
			},
			wantErr:     ErrInvalidAnimationDuration,
			errContains: "duration cannot be negative",
		},
		{
			name: "rejects duration exceeding 60 seconds in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Duration: ptrFloat64(61.0),
				},
			},
			wantErr:     ErrInvalidAnimationDuration,
			errContains: "duration cannot exceed 60 seconds",
		},
		{
			name: "rejects negative delay in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Delay: ptrFloat64(-1.0),
				},
			},
			wantErr:     ErrInvalidAnimationDelay,
			errContains: "delay cannot be negative",
		},
		{
			name: "rejects delay exceeding 60 seconds in properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					Delay: ptrFloat64(61.0),
				},
			},
			wantErr:     ErrInvalidAnimationDelay,
			errContains: "delay cannot exceed 60 seconds",
		},
		{
			name: "normalizes action to uppercase",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "LIST",
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
		{
			name: "accepts valid modify with all properties",
			input: ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties: &AnimationModifyProperties{
					AnimationType:     "FADE_IN",
					AnimationCategory: "entrance",
					Direction:         "FROM_LEFT",
					Duration:          ptrFloat64(0.5),
					Delay:             ptrFloat64(0.2),
					Trigger:           "ON_CLICK",
				},
			},
			wantErr: ErrManageAnimationsNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tools.ManageAnimations(ctx, tokenSource, tt.input)

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

func TestManageAnimations_AllActions(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test all valid actions (case insensitive)
	actionTests := []struct {
		action    string
		extraArgs ManageAnimationsInput
	}{
		{"list", ManageAnimationsInput{}},
		{"LIST", ManageAnimationsInput{}},
		{"List", ManageAnimationsInput{}},
		{"reorder", ManageAnimationsInput{AnimationIDs: []string{"a", "b"}}},
		{"REORDER", ManageAnimationsInput{AnimationIDs: []string{"a", "b"}}},
		{"modify", ManageAnimationsInput{AnimationID: "a", Properties: &AnimationModifyProperties{Duration: ptrFloat64(1.0)}}},
		{"MODIFY", ManageAnimationsInput{AnimationID: "a", Properties: &AnimationModifyProperties{Duration: ptrFloat64(1.0)}}},
		{"delete", ManageAnimationsInput{AnimationID: "a"}},
		{"DELETE", ManageAnimationsInput{AnimationID: "a"}},
	}

	for _, tt := range actionTests {
		t.Run(tt.action, func(t *testing.T) {
			input := ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         tt.action,
				AnimationIDs:   tt.extraArgs.AnimationIDs,
				AnimationID:    tt.extraArgs.AnimationID,
				Properties:     tt.extraArgs.Properties,
			}

			_, err := tools.ManageAnimations(ctx, tokenSource, input)
			if !errors.Is(err, ErrManageAnimationsNotSupported) {
				t.Errorf("expected ErrManageAnimationsNotSupported for action %s, got %v", tt.action, err)
			}
		})
	}
}

func TestManageAnimations_ValidPropertiesWithAllOptions(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test valid combinations of properties that pass validation
	validProperties := []AnimationModifyProperties{
		{AnimationType: "APPEAR"},
		{AnimationType: "FADE_IN"},
		{AnimationType: "FLY_IN", Direction: "FROM_LEFT"},
		{AnimationType: "ZOOM_IN"},
		{AnimationType: "FADE_OUT"},
		{AnimationType: "FLY_OUT", Direction: "FROM_RIGHT"},
		{AnimationType: "ZOOM_OUT"},
		{AnimationType: "SPIN"},
		{AnimationType: "FLOAT"},
		{AnimationType: "BOUNCE"},
		{AnimationCategory: "ENTRANCE"},
		{AnimationCategory: "EXIT"},
		{AnimationCategory: "EMPHASIS"},
		{Direction: "FROM_LEFT"},
		{Direction: "FROM_RIGHT"},
		{Direction: "FROM_TOP"},
		{Direction: "FROM_BOTTOM"},
		{Trigger: "ON_CLICK"},
		{Trigger: "AFTER_PREVIOUS"},
		{Trigger: "WITH_PREVIOUS"},
		{Duration: ptrFloat64(0.0)},
		{Duration: ptrFloat64(30.0)},
		{Duration: ptrFloat64(60.0)},
		{Delay: ptrFloat64(0.0)},
		{Delay: ptrFloat64(30.0)},
		{Delay: ptrFloat64(60.0)},
	}

	for i, props := range validProperties {
		t.Run("valid_properties_"+string(rune('A'+i)), func(t *testing.T) {
			propsCopy := props
			input := ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "modify",
				AnimationID:    "anim-1",
				Properties:     &propsCopy,
			}

			_, err := tools.ManageAnimations(ctx, tokenSource, input)
			// Valid properties should reach the API limitation error
			if !errors.Is(err, ErrManageAnimationsNotSupported) {
				t.Errorf("expected ErrManageAnimationsNotSupported for valid properties, got %v", err)
			}
		})
	}
}

func TestManageAnimations_ErrorMessageContainsIssueTracker(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	input := ManageAnimationsInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		Action:         "list",
	}

	_, err := tools.ManageAnimations(ctx, tokenSource, input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error message contains helpful information
	errMsg := err.Error()
	expectedContents := []string{
		"Google Slides API does not provide endpoints",
		"listing",
		"reordering",
		"modifying",
		"deleting",
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

func TestManageAnimations_SlideReferenceOptions(t *testing.T) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tools := NewTools(DefaultToolsConfig(), nil)

	// Test that both slide_index and slide_id work
	testCases := []struct {
		name       string
		slideIndex int
		slideID    string
	}{
		{"slide_index only", 1, ""},
		{"slide_id only", 0, "slide-123"},
		{"both provided", 1, "slide-123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := ManageAnimationsInput{
				PresentationID: "test-presentation",
				SlideIndex:     tc.slideIndex,
				SlideID:        tc.slideID,
				Action:         "list",
			}

			_, err := tools.ManageAnimations(ctx, tokenSource, input)
			if !errors.Is(err, ErrManageAnimationsNotSupported) {
				t.Errorf("expected ErrManageAnimationsNotSupported, got %v", err)
			}
		})
	}
}
