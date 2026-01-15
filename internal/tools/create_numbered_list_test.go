package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestCreateNumberedList(t *testing.T) {
	ctx := context.Background()

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
											EndIndex:   6,
											TextRun: &slides.TextRun{
												Content: "Item 1\n",
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
												Content: "Item 2\n",
											},
										},
										{
											StartIndex: 6,
											EndIndex:   13,
											ParagraphMarker: &slides.ParagraphMarker{},
										},
										{
											StartIndex: 13,
											EndIndex:   20,
											TextRun: &slides.TextRun{
												Content: "Item 3\n",
											},
										},
										{
											StartIndex: 13,
											EndIndex:   20,
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

	tests := []struct {
		name           string
		input          CreateNumberedListInput
		presentation   *slides.Presentation
		getErr         error
		batchUpdateErr error
		wantErr        error
		checkOutput    func(*testing.T, *CreateNumberedListOutput)
		checkRequests  func(*testing.T, []*slides.Request)
	}{
		// === Basic number style tests ===
		{
			name: "create numbered list with DECIMAL style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.ObjectID != "textbox-1" {
					t.Errorf("expected object_id 'textbox-1', got %s", output.ObjectID)
				}
				if output.NumberPreset != "NUMBERED_DECIMAL_ALPHA_ROMAN" {
					t.Errorf("expected number_preset 'NUMBERED_DECIMAL_ALPHA_ROMAN', got %s", output.NumberPreset)
				}
				if output.ParagraphScope != "ALL" {
					t.Errorf("expected paragraph_scope 'ALL', got %s", output.ParagraphScope)
				}
				if output.StartNumber != 1 {
					t.Errorf("expected start_number 1, got %d", output.StartNumber)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				req := requests[0].CreateParagraphBullets
				if req == nil {
					t.Fatalf("expected CreateParagraphBullets request, got nil")
				}
				if req.ObjectId != "textbox-1" {
					t.Errorf("expected object_id 'textbox-1', got %s", req.ObjectId)
				}
				if req.BulletPreset != "NUMBERED_DECIMAL_ALPHA_ROMAN" {
					t.Errorf("expected bullet_preset 'NUMBERED_DECIMAL_ALPHA_ROMAN', got %s", req.BulletPreset)
				}
				if req.TextRange.Type != "ALL" {
					t.Errorf("expected text range type 'ALL', got %s", req.TextRange.Type)
				}
			},
		},
		{
			name: "create numbered list with ALPHA_UPPER style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "ALPHA_UPPER",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.NumberPreset != "NUMBERED_UPPERALPHA_ALPHA_ROMAN" {
					t.Errorf("expected number_preset 'NUMBERED_UPPERALPHA_ALPHA_ROMAN', got %s", output.NumberPreset)
				}
			},
		},
		{
			name: "create numbered list with ALPHA_LOWER style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "ALPHA_LOWER",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.NumberPreset != "NUMBERED_ALPHA_ALPHA_ROMAN" {
					t.Errorf("expected number_preset 'NUMBERED_ALPHA_ALPHA_ROMAN', got %s", output.NumberPreset)
				}
			},
		},
		{
			name: "create numbered list with ROMAN_UPPER style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "ROMAN_UPPER",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.NumberPreset != "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL" {
					t.Errorf("expected number_preset 'NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL', got %s", output.NumberPreset)
				}
			},
		},
		{
			name: "create numbered list with ROMAN_LOWER style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "ROMAN_LOWER",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.NumberPreset != "NUMBERED_ROMAN_UPPERALPHA_DECIMAL" {
					t.Errorf("expected number_preset 'NUMBERED_ROMAN_UPPERALPHA_DECIMAL', got %s", output.NumberPreset)
				}
			},
		},
		{
			name: "lowercase number style is normalized",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "decimal", // lowercase
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.NumberPreset != "NUMBERED_DECIMAL_ALPHA_ROMAN" {
					t.Errorf("expected number_preset 'NUMBERED_DECIMAL_ALPHA_ROMAN', got %s", output.NumberPreset)
				}
			},
		},
		{
			name: "full preset name works",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "NUMBERED_DECIMAL_NESTED",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.NumberPreset != "NUMBERED_DECIMAL_NESTED" {
					t.Errorf("expected number_preset 'NUMBERED_DECIMAL_NESTED', got %s", output.NumberPreset)
				}
			},
		},

		// === Start number tests ===
		{
			name: "create numbered list with default start_number",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
				// StartNumber not specified, should default to 1
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.StartNumber != 1 {
					t.Errorf("expected start_number 1, got %d", output.StartNumber)
				}
			},
		},
		{
			name: "create numbered list with custom start_number",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
				StartNumber:    5,
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				// Note: Custom start number is stored in output but API limitation means
				// the actual list may still start at 1
				if output.StartNumber != 5 {
					t.Errorf("expected start_number 5, got %d", output.StartNumber)
				}
			},
		},

		// === Paragraph indices tests ===
		{
			name: "apply numbering to specific paragraphs",
			input: CreateNumberedListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				NumberStyle:      "DECIMAL",
				ParagraphIndices: []int{0, 1},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.ParagraphScope != "INDICES [0 1]" {
					t.Errorf("expected paragraph_scope 'INDICES [0 1]', got %s", output.ParagraphScope)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].CreateParagraphBullets
				if req.TextRange.Type != "FIXED_RANGE" {
					t.Errorf("expected text range type 'FIXED_RANGE', got %s", req.TextRange.Type)
				}
			},
		},
		{
			name: "apply numbering to single paragraph",
			input: CreateNumberedListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				NumberStyle:      "DECIMAL",
				ParagraphIndices: []int{1},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.ParagraphScope != "INDICES [1]" {
					t.Errorf("expected paragraph_scope 'INDICES [1]', got %s", output.ParagraphScope)
				}
			},
		},

		// === Error cases ===
		{
			name: "missing presentation_id",
			input: CreateNumberedListInput{
				ObjectID:    "textbox-1",
				NumberStyle: "DECIMAL",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing object_id",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				NumberStyle:    "DECIMAL",
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "missing number_style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
			},
			wantErr: ErrInvalidNumberStyle,
		},
		{
			name: "invalid number_style",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "INVALID_STYLE",
			},
			wantErr: ErrInvalidNumberStyle,
		},
		{
			name: "negative start_number",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
				StartNumber:    -1,
			},
			wantErr: ErrInvalidStartNumber,
		},
		{
			name: "zero start_number defaults to 1",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
				StartNumber:    0, // Should default to 1
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateNumberedListOutput) {
				if output.StartNumber != 1 {
					t.Errorf("expected start_number 1 (default), got %d", output.StartNumber)
				}
			},
		},
		{
			name: "negative paragraph index",
			input: CreateNumberedListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				NumberStyle:      "DECIMAL",
				ParagraphIndices: []int{-1},
			},
			wantErr: ErrInvalidParagraphIndex,
		},
		{
			name: "paragraph index out of range",
			input: CreateNumberedListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				NumberStyle:      "DECIMAL",
				ParagraphIndices: []int{10},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrInvalidParagraphIndex,
		},
		{
			name: "presentation not found",
			input: CreateNumberedListInput{
				PresentationID: "nonexistent",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
			},
			getErr:  errors.New("404 not found"),
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: CreateNumberedListInput{
				PresentationID: "forbidden",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
			},
			getErr:  errors.New("403 forbidden"),
			wantErr: ErrAccessDenied,
		},
		{
			name: "object not found",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "nonexistent-object",
				NumberStyle:    "DECIMAL",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrObjectNotFound,
		},
		{
			name: "image object - not text",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "image-1",
				NumberStyle:    "DECIMAL",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "table object - must be applied cell by cell",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "table-1",
				NumberStyle:    "DECIMAL",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "batch update fails",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("internal error"),
			wantErr:        ErrCreateNumberedListFailed,
		},
		{
			name: "batch update returns 404",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("404 not found"),
			wantErr:        ErrPresentationNotFound,
		},
		{
			name: "batch update returns 403",
			input: CreateNumberedListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				NumberStyle:    "DECIMAL",
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("403 forbidden"),
			wantErr:        ErrAccessDenied,
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

			output, err := tools.CreateNumberedList(ctx, nil, tt.input)

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

func TestBuildCreateNumberedListRequests(t *testing.T) {
	text := &slides.TextContent{
		TextElements: []*slides.TextElement{
			{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
		},
	}

	tests := []struct {
		name         string
		input        CreateNumberedListInput
		numberPreset string
		startNumber  int
		wantCount    int
		checkFirst   func(*testing.T, *slides.Request)
	}{
		{
			name: "basic numbered list request",
			input: CreateNumberedListInput{
				ObjectID:    "obj-1",
				NumberStyle: "DECIMAL",
			},
			numberPreset: "NUMBERED_DECIMAL_ALPHA_ROMAN",
			startNumber:  1,
			wantCount:    1,
			checkFirst: func(t *testing.T, req *slides.Request) {
				if req.CreateParagraphBullets == nil {
					t.Errorf("expected CreateParagraphBullets, got nil")
					return
				}
				if req.CreateParagraphBullets.BulletPreset != "NUMBERED_DECIMAL_ALPHA_ROMAN" {
					t.Errorf("expected preset NUMBERED_DECIMAL_ALPHA_ROMAN, got %s", req.CreateParagraphBullets.BulletPreset)
				}
			},
		},
		{
			name: "numbered list with roman numeral preset",
			input: CreateNumberedListInput{
				ObjectID:    "obj-1",
				NumberStyle: "ROMAN_UPPER",
			},
			numberPreset: "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL",
			startNumber:  1,
			wantCount:    1,
			checkFirst: func(t *testing.T, req *slides.Request) {
				if req.CreateParagraphBullets == nil {
					t.Errorf("expected CreateParagraphBullets, got nil")
					return
				}
				if req.CreateParagraphBullets.BulletPreset != "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL" {
					t.Errorf("expected preset NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL, got %s", req.CreateParagraphBullets.BulletPreset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildCreateNumberedListRequests(tt.input, tt.numberPreset, text, tt.startNumber)
			if len(requests) != tt.wantCount {
				t.Errorf("expected %d requests, got %d", tt.wantCount, len(requests))
			}
			if tt.checkFirst != nil && len(requests) > 0 {
				tt.checkFirst(t, requests[0])
			}
		})
	}
}

func TestValidNumberStyles(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		// User-friendly names
		{"DECIMAL", "NUMBERED_DECIMAL_ALPHA_ROMAN", true},
		{"ALPHA_UPPER", "NUMBERED_UPPERALPHA_ALPHA_ROMAN", true},
		{"ALPHA_LOWER", "NUMBERED_ALPHA_ALPHA_ROMAN", true},
		{"ROMAN_UPPER", "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL", true},
		{"ROMAN_LOWER", "NUMBERED_ROMAN_UPPERALPHA_DECIMAL", true},
		// Case insensitive
		{"decimal", "NUMBERED_DECIMAL_ALPHA_ROMAN", true},
		{"Decimal", "NUMBERED_DECIMAL_ALPHA_ROMAN", true},
		{"alpha_lower", "NUMBERED_ALPHA_ALPHA_ROMAN", true},
		{"roman_upper", "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL", true},
		// Full preset names
		{"NUMBERED_DECIMAL_ALPHA_ROMAN", "NUMBERED_DECIMAL_ALPHA_ROMAN", true},
		{"NUMBERED_DECIMAL_NESTED", "NUMBERED_DECIMAL_NESTED", true},
		{"NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS", "NUMBERED_DECIMAL_ALPHA_ROMAN_PARENS", true},
		{"NUMBERED_ZERODIGIT_ALPHA_ROMAN", "NUMBERED_ZERODIGIT_ALPHA_ROMAN", true},
		// Invalid
		{"INVALID", "", false},
		{"DISC", "", false}, // Disc is for bullets, not numbers
		{"BULLET", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := validNumberStyles[strings.ToUpper(tt.input)]
			if ok != tt.valid {
				t.Errorf("validNumberStyles[%s] valid = %v, want %v", tt.input, ok, tt.valid)
			}
			if ok && result != tt.expected {
				t.Errorf("validNumberStyles[%s] = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
