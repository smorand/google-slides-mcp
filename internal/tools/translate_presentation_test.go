package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// mockTranslateService is a mock implementation of TranslateService for testing.
type mockTranslateService struct {
	translations     map[string]string // Maps source text to translated text
	translateError   error
	translateCalled  bool
	lastSourceLang   string
	lastTargetLang   string
	lastTexts        []string
}

func (m *mockTranslateService) TranslateText(ctx context.Context, text, targetLanguage, sourceLanguage string) (string, error) {
	results, err := m.TranslateTexts(ctx, []string{text}, targetLanguage, sourceLanguage)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}
	return results[0], nil
}

func (m *mockTranslateService) TranslateTexts(ctx context.Context, texts []string, targetLanguage, sourceLanguage string) ([]string, error) {
	m.translateCalled = true
	m.lastTargetLang = targetLanguage
	m.lastSourceLang = sourceLanguage
	m.lastTexts = texts

	if m.translateError != nil {
		return nil, m.translateError
	}

	results := make([]string, len(texts))
	for i, text := range texts {
		if translated, ok := m.translations[text]; ok {
			results[i] = translated
		} else {
			// Default behavior: return original text with target language suffix
			results[i] = text + " [" + targetLanguage + "]"
		}
	}
	return results, nil
}

// mockSlidesServiceForTranslate is a mock for Slides API operations in translate tests.
type mockSlidesServiceForTranslate struct {
	presentation      *slides.Presentation
	getError          error
	batchUpdateError  error
	batchUpdateCalled bool
	lastRequests      []*slides.Request
}

func (m *mockSlidesServiceForTranslate) GetPresentation(ctx context.Context, presentationID string) (*slides.Presentation, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.presentation, nil
}

func (m *mockSlidesServiceForTranslate) GetThumbnail(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
	return nil, nil
}

func (m *mockSlidesServiceForTranslate) CreatePresentation(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
	return nil, nil
}

func (m *mockSlidesServiceForTranslate) BatchUpdate(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
	m.batchUpdateCalled = true
	m.lastRequests = requests
	if m.batchUpdateError != nil {
		return nil, m.batchUpdateError
	}
	return &slides.BatchUpdatePresentationResponse{}, nil
}

