package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestStyleText(t *testing.T) {
	ctx := context.Background()

	// Helper to create bool pointer
	boolPtr := func(b bool) *bool { return &b }
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
											TextRun: &slides.TextRun{
												Content: "Hello World",
											},
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
		input          StyleTextInput
		presentation   *slides.Presentation
		getErr         error
		batchUpdateErr error
		wantErr        error
		wantErrMsg     string
		checkOutput    func(*testing.T, *StyleTextOutput)
		checkRequests  func(*testing.T, []*slides.Request)
	}{
		{
			name: "apply font family",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					FontFamily: "Arial",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if output.ObjectID != "textbox-1" {
					t.Errorf("expected object_id 'textbox-1', got %s", output.ObjectID)
				}
				if output.TextRange != "ALL" {
					t.Errorf("expected text_range 'ALL', got %s", output.TextRange)
				}
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "font_family=Arial" {
					t.Errorf("expected applied_styles [font_family=Arial], got %v", output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				req := requests[0].UpdateTextStyle
				if req.Style.FontFamily != "Arial" {
					t.Errorf("expected font family Arial, got %s", req.Style.FontFamily)
				}
				if req.TextRange.Type != "ALL" {
					t.Errorf("expected text range type ALL, got %s", req.TextRange.Type)
				}
				if req.Fields != "fontFamily" {
					t.Errorf("expected fields 'fontFamily', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply font size",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					FontSize: 24,
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "font_size=24pt" {
					t.Errorf("expected applied_styles [font_size=24pt], got %v", output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if req.Style.FontSize == nil || req.Style.FontSize.Magnitude != 24 || req.Style.FontSize.Unit != "PT" {
					t.Errorf("expected font size 24pt, got %v", req.Style.FontSize)
				}
				if req.Fields != "fontSize" {
					t.Errorf("expected fields 'fontSize', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply bold true",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					Bold: boolPtr(true),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "bold=true" {
					t.Errorf("expected applied_styles [bold=true], got %v", output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if !req.Style.Bold {
					t.Errorf("expected bold true")
				}
			},
		},
		{
			name: "apply bold false",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					Bold: boolPtr(false),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "bold=false" {
					t.Errorf("expected applied_styles [bold=false], got %v", output.AppliedStyles)
				}
			},
		},
		{
			name: "apply italic",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					Italic: boolPtr(true),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "italic=true" {
					t.Errorf("expected applied_styles [italic=true], got %v", output.AppliedStyles)
				}
			},
		},
		{
			name: "apply underline",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					Underline: boolPtr(true),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "underline=true" {
					t.Errorf("expected applied_styles [underline=true], got %v", output.AppliedStyles)
				}
			},
		},
		{
			name: "apply strikethrough",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					Strikethrough: boolPtr(true),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "strikethrough=true" {
					t.Errorf("expected applied_styles [strikethrough=true], got %v", output.AppliedStyles)
				}
			},
		},
		{
			name: "apply foreground color",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					ForegroundColor: "#FF0000",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "foreground_color=#FF0000" {
					t.Errorf("expected applied_styles [foreground_color=#FF0000], got %v", output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if req.Style.ForegroundColor == nil || req.Style.ForegroundColor.OpaqueColor == nil {
					t.Fatal("expected foreground color to be set")
				}
				rgb := req.Style.ForegroundColor.OpaqueColor.RgbColor
				if rgb.Red != 1.0 || rgb.Green != 0 || rgb.Blue != 0 {
					t.Errorf("expected red (1,0,0), got (%f,%f,%f)", rgb.Red, rgb.Green, rgb.Blue)
				}
			},
		},
		{
			name: "apply background color",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					BackgroundColor: "#FFFF00",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "background_color=#FFFF00" {
					t.Errorf("expected applied_styles [background_color=#FFFF00], got %v", output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if req.Style.BackgroundColor == nil || req.Style.BackgroundColor.OpaqueColor == nil {
					t.Fatal("expected background color to be set")
				}
				rgb := req.Style.BackgroundColor.OpaqueColor.RgbColor
				if rgb.Red != 1.0 || rgb.Green != 1.0 || rgb.Blue != 0 {
					t.Errorf("expected yellow (1,1,0), got (%f,%f,%f)", rgb.Red, rgb.Green, rgb.Blue)
				}
			},
		},
		{
			name: "apply link URL",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					LinkURL: "https://example.com",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "link_url=https://example.com" {
					t.Errorf("expected applied_styles [link_url=https://example.com], got %v", output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if req.Style.Link == nil || req.Style.Link.Url != "https://example.com" {
					t.Errorf("expected link URL https://example.com, got %v", req.Style.Link)
				}
				if !strings.Contains(req.Fields, "link") {
					t.Errorf("expected fields to contain 'link', got %s", req.Fields)
				}
			},
		},
		{
			name: "apply multiple styles",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					FontFamily:      "Arial",
					FontSize:        18,
					Bold:            boolPtr(true),
					Italic:          boolPtr(false),
					ForegroundColor: "#0000FF",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if len(output.AppliedStyles) != 5 {
					t.Errorf("expected 5 applied styles, got %d: %v", len(output.AppliedStyles), output.AppliedStyles)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if req.Style.FontFamily != "Arial" {
					t.Error("expected font family Arial")
				}
				if req.Style.FontSize.Magnitude != 18 {
					t.Error("expected font size 18")
				}
				if !req.Style.Bold {
					t.Error("expected bold true")
				}
				if req.Style.Italic {
					t.Error("expected italic false")
				}
			},
		},
		{
			name: "apply partial style with indices",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				StartIndex:     intPtr(0),
				EndIndex:       intPtr(5),
				Style: &StyleTextStyleSpec{
					Bold: boolPtr(true),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if output.TextRange != "FIXED_RANGE (0-5)" {
					t.Errorf("expected text_range 'FIXED_RANGE (0-5)', got %s", output.TextRange)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateTextStyle
				if req.TextRange.Type != "FIXED_RANGE" {
					t.Errorf("expected text range type FIXED_RANGE, got %s", req.TextRange.Type)
				}
				if req.TextRange.StartIndex == nil || *req.TextRange.StartIndex != 0 {
					t.Errorf("expected start index 0")
				}
				if req.TextRange.EndIndex == nil || *req.TextRange.EndIndex != 5 {
					t.Errorf("expected end index 5")
				}
			},
		},
		{
			name: "apply style with only start index",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				StartIndex:     intPtr(3),
				Style: &StyleTextStyleSpec{
					Underline: boolPtr(true),
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				if output.TextRange != "FROM_START_INDEX (3)" {
					t.Errorf("expected text_range 'FROM_START_INDEX (3)', got %s", output.TextRange)
				}
			},
		},
		// Error cases
		{
			name: "missing presentation_id",
			input: StyleTextInput{
				ObjectID: "textbox-1",
				Style:    &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing object_id",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "missing style",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
			},
			wantErr: ErrNoStyleProvided,
		},
		{
			name: "empty style object",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style:          &StyleTextStyleSpec{},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNoStyleProvided,
		},
		{
			name: "negative start_index",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				StartIndex:     intPtr(-1),
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			wantErr: ErrInvalidTextRange,
		},
		{
			name: "negative end_index",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				EndIndex:       intPtr(-1),
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			wantErr: ErrInvalidTextRange,
		},
		{
			name: "start_index greater than end_index",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				StartIndex:     intPtr(10),
				EndIndex:       intPtr(5),
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			wantErr: ErrInvalidTextRange,
		},
		{
			name: "object not found",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "nonexistent",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrObjectNotFound,
		},
		{
			name: "presentation not found",
			input: StyleTextInput{
				PresentationID: "nonexistent",
				ObjectID:       "textbox-1",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			getErr:  errors.New("404 not found"),
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: StyleTextInput{
				PresentationID: "forbidden",
				ObjectID:       "textbox-1",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			getErr:  errors.New("403 forbidden"),
			wantErr: ErrAccessDenied,
		},
		{
			name: "image object - not text",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "image-1",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "table object - must be styled cell by cell",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "table-1",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "batch update fails",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style:          &StyleTextStyleSpec{Bold: boolPtr(true)},
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("internal error"),
			wantErr:        ErrStyleTextFailed,
		},
		{
			name: "invalid color format is ignored",
			input: StyleTextInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "textbox-1",
				Style: &StyleTextStyleSpec{
					ForegroundColor: "invalid",
					Bold:            boolPtr(true), // Need at least one valid style
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, output *StyleTextOutput) {
				// Should only have bold style, not foreground color
				if len(output.AppliedStyles) != 1 || output.AppliedStyles[0] != "bold=true" {
					t.Errorf("expected only [bold=true], got %v", output.AppliedStyles)
				}
			},
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

			output, err := tools.StyleText(ctx, nil, tt.input)

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

func TestBuildStyleTextRequest(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name           string
		input          StyleTextInput
		wantRequest    bool
		wantFields     []string
		wantRangeType  string
		wantStartIndex *int64
		wantEndIndex   *int64
	}{
		{
			name: "all fields",
			input: StyleTextInput{
				ObjectID: "obj-1",
				Style: &StyleTextStyleSpec{
					FontFamily:      "Times New Roman",
					FontSize:        12,
					Bold:            boolPtr(true),
					Italic:          boolPtr(true),
					Underline:       boolPtr(true),
					Strikethrough:   boolPtr(true),
					ForegroundColor: "#000000",
					BackgroundColor: "#FFFFFF",
					LinkURL:         "https://test.com",
				},
			},
			wantRequest: true,
			wantFields: []string{
				"fontFamily", "fontSize", "bold", "italic",
				"underline", "strikethrough", "foregroundColor", "backgroundColor", "link",
			},
			wantRangeType: "ALL",
		},
		{
			name: "fixed range",
			input: StyleTextInput{
				ObjectID:   "obj-1",
				StartIndex: intPtr(5),
				EndIndex:   intPtr(10),
				Style: &StyleTextStyleSpec{
					Bold: boolPtr(true),
				},
			},
			wantRequest:    true,
			wantRangeType:  "FIXED_RANGE",
			wantStartIndex: int64Ptr(5),
			wantEndIndex:   int64Ptr(10),
		},
		{
			name: "only start index",
			input: StyleTextInput{
				ObjectID:   "obj-1",
				StartIndex: intPtr(3),
				Style: &StyleTextStyleSpec{
					Italic: boolPtr(true),
				},
			},
			wantRequest:    true,
			wantRangeType:  "FIXED_RANGE",
			wantStartIndex: int64Ptr(3),
			wantEndIndex:   nil,
		},
		{
			name: "empty style",
			input: StyleTextInput{
				ObjectID: "obj-1",
				Style:    &StyleTextStyleSpec{},
			},
			wantRequest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, appliedStyles := buildStyleTextRequest(tt.input)

			if tt.wantRequest {
				if request == nil {
					t.Fatal("expected request, got nil")
				}
				if request.UpdateTextStyle == nil {
					t.Fatal("expected UpdateTextStyle, got nil")
				}
				uts := request.UpdateTextStyle

				if uts.TextRange.Type != tt.wantRangeType {
					t.Errorf("expected range type %s, got %s", tt.wantRangeType, uts.TextRange.Type)
				}

				if tt.wantStartIndex != nil {
					if uts.TextRange.StartIndex == nil {
						t.Error("expected start index, got nil")
					} else if *uts.TextRange.StartIndex != *tt.wantStartIndex {
						t.Errorf("expected start index %d, got %d", *tt.wantStartIndex, *uts.TextRange.StartIndex)
					}
				}

				if tt.wantEndIndex != nil {
					if uts.TextRange.EndIndex == nil {
						t.Error("expected end index, got nil")
					} else if *uts.TextRange.EndIndex != *tt.wantEndIndex {
						t.Errorf("expected end index %d, got %d", *tt.wantEndIndex, *uts.TextRange.EndIndex)
					}
				}

				// Check that all expected fields are present
				for _, field := range tt.wantFields {
					if !strings.Contains(uts.Fields, field) {
						t.Errorf("expected field %s in fields %s", field, uts.Fields)
					}
				}

				// Check that applied styles count matches fields
				if len(tt.wantFields) > 0 && len(appliedStyles) != len(tt.wantFields) {
					t.Errorf("expected %d applied styles, got %d", len(tt.wantFields), len(appliedStyles))
				}
			} else {
				if request != nil {
					t.Errorf("expected nil request, got %v", request)
				}
				if len(appliedStyles) > 0 {
					t.Errorf("expected no applied styles, got %v", appliedStyles)
				}
			}
		})
	}
}

// Helper function for tests
func int64Ptr(i int64) *int64 {
	return &i
}
