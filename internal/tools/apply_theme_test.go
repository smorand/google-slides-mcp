package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestApplyTheme(t *testing.T) {
	tests := []struct {
		name           string
		input          ApplyThemeInput
		mockService    *mockSlidesService
		expectedError  error
		expectedOutput *ApplyThemeOutput
		checkOutput    func(t *testing.T, output *ApplyThemeOutput)
	}{
		{
			name: "successfully applies theme from presentation",
			input: ApplyThemeInput{
				PresentationID:       "target-presentation-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-presentation-id",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "source-presentation-id" {
						return createPresentationWithColorScheme("source-presentation-id", "source-master-id"), nil
					}
					return createPresentationWithColorScheme("target-presentation-id", "target-master-id"), nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					// Verify the request structure
					if len(requests) != 1 {
						t.Errorf("expected 1 request, got %d", len(requests))
					}
					if requests[0].UpdatePageProperties == nil {
						t.Error("expected UpdatePageProperties request")
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			},
			checkOutput: func(t *testing.T, output *ApplyThemeOutput) {
				if !output.Success {
					t.Error("expected success to be true")
				}
				if output.SourceMasterID != "source-master-id" {
					t.Errorf("expected source master id 'source-master-id', got '%s'", output.SourceMasterID)
				}
				if output.TargetMasterID != "target-master-id" {
					t.Errorf("expected target master id 'target-master-id', got '%s'", output.TargetMasterID)
				}
				if len(output.UpdatedProperties) == 0 {
					t.Error("expected some updated properties")
				}
			},
		},
		{
			name: "theme source is case-insensitive",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "PRESENTATION",
				SourcePresentationID: "source-id",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return createPresentationWithColorScheme(presentationID, presentationID+"-master"), nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			},
			checkOutput: func(t *testing.T, output *ApplyThemeOutput) {
				if !output.Success {
					t.Error("expected success to be true")
				}
			},
		},
		{
			name: "gallery theme returns not supported error",
			input: ApplyThemeInput{
				PresentationID: "target-id",
				ThemeSource:    "gallery",
				ThemeID:        "theme-123",
			},
			mockService:   &mockSlidesService{},
			expectedError: ErrGalleryNotSupported,
		},
		{
			name: "gallery theme case-insensitive",
			input: ApplyThemeInput{
				PresentationID: "target-id",
				ThemeSource:    "GALLERY",
				ThemeID:        "theme-123",
			},
			mockService:   &mockSlidesService{},
			expectedError: ErrGalleryNotSupported,
		},
		{
			name: "missing presentation_id returns error",
			input: ApplyThemeInput{
				PresentationID:       "",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-id",
			},
			mockService:   &mockSlidesService{},
			expectedError: ErrInvalidPresentationID,
		},
		{
			name: "missing theme_source returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "",
				SourcePresentationID: "source-id",
			},
			mockService:   &mockSlidesService{},
			expectedError: ErrInvalidThemeSource,
		},
		{
			name: "invalid theme_source returns error",
			input: ApplyThemeInput{
				PresentationID: "target-id",
				ThemeSource:    "invalid",
			},
			mockService:   &mockSlidesService{},
			expectedError: ErrInvalidThemeSource,
		},
		{
			name: "presentation source without source_presentation_id returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "",
			},
			mockService:   &mockSlidesService{},
			expectedError: ErrInvalidSourcePresID,
		},
		{
			name: "source presentation not found returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "nonexistent-source",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "nonexistent-source" {
						return nil, errors.New("googleapi: Error 404: notFound")
					}
					return createPresentationWithColorScheme(presentationID, "master-id"), nil
				},
			},
			expectedError: ErrSourceNotFound,
		},
		{
			name: "access denied to source presentation returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "restricted-source",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "restricted-source" {
						return nil, errors.New("googleapi: Error 403: forbidden")
					}
					return createPresentationWithColorScheme(presentationID, "master-id"), nil
				},
			},
			expectedError: ErrAccessDenied,
		},
		{
			name: "target presentation not found returns error",
			input: ApplyThemeInput{
				PresentationID:       "nonexistent-target",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-id",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "nonexistent-target" {
						return nil, errors.New("googleapi: Error 404: notFound")
					}
					return createPresentationWithColorScheme(presentationID, "master-id"), nil
				},
			},
			expectedError: ErrPresentationNotFound,
		},
		{
			name: "source presentation without masters returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-no-masters",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "source-no-masters" {
						return &slides.Presentation{
							PresentationId: presentationID,
							Masters:        []*slides.Page{},
						}, nil
					}
					return createPresentationWithColorScheme(presentationID, "master-id"), nil
				},
			},
			expectedError: ErrNoMasterInSource,
		},
		{
			name: "source presentation without color scheme returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-no-colors",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "source-no-colors" {
						return &slides.Presentation{
							PresentationId: presentationID,
							Masters: []*slides.Page{
								{
									ObjectId:       "master-id",
									PageProperties: &slides.PageProperties{},
								},
							},
						}, nil
					}
					return createPresentationWithColorScheme(presentationID, "master-id"), nil
				},
			},
			expectedError: ErrNoColorScheme,
		},
		{
			name: "target presentation without masters returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-no-masters",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-id",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == "target-no-masters" {
						return &slides.Presentation{
							PresentationId: presentationID,
							Masters:        []*slides.Page{},
						}, nil
					}
					return createPresentationWithColorScheme(presentationID, "master-id"), nil
				},
			},
			expectedError: ErrNoMasterInTarget,
		},
		{
			name: "batch update failure returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-id",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return createPresentationWithColorScheme(presentationID, presentationID+"-master"), nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return nil, errors.New("batch update failed")
				},
			},
			expectedError: ErrApplyThemeFailed,
		},
		{
			name: "batch update access denied returns error",
			input: ApplyThemeInput{
				PresentationID:       "target-id",
				ThemeSource:          "presentation",
				SourcePresentationID: "source-id",
			},
			mockService: &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return createPresentationWithColorScheme(presentationID, presentationID+"-master"), nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return nil, errors.New("googleapi: Error 403: forbidden")
				},
			},
			expectedError: ErrAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return tt.mockService, nil
			})

			output, err := tools.ApplyTheme(context.Background(), nil, tt.input)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tt.expectedError)
				}
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, output)
			}
		})
	}
}

