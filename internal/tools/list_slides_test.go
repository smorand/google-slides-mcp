package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestListSlides_Success(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			if presentationID != "test-presentation-id" {
				t.Errorf("expected presentation ID 'test-presentation-id', got '%s'", presentationID)
			}
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						SlideProperties: &slides.SlideProperties{
							LayoutObjectId: "layout-1",
						},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "title-1",
								Shape: &slides.Shape{
									ShapeType:   "TEXT_BOX",
									Placeholder: &slides.Placeholder{Type: "TITLE"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Slide One Title"}},
										},
									},
								},
							},
							{
								ObjectId: "body-1",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
								},
							},
						},
					},
					{
						ObjectId: "slide-2",
						SlideProperties: &slides.SlideProperties{
							LayoutObjectId: "layout-2",
							NotesPage: &slides.Page{
								PageElements: []*slides.PageElement{
									{
										ObjectId: "notes-shape-1",
										Shape: &slides.Shape{
											Placeholder: &slides.Placeholder{Type: "BODY"},
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "Speaker notes for slide 2"}},
												},
											},
										},
									},
								},
							},
						},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
					{
						ObjectId: "slide-3",
						SlideProperties: &slides.SlideProperties{
							LayoutObjectId: "layout-3",
						},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "video-1",
								Video:    &slides.Video{},
							},
						},
					},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-1",
						LayoutProperties: &slides.LayoutProperties{
							DisplayName:    "Title Slide",
							MasterObjectId: "master-1",
							Name:           "TITLE",
						},
					},
					{
						ObjectId: "layout-2",
						LayoutProperties: &slides.LayoutProperties{
							DisplayName:    "Content",
							MasterObjectId: "master-1",
							Name:           "TITLE_AND_BODY",
						},
					},
					{
						ObjectId: "layout-3",
						LayoutProperties: &slides.LayoutProperties{
							DisplayName:    "Blank",
							MasterObjectId: "master-1",
							Name:           "BLANK",
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

	output, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify basic presentation info
	if output.PresentationID != "test-presentation-id" {
		t.Errorf("expected presentation ID 'test-presentation-id', got '%s'", output.PresentationID)
	}
	if output.Title != "Test Presentation" {
		t.Errorf("expected title 'Test Presentation', got '%s'", output.Title)
	}

	// Verify slides count
	if len(output.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(output.Slides))
	}

	// Verify statistics
	if output.Statistics.TotalSlides != 3 {
		t.Errorf("expected total_slides 3, got %d", output.Statistics.TotalSlides)
	}
	if output.Statistics.SlidesWithNotes != 1 {
		t.Errorf("expected slides_with_notes 1, got %d", output.Statistics.SlidesWithNotes)
	}
	if output.Statistics.SlidesWithVideos != 1 {
		t.Errorf("expected slides_with_videos 1, got %d", output.Statistics.SlidesWithVideos)
	}

	// Verify slide 1
	slide1 := output.Slides[0]
	if slide1.Index != 1 {
		t.Errorf("expected slide 1 index 1, got %d", slide1.Index)
	}
	if slide1.SlideID != "slide-1" {
		t.Errorf("expected slide 1 ID 'slide-1', got '%s'", slide1.SlideID)
	}
	if slide1.LayoutType != "TITLE" {
		t.Errorf("expected slide 1 layout type 'TITLE', got '%s'", slide1.LayoutType)
	}
	if slide1.Title != "Slide One Title" {
		t.Errorf("expected slide 1 title 'Slide One Title', got '%s'", slide1.Title)
	}
	if slide1.ObjectCount != 2 {
		t.Errorf("expected slide 1 object count 2, got %d", slide1.ObjectCount)
	}

	// Verify slide 2
	slide2 := output.Slides[1]
	if slide2.Index != 2 {
		t.Errorf("expected slide 2 index 2, got %d", slide2.Index)
	}
	if slide2.LayoutType != "TITLE_AND_BODY" {
		t.Errorf("expected slide 2 layout type 'TITLE_AND_BODY', got '%s'", slide2.LayoutType)
	}

	// Verify slide 3
	slide3 := output.Slides[2]
	if slide3.Index != 3 {
		t.Errorf("expected slide 3 index 3, got %d", slide3.Index)
	}
	if slide3.LayoutType != "BLANK" {
		t.Errorf("expected slide 3 layout type 'BLANK', got '%s'", slide3.LayoutType)
	}
}

