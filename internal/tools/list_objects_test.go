package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestListObjects_AllSlides(t *testing.T) {
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
								Shape: &slides.Shape{ShapeType: "TEXT_BOX"},
							},
							{
								ObjectId: "image-1",
								Image:    &slides.Image{ContentUrl: "https://example.com/img.png"},
							},
						},
					},
					{
						ObjectId:        "slide-2",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "table-1",
								Table: &slides.Table{
									TableRows: []*slides.TableRow{
										{TableCells: []*slides.TableCell{{}, {}}},
									},
								},
							},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 objects total from both slides
	if output.TotalCount != 3 {
		t.Errorf("expected total count 3, got %d", output.TotalCount)
	}

	if len(output.Objects) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(output.Objects))
	}

	// Verify first object
	if output.Objects[0].SlideIndex != 1 {
		t.Errorf("expected slide index 1, got %d", output.Objects[0].SlideIndex)
	}
	if output.Objects[0].ObjectID != "shape-1" {
		t.Errorf("expected object ID 'shape-1', got '%s'", output.Objects[0].ObjectID)
	}
	if output.Objects[0].ObjectType != "TEXT_BOX" {
		t.Errorf("expected object type 'TEXT_BOX', got '%s'", output.Objects[0].ObjectType)
	}

	// Verify no filter info when no filters applied
	if output.FilteredBy != nil {
		t.Error("expected no filter info when no filters applied")
	}
}

func TestListObjects_SlideIndicesFilter(t *testing.T) {
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
							{ObjectId: "obj-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
						},
					},
					{
						ObjectId:        "slide-2",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "obj-2", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
						},
					},
					{
						ObjectId:        "slide-3",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "obj-3", Shape: &slides.Shape{ShapeType: "TRIANGLE"}},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
		SlideIndices:   []int{1, 3}, // Only slides 1 and 3
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have objects from slides 1 and 3
	if output.TotalCount != 2 {
		t.Errorf("expected total count 2, got %d", output.TotalCount)
	}

	// Verify filter info is present
	if output.FilteredBy == nil {
		t.Fatal("expected filter info to be present")
	}
	if len(output.FilteredBy.SlideIndices) != 2 {
		t.Errorf("expected 2 slide indices in filter, got %d", len(output.FilteredBy.SlideIndices))
	}

	// Verify objects are from correct slides
	for _, obj := range output.Objects {
		if obj.SlideIndex != 1 && obj.SlideIndex != 3 {
			t.Errorf("expected object from slide 1 or 3, got slide %d", obj.SlideIndex)
		}
	}
}

func TestListObjects_ObjectTypesFilter(t *testing.T) {
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
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
							{ObjectId: "image-1", Image: &slides.Image{}},
							{ObjectId: "video-1", Video: &slides.Video{}},
							{ObjectId: "table-1", Table: &slides.Table{}},
							{ObjectId: "line-1", Line: &slides.Line{}},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
		ObjectTypes:    []string{"IMAGE", "VIDEO"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have IMAGE and VIDEO objects
	if output.TotalCount != 2 {
		t.Errorf("expected total count 2, got %d", output.TotalCount)
	}

	// Verify filter info is present
	if output.FilteredBy == nil {
		t.Fatal("expected filter info to be present")
	}
	if len(output.FilteredBy.ObjectTypes) != 2 {
		t.Errorf("expected 2 object types in filter, got %d", len(output.FilteredBy.ObjectTypes))
	}

	// Verify object types
	for _, obj := range output.Objects {
		if obj.ObjectType != "IMAGE" && obj.ObjectType != "VIDEO" {
			t.Errorf("expected IMAGE or VIDEO, got '%s'", obj.ObjectType)
		}
	}
}

func TestListObjects_CombinedFilters(t *testing.T) {
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
							{ObjectId: "shape-1-1", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
							{ObjectId: "image-1-1", Image: &slides.Image{}},
						},
					},
					{
						ObjectId:        "slide-2",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-2-1", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
							{ObjectId: "image-2-1", Image: &slides.Image{}},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
		SlideIndices:   []int{2},
		ObjectTypes:    []string{"IMAGE"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have IMAGE from slide 2
	if output.TotalCount != 1 {
		t.Errorf("expected total count 1, got %d", output.TotalCount)
	}

	if len(output.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(output.Objects))
	}

	if output.Objects[0].ObjectID != "image-2-1" {
		t.Errorf("expected object ID 'image-2-1', got '%s'", output.Objects[0].ObjectID)
	}
}

func TestListObjects_PositionAndSize(t *testing.T) {
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
									Height: &slides.Dimension{Magnitude: 150, Unit: "PT"},
								},
								Shape: &slides.Shape{ShapeType: "RECTANGLE"},
							},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
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
	if obj.Size.Height != 150 {
		t.Errorf("expected height 150, got %f", obj.Size.Height)
	}
}