func TestApplyTheme_VerifyRequestStructure(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return createPresentationWithColorScheme(presentationID, presentationID+"-master"), nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	_, err := tools.ApplyTheme(context.Background(), nil, ApplyThemeInput{
		PresentationID:       "target-id",
		ThemeSource:          "presentation",
		SourcePresentationID: "source-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.UpdatePageProperties == nil {
		t.Fatal("expected UpdatePageProperties request")
	}

	uppr := req.UpdatePageProperties
	if uppr.ObjectId != "target-id-master" {
		t.Errorf("expected object_id 'target-id-master', got '%s'", uppr.ObjectId)
	}

	if uppr.Fields != "colorScheme" {
		t.Errorf("expected fields 'colorScheme', got '%s'", uppr.Fields)
	}

	if uppr.PageProperties == nil || uppr.PageProperties.ColorScheme == nil {
		t.Fatal("expected color scheme in page properties")
	}

	// Verify color scheme has colors
	colorScheme := uppr.PageProperties.ColorScheme
	if len(colorScheme.Colors) == 0 {
		t.Error("expected colors in color scheme")
	}

	// Verify all required color types are present
	colorTypes := make(map[string]bool)
	for _, color := range colorScheme.Colors {
		colorTypes[color.Type] = true
	}

	requiredTypes := []string{"DARK1", "LIGHT1", "DARK2", "LIGHT2", "ACCENT1", "ACCENT2", "ACCENT3", "ACCENT4", "ACCENT5", "ACCENT6", "HYPERLINK", "FOLLOWED_HYPERLINK"}
	for _, reqType := range requiredTypes {
		if !colorTypes[reqType] {
			t.Errorf("expected color type %s to be present", reqType)
		}
	}
}

func TestBuildColorSchemeFromSource(t *testing.T) {
	tests := []struct {
		name           string
		source         *slides.ColorScheme
		expectedColors int
		expectedNil    bool
	}{
		{
			name:        "nil source returns nil",
			source:      nil,
			expectedNil: true,
		},
		{
			name: "empty colors returns nil",
			source: &slides.ColorScheme{
				Colors: []*slides.ThemeColorPair{},
			},
			expectedNil: true,
		},
		{
			name: "extracts all 12 theme colors",
			source: &slides.ColorScheme{
				Colors: []*slides.ThemeColorPair{
					{Type: "DARK1", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 0}},
					{Type: "LIGHT1", Color: &slides.RgbColor{Red: 1, Green: 1, Blue: 1}},
					{Type: "DARK2", Color: &slides.RgbColor{Red: 0.1, Green: 0.1, Blue: 0.1}},
					{Type: "LIGHT2", Color: &slides.RgbColor{Red: 0.9, Green: 0.9, Blue: 0.9}},
					{Type: "ACCENT1", Color: &slides.RgbColor{Red: 1, Green: 0, Blue: 0}},
					{Type: "ACCENT2", Color: &slides.RgbColor{Red: 0, Green: 1, Blue: 0}},
					{Type: "ACCENT3", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 1}},
					{Type: "ACCENT4", Color: &slides.RgbColor{Red: 1, Green: 1, Blue: 0}},
					{Type: "ACCENT5", Color: &slides.RgbColor{Red: 1, Green: 0, Blue: 1}},
					{Type: "ACCENT6", Color: &slides.RgbColor{Red: 0, Green: 1, Blue: 1}},
					{Type: "HYPERLINK", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 0.8}},
					{Type: "FOLLOWED_HYPERLINK", Color: &slides.RgbColor{Red: 0.5, Green: 0, Blue: 0.5}},
				},
			},
			expectedColors: 12,
		},
		{
			name: "ignores non-editable color types",
			source: &slides.ColorScheme{
				Colors: []*slides.ThemeColorPair{
					{Type: "DARK1", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 0}},
					{Type: "LIGHT1", Color: &slides.RgbColor{Red: 1, Green: 1, Blue: 1}},
					{Type: "TEXT1", Color: &slides.RgbColor{Red: 0.5, Green: 0.5, Blue: 0.5}}, // Not in the 12
					{Type: "BACKGROUND1", Color: &slides.RgbColor{Red: 0.9, Green: 0.9, Blue: 0.9}}, // Not in the 12
				},
			},
			expectedColors: 2,
		},
		{
			name: "skips colors with nil Color field",
			source: &slides.ColorScheme{
				Colors: []*slides.ThemeColorPair{
					{Type: "DARK1", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 0}},
					{Type: "LIGHT1", Color: nil}, // Should be skipped
					{Type: "DARK2", Color: &slides.RgbColor{Red: 0.1, Green: 0.1, Blue: 0.1}},
				},
			},
			expectedColors: 2,
		},
		{
			name: "skips colors with empty Type field",
			source: &slides.ColorScheme{
				Colors: []*slides.ThemeColorPair{
					{Type: "DARK1", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 0}},
					{Type: "", Color: &slides.RgbColor{Red: 1, Green: 1, Blue: 1}}, // Should be skipped
				},
			},
			expectedColors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildColorSchemeFromSource(tt.source)

			if tt.expectedNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if len(result.Colors) != tt.expectedColors {
				t.Errorf("expected %d colors, got %d", tt.expectedColors, len(result.Colors))
			}
		})
	}
}

