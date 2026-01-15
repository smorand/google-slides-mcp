package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestCreateBulletList(t *testing.T) {
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
		input          CreateBulletListInput
		presentation   *slides.Presentation
		getErr         error
		batchUpdateErr error
		wantErr        error
		checkOutput    func(*testing.T, *CreateBulletListOutput)
		checkRequests  func(*testing.T, []*slides.Request)
	}{
		// === Basic bullet style tests ===
		{
			name: "create bullet list with DISC style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.ObjectID != "textbox-1" {
					t.Errorf("expected object_id 'textbox-1', got %s", output.ObjectID)
				}
				if output.BulletPreset != "BULLET_DISC_CIRCLE_SQUARE" {
					t.Errorf("expected bullet_preset 'BULLET_DISC_CIRCLE_SQUARE', got %s", output.BulletPreset)
				}
				if output.ParagraphScope != "ALL" {
					t.Errorf("expected paragraph_scope 'ALL', got %s", output.ParagraphScope)
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
				if req.BulletPreset != "BULLET_DISC_CIRCLE_SQUARE" {
					t.Errorf("expected bullet_preset 'BULLET_DISC_CIRCLE_SQUARE', got %s", req.BulletPreset)
				}
				if req.TextRange.Type != "ALL" {
					t.Errorf("expected text range type 'ALL', got %s", req.TextRange.Type)
				}
			},
		},
		{
			name: "create bullet list with ARROW style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "ARROW",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletPreset != "BULLET_ARROW_DIAMOND_DISC" {
					t.Errorf("expected bullet_preset 'BULLET_ARROW_DIAMOND_DISC', got %s", output.BulletPreset)
				}
			},
		},
		{
			name: "create bullet list with STAR style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "STAR",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletPreset != "BULLET_STAR_CIRCLE_SQUARE" {
					t.Errorf("expected bullet_preset 'BULLET_STAR_CIRCLE_SQUARE', got %s", output.BulletPreset)
				}
			},
		},
		{
			name: "create bullet list with CHECKBOX style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "CHECKBOX",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletPreset != "BULLET_CHECKBOX" {
					t.Errorf("expected bullet_preset 'BULLET_CHECKBOX', got %s", output.BulletPreset)
				}
			},
		},
		{
			name: "create bullet list with DIAMOND style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DIAMOND",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletPreset != "BULLET_DIAMOND_CIRCLE_SQUARE" {
					t.Errorf("expected bullet_preset 'BULLET_DIAMOND_CIRCLE_SQUARE', got %s", output.BulletPreset)
				}
			},
		},
		{
			name: "lowercase bullet style is normalized",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "disc", // lowercase
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletPreset != "BULLET_DISC_CIRCLE_SQUARE" {
					t.Errorf("expected bullet_preset 'BULLET_DISC_CIRCLE_SQUARE', got %s", output.BulletPreset)
				}
			},
		},
		{
			name: "full preset name works",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "BULLET_ARROW_DIAMOND_DISC",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletPreset != "BULLET_ARROW_DIAMOND_DISC" {
					t.Errorf("expected bullet_preset 'BULLET_ARROW_DIAMOND_DISC', got %s", output.BulletPreset)
				}
			},
		},

		// === Bullet color tests ===
		{
			name: "create bullet list with color",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
				BulletColor:    "#FF0000",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.BulletColor != "#FF0000" {
					t.Errorf("expected bullet_color '#FF0000', got %s", output.BulletColor)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 2 {
					t.Fatalf("expected 2 requests (bullets + color), got %d", len(requests))
				}
				// First request should be CreateParagraphBullets
				if requests[0].CreateParagraphBullets == nil {
					t.Errorf("first request should be CreateParagraphBullets")
				}
				// Second request should be UpdateTextStyle for color
				colorReq := requests[1].UpdateTextStyle
				if colorReq == nil {
					t.Fatalf("second request should be UpdateTextStyle, got nil")
				}
				if colorReq.Style.ForegroundColor == nil {
					t.Fatalf("expected foreground color, got nil")
				}
				// Verify the color is red
				rgb := colorReq.Style.ForegroundColor.OpaqueColor.RgbColor
				if rgb.Red != 1.0 || rgb.Green != 0.0 || rgb.Blue != 0.0 {
					t.Errorf("expected red color (1,0,0), got (%v,%v,%v)", rgb.Red, rgb.Green, rgb.Blue)
				}
			},
		},
		{
			name: "create bullet list with invalid color - no color request",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
				BulletColor:    "invalid-color",
			},
			presentation: createTestPresentation(),
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				// Invalid color should be silently ignored
				if len(requests) != 1 {
					t.Fatalf("expected 1 request (invalid color ignored), got %d", len(requests))
				}
			},
		},

		// === Paragraph indices tests ===
		{
			name: "apply bullets to specific paragraphs",
			input: CreateBulletListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				BulletStyle:      "DISC",
				ParagraphIndices: []int{0, 1},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
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
			name: "apply bullets to single paragraph",
			input: CreateBulletListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				BulletStyle:      "DISC",
				ParagraphIndices: []int{1},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *CreateBulletListOutput) {
				if output.ParagraphScope != "INDICES [1]" {
					t.Errorf("expected paragraph_scope 'INDICES [1]', got %s", output.ParagraphScope)
				}
			},
		},

		// === Error cases ===
		{
			name: "missing presentation_id",
			input: CreateBulletListInput{
				ObjectID:    "textbox-1",
				BulletStyle: "DISC",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing object_id",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				BulletStyle:    "DISC",
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "missing bullet_style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
			},
			wantErr: ErrInvalidBulletStyle,
		},
		{
			name: "invalid bullet_style",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "INVALID_STYLE",
			},
			wantErr: ErrInvalidBulletStyle,
		},
		{
			name: "negative paragraph index",
			input: CreateBulletListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				BulletStyle:      "DISC",
				ParagraphIndices: []int{-1},
			},
			wantErr: ErrInvalidParagraphIndex,
		},
		{
			name: "paragraph index out of range",
			input: CreateBulletListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "textbox-1",
				BulletStyle:      "DISC",
				ParagraphIndices: []int{10},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrInvalidParagraphIndex,
		},
		{
			name: "presentation not found",
			input: CreateBulletListInput{
				PresentationID: "nonexistent",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
			},
			getErr:  errors.New("404 not found"),
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: CreateBulletListInput{
				PresentationID: "forbidden",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
			},
			getErr:  errors.New("403 forbidden"),
			wantErr: ErrAccessDenied,
		},
		{
			name: "object not found",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "nonexistent-object",
				BulletStyle:    "DISC",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrObjectNotFound,
		},
		{
			name: "image object - not text",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "image-1",
				BulletStyle:    "DISC",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "table object - must be applied cell by cell",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "table-1",
				BulletStyle:    "DISC",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "batch update fails",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("internal error"),
			wantErr:        ErrCreateBulletListFailed,
		},
		{
			name: "batch update returns 404",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("404 not found"),
			wantErr:        ErrPresentationNotFound,
		},
		{
			name: "batch update returns 403",
			input: CreateBulletListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				BulletStyle:    "DISC",
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

			output, err := tools.CreateBulletList(ctx, nil, tt.input)

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

func TestGetBulletTextRange(t *testing.T) {
	tests := []struct {
		name             string
		text             *slides.TextContent
		paragraphIndices []int
		wantType         string
		wantStart        int64
		wantEnd          int64
	}{
		{
			name:             "empty paragraph indices - all paragraphs",
			text:             &slides.TextContent{},
			paragraphIndices: nil,
			wantType:         "ALL",
		},
		{
			name:             "empty slice - all paragraphs",
			text:             &slides.TextContent{},
			paragraphIndices: []int{},
			wantType:         "ALL",
		},
		{
			name: "single paragraph index",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "First\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 6, EndIndex: 13, TextRun: &slides.TextRun{Content: "Second\n"}},
					{StartIndex: 6, EndIndex: 13, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			paragraphIndices: []int{0},
			wantType:         "FIXED_RANGE",
			wantStart:        0,
			wantEnd:          6,
		},
		{
			name: "multiple paragraph indices",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "First\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 6, EndIndex: 13, TextRun: &slides.TextRun{Content: "Second\n"}},
					{StartIndex: 6, EndIndex: 13, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 13, EndIndex: 19, TextRun: &slides.TextRun{Content: "Third\n"}},
					{StartIndex: 13, EndIndex: 19, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			paragraphIndices: []int{0, 2}, // First and third
			wantType:         "FIXED_RANGE",
			wantStart:        0,
			wantEnd:          19,
		},
		{
			name: "non-contiguous paragraph indices covers all in range",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "First\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 6, EndIndex: 13, TextRun: &slides.TextRun{Content: "Second\n"}},
					{StartIndex: 6, EndIndex: 13, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 13, EndIndex: 19, TextRun: &slides.TextRun{Content: "Third\n"}},
					{StartIndex: 13, EndIndex: 19, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			paragraphIndices: []int{1}, // Only second
			wantType:         "FIXED_RANGE",
			wantStart:        6,
			wantEnd:          13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBulletTextRange(tt.text, tt.paragraphIndices)
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

func TestGetParagraphRanges(t *testing.T) {
	tests := []struct {
		name       string
		text       *slides.TextContent
		wantCount  int
		wantRanges []paragraphRange
	}{
		{
			name:      "nil text",
			text:      nil,
			wantCount: 0,
		},
		{
			name:      "empty text elements",
			text:      &slides.TextContent{TextElements: []*slides.TextElement{}},
			wantCount: 0,
		},
		{
			name: "single paragraph",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "Hello\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			wantCount: 1,
			wantRanges: []paragraphRange{
				{start: 0, end: 6},
			},
		},
		{
			name: "multiple paragraphs",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{StartIndex: 0, EndIndex: 6, TextRun: &slides.TextRun{Content: "First\n"}},
					{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 6, EndIndex: 13, TextRun: &slides.TextRun{Content: "Second\n"}},
					{StartIndex: 6, EndIndex: 13, ParagraphMarker: &slides.ParagraphMarker{}},
					{StartIndex: 13, EndIndex: 19, TextRun: &slides.TextRun{Content: "Third\n"}},
					{StartIndex: 13, EndIndex: 19, ParagraphMarker: &slides.ParagraphMarker{}},
				},
			},
			wantCount: 3,
			wantRanges: []paragraphRange{
				{start: 0, end: 6},
				{start: 6, end: 13},
				{start: 13, end: 19},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getParagraphRanges(tt.text)
			if len(got) != tt.wantCount {
				t.Errorf("getParagraphRanges() returned %d ranges, want %d", len(got), tt.wantCount)
			}
			for i, want := range tt.wantRanges {
				if i >= len(got) {
					break
				}
				if got[i].start != want.start || got[i].end != want.end {
					t.Errorf("range[%d] = {%d, %d}, want {%d, %d}", i, got[i].start, got[i].end, want.start, want.end)
				}
			}
		})
	}
}

func TestBuildCreateBulletListRequests(t *testing.T) {
	text := &slides.TextContent{
		TextElements: []*slides.TextElement{
			{StartIndex: 0, EndIndex: 6, ParagraphMarker: &slides.ParagraphMarker{}},
		},
	}

	tests := []struct {
		name         string
		input        CreateBulletListInput
		bulletPreset string
		wantCount    int
		checkFirst   func(*testing.T, *slides.Request)
		checkSecond  func(*testing.T, *slides.Request)
	}{
		{
			name: "basic bullet request",
			input: CreateBulletListInput{
				ObjectID:    "obj-1",
				BulletStyle: "DISC",
			},
			bulletPreset: "BULLET_DISC_CIRCLE_SQUARE",
			wantCount:    1,
			checkFirst: func(t *testing.T, req *slides.Request) {
				if req.CreateParagraphBullets == nil {
					t.Errorf("expected CreateParagraphBullets, got nil")
					return
				}
				if req.CreateParagraphBullets.BulletPreset != "BULLET_DISC_CIRCLE_SQUARE" {
					t.Errorf("expected preset BULLET_DISC_CIRCLE_SQUARE, got %s", req.CreateParagraphBullets.BulletPreset)
				}
			},
		},
		{
			name: "bullet request with color",
			input: CreateBulletListInput{
				ObjectID:    "obj-1",
				BulletStyle: "DISC",
				BulletColor: "#00FF00", // Green
			},
			bulletPreset: "BULLET_DISC_CIRCLE_SQUARE",
			wantCount:    2,
			checkFirst: func(t *testing.T, req *slides.Request) {
				if req.CreateParagraphBullets == nil {
					t.Errorf("expected CreateParagraphBullets, got nil")
				}
			},
			checkSecond: func(t *testing.T, req *slides.Request) {
				if req.UpdateTextStyle == nil {
					t.Errorf("expected UpdateTextStyle, got nil")
					return
				}
				rgb := req.UpdateTextStyle.Style.ForegroundColor.OpaqueColor.RgbColor
				if rgb.Green != 1.0 {
					t.Errorf("expected green=1.0, got %v", rgb.Green)
				}
			},
		},
		{
			name: "invalid color - only bullet request",
			input: CreateBulletListInput{
				ObjectID:    "obj-1",
				BulletStyle: "DISC",
				BulletColor: "not-a-color",
			},
			bulletPreset: "BULLET_DISC_CIRCLE_SQUARE",
			wantCount:    1, // Invalid color is ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildCreateBulletListRequests(tt.input, tt.bulletPreset, text)
			if len(requests) != tt.wantCount {
				t.Errorf("expected %d requests, got %d", tt.wantCount, len(requests))
			}
			if tt.checkFirst != nil && len(requests) > 0 {
				tt.checkFirst(t, requests[0])
			}
			if tt.checkSecond != nil && len(requests) > 1 {
				tt.checkSecond(t, requests[1])
			}
		})
	}
}

func TestValidBulletStyles(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		// User-friendly names
		{"DISC", "BULLET_DISC_CIRCLE_SQUARE", true},
		{"CIRCLE", "BULLET_DISC_CIRCLE_SQUARE", true},
		{"SQUARE", "BULLET_DISC_CIRCLE_SQUARE", true},
		{"DIAMOND", "BULLET_DIAMOND_CIRCLE_SQUARE", true},
		{"ARROW", "BULLET_ARROW_DIAMOND_DISC", true},
		{"STAR", "BULLET_STAR_CIRCLE_SQUARE", true},
		{"CHECKBOX", "BULLET_CHECKBOX", true},
		// Case insensitive
		{"disc", "BULLET_DISC_CIRCLE_SQUARE", true},
		{"Disc", "BULLET_DISC_CIRCLE_SQUARE", true},
		// Full preset names
		{"BULLET_DISC_CIRCLE_SQUARE", "BULLET_DISC_CIRCLE_SQUARE", true},
		{"BULLET_CHECKBOX", "BULLET_CHECKBOX", true},
		{"BULLET_ARROW_DIAMOND_DISC", "BULLET_ARROW_DIAMOND_DISC", true},
		// Invalid
		{"INVALID", "", false},
		{"NUMBERED", "", false}, // Numbered lists use different presets
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := validBulletStyles[strings.ToUpper(tt.input)]
			if ok != tt.valid {
				t.Errorf("validBulletStyles[%s] valid = %v, want %v", tt.input, ok, tt.valid)
			}
			if ok && result != tt.expected {
				t.Errorf("validBulletStyles[%s] = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
