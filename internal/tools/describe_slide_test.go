package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestDescribeSlide_ByIndex(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			if presentationID != "test-presentation-id" {
				t.Errorf("expected presentation ID 'test-presentation-id', got '%s'", presentationID)
			}
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				PageSize: &slides.Size{
					Width:  &slides.Dimension{Magnitude: 720, Unit: "PT"},
					Height: &slides.Dimension{Magnitude: 405, Unit: "PT"},
				},
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						SlideProperties: &slides.SlideProperties{
							LayoutObjectId: "layout-1",
						},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "title-1",
								Transform: &slides.AffineTransform{
									TranslateX: 0,
									TranslateY: 0,
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 600, Unit: "PT"},
									Height: &slides.Dimension{Magnitude: 50, Unit: "PT"},
								},
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
								Transform: &slides.AffineTransform{
									TranslateX: 50 * emuPerPoint,
									TranslateY: 100 * emuPerPoint,
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 500, Unit: "PT"},
									Height: &slides.Dimension{Magnitude: 200, Unit: "PT"},
								},
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Body content here"}},
										},
									},
								},
							},
						},
					},
					{
						ObjectId: "slide-2",
						SlideProperties: &slides.SlideProperties{
							LayoutObjectId: "layout-2",
						},
						PageElements: []*slides.PageElement{},
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
							DisplayName:    "Blank",
							MasterObjectId: "master-1",
							Name:           "BLANK",
						},
					},
				},
			}, nil
		},
		GetThumbnailFunc: func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
			return &slides.Thumbnail{ContentUrl: ""}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-presentation-id",
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify basic info
	if output.PresentationID != "test-presentation-id" {
		t.Errorf("expected presentation ID 'test-presentation-id', got '%s'", output.PresentationID)
	}
	if output.SlideID != "slide-1" {
		t.Errorf("expected slide ID 'slide-1', got '%s'", output.SlideID)
	}
	if output.SlideIndex != 1 {
		t.Errorf("expected slide index 1, got %d", output.SlideIndex)
	}
	if output.Title != "Slide One Title" {
		t.Errorf("expected title 'Slide One Title', got '%s'", output.Title)
	}
	if output.LayoutType != "TITLE" {
		t.Errorf("expected layout type 'TITLE', got '%s'", output.LayoutType)
	}

	// Verify objects
	if len(output.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(output.Objects))
	}

	// Verify first object (title)
	obj1 := output.Objects[0]
	if obj1.ObjectID != "title-1" {
		t.Errorf("expected object ID 'title-1', got '%s'", obj1.ObjectID)
	}
	if obj1.ObjectType != "TEXT_BOX" {
		t.Errorf("expected object type 'TEXT_BOX', got '%s'", obj1.ObjectType)
	}
	if obj1.ZOrder != 0 {
		t.Errorf("expected z-order 0, got %d", obj1.ZOrder)
	}

	// Verify layout description is generated
	if output.LayoutDescription == "" {
		t.Error("expected non-empty layout description")
	}
}

func TestDescribeSlide_BySlideID(t *testing.T) {
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
					{
						ObjectId:        "target-slide-id",
						SlideProperties: &slides.SlideProperties{},
						PageElements:    []*slides.PageElement{},
					},
					{
						ObjectId:        "slide-3",
						SlideProperties: &slides.SlideProperties{},
						PageElements:    []*slides.PageElement{},
					},
				},
			}, nil
		},
		GetThumbnailFunc: func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
			return &slides.Thumbnail{ContentUrl: ""}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-presentation-id",
		SlideID:        "target-slide-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SlideID != "target-slide-id" {
		t.Errorf("expected slide ID 'target-slide-id', got '%s'", output.SlideID)
	}
	if output.SlideIndex != 2 {
		t.Errorf("expected slide index 2, got %d", output.SlideIndex)
	}
}

func TestDescribeSlide_EmptyPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got: %v", err)
	}
}

func TestDescribeSlide_NoSlideReference(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		// Neither SlideIndex nor SlideID provided
	})

	if err == nil {
		t.Fatal("expected error when no slide reference provided")
	}
	if !errors.Is(err, ErrInvalidSlideReference) {
		t.Errorf("expected ErrInvalidSlideReference, got: %v", err)
	}
}

func TestDescribeSlide_SlideIndexOutOfRange(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1", SlideProperties: &slides.SlideProperties{}},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		SlideIndex:     5, // Out of range
	})

	if err == nil {
		t.Fatal("expected error for out of range slide index")
	}
	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got: %v", err)
	}
}