func TestCloneRgbColor(t *testing.T) {
	tests := []struct {
		name   string
		color  *slides.RgbColor
		isNil  bool
	}{
		{
			name:  "nil returns nil",
			color: nil,
			isNil: true,
		},
		{
			name: "clones color correctly",
			color: &slides.RgbColor{
				Red:   0.5,
				Green: 0.6,
				Blue:  0.7,
			},
			isNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cloneRgbColor(tt.color)

			if tt.isNil {
				if result != nil {
					t.Error("expected nil")
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Verify values are copied
			if result.Red != tt.color.Red {
				t.Errorf("expected red %f, got %f", tt.color.Red, result.Red)
			}
			if result.Green != tt.color.Green {
				t.Errorf("expected green %f, got %f", tt.color.Green, result.Green)
			}
			if result.Blue != tt.color.Blue {
				t.Errorf("expected blue %f, got %f", tt.color.Blue, result.Blue)
			}

			// Verify it's a separate instance
			if result == tt.color {
				t.Error("expected a new instance, not the same pointer")
			}
		})
	}
}

// createPresentationWithColorScheme creates a test presentation with a color scheme.
func createPresentationWithColorScheme(presentationID, masterID string) *slides.Presentation {
	return &slides.Presentation{
		PresentationId: presentationID,
		Masters: []*slides.Page{
			{
				ObjectId: masterID,
				PageProperties: &slides.PageProperties{
					ColorScheme: &slides.ColorScheme{
						Colors: []*slides.ThemeColorPair{
							{Type: "DARK1", Color: &slides.RgbColor{Red: 0, Green: 0, Blue: 0}},
							{Type: "LIGHT1", Color: &slides.RgbColor{Red: 1, Green: 1, Blue: 1}},
							{Type: "DARK2", Color: &slides.RgbColor{Red: 0.2, Green: 0.2, Blue: 0.2}},
							{Type: "LIGHT2", Color: &slides.RgbColor{Red: 0.95, Green: 0.95, Blue: 0.95}},
							{Type: "ACCENT1", Color: &slides.RgbColor{Red: 0.26, Green: 0.52, Blue: 0.96}},
							{Type: "ACCENT2", Color: &slides.RgbColor{Red: 0.85, Green: 0.18, Blue: 0.18}},
							{Type: "ACCENT3", Color: &slides.RgbColor{Red: 0.98, Green: 0.74, Blue: 0.02}},
							{Type: "ACCENT4", Color: &slides.RgbColor{Red: 0.13, Green: 0.59, Blue: 0.95}},
							{Type: "ACCENT5", Color: &slides.RgbColor{Red: 0.4, Green: 0.73, Blue: 0.42}},
							{Type: "ACCENT6", Color: &slides.RgbColor{Red: 1, Green: 0.43, Blue: 0}},
							{Type: "HYPERLINK", Color: &slides.RgbColor{Red: 0.06, Green: 0.46, Blue: 0.88}},
							{Type: "FOLLOWED_HYPERLINK", Color: &slides.RgbColor{Red: 0.66, Green: 0.13, Blue: 0.7}},
						},
					},
				},
				MasterProperties: &slides.MasterProperties{
					DisplayName: "Default Master",
				},
			},
		},
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
			},
		},
	}
}
