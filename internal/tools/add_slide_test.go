package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestAddSlide_Success(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{
							Name: "BLANK",
						},
					},
					{
						ObjectId: "layout-title",
						LayoutProperties: &slides.LayoutProperties{
							Name: "TITLE",
						},
					},
					{
						ObjectId: "layout-title-and-body",
						LayoutProperties: &slides.LayoutProperties{
							Name: "TITLE_AND_BODY",
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if presentationID != "test-pres-id" {
				t.Errorf("expected presentation ID 'test-pres-id', got '%s'", presentationID)
			}
			if len(requests) != 1 {
				t.Errorf("expected 1 request, got %d", len(requests))
			}
			if requests[0].CreateSlide == nil {
				t.Fatal("expected CreateSlide request")
			}
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{
						CreateSlide: &slides.CreateSlideResponse{
							ObjectId: "new-slide-id",
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.AddSlide(context.Background(), tokenSource, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "BLANK",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SlideID != "new-slide-id" {
		t.Errorf("expected slide ID 'new-slide-id', got '%s'", output.SlideID)
	}
	// Default position is end (after 2 existing slides)
	if output.SlideIndex != 3 {
		t.Errorf("expected slide index 3, got %d", output.SlideIndex)
	}
}

func TestAddSlide_WithPosition(t *testing.T) {
	tests := []struct {
		name              string
		position          int
		numExistingSlides int
		expectedIndex     int64 // 0-based API index
		expectedOutput    int   // 1-based output index
	}{
		{
			name:              "position at beginning",
			position:          1,
			numExistingSlides: 3,
			expectedIndex:     0,
			expectedOutput:    1,
		},
		{
			name:              "position in middle",
			position:          2,
			numExistingSlides: 3,
			expectedIndex:     1,
			expectedOutput:    2,
		},
		{
			name:              "position at end explicitly",
			position:          3,
			numExistingSlides: 3,
			expectedIndex:     2,
			expectedOutput:    3,
		},
		{
			name:              "position beyond end defaults to end",
			position:          10,
			numExistingSlides: 3,
			expectedIndex:     3,
			expectedOutput:    4,
		},
		{
			name:              "position zero defaults to end",
			position:          0,
			numExistingSlides: 3,
			expectedIndex:     3,
			expectedOutput:    4,
		},
		{
			name:              "negative position defaults to end",
			position:          -1,
			numExistingSlides: 3,
			expectedIndex:     3,
			expectedOutput:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedIndex int64

			existingSlides := make([]*slides.Page, tt.numExistingSlides)
			for i := 0; i < tt.numExistingSlides; i++ {
				existingSlides[i] = &slides.Page{ObjectId: "existing-slide"}
			}

			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return &slides.Presentation{
						PresentationId: presentationID,
						Slides:         existingSlides,
						Layouts: []*slides.Page{
							{
								ObjectId: "layout-blank",
								LayoutProperties: &slides.LayoutProperties{
									Name: "BLANK",
								},
							},
						},
					}, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedIndex = requests[0].CreateSlide.InsertionIndex
					return &slides.BatchUpdatePresentationResponse{
						Replies: []*slides.Response{
							{
								CreateSlide: &slides.CreateSlideResponse{
									ObjectId: "new-slide-id",
								},
							},
						},
					}, nil
				},
			}

			factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			}

			tools := NewTools(DefaultToolsConfig(), factory)
			tokenSource := &mockTokenSource{}

			output, err := tools.AddSlide(context.Background(), tokenSource, AddSlideInput{
				PresentationID: "test-pres-id",
				Position:       tt.position,
				Layout:         "BLANK",
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if capturedIndex != tt.expectedIndex {
				t.Errorf("expected insertion index %d, got %d", tt.expectedIndex, capturedIndex)
			}

			if output.SlideIndex != tt.expectedOutput {
				t.Errorf("expected output index %d, got %d", tt.expectedOutput, output.SlideIndex)
			}
		})
	}
}

func TestAddSlide_LayoutTypes(t *testing.T) {
	validLayouts := []string{
		"BLANK",
		"CAPTION_ONLY",
		"TITLE",
		"TITLE_AND_BODY",
		"TITLE_AND_TWO_COLUMNS",
		"TITLE_ONLY",
		"ONE_COLUMN_TEXT",
		"MAIN_POINT",
		"BIG_NUMBER",
		"SECTION_HEADER",
		"SECTION_TITLE_AND_DESCRIPTION",
	}

	for _, layout := range validLayouts {
		t.Run(layout, func(t *testing.T) {
			var capturedLayoutRef *slides.LayoutReference

			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return &slides.Presentation{
						PresentationId: presentationID,
						Slides:         []*slides.Page{},
						Layouts: []*slides.Page{
							{
								ObjectId: "layout-" + layout,
								LayoutProperties: &slides.LayoutProperties{
									Name: layout,
								},
							},
						},
					}, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedLayoutRef = requests[0].CreateSlide.SlideLayoutReference
					return &slides.BatchUpdatePresentationResponse{
						Replies: []*slides.Response{
							{CreateSlide: &slides.CreateSlideResponse{ObjectId: "new-slide"}},
						},
					}, nil
				},
			}

			factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			}

			tools := NewTools(DefaultToolsConfig(), factory)

			_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
				PresentationID: "test-pres-id",
				Layout:         layout,
			})

			if err != nil {
				t.Fatalf("unexpected error for layout %s: %v", layout, err)
			}

			if capturedLayoutRef == nil {
				t.Fatal("expected layout reference to be set")
			}
			if capturedLayoutRef.LayoutId != "layout-"+layout {
				t.Errorf("expected layout ID 'layout-%s', got '%s'", layout, capturedLayoutRef.LayoutId)
			}
		})
	}
}