func TestTranslatePresentation(t *testing.T) {
	tests := []struct {
		name             string
		input            TranslatePresentationInput
		presentation     *slides.Presentation
		translations     map[string]string
		getError         error
		batchUpdateError error
		translateError   error
		wantErr          error
		wantErrContains  string
		checkOutput      func(t *testing.T, output *TranslatePresentationOutput)
		checkRequests    func(t *testing.T, requests []*slides.Request)
	}{
		{
			name: "translate all text in presentation",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Hello World"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Hello World": "Bonjour le monde",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 1 {
					t.Errorf("TranslatedCount = %d, want 1", output.TranslatedCount)
				}
				if output.TargetLanguage != "fr" {
					t.Errorf("TargetLanguage = %s, want 'fr'", output.TargetLanguage)
				}
				if len(output.AffectedSlides) != 1 || output.AffectedSlides[0] != 1 {
					t.Errorf("AffectedSlides = %v, want [1]", output.AffectedSlides)
				}
				if len(output.TranslatedElements) != 1 {
					t.Fatalf("TranslatedElements count = %d, want 1", len(output.TranslatedElements))
				}
				elem := output.TranslatedElements[0]
				if elem.OriginalText != "Hello World" {
					t.Errorf("OriginalText = %s, want 'Hello World'", elem.OriginalText)
				}
				if elem.TranslatedText != "Bonjour le monde" {
					t.Errorf("TranslatedText = %s, want 'Bonjour le monde'", elem.TranslatedText)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				// Should have 2 requests: DeleteText and InsertText
				if len(requests) != 2 {
					t.Fatalf("Expected 2 requests, got %d", len(requests))
				}
				// First should be DeleteText
				if requests[0].DeleteText == nil {
					t.Error("First request should be DeleteText")
				}
				// Second should be InsertText
				if requests[1].InsertText == nil {
					t.Error("Second request should be InsertText")
				}
				if requests[1].InsertText.Text != "Bonjour le monde" {
					t.Errorf("InsertText.Text = %s, want 'Bonjour le monde'", requests[1].InsertText.Text)
				}
			},
		},
		{
			name: "translate with source language specified",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "es",
				SourceLanguage: "en",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Good morning"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Good morning": "Buenos días",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.SourceLanguage != "en" {
					t.Errorf("SourceLanguage = %s, want 'en'", output.SourceLanguage)
				}
				if output.TargetLanguage != "es" {
					t.Errorf("TargetLanguage = %s, want 'es'", output.TargetLanguage)
				}
			},
		},
		{
			name: "translate specific slide only",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "de",
				Scope:          "slide",
				SlideIndex:     2,
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Slide 1 text"}},
										},
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
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Slide 2 text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Slide 2 text": "Folie 2 Text",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 1 {
					t.Errorf("TranslatedCount = %d, want 1", output.TranslatedCount)
				}
				if len(output.AffectedSlides) != 1 || output.AffectedSlides[0] != 2 {
					t.Errorf("AffectedSlides = %v, want [2]", output.AffectedSlides)
				}
				if len(output.TranslatedElements) != 1 {
					t.Fatalf("TranslatedElements count = %d, want 1", len(output.TranslatedElements))
				}
				if output.TranslatedElements[0].SlideIndex != 2 {
					t.Errorf("SlideIndex = %d, want 2", output.TranslatedElements[0].SlideIndex)
				}
			},
		},
		{
			name: "translate specific slide by ID",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "ja",
				Scope:          "slide",
				SlideID:        "slide-2",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Slide 1"}},
										},
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
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Slide 2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Slide 2": "スライド2",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 1 {
					t.Errorf("TranslatedCount = %d, want 1", output.TranslatedCount)
				}
				if len(output.TranslatedElements) != 1 {
					t.Fatalf("TranslatedElements count = %d, want 1", len(output.TranslatedElements))
				}
				if output.TranslatedElements[0].TranslatedText != "スライド2" {
					t.Errorf("TranslatedText = %s, want 'スライド2'", output.TranslatedElements[0].TranslatedText)
				}
			},
		},
		{
			name: "translate specific object only",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "it",
				Scope:          "object",
				ObjectID:       "shape-2",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Shape 1"}},
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
											{TextRun: &slides.TextRun{Content: "Shape 2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Shape 2": "Forma 2",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 1 {
					t.Errorf("TranslatedCount = %d, want 1", output.TranslatedCount)
				}
				if len(output.TranslatedElements) != 1 {
					t.Fatalf("TranslatedElements count = %d, want 1", len(output.TranslatedElements))
				}
				if output.TranslatedElements[0].ObjectID != "shape-2" {
					t.Errorf("ObjectID = %s, want 'shape-2'", output.TranslatedElements[0].ObjectID)
				}
			},
		},
		{
			name: "translate multiple shapes across slides",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "pt",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Hello"}},
										},
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
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "World"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Hello": "Olá",
				"World": "Mundo",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 2 {
					t.Errorf("TranslatedCount = %d, want 2", output.TranslatedCount)
				}
				if len(output.AffectedSlides) != 2 {
					t.Errorf("AffectedSlides count = %d, want 2", len(output.AffectedSlides))
				}
			},
		},
		{
			name: "skip whitespace-only text",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "zh",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "   "}}, // whitespace only
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
											{TextRun: &slides.TextRun{Content: "Translate me"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Translate me": "翻译我",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 1 {
					t.Errorf("TranslatedCount = %d, want 1", output.TranslatedCount)
				}
			},
		},
		{
			name: "skip unchanged translations",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "ko",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Same text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Same text": "Same text", // Translation unchanged
			},
			wantErr: ErrNoTextToTranslate,
		},
		{
			name: "translate text in nested groups",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "nl",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{
											ObjectId: "shape-in-group",
											Shape: &slides.Shape{
												ShapeType: "TEXT_BOX",
												Text: &slides.TextContent{
													TextElements: []*slides.TextElement{
														{TextRun: &slides.TextRun{Content: "Grouped text"}},
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
			translations: map[string]string{
				"Grouped text": "Gegroepeerde tekst",
			},
			checkOutput: func(t *testing.T, output *TranslatePresentationOutput) {
				if output.TranslatedCount != 1 {
					t.Errorf("TranslatedCount = %d, want 1", output.TranslatedCount)
				}
				if len(output.TranslatedElements) != 1 {
					t.Fatalf("TranslatedElements count = %d, want 1", len(output.TranslatedElements))
				}
				if output.TranslatedElements[0].ObjectID != "shape-in-group" {
					t.Errorf("ObjectID = %s, want 'shape-in-group'", output.TranslatedElements[0].ObjectID)
				}
			},
		},
		// Error cases
		{
			name: "missing presentation_id",
			input: TranslatePresentationInput{
				TargetLanguage: "fr",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing target_language",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
			},
			wantErr: ErrInvalidTargetLanguage,
		},
		{
			name: "invalid scope",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
				Scope:          "invalid",
			},
			wantErr: ErrInvalidScope,
		},
		{
			name: "scope slide without slide reference",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
				Scope:          "slide",
			},
			wantErr:         ErrInvalidScope,
			wantErrContains: "slide_index or slide_id is required",
		},
		{
			name: "scope object without object_id",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
				Scope:          "object",
			},
			wantErr:         ErrInvalidScope,
			wantErrContains: "object_id is required",
		},
		{
			name: "slide not found by index",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
				Scope:          "slide",
				SlideIndex:     99,
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "slide not found by ID",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
				Scope:          "slide",
				SlideID:        "nonexistent",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "object not found",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
				Scope:          "object",
				ObjectID:       "nonexistent",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
				},
			},
			wantErr: ErrObjectNotFound,
		},
		{
			name: "presentation not found",
			input: TranslatePresentationInput{
				PresentationID: "nonexistent",
				TargetLanguage: "fr",
			},
			getError: errors.New("404 Not Found"),
			wantErr:  ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
			},
			getError: errors.New("403 Forbidden"),
			wantErr:  ErrAccessDenied,
		},
		{
			name: "translate API error",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translateError: errors.New("Translation API error"),
			wantErr:        ErrTranslateAPIError,
		},
		{
			name: "batch update fails",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
											{TextRun: &slides.TextRun{Content: "Text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			translations: map[string]string{
				"Text": "Texte",
			},
			batchUpdateError: errors.New("API error"),
			wantErr:          ErrTranslateFailed,
		},
		{
			name: "no text to translate (empty presentation)",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides:         []*slides.Page{},
			},
			wantErr: ErrNoTextToTranslate,
		},
		{
			name: "no text to translate (shapes without text)",
			input: TranslatePresentationInput{
				PresentationID: "pres-123",
				TargetLanguage: "fr",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
				},
			},
			wantErr: ErrNoTextToTranslate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slidesMock := &mockSlidesServiceForTranslate{
				presentation:     tt.presentation,
				getError:         tt.getError,
				batchUpdateError: tt.batchUpdateError,
			}

			translateMock := &mockTranslateService{
				translations:   tt.translations,
				translateError: tt.translateError,
			}

			tools := NewToolsWithAllServices(
				DefaultToolsConfig(),
				func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
					return slidesMock, nil
				},
				nil, // DriveService not needed for translate tests
				func(ctx context.Context, ts oauth2.TokenSource) (TranslateService, error) {
					return translateMock, nil
				},
			)

			output, err := tools.TranslatePresentation(context.Background(), nil, tt.input)

			// Check error
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantErrContains != "" && !containsString(err.Error(), tt.wantErrContains) {
					t.Errorf("Error message = %q, want to contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check output
			if tt.checkOutput != nil {
				tt.checkOutput(t, output)
			}

			// Check requests
			if tt.checkRequests != nil && slidesMock.batchUpdateCalled {
				tt.checkRequests(t, slidesMock.lastRequests)
			}
		})
	}
}

