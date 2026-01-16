package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSetTransition_APINotSupported(t *testing.T) {
	// Create tools instance
	tools := NewTools(DefaultToolsConfig(), nil)
	ctx := context.Background()
	tokenSource := &mockTokenSource{}

	tests := []struct {
		name        string
		input       SetTransitionInput
		wantErr     error
		wantErrMsg  string
	}{
		{
			name: "single slide transition returns API not supported error",
			input: SetTransitionInput{
				PresentationID: "test-presentation-id",
				SlideIndex:     1,
				TransitionType: "FADE",
			},
			wantErr: ErrTransitionNotSupported,
		},
		{
			name: "all slides transition returns API not supported error",
			input: SetTransitionInput{
				PresentationID: "test-presentation-id",
				TransitionType: "DISSOLVE",
			},
			wantErr: ErrTransitionNotSupported,
		},
		{
			name: "transition with duration returns API not supported error",
			input: SetTransitionInput{
				PresentationID: "test-presentation-id",
				SlideIndex:     1,
				TransitionType: "SLIDE_FROM_RIGHT",
				Duration:       ptrFloat64(0.5),
			},
			wantErr: ErrTransitionNotSupported,
		},
		{
			name: "NONE transition returns API not supported error",
			input: SetTransitionInput{
				PresentationID: "test-presentation-id",
				SlideID:        "slide-123",
				TransitionType: "NONE",
			},
			wantErr: ErrTransitionNotSupported,
		},
		{
			name: "different transition types all return API not supported",
			input: SetTransitionInput{
				PresentationID: "test-presentation-id",
				TransitionType: "CUBE",
			},
			wantErr: ErrTransitionNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.SetTransition(ctx, tokenSource, tt.input)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSetTransition_InputValidation(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	ctx := context.Background()
	tokenSource := &mockTokenSource{}

	tests := []struct {
		name    string
		input   SetTransitionInput
		wantErr error
	}{
		{
			name: "missing presentation_id",
			input: SetTransitionInput{
				TransitionType: "FADE",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "empty transition type",
			input: SetTransitionInput{
				PresentationID: "test-id",
				TransitionType: "",
			},
			wantErr: ErrInvalidTransitionType,
		},
		{
			name: "invalid transition type",
			input: SetTransitionInput{
				PresentationID: "test-id",
				TransitionType: "INVALID_TYPE",
			},
			wantErr: ErrInvalidTransitionType,
		},
		{
			name: "negative duration",
			input: SetTransitionInput{
				PresentationID: "test-id",
				TransitionType: "FADE",
				Duration:       ptrFloat64(-1.0),
			},
			wantErr: ErrInvalidTransitionDuration,
		},
		{
			name: "duration too long",
			input: SetTransitionInput{
				PresentationID: "test-id",
				TransitionType: "FADE",
				Duration:       ptrFloat64(15.0),
			},
			wantErr: ErrInvalidTransitionDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.SetTransition(ctx, tokenSource, tt.input)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSetTransition_AllTransitionTypesValidate(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	ctx := context.Background()
	tokenSource := &mockTokenSource{}

	// All valid transition types should pass validation but return API not supported error
	transitionTypes := []string{
		"NONE",
		"FADE",
		"SLIDE_FROM_RIGHT",
		"SLIDE_FROM_LEFT",
		"SLIDE_FROM_TOP",
		"SLIDE_FROM_BOTTOM",
		"FLIP",
		"CUBE",
		"GALLERY",
		"ZOOM",
		"DISSOLVE",
	}

	for _, transitionType := range transitionTypes {
		t.Run("transition_type_"+transitionType, func(t *testing.T) {
			input := SetTransitionInput{
				PresentationID: "test-id",
				TransitionType: transitionType,
			}

			_, err := tools.SetTransition(ctx, tokenSource, input)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			// Should get API not supported error, not invalid type error
			if !errors.Is(err, ErrTransitionNotSupported) {
				t.Errorf("expected ErrTransitionNotSupported for type %s, got %v", transitionType, err)
			}
		})
	}
}

func TestSetTransition_CaseInsensitiveTransitionType(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	ctx := context.Background()
	tokenSource := &mockTokenSource{}

	// Transition types should be case-insensitive
	caseVariants := []string{
		"fade",
		"FADE",
		"Fade",
		"FaDe",
	}

	for _, variant := range caseVariants {
		t.Run("case_"+variant, func(t *testing.T) {
			input := SetTransitionInput{
				PresentationID: "test-id",
				TransitionType: variant,
			}

			_, err := tools.SetTransition(ctx, tokenSource, input)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			// Should get API not supported error (not invalid type due to case)
			if !errors.Is(err, ErrTransitionNotSupported) {
				t.Errorf("expected ErrTransitionNotSupported for variant %s, got %v", variant, err)
			}
		})
	}
}

func TestSetTransition_ErrorMessageIsInformative(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	ctx := context.Background()
	tokenSource := &mockTokenSource{}

	input := SetTransitionInput{
		PresentationID: "test-id",
		TransitionType: "FADE",
	}

	_, err := tools.SetTransition(ctx, tokenSource, input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()

	// Error message should be informative and provide alternatives
	expectedPhrases := []string{
		"Google Slides API",
		"not",
		"transition",
		"user interface",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(errMsg, phrase) {
			t.Errorf("error message should contain '%s', got: %s", phrase, errMsg)
		}
	}
}
