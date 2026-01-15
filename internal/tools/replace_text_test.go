package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// mockSlidesServiceForReplace is a mock that tracks ReplaceAllText calls.
type mockSlidesServiceForReplace struct {
	presentation      *slides.Presentation
	getError          error
	batchUpdateError  error
	batchUpdateCalled bool
	lastRequests      []*slides.Request
	occurrencesChanged int64
}

func (m *mockSlidesServiceForReplace) GetPresentation(ctx context.Context, presentationID string) (*slides.Presentation, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.presentation, nil
}

func (m *mockSlidesServiceForReplace) GetThumbnail(ctx context.Context, presentationID, pageObjectID string) (*slides.Thumbnail, error) {
	return nil, nil
}

func (m *mockSlidesServiceForReplace) CreatePresentation(ctx context.Context, presentation *slides.Presentation) (*slides.Presentation, error) {
	return nil, nil
}

func (m *mockSlidesServiceForReplace) BatchUpdate(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
	m.batchUpdateCalled = true
	m.lastRequests = requests
	if m.batchUpdateError != nil {
		return nil, m.batchUpdateError
	}
	return &slides.BatchUpdatePresentationResponse{
		Replies: []*slides.Response{
			{
				ReplaceAllText: &slides.ReplaceAllTextResponse{
					OccurrencesChanged: m.occurrencesChanged,
				},
			},
		},
	}, nil
}