func TestDescribeSlide_SlideIDNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1", SlideProperties: &slides.SlideProperties{}},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		SlideID:        "nonexistent-slide-id",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent slide ID")
	}
	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got: %v", err)
	}
}

func TestDescribeSlide_PresentationNotFound(t *testing.T) {
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

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "nonexistent-id",
		SlideIndex:     1,
	})

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestDescribeSlide_AccessDenied(t *testing.T) {
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

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "forbidden-id",
		SlideIndex:     1,
	})

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDescribeSlide_ObjectPositions(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-1",
								Transform: &slides.AffineTransform{
									TranslateX: 100 * emuPerPoint,
									TranslateY: 200 * emuPerPoint,
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 300, Unit: "PT"},
									Height: &slides.Dimension{Magnitude: 100, Unit: "PT"},
								},
								Shape: &slides.Shape{ShapeType: "RECTANGLE"},
							},
						},
					},
				},
			}, nil
		},
		GetThumbnailFunc: func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
			return &slides.Thumbnail{ContentUrl: ""}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(output.Objects))
	}

	obj := output.Objects[0]

	// Verify position
	if obj.Position == nil {
		t.Fatal("expected position to be set")
	}
	if obj.Position.X != 100 {
		t.Errorf("expected X position 100, got %f", obj.Position.X)
	}
	if obj.Position.Y != 200 {
		t.Errorf("expected Y position 200, got %f", obj.Position.Y)
	}

	// Verify size
	if obj.Size == nil {
		t.Fatal("expected size to be set")
	}
	if obj.Size.Width != 300 {
		t.Errorf("expected width 300, got %f", obj.Size.Width)
	}
	if obj.Size.Height != 100 {
		t.Errorf("expected height 100, got %f", obj.Size.Height)
	}
}

func TestDescribeSlide_ContentSummary(t *testing.T) {
	testCases := []struct {
		name            string
		element         *slides.PageElement
		expectedSummary string
	}{
		{
			name: "shape with text",
			element: &slides.PageElement{
				ObjectId: "shape-1",
				Shape: &slides.Shape{
					Text: &slides.TextContent{
						TextElements: []*slides.TextElement{
							{TextRun: &slides.TextRun{Content: "Hello World"}},
						},
					},
				},
			},
			expectedSummary: "Hello World",
		},
		{
			name: "shape with placeholder",
			element: &slides.PageElement{
				ObjectId: "shape-2",
				Shape: &slides.Shape{
					Placeholder: &slides.Placeholder{Type: "TITLE"},
				},
			},
			expectedSummary: "[TITLE placeholder]",
		},
		{
			name: "image",
			element: &slides.PageElement{
				ObjectId: "image-1",
				Image:    &slides.Image{ContentUrl: "https://example.com/img.png"},
			},
			expectedSummary: "Image (external)",
		},
		{
			name: "YouTube video",
			element: &slides.PageElement{
				ObjectId: "video-1",
				Video: &slides.Video{
					Id:     "dQw4w9WgXcQ",
					Source: "YOUTUBE",
				},
			},
			expectedSummary: "YouTube video: dQw4w9WgXcQ",
		},
		{
			name: "table",
			element: &slides.PageElement{
				ObjectId: "table-1",
				Table: &slides.Table{
					TableRows: []*slides.TableRow{
						{TableCells: []*slides.TableCell{{}, {}, {}}},
						{TableCells: []*slides.TableCell{{}, {}, {}}},
					},
				},
			},
			expectedSummary: "Table (2x3)",
		},
		{
			name: "line",
			element: &slides.PageElement{
				ObjectId: "line-1",
				Line:     &slides.Line{LineType: "STRAIGHT_LINE"},
			},
			expectedSummary: "STRAIGHT_LINE",
		},
		{
			name: "group",
			element: &slides.PageElement{
				ObjectId: "group-1",
				ElementGroup: &slides.Group{
					Children: []*slides.PageElement{
						{ObjectId: "child-1"},
						{ObjectId: "child-2"},
						{ObjectId: "child-3"},
					},
				},
			},
			expectedSummary: "Group (3 items)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary := extractContentSummary(tc.element)
			if summary != tc.expectedSummary {
				t.Errorf("expected summary '%s', got '%s'", tc.expectedSummary, summary)
			}
		})
	}
}