func TestCollectTextElements(t *testing.T) {
	tests := []struct {
		name         string
		presentation *slides.Presentation
		input        TranslatePresentationInput
		wantCount    int
		wantErr      error
	}{
		{
			name: "collect from single shape",
			presentation: &slides.Presentation{
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
											{TextRun: &slides.TextRun{Content: "Test text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			input: TranslatePresentationInput{
				Scope: "all",
			},
			wantCount: 1,
		},
		{
			name: "skip empty text",
			presentation: &slides.Presentation{
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
											{TextRun: &slides.TextRun{Content: ""}},
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
											{TextRun: &slides.TextRun{Content: "Valid text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			input: TranslatePresentationInput{
				Scope: "all",
			},
			wantCount: 1,
		},
		{
			name: "collect from group children",
			presentation: &slides.Presentation{
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{
											ObjectId: "shape-in-group",
											Shape: &slides.Shape{
												ShapeType: "TEXT_BOX",
												Text: &slides.TextContent{
													TextElements: []*slides.TextElement{
														{TextRun: &slides.TextRun{Content: "Grouped"}},
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
			input: TranslatePresentationInput{
				Scope: "all",
			},
			wantCount: 1,
		},
		{
			name: "filter by slide index",
			presentation: &slides.Presentation{
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
											{TextRun: &slides.TextRun{Content: "Slide 1"}},
										},
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
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Slide 2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			input: TranslatePresentationInput{
				Scope:      "slide",
				SlideIndex: 2,
			},
			wantCount: 1,
		},
		{
			name: "filter by object ID",
			presentation: &slides.Presentation{
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
											{TextRun: &slides.TextRun{Content: "Shape 1"}},
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
											{TextRun: &slides.TextRun{Content: "Shape 2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			input: TranslatePresentationInput{
				Scope:    "object",
				ObjectID: "shape-2",
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := NewTools(DefaultToolsConfig(), nil)
			elements, err := tools.collectTextElements(tt.presentation, tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(elements) != tt.wantCount {
				t.Errorf("Element count = %d, want %d", len(elements), tt.wantCount)
			}
		})
	}
}
