package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// mockSlidesService implements SlidesService for testing.
type mockSlidesService struct {
	GetPresentationFunc    func(ctx context.Context, presentationID string) (*slides.Presentation, error)
	GetThumbnailFunc       func(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error)
	CreatePresentationFunc func(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error)
	BatchUpdateFunc        func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error)
}

func (m *mockSlidesService) GetPresentation(ctx context.Context, presentationID string) (*slides.Presentation, error) {
	if m.GetPresentationFunc != nil {
		return m.GetPresentationFunc(ctx, presentationID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSlidesService) GetThumbnail(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
	if m.GetThumbnailFunc != nil {
		return m.GetThumbnailFunc(ctx, presentationID, pageObjectID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSlidesService) CreatePresentation(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
	if m.CreatePresentationFunc != nil {
		return m.CreatePresentationFunc(ctx, presentation)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSlidesService) BatchUpdate(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
	if m.BatchUpdateFunc != nil {
		return m.BatchUpdateFunc(ctx, presentationID, requests)
	}
	return nil, errors.New("not implemented")
}

// mockTokenSource implements oauth2.TokenSource for testing.
type mockTokenSource struct{}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "test-token"}, nil
}

func TestGetPresentation_Success(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			if presentationID != "test-presentation-id" {
				t.Errorf("expected presentation ID 'test-presentation-id', got '%s'", presentationID)
			}
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Locale:         "en_US",
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
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Hello World"}},
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
				},
				Masters: []*slides.Page{
					{
						ObjectId: "master-1",
						MasterProperties: &slides.MasterProperties{
							DisplayName: "Default Master",
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

	output, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
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
	if output.Locale != "en_US" {
		t.Errorf("expected locale 'en_US', got '%s'", output.Locale)
	}
	if output.SlidesCount != 2 {
		t.Errorf("expected 2 slides, got %d", output.SlidesCount)
	}

	// Verify page size
	if output.PageSize == nil {
		t.Fatal("expected page size to be set")
	}
	if output.PageSize.Width.Magnitude != 720 {
		t.Errorf("expected width 720, got %f", output.PageSize.Width.Magnitude)
	}
	if output.PageSize.Height.Magnitude != 405 {
		t.Errorf("expected height 405, got %f", output.PageSize.Height.Magnitude)
	}

	// Verify slides
	if len(output.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(output.Slides))
	}

	// Slide 1
	slide1 := output.Slides[0]
	if slide1.Index != 1 {
		t.Errorf("expected slide 1 index 1, got %d", slide1.Index)
	}
	if slide1.ObjectID != "slide-1" {
		t.Errorf("expected slide 1 object ID 'slide-1', got '%s'", slide1.ObjectID)
	}
	if slide1.LayoutID != "layout-1" {
		t.Errorf("expected slide 1 layout ID 'layout-1', got '%s'", slide1.LayoutID)
	}
	if slide1.LayoutName != "Title Slide" {
		t.Errorf("expected slide 1 layout name 'Title Slide', got '%s'", slide1.LayoutName)
	}
	if len(slide1.TextContent) != 1 {
		t.Errorf("expected 1 text block in slide 1, got %d", len(slide1.TextContent))
	}
	if slide1.TextContent[0].Text != "Hello World" {
		t.Errorf("expected text 'Hello World', got '%s'", slide1.TextContent[0].Text)
	}

	// Slide 2
	slide2 := output.Slides[1]
	if slide2.Index != 2 {
		t.Errorf("expected slide 2 index 2, got %d", slide2.Index)
	}
	if slide2.SpeakerNotes != "Speaker notes for slide 2" {
		t.Errorf("expected speaker notes 'Speaker notes for slide 2', got '%s'", slide2.SpeakerNotes)
	}

	// Verify masters
	if len(output.Masters) != 1 {
		t.Errorf("expected 1 master, got %d", len(output.Masters))
	}
	if output.Masters[0].Name != "Default Master" {
		t.Errorf("expected master name 'Default Master', got '%s'", output.Masters[0].Name)
	}

	// Verify layouts
	if len(output.Layouts) != 2 {
		t.Errorf("expected 2 layouts, got %d", len(output.Layouts))
	}
}

func TestGetPresentation_TextContentExtraction(t *testing.T) {
	testCases := []struct {
		name         string
		pageElements []*slides.PageElement
		expectedText []string
	}{
		{
			name: "text box with simple text",
			pageElements: []*slides.PageElement{
				{
					ObjectId: "text-1",
					Shape: &slides.Shape{
						ShapeType: "TEXT_BOX",
						Text: &slides.TextContent{
							TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "Simple text"}},
							},
						},
					},
				},
			},
			expectedText: []string{"Simple text"},
		},
		{
			name: "text box with multiple paragraphs",
			pageElements: []*slides.PageElement{
				{
					ObjectId: "text-1",
					Shape: &slides.Shape{
						ShapeType: "TEXT_BOX",
						Text: &slides.TextContent{
							TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "First paragraph\n"}},
								{TextRun: &slides.TextRun{Content: "Second paragraph"}},
							},
						},
					},
				},
			},
			expectedText: []string{"First paragraph\nSecond paragraph"},
		},
		{
			name: "table with cells",
			pageElements: []*slides.PageElement{
				{
					ObjectId: "table-1",
					Table: &slides.Table{
						TableRows: []*slides.TableRow{
							{
								TableCells: []*slides.TableCell{
									{Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Cell A1"}},
										},
									}},
									{Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Cell B1"}},
										},
									}},
								},
							},
						},
					},
				},
			},
			expectedText: []string{"[0,0]: Cell A1 | [0,1]: Cell B1"},
		},
		{
			name: "image element (no text)",
			pageElements: []*slides.PageElement{
				{
					ObjectId: "image-1",
					Image:    &slides.Image{},
				},
			},
			expectedText: []string{},
		},
		{
			name: "mixed elements",
			pageElements: []*slides.PageElement{
				{
					ObjectId: "text-1",
					Shape: &slides.Shape{
						ShapeType: "TEXT_BOX",
						Text: &slides.TextContent{
							TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "Title"}},
							},
						},
					},
				},
				{
					ObjectId: "image-1",
					Image:    &slides.Image{},
				},
				{
					ObjectId: "text-2",
					Shape: &slides.Shape{
						ShapeType: "RECTANGLE",
						Text: &slides.TextContent{
							TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: "Body text"}},
							},
						},
					},
				},
			},
			expectedText: []string{"Title", "Body text"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			textBlocks, _ := extractPageContent(tc.pageElements)

			if len(textBlocks) != len(tc.expectedText) {
				t.Errorf("expected %d text blocks, got %d", len(tc.expectedText), len(textBlocks))
				return
			}

			for i, expected := range tc.expectedText {
				if textBlocks[i].Text != expected {
					t.Errorf("expected text '%s', got '%s'", expected, textBlocks[i].Text)
				}
			}
		})
	}
}