func TestListSlides_EmptyPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got: %v", err)
	}
}

func TestListSlides_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: Requested entity was not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "nonexistent-id",
	})

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestListSlides_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: The caller does not have permission")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "forbidden-id",
	})

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestListSlides_ServiceFactoryError(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "test-id",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestListSlides_WithThumbnails(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements:    []*slides.PageElement{},
					},
				},
			}, nil
		},
		GetThumbnailFunc: func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
			// Return a thumbnail with an empty URL (can't test HTTP fetching without a server)
			return &slides.Thumbnail{
				ContentUrl: "",
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID:    "test-presentation-id",
		IncludeThumbnails: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Thumbnail should be empty since we couldn't fetch it (empty URL)
	if output.Slides[0].ThumbnailBase64 != "" {
		t.Error("expected empty thumbnail due to fetch failure")
	}
}

func TestListSlides_IndexIs1Based(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1", SlideProperties: &slides.SlideProperties{}},
					{ObjectId: "slide-2", SlideProperties: &slides.SlideProperties{}},
					{ObjectId: "slide-3", SlideProperties: &slides.SlideProperties{}},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "test-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify 1-based indexing
	for i, slide := range output.Slides {
		expectedIndex := i + 1
		if slide.Index != expectedIndex {
			t.Errorf("slide %d: expected index %d, got %d", i, expectedIndex, slide.Index)
		}
	}
}

func TestListSlides_TitleExtraction(t *testing.T) {
	testCases := []struct {
		name          string
		slide         *slides.Page
		expectedTitle string
	}{
		{
			name: "slide with TITLE placeholder",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "title-shape",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{Type: "TITLE"},
							Text: &slides.TextContent{
								TextElements: []*slides.TextElement{
									{TextRun: &slides.TextRun{Content: "My Title"}},
								},
							},
						},
					},
				},
			},
			expectedTitle: "My Title",
		},
		{
			name: "slide with CENTERED_TITLE placeholder",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "title-shape",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{Type: "CENTERED_TITLE"},
							Text: &slides.TextContent{
								TextElements: []*slides.TextElement{
									{TextRun: &slides.TextRun{Content: "Centered Title"}},
								},
							},
						},
					},
				},
			},
			expectedTitle: "Centered Title",
		},
		{
			name: "slide without title placeholder",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "body-shape",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{Type: "BODY"},
							Text: &slides.TextContent{
								TextElements: []*slides.TextElement{
									{TextRun: &slides.TextRun{Content: "Body text"}},
								},
							},
						},
					},
				},
			},
			expectedTitle: "",
		},
		{
			name: "slide with no page elements",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
				PageElements:    []*slides.PageElement{},
			},
			expectedTitle: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			title := extractSlideTitle(tc.slide)
			if title != tc.expectedTitle {
				t.Errorf("expected title '%s', got '%s'", tc.expectedTitle, title)
			}
		})
	}
}