func TestDescribeSlide_TextTruncation(t *testing.T) {
	longText := "This is a very long text that exceeds one hundred characters and should be truncated with an ellipsis at the end to keep it readable"

	truncated := truncateText(longText, 100)

	if len(truncated) != 100 {
		t.Errorf("expected truncated text length 100, got %d", len(truncated))
	}
	if truncated[len(truncated)-3:] != "..." {
		t.Error("expected truncated text to end with '...'")
	}
}

func TestDescribeSlide_GroupChildren(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{
											ObjectId: "child-shape",
											Shape:    &slides.Shape{ShapeType: "RECTANGLE"},
										},
										{
											ObjectId: "child-image",
											Image:    &slides.Image{},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
		GetThumbnailFunc: func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
			return &slides.Thumbnail{ContentUrl: ""}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Objects) != 1 {
		t.Fatalf("expected 1 top-level object, got %d", len(output.Objects))
	}

	group := output.Objects[0]
	if group.ObjectType != "GROUP" {
		t.Errorf("expected GROUP type, got '%s'", group.ObjectType)
	}

	if len(group.Children) != 2 {
		t.Fatalf("expected 2 children in group, got %d", len(group.Children))
	}

	if group.Children[0].ObjectType != "RECTANGLE" {
		t.Errorf("expected first child type RECTANGLE, got '%s'", group.Children[0].ObjectType)
	}
	if group.Children[1].ObjectType != "IMAGE" {
		t.Errorf("expected second child type IMAGE, got '%s'", group.Children[1].ObjectType)
	}
}

func TestDescribeSlide_WithSpeakerNotes(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						SlideProperties: &slides.SlideProperties{
							NotesPage: &slides.Page{
								PageElements: []*slides.PageElement{
									{
										Shape: &slides.Shape{
											Placeholder: &slides.Placeholder{Type: "BODY"},
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "These are the speaker notes"}},
												},
											},
										},
									},
								},
							},
						},
						PageElements: []*slides.PageElement{},
					},
				},
			}, nil
		},
		GetThumbnailFunc: func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
			return &slides.Thumbnail{ContentUrl: ""}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SpeakerNotes != "These are the speaker notes" {
		t.Errorf("expected speaker notes 'These are the speaker notes', got '%s'", output.SpeakerNotes)
	}
}

func TestDescribeSlide_LayoutDescription(t *testing.T) {
	testCases := []struct {
		name        string
		objects     []ObjectDescription
		pageSize    *PageSize
		title       string
		shouldMatch func(desc string) bool
	}{
		{
			name:    "empty slide",
			objects: []ObjectDescription{},
			pageSize: &PageSize{
				Width:  &Dimension{Magnitude: 720, Unit: "PT"},
				Height: &Dimension{Magnitude: 405, Unit: "PT"},
			},
			title: "",
			shouldMatch: func(desc string) bool {
				return desc == "Empty slide with no objects"
			},
		},
		{
			name: "slide with title",
			objects: []ObjectDescription{
				{
					ObjectID:   "title-1",
					ObjectType: "TEXT_BOX",
					Position:   &Position{X: 60, Y: 30},
					Size:       &Size{Width: 600, Height: 50},
				},
			},
			pageSize: &PageSize{
				Width:  &Dimension{Magnitude: 720, Unit: "PT"},
				Height: &Dimension{Magnitude: 405, Unit: "PT"},
			},
			title: "My Title",
			shouldMatch: func(desc string) bool {
				return len(desc) > 0 && desc != "Empty slide with no objects"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			desc := generateLayoutDescription(tc.objects, tc.pageSize, tc.title)
			if !tc.shouldMatch(desc) {
				t.Errorf("layout description '%s' did not match expected criteria", desc)
			}
		})
	}
}

func TestDescribeSlide_ServiceFactoryError(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.DescribeSlide(context.Background(), tokenSource, DescribeSlideInput{
		PresentationID: "test-id",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestDescribeSlide_EmuToPointsConversion(t *testing.T) {
	testCases := []struct {
		emu            float64
		expectedPoints float64
	}{
		{0, 0},
		{12700, 1},
		{127000, 10},
		{9144000, 720}, // Standard slide width
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			points := emuToPoints(tc.emu)
			if points != tc.expectedPoints {
				t.Errorf("emuToPoints(%f) = %f, expected %f", tc.emu, points, tc.expectedPoints)
			}
		})
	}
}