func TestGetPresentation_ObjectTypeDetection(t *testing.T) {
	testCases := []struct {
		name         string
		element      *slides.PageElement
		expectedType string
	}{
		{
			name: "text box shape",
			element: &slides.PageElement{
				Shape: &slides.Shape{ShapeType: "TEXT_BOX"},
			},
			expectedType: "TEXT_BOX",
		},
		{
			name: "rectangle shape",
			element: &slides.PageElement{
				Shape: &slides.Shape{ShapeType: "RECTANGLE"},
			},
			expectedType: "RECTANGLE",
		},
		{
			name: "shape without type",
			element: &slides.PageElement{
				Shape: &slides.Shape{},
			},
			expectedType: "SHAPE",
		},
		{
			name: "image",
			element: &slides.PageElement{
				Image: &slides.Image{},
			},
			expectedType: "IMAGE",
		},
		{
			name: "video",
			element: &slides.PageElement{
				Video: &slides.Video{},
			},
			expectedType: "VIDEO",
		},
		{
			name: "table",
			element: &slides.PageElement{
				Table: &slides.Table{},
			},
			expectedType: "TABLE",
		},
		{
			name: "line",
			element: &slides.PageElement{
				Line: &slides.Line{},
			},
			expectedType: "LINE",
		},
		{
			name: "group",
			element: &slides.PageElement{
				ElementGroup: &slides.Group{},
			},
			expectedType: "GROUP",
		},
		{
			name: "sheets chart",
			element: &slides.PageElement{
				SheetsChart: &slides.SheetsChart{},
			},
			expectedType: "SHEETS_CHART",
		},
		{
			name: "word art",
			element: &slides.PageElement{
				WordArt: &slides.WordArt{},
			},
			expectedType: "WORD_ART",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objectType := determineObjectType(tc.element)
			if objectType != tc.expectedType {
				t.Errorf("expected type '%s', got '%s'", tc.expectedType, objectType)
			}
		})
	}
}