func TestReplaceText(t *testing.T) {
	tests := []struct {
		name               string
		input              ReplaceTextInput
		presentation       *slides.Presentation
		getError           error
		batchUpdateError   error
		occurrencesChanged int64
		wantErr            error
		wantErrContains    string
		checkOutput        func(t *testing.T, output *ReplaceTextOutput)
		checkRequests      func(t *testing.T, requests []*slides.Request)
	}{
		{
			name: "replace all occurrences in presentation",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "old text",
				ReplaceWith:    "new text",
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
											{TextRun: &slides.TextRun{Content: "This has old text here"}},
										},
									},
								},
							},
						},
					},
				},
			},
			occurrencesChanged: 3,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if output.ReplacementCount != 3 {
					t.Errorf("ReplacementCount = %d, want 3", output.ReplacementCount)
				}
				if output.Scope != "all" {
					t.Errorf("Scope = %s, want 'all'", output.Scope)
				}
				if len(output.AffectedObjects) != 1 {
					t.Errorf("AffectedObjects count = %d, want 1", len(output.AffectedObjects))
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("Expected 1 request, got %d", len(requests))
				}
				req := requests[0].ReplaceAllText
				if req == nil {
					t.Fatal("Expected ReplaceAllText request")
				}
				if req.ContainsText.Text != "old text" {
					t.Errorf("Find text = %s, want 'old text'", req.ContainsText.Text)
				}
				if req.ReplaceText != "new text" {
					t.Errorf("Replace text = %s, want 'new text'", req.ReplaceText)
				}
				if req.ContainsText.MatchCase {
					t.Error("MatchCase should be false by default")
				}
				if len(req.PageObjectIds) != 0 {
					t.Errorf("PageObjectIds should be empty for 'all' scope, got %v", req.PageObjectIds)
				}
			},
		},
		{
			name: "case sensitive replacement",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "Old",
				ReplaceWith:    "New",
				CaseSensitive:  true,
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
											{TextRun: &slides.TextRun{Content: "Old text, not old text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			occurrencesChanged: 1,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if !output.CaseSensitive {
					t.Error("CaseSensitive should be true")
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].ReplaceAllText
				if !req.ContainsText.MatchCase {
					t.Error("MatchCase should be true")
				}
			},
		},
		{
			name: "scope slide limits to specific slide",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "target",
				ReplaceWith:    "replaced",
				Scope:          "slide",
				SlideID:        "slide-2",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{
						ObjectId: "slide-2",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-1",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "target text"}},
										},
									},
								},
							},
						},
					},
					{ObjectId: "slide-3"},
				},
			},
			occurrencesChanged: 1,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if output.Scope != "slide" {
					t.Errorf("Scope = %s, want 'slide'", output.Scope)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].ReplaceAllText
				if len(req.PageObjectIds) != 1 || req.PageObjectIds[0] != "slide-2" {
					t.Errorf("PageObjectIds = %v, want [slide-2]", req.PageObjectIds)
				}
			},
		},
		{
			name: "scope object limits to slide containing object",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "target",
				ReplaceWith:    "replaced",
				Scope:          "object",
				ObjectID:       "shape-in-slide-2",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{
						ObjectId: "slide-2",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-in-slide-2",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "target here"}},
										},
									},
								},
							},
						},
					},
				},
			},
			occurrencesChanged: 1,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if output.Scope != "object" {
					t.Errorf("Scope = %s, want 'object'", output.Scope)
				}
				// Only the specific object should be in affected list
				if len(output.AffectedObjects) != 1 {
					t.Errorf("AffectedObjects count = %d, want 1", len(output.AffectedObjects))
				}
				if len(output.AffectedObjects) > 0 && output.AffectedObjects[0].ObjectID != "shape-in-slide-2" {
					t.Errorf("AffectedObjects[0].ObjectID = %s, want 'shape-in-slide-2'", output.AffectedObjects[0].ObjectID)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].ReplaceAllText
				// API scopes to slide level, not object level
				if len(req.PageObjectIds) != 1 || req.PageObjectIds[0] != "slide-2" {
					t.Errorf("PageObjectIds = %v, want [slide-2]", req.PageObjectIds)
				}
			},
		},
		{
			name: "no replacements returns zero count",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "not found",
				ReplaceWith:    "replacement",
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
											{TextRun: &slides.TextRun{Content: "different text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			occurrencesChanged: 0,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if output.ReplacementCount != 0 {
					t.Errorf("ReplacementCount = %d, want 0", output.ReplacementCount)
				}
				if len(output.AffectedObjects) != 0 {
					t.Errorf("AffectedObjects should be empty, got %d", len(output.AffectedObjects))
				}
			},
		},
		{
			name: "replace with empty string (delete)",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "delete me",
				ReplaceWith:    "",
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
											{TextRun: &slides.TextRun{Content: "delete me please"}},
										},
									},
								},
							},
						},
					},
				},
			},
			occurrencesChanged: 1,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if output.ReplaceWith != "" {
					t.Errorf("ReplaceWith = %s, want empty string", output.ReplaceWith)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].ReplaceAllText
				if req.ReplaceText != "" {
					t.Errorf("ReplaceText = %s, want empty string", req.ReplaceText)
				}
			},
		},
		{
			name: "affected objects includes tables",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "table text",
				ReplaceWith:    "replaced",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
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
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "table text here"}},
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
			},
			occurrencesChanged: 1,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if len(output.AffectedObjects) != 1 {
					t.Fatalf("AffectedObjects count = %d, want 1", len(output.AffectedObjects))
				}
				if output.AffectedObjects[0].ObjectType != "TABLE" {
					t.Errorf("ObjectType = %s, want 'TABLE'", output.AffectedObjects[0].ObjectType)
				}
			},
		},
		{
			name: "affected objects in groups",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "group text",
				ReplaceWith:    "replaced",
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
														{TextRun: &slides.TextRun{Content: "group text inside"}},
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
			occurrencesChanged: 1,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if len(output.AffectedObjects) != 1 {
					t.Fatalf("AffectedObjects count = %d, want 1", len(output.AffectedObjects))
				}
				if output.AffectedObjects[0].ObjectID != "shape-in-group" {
					t.Errorf("ObjectID = %s, want 'shape-in-group'", output.AffectedObjects[0].ObjectID)
				}
			},
		},
		{
			name: "multiple slides with matches",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "common",
				ReplaceWith:    "replaced",
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
											{TextRun: &slides.TextRun{Content: "common text"}},
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
											{TextRun: &slides.TextRun{Content: "also common"}},
										},
									},
								},
							},
						},
					},
				},
			},
			occurrencesChanged: 2,
			checkOutput: func(t *testing.T, output *ReplaceTextOutput) {
				if len(output.AffectedObjects) != 2 {
					t.Fatalf("AffectedObjects count = %d, want 2", len(output.AffectedObjects))
				}
				if output.AffectedObjects[0].SlideIndex != 1 {
					t.Errorf("First object SlideIndex = %d, want 1", output.AffectedObjects[0].SlideIndex)
				}
				if output.AffectedObjects[1].SlideIndex != 2 {
					t.Errorf("Second object SlideIndex = %d, want 2", output.AffectedObjects[1].SlideIndex)
				}
			},
		},
		// Error cases
		{
			name: "missing presentation_id",
			input: ReplaceTextInput{
				Find:        "text",
				ReplaceWith: "replacement",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "empty find text",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "",
				ReplaceWith:    "replacement",
			},
			wantErr: ErrInvalidFind,
		},
		{
			name: "invalid scope",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
				Scope:          "invalid",
			},
			wantErr: ErrInvalidScope,
		},
		{
			name: "scope slide without slide_id",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
				Scope:          "slide",
			},
			wantErr:         ErrInvalidScope,
			wantErrContains: "slide_id is required",
		},
		{
			name: "scope object without object_id",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
				Scope:          "object",
			},
			wantErr:         ErrInvalidScope,
			wantErrContains: "object_id is required",
		},
		{
			name: "slide not found",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
				Scope:          "slide",
				SlideID:        "nonexistent-slide",
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
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
				Scope:          "object",
				ObjectID:       "nonexistent-object",
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
			input: ReplaceTextInput{
				PresentationID: "nonexistent",
				Find:           "text",
				ReplaceWith:    "replacement",
			},
			getError: errors.New("404 Not Found"),
			wantErr:  ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
			},
			getError: errors.New("403 Forbidden"),
			wantErr:  ErrAccessDenied,
		},
		{
			name: "batch update fails",
			input: ReplaceTextInput{
				PresentationID: "pres-123",
				Find:           "text",
				ReplaceWith:    "replacement",
			},
			presentation: &slides.Presentation{
				PresentationId: "pres-123",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			},
			batchUpdateError: errors.New("API error"),
			wantErr:          ErrReplaceTextFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSlidesServiceForReplace{
				presentation:       tt.presentation,
				getError:           tt.getError,
				batchUpdateError:   tt.batchUpdateError,
				occurrencesChanged: tt.occurrencesChanged,
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mock, nil
			})

			output, err := tools.ReplaceText(context.Background(), nil, tt.input)

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
			if tt.checkRequests != nil && mock.batchUpdateCalled {
				tt.checkRequests(t, mock.lastRequests)
			}
		})
	}
}