func TestAddSlide_InvalidLayout(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{},
				Layouts:        []*slides.Page{},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "INVALID_LAYOUT_TYPE",
	})

	if err == nil {
		t.Fatal("expected error for invalid layout")
	}

	if !errors.Is(err, ErrInvalidLayout) {
		t.Errorf("expected ErrInvalidLayout, got %v", err)
	}
}

func TestAddSlide_MissingLayout(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "",
	})

	if err == nil {
		t.Fatal("expected error for missing layout")
	}

	if !errors.Is(err, ErrInvalidLayout) {
		t.Errorf("expected ErrInvalidLayout, got %v", err)
	}
}

func TestAddSlide_MissingPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		Layout: "BLANK",
	})

	if err == nil {
		t.Fatal("expected error for missing presentation ID")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestAddSlide_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "nonexistent-pres-id",
		Layout:         "BLANK",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestAddSlide_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: forbidden")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "restricted-pres-id",
		Layout:         "BLANK",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestAddSlide_BatchUpdateFails(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{
							Name: "BLANK",
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("batch update failed")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "BLANK",
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrAddSlideFailed) {
		t.Errorf("expected ErrAddSlideFailed, got %v", err)
	}
}

func TestAddSlide_FallbackToFirstLayout(t *testing.T) {
	var capturedLayoutRef *slides.LayoutReference

	// Presentation has layouts but none match the requested type
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{},
				Layouts: []*slides.Page{
					{
						ObjectId: "custom-layout-1",
						LayoutProperties: &slides.LayoutProperties{
							Name: "CUSTOM_LAYOUT",
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedLayoutRef = requests[0].CreateSlide.SlideLayoutReference
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{CreateSlide: &slides.CreateSlideResponse{ObjectId: "new-slide"}},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "BLANK",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to first layout
	if capturedLayoutRef == nil {
		t.Fatal("expected layout reference to be set")
	}
	if capturedLayoutRef.LayoutId != "custom-layout-1" {
		t.Errorf("expected fallback to first layout, got '%s'", capturedLayoutRef.LayoutId)
	}
}

func TestAddSlide_UsePredefinedLayout(t *testing.T) {
	var capturedLayoutRef *slides.LayoutReference

	// Presentation has no layouts
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{},
				Layouts:        []*slides.Page{}, // Empty layouts
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedLayoutRef = requests[0].CreateSlide.SlideLayoutReference
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{CreateSlide: &slides.CreateSlideResponse{ObjectId: "new-slide"}},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "BLANK",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use predefined layout
	if capturedLayoutRef == nil {
		t.Fatal("expected layout reference to be set")
	}
	if capturedLayoutRef.PredefinedLayout != "BLANK" {
		t.Errorf("expected predefined layout 'BLANK', got '%s'", capturedLayoutRef.PredefinedLayout)
	}
}

func TestAddSlide_ServiceCreationFails(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.AddSlide(context.Background(), &mockTokenSource{}, AddSlideInput{
		PresentationID: "test-pres-id",
		Layout:         "BLANK",
	})

	if err == nil {
		t.Fatal("expected error for service creation failure")
	}

	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestFindLayoutByType(t *testing.T) {
	layouts := []*slides.Page{
		{
			ObjectId: "layout-1",
			LayoutProperties: &slides.LayoutProperties{
				Name: "BLANK",
			},
		},
		{
			ObjectId: "layout-2",
			LayoutProperties: &slides.LayoutProperties{
				Name: "TITLE_AND_BODY",
			},
		},
		{
			ObjectId:         "layout-no-props",
			LayoutProperties: nil,
		},
	}

	tests := []struct {
		layoutType string
		expectedID string
	}{
		{"BLANK", "layout-1"},
		{"TITLE_AND_BODY", "layout-2"},
		{"NONEXISTENT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.layoutType, func(t *testing.T) {
			result := findLayoutByType(layouts, tt.layoutType)
			if result != tt.expectedID {
				t.Errorf("expected '%s', got '%s'", tt.expectedID, result)
			}
		})
	}
}