func TestGetPresentation_SpeakerNotesExtraction(t *testing.T) {
	testCases := []struct {
		name          string
		slide         *slides.Page
		expectedNotes string
	}{
		{
			name:          "nil slide",
			slide:         nil,
			expectedNotes: "",
		},
		{
			name: "slide without slide properties",
			slide: &slides.Page{
				ObjectId: "slide-1",
			},
			expectedNotes: "",
		},
		{
			name: "slide without notes page",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
			},
			expectedNotes: "",
		},
		{
			name: "slide with speaker notes in BODY placeholder",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-body",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "These are my speaker notes"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedNotes: "These are my speaker notes",
		},
		{
			name: "slide with notes in non-BODY shape (fallback)",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-text",
								Shape: &slides.Shape{
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Fallback notes text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedNotes: "Fallback notes text",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notes := extractSpeakerNotes(tc.slide)
			if notes != tc.expectedNotes {
				t.Errorf("expected notes '%s', got '%s'", tc.expectedNotes, notes)
			}
		})
	}
}

func TestGetPresentation_EmptyPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
		PresentationID: "",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if err.Error() != "presentation_id is required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestGetPresentation_NotFound(t *testing.T) {
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

	_, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
		PresentationID: "nonexistent-id",
	})

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestGetPresentation_AccessDenied(t *testing.T) {
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

	_, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
		PresentationID: "forbidden-id",
	})

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestGetPresentation_ServiceFactoryError(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
		PresentationID: "test-id",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestGetPresentation_WithThumbnails(t *testing.T) {
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
			// Return a thumbnail with an empty URL (we can't actually test HTTP fetching without a server)
			return &slides.Thumbnail{
				ContentUrl: "", // Empty URL will cause fetch to fail gracefully
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
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

func TestGetPresentation_GroupedElements(t *testing.T) {
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
											ObjectId: "text-in-group",
											Shape: &slides.Shape{
												ShapeType: "TEXT_BOX",
												Text: &slides.TextContent{
													TextElements: []*slides.TextElement{
														{TextRun: &slides.TextRun{Content: "Text inside group"}},
													},
												},
											},
										},
										{
											ObjectId: "image-in-group",
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
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetPresentation(context.Background(), tokenSource, GetPresentationInput{
		PresentationID: "test-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	slide := output.Slides[0]

	// Should have text from inside the group
	found := false
	for _, tb := range slide.TextContent {
		if tb.Text == "Text inside group" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find text from grouped element")
	}

	// Should have objects from group: group itself + 2 children
	if len(slide.Objects) != 3 {
		t.Errorf("expected 3 objects (group + 2 children), got %d", len(slide.Objects))
	}
}

func TestIsNotFoundError(t *testing.T) {
	testCases := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("some error"), false},
		{errors.New("googleapi: Error 404: not found"), true},
		{errors.New("notFound"), true},
		{errors.New("file not found"), true},
		{errors.New("googleapi: Error 403: forbidden"), false},
	}

	for _, tc := range testCases {
		result := isNotFoundError(tc.err)
		if result != tc.expected {
			t.Errorf("isNotFoundError(%v) = %v, expected %v", tc.err, result, tc.expected)
		}
	}
}

func TestIsForbiddenError(t *testing.T) {
	testCases := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("some error"), false},
		{errors.New("googleapi: Error 403: forbidden"), true},
		{errors.New("access denied"), true},
		{errors.New("permission denied"), true},
		{errors.New("googleapi: Error 404: not found"), false},
	}

	for _, tc := range testCases {
		result := isForbiddenError(tc.err)
		if result != tc.expected {
			t.Errorf("isForbiddenError(%v) = %v, expected %v", tc.err, result, tc.expected)
		}
	}
}

func TestExtractTextFromTextContent(t *testing.T) {
	testCases := []struct {
		name     string
		content  *slides.TextContent
		expected string
	}{
		{
			name:     "nil content",
			content:  nil,
			expected: "",
		},
		{
			name:     "empty elements",
			content:  &slides.TextContent{TextElements: []*slides.TextElement{}},
			expected: "",
		},
		{
			name: "single text run",
			content: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{TextRun: &slides.TextRun{Content: "Hello"}},
				},
			},
			expected: "Hello",
		},
		{
			name: "multiple text runs",
			content: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{TextRun: &slides.TextRun{Content: "Hello "}},
					{TextRun: &slides.TextRun{Content: "World"}},
				},
			},
			expected: "Hello World",
		},
		{
			name: "text with whitespace trimming",
			content: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{TextRun: &slides.TextRun{Content: "  Trimmed  "}},
				},
			},
			expected: "Trimmed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTextFromTextContent(tc.content)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}