func TestListSlides_HasSpeakerNotes(t *testing.T) {
	testCases := []struct {
		name     string
		slide    *slides.Page
		expected bool
	}{
		{
			name:     "nil slide",
			slide:    nil,
			expected: false,
		},
		{
			name: "slide without slide properties",
			slide: &slides.Page{
				ObjectId: "slide-1",
			},
			expected: false,
		},
		{
			name: "slide without notes page",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
			},
			expected: false,
		},
		{
			name: "slide with empty notes",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-shape",
								Shape: &slides.Shape{
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "   "}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "slide with speaker notes",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-shape",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "These are my notes"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasSpeakerNotes(tc.slide)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestListSlides_HasVideos(t *testing.T) {
	testCases := []struct {
		name     string
		elements []*slides.PageElement
		expected bool
	}{
		{
			name:     "empty elements",
			elements: []*slides.PageElement{},
			expected: false,
		},
		{
			name: "elements without video",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1", Shape: &slides.Shape{}},
				{ObjectId: "image-1", Image: &slides.Image{}},
			},
			expected: false,
		},
		{
			name: "elements with video",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1", Shape: &slides.Shape{}},
				{ObjectId: "video-1", Video: &slides.Video{}},
			},
			expected: true,
		},
		{
			name: "video inside group",
			elements: []*slides.PageElement{
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{ObjectId: "video-in-group", Video: &slides.Video{}},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "nested groups with video",
			elements: []*slides.PageElement{
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{
								ObjectId: "nested-group",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "nested-video", Video: &slides.Video{}},
									},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasVideos(tc.elements)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestListSlides_GetLayoutType(t *testing.T) {
	layouts := []*slides.Page{
		{
			ObjectId: "layout-1",
			LayoutProperties: &slides.LayoutProperties{
				Name:        "TITLE",
				DisplayName: "Title Slide",
			},
		},
		{
			ObjectId: "layout-2",
			LayoutProperties: &slides.LayoutProperties{
				Name:        "TITLE_AND_BODY",
				DisplayName: "Content",
			},
		},
		{
			ObjectId: "layout-3",
			LayoutProperties: &slides.LayoutProperties{
				DisplayName: "Custom Layout",
			},
		},
	}

	testCases := []struct {
		name         string
		slide        *slides.Page
		expectedType string
	}{
		{
			name: "slide with TITLE layout",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
			},
			expectedType: "TITLE",
		},
		{
			name: "slide with TITLE_AND_BODY layout",
			slide: &slides.Page{
				ObjectId: "slide-2",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-2",
				},
			},
			expectedType: "TITLE_AND_BODY",
		},
		{
			name: "slide with custom layout (fallback to DisplayName)",
			slide: &slides.Page{
				ObjectId: "slide-3",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-3",
				},
			},
			expectedType: "Custom Layout",
		},
		{
			name: "slide without layout",
			slide: &slides.Page{
				ObjectId: "slide-4",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "",
				},
			},
			expectedType: "",
		},
		{
			name: "slide with unknown layout ID",
			slide: &slides.Page{
				ObjectId: "slide-5",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "unknown-layout",
				},
			},
			expectedType: "",
		},
		{
			name: "slide without slide properties",
			slide: &slides.Page{
				ObjectId: "slide-6",
			},
			expectedType: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			layoutType := getLayoutType(tc.slide, layouts)
			if layoutType != tc.expectedType {
				t.Errorf("expected layout type '%s', got '%s'", tc.expectedType, layoutType)
			}
		})
	}
}

func TestListSlides_StatisticsAccurate(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					// Slide 1: no notes, no video
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{}},
						},
					},
					// Slide 2: has notes, no video
					{
						ObjectId: "slide-2",
						SlideProperties: &slides.SlideProperties{
							NotesPage: &slides.Page{
								PageElements: []*slides.PageElement{
									{
										Shape: &slides.Shape{
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "Notes for slide 2"}},
												},
											},
										},
									},
								},
							},
						},
						PageElements: []*slides.PageElement{},
					},
					// Slide 3: no notes, has video
					{
						ObjectId:        "slide-3",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "video-1", Video: &slides.Video{}},
						},
					},
					// Slide 4: has notes, has video
					{
						ObjectId: "slide-4",
						SlideProperties: &slides.SlideProperties{
							NotesPage: &slides.Page{
								PageElements: []*slides.PageElement{
									{
										Shape: &slides.Shape{
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "Notes for slide 4"}},
												},
											},
										},
									},
								},
							},
						},
						PageElements: []*slides.PageElement{
							{ObjectId: "video-2", Video: &slides.Video{}},
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

	output, err := tools.ListSlides(context.Background(), tokenSource, ListSlidesInput{
		PresentationID: "test-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify statistics
	if output.Statistics.TotalSlides != 4 {
		t.Errorf("expected total_slides 4, got %d", output.Statistics.TotalSlides)
	}
	if output.Statistics.SlidesWithNotes != 2 {
		t.Errorf("expected slides_with_notes 2, got %d", output.Statistics.SlidesWithNotes)
	}
	if output.Statistics.SlidesWithVideos != 2 {
		t.Errorf("expected slides_with_videos 2, got %d", output.Statistics.SlidesWithVideos)
	}
}