func TestListObjects_ContentPreview(t *testing.T) {
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
								ObjectId: "text-shape",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Hello World"}},
										},
									},
								},
							},
							{
								ObjectId: "image-no-text",
								Image:    &slides.Image{},
							},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(output.Objects))
	}

	// First object should have content preview
	if output.Objects[0].ContentPreview != "Hello World" {
		t.Errorf("expected content preview 'Hello World', got '%s'", output.Objects[0].ContentPreview)
	}

	// Second object (image) should have no content preview
	if output.Objects[1].ContentPreview != "" {
		t.Errorf("expected no content preview for image, got '%s'", output.Objects[1].ContentPreview)
	}
}

func TestListObjects_ContentPreviewTruncation(t *testing.T) {
	longText := "This is a very long text that exceeds one hundred characters and should be truncated with an ellipsis at the end to keep it readable and manageable"

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
								ObjectId: "text-shape",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: longText}},
										},
									},
								},
							},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(output.Objects))
	}

	preview := output.Objects[0].ContentPreview
	if len(preview) != 100 {
		t.Errorf("expected content preview length 100, got %d", len(preview))
	}
	if preview[len(preview)-3:] != "..." {
		t.Error("expected content preview to end with '...'")
	}
}

func TestListObjects_TableContentPreview(t *testing.T) {
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
								ObjectId: "table-1",
								Table: &slides.Table{
									TableRows: []*slides.TableRow{
										{
											TableCells: []*slides.TableCell{
												{
													Text: &slides.TextContent{
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "First cell content"}},
														},
													},
												},
												{
													Text: &slides.TextContent{
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "Second cell"}},
														},
													},
												},
											},
										},
									},
								},
							},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(output.Objects))
	}

	// Table preview should contain first cell content
	if output.Objects[0].ContentPreview != "First cell content" {
		t.Errorf("expected content preview 'First cell content', got '%s'", output.Objects[0].ContentPreview)
	}
}

func TestListObjects_ZOrder(t *testing.T) {
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
							{ObjectId: "obj-0", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
							{ObjectId: "obj-1", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
							{ObjectId: "obj-2", Shape: &slides.Shape{ShapeType: "TRIANGLE"}},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Objects) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(output.Objects))
	}

	// Verify z-order
	for i, obj := range output.Objects {
		if obj.ZOrder != i {
			t.Errorf("expected z-order %d for object %d, got %d", i, i, obj.ZOrder)
		}
	}
}

func TestListObjects_GroupedElements(t *testing.T) {
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
										{ObjectId: "child-shape", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
										{ObjectId: "child-image", Image: &slides.Image{}},
									},
								},
							},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have group + 2 children = 3 objects
	if output.TotalCount != 3 {
		t.Errorf("expected total count 3 (group + children), got %d", output.TotalCount)
	}

	// Verify group is first
	if output.Objects[0].ObjectType != "GROUP" {
		t.Errorf("expected first object to be GROUP, got '%s'", output.Objects[0].ObjectType)
	}
}

func TestListObjects_EmptyPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got: %v", err)
	}
}

func TestListObjects_PresentationNotFound(t *testing.T) {
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

	_, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "nonexistent-id",
	})

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestListObjects_AccessDenied(t *testing.T) {
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

	_, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "forbidden-id",
	})

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestListObjects_ServiceFactoryError(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-id",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestListObjects_EmptyPresentation(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Empty Presentation",
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements:    []*slides.PageElement{},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.TotalCount != 0 {
		t.Errorf("expected total count 0, got %d", output.TotalCount)
	}

	if len(output.Objects) != 0 {
		t.Errorf("expected 0 objects, got %d", len(output.Objects))
	}
}

func TestListObjects_InvalidSlideIndex(t *testing.T) {
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
							{ObjectId: "obj-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
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

	// Request slide index 99 which doesn't exist
	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
		SlideIndices:   []int{99},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty results (no error, just no matching slides)
	if output.TotalCount != 0 {
		t.Errorf("expected total count 0 for invalid slide index, got %d", output.TotalCount)
	}
}

func TestListObjects_AllObjectTypes(t *testing.T) {
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
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
							{ObjectId: "image-1", Image: &slides.Image{}},
							{ObjectId: "video-1", Video: &slides.Video{}},
							{ObjectId: "table-1", Table: &slides.Table{TableRows: []*slides.TableRow{}}},
							{ObjectId: "line-1", Line: &slides.Line{}},
							{ObjectId: "chart-1", SheetsChart: &slides.SheetsChart{}},
							{ObjectId: "wordart-1", WordArt: &slides.WordArt{}},
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

	output, err := tools.ListObjects(context.Background(), tokenSource, ListObjectsInput{
		PresentationID: "test-presentation-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.TotalCount != 7 {
		t.Errorf("expected total count 7, got %d", output.TotalCount)
	}

	// Verify all types are present
	typeMap := make(map[string]bool)
	for _, obj := range output.Objects {
		typeMap[obj.ObjectType] = true
	}

	expectedTypes := []string{"TEXT_BOX", "IMAGE", "VIDEO", "TABLE", "LINE", "SHEETS_CHART", "WORD_ART"}
	for _, expectedType := range expectedTypes {
		if !typeMap[expectedType] {
			t.Errorf("expected object type '%s' to be present", expectedType)
		}
	}
}