func TestTextContains(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		find          string
		caseSensitive bool
		want          bool
	}{
		{
			name:          "case insensitive match lowercase",
			text:          "Hello World",
			find:          "hello",
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case insensitive match uppercase",
			text:          "Hello World",
			find:          "WORLD",
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case sensitive no match",
			text:          "Hello World",
			find:          "hello",
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "case sensitive match",
			text:          "Hello World",
			find:          "Hello",
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "empty find",
			text:          "Hello World",
			find:          "",
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "empty text",
			text:          "",
			find:          "hello",
			caseSensitive: false,
			want:          false,
		},
		{
			name:          "partial match",
			text:          "The quick brown fox",
			find:          "quick",
			caseSensitive: false,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textContains(tt.text, tt.find, tt.caseSensitive)
			if got != tt.want {
				t.Errorf("textContains(%q, %q, %v) = %v, want %v", tt.text, tt.find, tt.caseSensitive, got, tt.want)
			}
		})
	}
}

func TestFindSlideContainingObject(t *testing.T) {
	slides := []*slides.Page{
		{
			ObjectId: "slide-1",
			PageElements: []*slides.PageElement{
				{ObjectId: "shape-1"},
				{ObjectId: "shape-2"},
			},
		},
		{
			ObjectId: "slide-2",
			PageElements: []*slides.PageElement{
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{ObjectId: "nested-shape"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		objectID string
		wantID   string
	}{
		{"direct element", "shape-1", "slide-1"},
		{"second slide", "group-1", "slide-2"},
		{"nested in group", "nested-shape", "slide-2"},
		{"not found", "nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSlideContainingObject(slides, tt.objectID)
			if tt.wantID == "" {
				if result != nil {
					t.Errorf("Expected nil, got slide %s", result.ObjectId)
				}
			} else {
				if result == nil {
					t.Fatalf("Expected slide %s, got nil", tt.wantID)
				}
				if result.ObjectId != tt.wantID {
					t.Errorf("Slide ID = %s, want %s", result.ObjectId, tt.wantID)
				}
			}
		})
	}
}

func TestContainsSearchText(t *testing.T) {
	tests := []struct {
		name          string
		element       *slides.PageElement
		find          string
		caseSensitive bool
		want          bool
	}{
		{
			name: "shape with matching text",
			element: &slides.PageElement{
				Shape: &slides.Shape{
					Text: &slides.TextContent{
						TextElements: []*slides.TextElement{
							{TextRun: &slides.TextRun{Content: "Hello World"}},
						},
					},
				},
			},
			find:          "world",
			caseSensitive: false,
			want:          true,
		},
		{
			name: "shape without match",
			element: &slides.PageElement{
				Shape: &slides.Shape{
					Text: &slides.TextContent{
						TextElements: []*slides.TextElement{
							{TextRun: &slides.TextRun{Content: "Hello"}},
						},
					},
				},
			},
			find:          "world",
			caseSensitive: false,
			want:          false,
		},
		{
			name: "table with matching text",
			element: &slides.PageElement{
				Table: &slides.Table{
					TableRows: []*slides.TableRow{
						{
							TableCells: []*slides.TableCell{
								{
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "target text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			find:          "target",
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "nil element",
			element:       nil,
			find:          "test",
			caseSensitive: false,
			want:          false,
		},
		{
			name: "element without text",
			element: &slides.PageElement{
				Image: &slides.Image{},
			},
			find:          "test",
			caseSensitive: false,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsSearchText(tt.element, tt.find, tt.caseSensitive)
			if got != tt.want {
				t.Errorf("containsSearchText() = %v, want %v", got, tt.want)
			}
		})
	}
}

// containsString is a simple helper for checking if a string contains a substring.
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
