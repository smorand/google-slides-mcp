package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Helper error types for testing
type hyperlinkNotFoundError struct{}

func (e *hyperlinkNotFoundError) Error() string { return "not found" }

type hyperlinkForbiddenError struct{}

func (e *hyperlinkForbiddenError) Error() string { return "forbidden" }

// createHyperlinkMockFactory creates a SlidesServiceFactory for testing.
func createHyperlinkMockFactory(mockService *mockSlidesService) SlidesServiceFactory {
	return func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}
}

// Helper to create text elements with links
func createTextElementsWithLink(text string, linkURL string) []*slides.TextElement {
	return []*slides.TextElement{
		{
			TextRun: &slides.TextRun{
				Content: text,
				Style: &slides.TextStyle{
					Link: &slides.Link{
						Url: linkURL,
					},
				},
			},
		},
	}
}

// Helper to create text elements without links
func createTextElementsNoLink(text string) []*slides.TextElement {
	return []*slides.TextElement{
		{
			TextRun: &slides.TextRun{
				Content: text,
				Style:   &slides.TextStyle{},
			},
		},
	}
}

func TestManageHyperlinks(t *testing.T) {
	ctx := context.Background()

	t.Run("list returns all hyperlinks with their URLs", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Click here", "https://example.com"),
										},
									},
								},
								{
									ObjectId: "shape-2",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Go to docs", "https://docs.example.com"),
										},
									},
								},
							},
						},
						{
							ObjectId: "slide-2",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-3",
									Shape: &slides.Shape{
										ShapeType: "RECTANGLE",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Another link", "https://other.com"),
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
			Scope:          "all",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output == nil {
			t.Fatal("expected output, got nil")
		}
		if len(output.Links) != 3 {
			t.Errorf("expected 3 links, got %d", len(output.Links))
		}

		// Verify first link
		if output.Links[0].URL != "https://example.com" {
			t.Errorf("expected URL 'https://example.com', got '%s'", output.Links[0].URL)
		}
		if output.Links[0].Text != "Click here" {
			t.Errorf("expected text 'Click here', got '%s'", output.Links[0].Text)
		}
		if output.Links[0].LinkType != "external" {
			t.Errorf("expected link type 'external', got '%s'", output.Links[0].LinkType)
		}
		if output.Links[0].SlideIndex != 1 {
			t.Errorf("expected slide index 1, got %d", output.Links[0].SlideIndex)
		}
	})

	t.Run("add creates link on text range", func(t *testing.T) {
		batchUpdateCalled := false
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsNoLink("Some text here"),
										},
									},
								},
							},
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				batchUpdateCalled = true
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		startIdx := 5
		endIdx := 9
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "shape-1",
			StartIndex:     &startIdx,
			EndIndex:       &endIdx,
			URL:            "https://example.com",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !batchUpdateCalled {
			t.Error("expected batch update to be called")
		}
		if len(capturedRequests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(capturedRequests))
		}

		req := capturedRequests[0].UpdateTextStyle
		if req == nil {
			t.Fatal("expected UpdateTextStyle request")
		}
		if req.ObjectId != "shape-1" {
			t.Errorf("expected object ID 'shape-1', got '%s'", req.ObjectId)
		}
		if req.Style.Link.Url != "https://example.com" {
			t.Errorf("expected URL 'https://example.com', got '%s'", req.Style.Link.Url)
		}
		if req.TextRange.Type != "FIXED_RANGE" {
			t.Errorf("expected range type 'FIXED_RANGE', got '%s'", req.TextRange.Type)
		}
		if *req.TextRange.StartIndex != 5 {
			t.Errorf("expected start index 5, got %d", *req.TextRange.StartIndex)
		}
		if *req.TextRange.EndIndex != 9 {
			t.Errorf("expected end index 9, got %d", *req.TextRange.EndIndex)
		}
		if !output.Success {
			t.Error("expected success to be true")
		}
	})

	t.Run("add creates link on entire shape", func(t *testing.T) {
		batchUpdateCalled := false
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "RECTANGLE",
									},
								},
							},
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				batchUpdateCalled = true
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "shape-1",
			URL:            "https://example.com",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !batchUpdateCalled {
			t.Error("expected batch update to be called")
		}
		if len(capturedRequests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(capturedRequests))
		}

		req := capturedRequests[0].UpdateShapeProperties
		if req == nil {
			t.Fatal("expected UpdateShapeProperties request")
		}
		if req.ObjectId != "shape-1" {
			t.Errorf("expected object ID 'shape-1', got '%s'", req.ObjectId)
		}
		if req.ShapeProperties.Link.Url != "https://example.com" {
			t.Errorf("expected URL 'https://example.com', got '%s'", req.ShapeProperties.Link.Url)
		}
		if !output.Success {
			t.Error("expected success to be true")
		}
	})

	t.Run("add creates link on image", func(t *testing.T) {
		batchUpdateCalled := false
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "image-1",
									Image: &slides.Image{
										ContentUrl: "https://example.com/image.png",
									},
								},
							},
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				batchUpdateCalled = true
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "image-1",
			URL:            "https://target.com",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !batchUpdateCalled {
			t.Error("expected batch update to be called")
		}
		if len(capturedRequests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(capturedRequests))
		}

		req := capturedRequests[0].UpdateImageProperties
		if req == nil {
			t.Fatal("expected UpdateImageProperties request")
		}
		if req.ObjectId != "image-1" {
			t.Errorf("expected object ID 'image-1', got '%s'", req.ObjectId)
		}
		if req.ImageProperties.Link.Url != "https://target.com" {
			t.Errorf("expected URL 'https://target.com', got '%s'", req.ImageProperties.Link.Url)
		}
		if !output.Success {
			t.Error("expected success to be true")
		}
	})

	t.Run("remove clears link from text", func(t *testing.T) {
		batchUpdateCalled := false
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Click me", "https://example.com"),
										},
									},
								},
							},
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				batchUpdateCalled = true
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		startIdx := 0
		endIdx := 8
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "remove",
			ObjectID:       "shape-1",
			StartIndex:     &startIdx,
			EndIndex:       &endIdx,
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !batchUpdateCalled {
			t.Error("expected batch update to be called")
		}
		if len(capturedRequests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(capturedRequests))
		}

		req := capturedRequests[0].UpdateTextStyle
		if req == nil {
			t.Fatal("expected UpdateTextStyle request")
		}
		if req.ObjectId != "shape-1" {
			t.Errorf("expected object ID 'shape-1', got '%s'", req.ObjectId)
		}
		if req.Style.Link != nil {
			t.Error("expected link to be nil to remove it")
		}
		if req.Fields != "link" {
			t.Errorf("expected fields 'link', got '%s'", req.Fields)
		}
		if !output.Success {
			t.Error("expected success to be true")
		}
	})

	t.Run("internal slide links work", func(t *testing.T) {
		batchUpdateCalled := false
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsNoLink("Go to slide 2"),
										},
									},
								},
							},
						},
						{
							ObjectId: "slide-2",
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				batchUpdateCalled = true
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))

		// Test #slide=N format (1-based slide index)
		startIdx := 0
		endIdx := 13
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "shape-1",
			StartIndex:     &startIdx,
			EndIndex:       &endIdx,
			URL:            "#slide=2",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !batchUpdateCalled {
			t.Error("expected batch update to be called")
		}
		if len(capturedRequests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(capturedRequests))
		}

		req := capturedRequests[0].UpdateTextStyle
		if req == nil {
			t.Fatal("expected UpdateTextStyle request")
		}
		// API uses 0-based index, so #slide=2 should become SlideIndex=1
		if req.Style.Link.SlideIndex != 1 {
			t.Errorf("expected slide index 1 (0-based), got %d", req.Style.Link.SlideIndex)
		}
		if !output.Success {
			t.Error("expected success to be true")
		}
	})

	t.Run("internal slide ID links work", func(t *testing.T) {
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsNoLink("Link to specific slide"),
										},
									},
								},
							},
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		startIdx := 0
		endIdx := 22
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "shape-1",
			StartIndex:     &startIdx,
			EndIndex:       &endIdx,
			URL:            "#slideId=target-slide-id",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		req := capturedRequests[0].UpdateTextStyle
		if req.Style.Link.PageObjectId != "target-slide-id" {
			t.Errorf("expected page object ID 'target-slide-id', got '%s'", req.Style.Link.PageObjectId)
		}
	})

	t.Run("relative slide links work", func(t *testing.T) {
		var capturedRequests []*slides.Request

		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "RECTANGLE",
									},
								},
							},
						},
					},
				}, nil
			},
			BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
				capturedRequests = requests
				return &slides.BatchUpdatePresentationResponse{}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))

		testCases := []struct {
			url          string
			expectedLink string
		}{
			{"#next", "NEXT_SLIDE"},
			{"#previous", "PREVIOUS_SLIDE"},
			{"#first", "FIRST_SLIDE"},
			{"#last", "LAST_SLIDE"},
		}

		for _, tc := range testCases {
			t.Run(tc.url, func(t *testing.T) {
				_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
					PresentationID: "test-pres",
					Action:         "add",
					ObjectID:       "shape-1",
					URL:            tc.url,
				})

				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				req := capturedRequests[0].UpdateShapeProperties
				if req.ShapeProperties.Link.RelativeLink != tc.expectedLink {
					t.Errorf("expected relative link '%s', got '%s'", tc.expectedLink, req.ShapeProperties.Link.RelativeLink)
				}
			})
		}
	})

	t.Run("list with scope slide filters correctly", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Slide 1 link", "https://slide1.com"),
										},
									},
								},
							},
						},
						{
							ObjectId: "slide-2",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-2",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Slide 2 link", "https://slide2.com"),
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
			Scope:          "slide",
			SlideID:        "slide-2",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(output.Links) != 1 {
			t.Errorf("expected 1 link, got %d", len(output.Links))
		}
		if output.Links[0].URL != "https://slide2.com" {
			t.Errorf("expected URL 'https://slide2.com', got '%s'", output.Links[0].URL)
		}
	})

	t.Run("list with scope object filters correctly", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Link 1", "https://link1.com"),
										},
									},
								},
								{
									ObjectId: "shape-2",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: createTextElementsWithLink("Link 2", "https://link2.com"),
										},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
			Scope:          "object",
			ObjectID:       "shape-2",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(output.Links) != 1 {
			t.Errorf("expected 1 link, got %d", len(output.Links))
		}
		if output.Links[0].URL != "https://link2.com" {
			t.Errorf("expected URL 'https://link2.com', got '%s'", output.Links[0].URL)
		}
	})

	t.Run("list links from image with ImageProperties link", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "image-1",
									Image: &slides.Image{
										ContentUrl: "https://example.com/image.png",
										ImageProperties: &slides.ImageProperties{
											Link: &slides.Link{
												Url: "https://linked-site.com",
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

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(output.Links) != 1 {
			t.Errorf("expected 1 link, got %d", len(output.Links))
		}
		if output.Links[0].URL != "https://linked-site.com" {
			t.Errorf("expected URL 'https://linked-site.com', got '%s'", output.Links[0].URL)
		}
		if output.Links[0].ObjectType != "IMAGE" {
			t.Errorf("expected object type 'IMAGE', got '%s'", output.Links[0].ObjectType)
		}
	})

	t.Run("list links from shape with ShapeProperties link", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "RECTANGLE",
										ShapeProperties: &slides.ShapeProperties{
											Link: &slides.Link{
												Url: "https://shape-link.com",
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

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(output.Links) != 1 {
			t.Errorf("expected 1 link, got %d", len(output.Links))
		}
		if output.Links[0].URL != "https://shape-link.com" {
			t.Errorf("expected URL 'https://shape-link.com', got '%s'", output.Links[0].URL)
		}
	})

	t.Run("error on invalid action", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{PresentationId: "test-pres"}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "invalid",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidHyperlinkAction) {
			t.Errorf("expected ErrInvalidHyperlinkAction, got %v", err)
		}
	})

	t.Run("error on missing presentation ID", func(t *testing.T) {
		tools := NewTools(DefaultToolsConfig(), nil)
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			Action: "list",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidPresentationID) {
			t.Errorf("expected ErrInvalidPresentationID, got %v", err)
		}
	})

	t.Run("error on add without URL", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape:    &slides.Shape{ShapeType: "TEXT_BOX"},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "shape-1",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidHyperlinkURL) {
			t.Errorf("expected ErrInvalidHyperlinkURL, got %v", err)
		}
	})

	t.Run("error on object not found", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides:         []*slides.Page{{ObjectId: "slide-1"}},
				}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "non-existent",
			URL:            "https://example.com",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrObjectNotFound) {
			t.Errorf("expected ErrObjectNotFound, got %v", err)
		}
	})

	t.Run("error on invalid text range", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text:      &slides.TextContent{},
									},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))

		// Test start >= end
		startIdx := 10
		endIdx := 5
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "add",
			ObjectID:       "shape-1",
			StartIndex:     &startIdx,
			EndIndex:       &endIdx,
			URL:            "https://example.com",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidTextRange) {
			t.Errorf("expected ErrInvalidTextRange, got %v", err)
		}
	})

	t.Run("list returns internal slide links correctly", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "shape-1",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: []*slides.TextElement{
												{
													TextRun: &slides.TextRun{
														Content: "Internal link",
														Style: &slides.TextStyle{
															Link: &slides.Link{
																PageObjectId: "slide-2",
															},
														},
													},
												},
											},
										},
									},
								},
								{
									ObjectId: "shape-2",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: []*slides.TextElement{
												{
													TextRun: &slides.TextRun{
														Content: "Relative link",
														Style: &slides.TextStyle{
															Link: &slides.Link{
																RelativeLink: "NEXT_SLIDE",
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

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(output.Links) != 2 {
			t.Errorf("expected 2 links, got %d", len(output.Links))
		}

		// First link - internal slide link
		if output.Links[0].LinkType != "internal_slide" {
			t.Errorf("expected link type 'internal_slide', got '%s'", output.Links[0].LinkType)
		}
		if output.Links[0].SlideLink != "slide-2" {
			t.Errorf("expected slide link 'slide-2', got '%s'", output.Links[0].SlideLink)
		}

		// Second link - relative link
		if output.Links[1].LinkType != "internal_position" {
			t.Errorf("expected link type 'internal_position', got '%s'", output.Links[1].LinkType)
		}
		if output.Links[1].SlideLink != "NEXT_SLIDE" {
			t.Errorf("expected slide link 'NEXT_SLIDE', got '%s'", output.Links[1].SlideLink)
		}
	})

	t.Run("list links from table cells", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{
					PresentationId: "test-pres",
					Slides: []*slides.Page{
						{
							ObjectId: "slide-1",
							PageElements: []*slides.PageElement{
								{
									ObjectId: "table-1",
									Table: &slides.Table{
										Rows:    2,
										Columns: 2,
										TableRows: []*slides.TableRow{
											{
												TableCells: []*slides.TableCell{
													{
														Text: &slides.TextContent{
															TextElements: createTextElementsWithLink("Cell 0,0 link", "https://cell00.com"),
														},
													},
													{
														Text: &slides.TextContent{
															TextElements: createTextElementsNoLink("No link"),
														},
													},
												},
											},
											{
												TableCells: []*slides.TableCell{
													{
														Text: &slides.TextContent{
															TextElements: createTextElementsNoLink("No link"),
														},
													},
													{
														Text: &slides.TextContent{
															TextElements: createTextElementsWithLink("Cell 1,1 link", "https://cell11.com"),
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

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		output, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(output.Links) != 2 {
			t.Errorf("expected 2 links, got %d", len(output.Links))
		}

		// Verify cell links have correct object IDs
		if output.Links[0].ObjectID != "table-1[0,0]" {
			t.Errorf("expected object ID 'table-1[0,0]', got '%s'", output.Links[0].ObjectID)
		}
		if output.Links[1].ObjectID != "table-1[1,1]" {
			t.Errorf("expected object ID 'table-1[1,1]', got '%s'", output.Links[1].ObjectID)
		}
		if output.Links[0].ObjectType != "TABLE_CELL" {
			t.Errorf("expected object type 'TABLE_CELL', got '%s'", output.Links[0].ObjectType)
		}
	})

	t.Run("presentation not found", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return nil, &hyperlinkNotFoundError{}
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "non-existent",
			Action:         "list",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrPresentationNotFound) {
			t.Errorf("expected ErrPresentationNotFound, got %v", err)
		}
	})

	t.Run("access denied", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return nil, &hyperlinkForbiddenError{}
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "forbidden",
			Action:         "list",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("list scope slide requires slide_id", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{PresentationId: "test-pres"}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
			Scope:          "slide",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidSlideReference) {
			t.Errorf("expected ErrInvalidSlideReference, got %v", err)
		}
	})

	t.Run("list scope object requires object_id", func(t *testing.T) {
		mockService := &mockSlidesService{
			GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
				return &slides.Presentation{PresentationId: "test-pres"}, nil
			},
		}

		tools := NewTools(DefaultToolsConfig(), createHyperlinkMockFactory(mockService))
		_, err := tools.ManageHyperlinks(ctx, nil, ManageHyperlinksInput{
			PresentationID: "test-pres",
			Action:         "list",
			Scope:          "object",
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidObjectID) {
			t.Errorf("expected ErrInvalidObjectID, got %v", err)
		}
	})
}
