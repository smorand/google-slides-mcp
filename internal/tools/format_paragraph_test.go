package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestFormatParagraph(t *testing.T) {
	ctx := context.Background()

	// Helper to create float64 pointer
	floatPtr := func(f float64) *float64 { return &f }
	intPtr := func(i int) *int { return &i }

	// Create a basic presentation with a text box for testing
	createTestPresentation := func() *slides.Presentation {
		return &slides.Presentation{
			PresentationId: "test-presentation-id",
			Slides: []*slides.Page{
				{
					ObjectId: "slide-1",
					PageElements: []*slides.PageElement{
						{
							ObjectId: "textbox-1",
							Shape: &slides.Shape{
								ShapeType: "TEXT_BOX",
								Text: &slides.TextContent{
									TextElements: []*slides.TextElement{
										{
											StartIndex: 0,
											EndIndex:   12,
											TextRun: &slides.TextRun{
												Content: "Hello World\n",
											},
										},
										{
											StartIndex: 0,
											EndIndex:   12,
											ParagraphMarker: &slides.ParagraphMarker{},
										},
									},
								},
							},
						},
						{
							ObjectId: "image-1",
							Image:    &slides.Image{},
						},
						{
							ObjectId: "table-1",
							Table:    &slides.Table{},
						},
					},
				},
			},
		}
	}

	// Create a presentation with multiple paragraphs
	createMultiParagraphPresentation := func() *slides.Presentation {
		return &slides.Presentation{
			PresentationId: "test-presentation-id",
			Slides: []*slides.Page{
				{
					ObjectId: "slide-1",
					PageElements: []*slides.PageElement{
						{
							ObjectId: "textbox-multi",
							Shape: &slides.Shape{
								ShapeType: "TEXT_BOX",
								Text: &slides.TextContent{
									TextElements: []*slides.TextElement{
										{
											StartIndex: 0,
											EndIndex:   6,
											TextRun: &slides.TextRun{
												Content: "First\n",
											},
										},
										{
											StartIndex: 0,
											EndIndex:   6,
											ParagraphMarker: &slides.ParagraphMarker{},
										},
										{
											StartIndex: 6,
											EndIndex:   13,
											TextRun: &slides.TextRun{
												Content: "Second\n",
											},
										},
										{
											StartIndex: 6,
											EndIndex:   13,
											ParagraphMarker: &slides.ParagraphMarker{},
										},
										{
											StartIndex: 13,
											EndIndex:   19,
											TextRun: &slides.TextRun{
												Content: "Third\n",
											},
										},
										{
											StartIndex: 13,
											EndIndex:   19,
											ParagraphMarker: &slides.ParagraphMarker{},
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name           string
		input          FormatParagraphInput
		presentation   *slides.Presentation
		getErr         error
		batchUpdateErr error
		wantErr        error
		wantErrMsg     string
		checkOutput    func(*testing.T, *FormatParagraphOutput)
		checkRequests  func(*testing.T, []*slides.Request)
	}{
		// === Alignment tests ===
		{
			name: "apply alignment START",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "START",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if output.ObjectID != "textbox-1" {
					t.Errorf("expected object_id 'textbox-1', got %s", output.ObjectID)
				}
				if output.ParagraphScope != "ALL" {
					t.Errorf("expected paragraph_scope 'ALL', got %s", output.ParagraphScope)
				}
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "alignment=START" {
					t.Errorf("expected applied_formatting [alignment=START], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				req := requests[0].UpdateParagraphStyle
				if req.Style.Alignment != "START" {
					t.Errorf("expected alignment START, got %s", req.Style.Alignment)
				}
				if req.TextRange.Type != "ALL" {
					t.Errorf("expected text range type ALL, got %s", req.TextRange.Type)
				}
				if req.Fields != "alignment" {
					t.Errorf("expected fields 'alignment', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply alignment CENTER",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "center", // lowercase should be normalized
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "alignment=CENTER" {
					t.Errorf("expected applied_formatting [alignment=CENTER], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.Alignment != "CENTER" {
					t.Errorf("expected alignment CENTER, got %s", req.Style.Alignment)
				}
			},
		},
		{
			name: "apply alignment END",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "END",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "alignment=END" {
					t.Errorf("expected applied_formatting [alignment=END], got %v", output.AppliedFormatting)
				}
			},
		},
		{
			name: "apply alignment JUSTIFIED",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "JUSTIFIED",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "alignment=JUSTIFIED" {
					t.Errorf("expected applied_formatting [alignment=JUSTIFIED], got %v", output.AppliedFormatting)
				}
			},
		},
		{
			name: "invalid alignment value",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "INVALID",
				},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrInvalidAlignment,
		},

		// === Line spacing tests ===
		{
			name: "apply line spacing 100% (normal)",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					LineSpacing: floatPtr(100.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "line_spacing=100.0%" {
					t.Errorf("expected applied_formatting [line_spacing=100.0%%], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.LineSpacing != 100.0 {
					t.Errorf("expected line spacing 100.0, got %v", req.Style.LineSpacing)
				}
				if req.Fields != "lineSpacing" {
					t.Errorf("expected fields 'lineSpacing', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply line spacing 150% (1.5 lines)",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					LineSpacing: floatPtr(150.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "line_spacing=150.0%" {
					t.Errorf("expected applied_formatting [line_spacing=150.0%%], got %v", output.AppliedFormatting)
				}
			},
		},

		// === Space above/below tests ===
		{
			name: "apply space above",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					SpaceAbove: floatPtr(12.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "space_above=12.0pt" {
					t.Errorf("expected applied_formatting [space_above=12.0pt], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.SpaceAbove == nil || req.Style.SpaceAbove.Magnitude != 12.0 || req.Style.SpaceAbove.Unit != "PT" {
					t.Errorf("expected space above 12.0pt, got %v", req.Style.SpaceAbove)
				}
				if req.Fields != "spaceAbove" {
					t.Errorf("expected fields 'spaceAbove', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply space below",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					SpaceBelow: floatPtr(6.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "space_below=6.0pt" {
					t.Errorf("expected applied_formatting [space_below=6.0pt], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.SpaceBelow == nil || req.Style.SpaceBelow.Magnitude != 6.0 || req.Style.SpaceBelow.Unit != "PT" {
					t.Errorf("expected space below 6.0pt, got %v", req.Style.SpaceBelow)
				}
			},
		},

		// === Indentation tests ===
		{
			name: "apply indent first line",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					IndentFirstLine: floatPtr(36.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "indent_first_line=36.0pt" {
					t.Errorf("expected applied_formatting [indent_first_line=36.0pt], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.IndentFirstLine == nil || req.Style.IndentFirstLine.Magnitude != 36.0 || req.Style.IndentFirstLine.Unit != "PT" {
					t.Errorf("expected indent first line 36.0pt, got %v", req.Style.IndentFirstLine)
				}
				if req.Fields != "indentFirstLine" {
					t.Errorf("expected fields 'indentFirstLine', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply indent start",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					IndentStart: floatPtr(24.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "indent_start=24.0pt" {
					t.Errorf("expected applied_formatting [indent_start=24.0pt], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.IndentStart == nil || req.Style.IndentStart.Magnitude != 24.0 || req.Style.IndentStart.Unit != "PT" {
					t.Errorf("expected indent start 24.0pt, got %v", req.Style.IndentStart)
				}
			},
		},
		{
			name: "apply indent end",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					IndentEnd: floatPtr(18.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 1 || output.AppliedFormatting[0] != "indent_end=18.0pt" {
					t.Errorf("expected applied_formatting [indent_end=18.0pt], got %v", output.AppliedFormatting)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.IndentEnd == nil || req.Style.IndentEnd.Magnitude != 18.0 || req.Style.IndentEnd.Unit != "PT" {
					t.Errorf("expected indent end 18.0pt, got %v", req.Style.IndentEnd)
				}
			},
		},

		// === Multiple formatting options ===
		{
			name: "apply multiple formatting options",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment:   "CENTER",
					LineSpacing: floatPtr(150.0),
					SpaceAbove:  floatPtr(10.0),
					SpaceBelow:  floatPtr(5.0),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if len(output.AppliedFormatting) != 4 {
					t.Errorf("expected 4 applied formatting, got %d", len(output.AppliedFormatting))
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.Style.Alignment != "CENTER" {
					t.Errorf("expected alignment CENTER, got %s", req.Style.Alignment)
				}
				if req.Style.LineSpacing != 150.0 {
					t.Errorf("expected line spacing 150.0, got %v", req.Style.LineSpacing)
				}
				if !strings.Contains(req.Fields, "alignment") || !strings.Contains(req.Fields, "lineSpacing") {
					t.Errorf("expected fields to contain alignment and lineSpacing, got %s", req.Fields)
				}
			},
		},

		// === Paragraph index tests ===
		{
			name: "apply to specific paragraph index",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-multi",
				ParagraphIndex: intPtr(1),
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation: createMultiParagraphPresentation(),
			checkOutput: func(t *testing.T, output *FormatParagraphOutput) {
				if output.ParagraphScope != "INDEX (1)" {
					t.Errorf("expected paragraph_scope 'INDEX (1)', got %s", output.ParagraphScope)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req.TextRange.Type != "FIXED_RANGE" {
					t.Errorf("expected text range type FIXED_RANGE, got %s", req.TextRange.Type)
				}
			},
		},
		{
			name: "paragraph index out of range",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				ParagraphIndex: intPtr(10),
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrInvalidParagraphIndex,
		},
		{
			name: "negative paragraph index",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				ParagraphIndex: intPtr(-1),
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrInvalidParagraphIndex,
		},

		// === Error cases ===
		{
			name: "missing presentation_id",
			input: FormatParagraphInput{
				ObjectID: "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing object_id",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "missing formatting",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
			},
			wantErr: ErrNoFormattingProvided,
		},
		{
			name: "empty formatting options",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting:     &ParagraphFormattingOptions{},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNoFormattingProvided,
		},
		{
			name: "presentation not found",
			input: FormatParagraphInput{
				PresentationID: "nonexistent",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			getErr:  errors.New("404 not found"),
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: FormatParagraphInput{
				PresentationID: "forbidden",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			getErr:  errors.New("403 forbidden"),
			wantErr: ErrAccessDenied,
		},
		{
			name: "object not found",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "nonexistent-object",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrObjectNotFound,
		},
		{
			name: "image object - not text",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "image-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "table object - must be formatted cell by cell",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "table-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "batch update fails",
			input: FormatParagraphInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "CENTER",
				},
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("internal error"),
			wantErr:        ErrFormatParagraphFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequests []*slides.Request

			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					if tt.presentation != nil {
						return tt.presentation, nil
					}
					return nil, errors.New("not found")
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedRequests = requests
					if tt.batchUpdateErr != nil {
						return nil, tt.batchUpdateErr
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.FormatParagraph(ctx, nil, tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("expected error containing %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, output)
			}

			if tt.checkRequests != nil && len(capturedRequests) > 0 {
				tt.checkRequests(t, capturedRequests)
			}
		})
	}
}

func TestCountParagraphs(t *testing.T) {
	tests := []struct {
		name string
		text *slides.TextContent
		want int
	}{
		{
			name: "nil text",
			text: nil,
			want: 0,
		},
		{
			name: "empty text elements",
			text: &slides.TextContent{TextElements: []*slides.TextElement{}},
			want: 0,
		},
		{
			name: "single paragraph",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{TextRun: &slides.TextRun{Content: "Hello\n"}},
					{ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			want: 1,
		},
		{
			name: "multiple paragraphs",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{TextRun: &slides.TextRun{Content: "First\n"}},
					{ParagraphMarker: &slides.ParagraphMarker{}},
					{TextRun: &slides.TextRun{Content: "Second\n"}},
					{ParagraphMarker: &slides.ParagraphMarker{}},
					{TextRun: &slides.TextRun{Content: "Third\n"}},
					{ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			want: 3,
		},
		{
			name: "text run without paragraph marker",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{TextRun: &slides.TextRun{Content: "No marker"}},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countParagraphs(tt.text)
			if got != tt.want {
				t.Errorf("countParagraphs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetParagraphRange(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name           string
		text           *slides.TextContent
		paragraphIndex *int
		wantType       string
		wantStart      int64
		wantEnd        int64
	}{
		{
			name:           "nil paragraph index - all paragraphs",
			text:           &slides.TextContent{},
			paragraphIndex: nil,
			wantType:       "ALL",
		},
		{
			name: "first paragraph (index 0)",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "First\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 6, EndIndex: 13, TextRun: &slides.TextRun{Content: "Second\n"}},
					{StartIndex: 6, EndIndex: 13, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			paragraphIndex: intPtr(0),
			wantType:       "FIXED_RANGE",
			wantStart:      0,
			wantEnd:        6,
		},
		{
			name: "second paragraph (index 1)",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "First\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 6, EndIndex: 13, TextRun: &slides.TextRun{Content: "Second\n"}},
					{StartIndex: 6, EndIndex: 13, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			paragraphIndex: intPtr(1),
			wantType:       "FIXED_RANGE",
			wantStart:      6,
			wantEnd:        13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getParagraphRange(tt.text, tt.paragraphIndex)
			if got.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantType)
			}
			if tt.wantType == "FIXED_RANGE" {
				if got.StartIndex == nil || *got.StartIndex != tt.wantStart {
					var start int64
					if got.StartIndex != nil {
						start = *got.StartIndex
					}
					t.Errorf("StartIndex = %v, want %v", start, tt.wantStart)
				}
				if got.EndIndex == nil || *got.EndIndex != tt.wantEnd {
					var end int64
					if got.EndIndex != nil {
						end = *got.EndIndex
					}
					t.Errorf("EndIndex = %v, want %v", end, tt.wantEnd)
				}
			}
		})
	}
}

func TestBuildFormatParagraphRequest(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		name        string
		input       FormatParagraphInput
		text        *slides.TextContent
		wantRequest bool
		wantFields  []string
	}{
		{
			name: "all formatting options",
			input: FormatParagraphInput{
				ObjectID: "obj-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment:       "CENTER",
					LineSpacing:     floatPtr(150.0),
					SpaceAbove:      floatPtr(10.0),
					SpaceBelow:      floatPtr(5.0),
					IndentFirstLine: floatPtr(36.0),
					IndentStart:     floatPtr(24.0),
					IndentEnd:       floatPtr(12.0),
				},
			},
			text:        &slides.TextContent{},
			wantRequest: true,
			wantFields:  []string{"alignment", "lineSpacing", "spaceAbove", "spaceBelow", "indentFirstLine", "indentStart", "indentEnd"},
		},
		{
			name: "only alignment",
			input: FormatParagraphInput{
				ObjectID: "obj-1",
				Formatting: &ParagraphFormattingOptions{
					Alignment: "START",
				},
			},
			text:        &slides.TextContent{},
			wantRequest: true,
			wantFields:  []string{"alignment"},
		},
		{
			name: "empty formatting - no request",
			input: FormatParagraphInput{
				ObjectID:   "obj-1",
				Formatting: &ParagraphFormattingOptions{},
			},
			text:        &slides.TextContent{},
			wantRequest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, _ := buildFormatParagraphRequest(tt.input, tt.text)

			if tt.wantRequest && request == nil {
				t.Errorf("expected request, got nil")
				return
			}
			if !tt.wantRequest && request != nil {
				t.Errorf("expected no request, got %v", request)
				return
			}

			if !tt.wantRequest {
				return
			}

			req := request.UpdateParagraphStyle
			if req == nil {
				t.Errorf("expected UpdateParagraphStyle, got nil")
				return
			}

			for _, field := range tt.wantFields {
				if !strings.Contains(req.Fields, field) {
					t.Errorf("expected field %s in %s", field, req.Fields)
				}
			}
		})
	}
}
