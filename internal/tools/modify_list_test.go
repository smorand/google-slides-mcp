package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestModifyList(t *testing.T) {
	ctx := context.Background()

	// Create a basic presentation with list-formatted text for testing
	createTestPresentation := func() *slides.Presentation {
		return &slides.Presentation{
			PresentationId: "test-presentation-id",
			Slides: []*slides.Page{
				{
					ObjectId: "slide-1",
					PageElements: []*slides.PageElement{
						{
							ObjectId: "shape-with-bullets",
							Shape: &slides.Shape{
								ShapeType: "TEXT_BOX",
								Text: &slides.TextContent{
									TextElements: []*slides.TextElement{
										{
											StartIndex: 0,
											EndIndex:   10,
											TextRun: &slides.TextRun{
												Content: "Item one\n",
											},
										},
										{
											StartIndex: 0,
											EndIndex:   10,
											ParagraphMarker: &slides.ParagraphMarker{
												Style: &slides.ParagraphStyle{
													IndentStart: &slides.Dimension{
														Magnitude: 18,
														Unit:      "PT",
													},
												},
												Bullet: &slides.Bullet{
													ListId: "list-1",
												},
											},
										},
										{
											StartIndex: 10,
											EndIndex:   20,
											TextRun: &slides.TextRun{
												Content: "Item two\n",
											},
										},
										{
											StartIndex: 10,
											EndIndex:   20,
											ParagraphMarker: &slides.ParagraphMarker{
												Style: &slides.ParagraphStyle{
													IndentStart: &slides.Dimension{
														Magnitude: 18,
														Unit:      "PT",
													},
												},
												Bullet: &slides.Bullet{
													ListId: "list-1",
												},
											},
										},
										{
											StartIndex: 20,
											EndIndex:   32,
											TextRun: &slides.TextRun{
												Content: "Item three\n",
											},
										},
										{
											StartIndex: 20,
											EndIndex:   32,
											ParagraphMarker: &slides.ParagraphMarker{
												Style: &slides.ParagraphStyle{
													IndentStart: &slides.Dimension{
														Magnitude: 18,
														Unit:      "PT",
													},
												},
												Bullet: &slides.Bullet{
													ListId: "list-1",
												},
											},
										},
									},
								},
							},
						},
						{
							ObjectId: "shape-plain-text",
							Shape: &slides.Shape{
								ShapeType: "TEXT_BOX",
								Text: &slides.TextContent{
									TextElements: []*slides.TextElement{
										{
											StartIndex: 0,
											EndIndex:   11,
											TextRun: &slides.TextRun{
												Content: "Plain text\n",
											},
										},
										{
											StartIndex: 0,
											EndIndex:   11,
											ParagraphMarker: &slides.ParagraphMarker{
												Style: &slides.ParagraphStyle{},
											},
										},
									},
								},
							},
						},
						{
							ObjectId: "table-1",
							Table: &slides.Table{
								Rows:    2,
								Columns: 2,
							},
						},
						{
							ObjectId: "image-1",
							Image:    &slides.Image{},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name           string
		input          ModifyListInput
		presentation   *slides.Presentation
		getErr         error
		batchUpdateErr error
		wantErr        error
		checkOutput    func(*testing.T, *ModifyListOutput)
		checkRequests  func(*testing.T, []*slides.Request)
	}{
		// === Remove action tests ===
		{
			name: "remove bullets from all paragraphs",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "remove",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.Action != "remove" {
					t.Errorf("expected action 'remove', got '%s'", out.Action)
				}
				if out.ParagraphScope != "ALL" {
					t.Errorf("expected paragraph scope 'ALL', got '%s'", out.ParagraphScope)
				}
				if out.Result != "Removed list formatting (converted to plain text)" {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				if requests[0].DeleteParagraphBullets == nil {
					t.Error("expected DeleteParagraphBullets request")
				}
				if requests[0].DeleteParagraphBullets.TextRange.Type != "ALL" {
					t.Errorf("expected text range type 'ALL', got '%s'", requests[0].DeleteParagraphBullets.TextRange.Type)
				}
			},
		},
		{
			name: "remove bullets from specific paragraphs",
			input: ModifyListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "shape-with-bullets",
				Action:           "remove",
				ParagraphIndices: []int{0, 2},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.ParagraphScope != "INDICES [0 2]" {
					t.Errorf("expected paragraph scope 'INDICES [0 2]', got '%s'", out.ParagraphScope)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].DeleteParagraphBullets
				if req == nil {
					t.Fatal("expected DeleteParagraphBullets request")
				}
				if req.TextRange.Type != "FIXED_RANGE" {
					t.Errorf("expected FIXED_RANGE, got %s", req.TextRange.Type)
				}
			},
		},

		// === Indent action tests ===
		{
			name: "increase indentation",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "increase_indent",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.Action != "increase_indent" {
					t.Errorf("expected action 'increase_indent', got '%s'", out.Action)
				}
				if out.Result != "Increased indentation to 36 points" {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				req := requests[0].UpdateParagraphStyle
				if req == nil {
					t.Fatal("expected UpdateParagraphStyle request")
				}
				if req.Style.IndentStart == nil {
					t.Fatal("expected IndentStart in style")
				}
				// Current indent is 18, should be increased to 36
				if req.Style.IndentStart.Magnitude != 36 {
					t.Errorf("expected indent 36, got %f", req.Style.IndentStart.Magnitude)
				}
				if req.Fields != "indentStart" {
					t.Errorf("expected fields 'indentStart', got '%s'", req.Fields)
				}
			},
		},
		{
			name: "decrease indentation",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "decrease_indent",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.Action != "decrease_indent" {
					t.Errorf("expected action 'decrease_indent', got '%s'", out.Action)
				}
				if out.Result != "Decreased indentation to 0 points" {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].UpdateParagraphStyle
				if req == nil {
					t.Fatal("expected UpdateParagraphStyle request")
				}
				// Current indent is 18, should be decreased to 0
				if req.Style.IndentStart.Magnitude != 0 {
					t.Errorf("expected indent 0, got %f", req.Style.IndentStart.Magnitude)
				}
			},
		},
		{
			name: "decrease indentation on plain text (no indent)",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-plain-text",
				Action:         "decrease_indent",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				// Should still return 0 points
				if out.Result != "Decreased indentation to 0 points" {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
		},

		// === Modify action tests ===
		{
			name: "modify bullet style",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "modify",
				Properties: &ListModifyProperties{
					BulletStyle: "STAR",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.Action != "modify" {
					t.Errorf("expected action 'modify', got '%s'", out.Action)
				}
				if !strings.Contains(out.Result, "bullet_style=BULLET_STAR_CIRCLE_SQUARE") {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				req := requests[0].CreateParagraphBullets
				if req == nil {
					t.Fatal("expected CreateParagraphBullets request")
				}
				if req.BulletPreset != "BULLET_STAR_CIRCLE_SQUARE" {
					t.Errorf("expected BULLET_STAR_CIRCLE_SQUARE preset, got %s", req.BulletPreset)
				}
			},
		},
		{
			name: "modify number style",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "modify",
				Properties: &ListModifyProperties{
					NumberStyle: "ROMAN_UPPER",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if !strings.Contains(out.Result, "number_style=NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL") {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].CreateParagraphBullets
				if req == nil {
					t.Fatal("expected CreateParagraphBullets request")
				}
				if req.BulletPreset != "NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL" {
					t.Errorf("expected NUMBERED_UPPERROMAN_UPPERALPHA_DECIMAL preset, got %s", req.BulletPreset)
				}
			},
		},
		{
			name: "modify list color",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "modify",
				Properties: &ListModifyProperties{
					Color: "#FF0000",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if !strings.Contains(out.Result, "color=#FF0000") {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Fatalf("expected 1 request, got %d", len(requests))
				}
				req := requests[0].UpdateTextStyle
				if req == nil {
					t.Fatal("expected UpdateTextStyle request")
				}
				if req.Fields != "foregroundColor" {
					t.Errorf("expected fields 'foregroundColor', got '%s'", req.Fields)
				}
			},
		},
		{
			name: "modify multiple properties",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "modify",
				Properties: &ListModifyProperties{
					BulletStyle: "DIAMOND",
					Color:       "#00FF00",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if !strings.Contains(out.Result, "bullet_style=") {
					t.Errorf("expected bullet_style in result: %s", out.Result)
				}
				if !strings.Contains(out.Result, "color=#00FF00") {
					t.Errorf("expected color in result: %s", out.Result)
				}
			},
			checkRequests: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 2 {
					t.Fatalf("expected 2 requests, got %d", len(requests))
				}
				// First should be bullet preset
				if requests[0].CreateParagraphBullets == nil {
					t.Error("expected CreateParagraphBullets as first request")
				}
				// Second should be color
				if requests[1].UpdateTextStyle == nil {
					t.Error("expected UpdateTextStyle as second request")
				}
			},
		},

		// === Case sensitivity tests ===
		{
			name: "case insensitive action - REMOVE",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "REMOVE",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.Action != "remove" {
					t.Errorf("expected action 'remove', got '%s'", out.Action)
				}
			},
		},
		{
			name: "case insensitive action - Increase_Indent",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "Increase_Indent",
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if out.Action != "increase_indent" {
					t.Errorf("expected action 'increase_indent', got '%s'", out.Action)
				}
			},
		},
		{
			name: "lowercase bullet style in modify",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "modify",
				Properties: &ListModifyProperties{
					BulletStyle: "star",
				},
			},
			presentation: createTestPresentation(),
			checkOutput: func(t *testing.T, out *ModifyListOutput) {
				if !strings.Contains(out.Result, "BULLET_STAR_CIRCLE_SQUARE") {
					t.Errorf("unexpected result: %s", out.Result)
				}
			},
		},

		// === Error cases ===
		{
			name: "error: empty presentation_id",
			input: ModifyListInput{
				PresentationID: "",
				ObjectID:       "shape-1",
				Action:         "remove",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "error: empty object_id",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "",
				Action:         "remove",
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "error: invalid action",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-1",
				Action:         "invalid",
			},
			wantErr: ErrInvalidListAction,
		},
		{
			name: "error: modify without properties",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-1",
				Action:         "modify",
			},
			wantErr: ErrNoListProperties,
		},
		{
			name: "error: modify with empty properties",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-1",
				Action:         "modify",
				Properties:     &ListModifyProperties{},
			},
			wantErr: ErrNoListProperties,
		},
		{
			name: "error: invalid bullet style in modify",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-1",
				Action:         "modify",
				Properties: &ListModifyProperties{
					BulletStyle: "INVALID",
				},
			},
			wantErr: ErrInvalidBulletStyle,
		},
		{
			name: "error: invalid number style in modify",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-1",
				Action:         "modify",
				Properties: &ListModifyProperties{
					NumberStyle: "INVALID",
				},
			},
			wantErr: ErrInvalidNumberStyle,
		},
		{
			name: "error: negative paragraph index",
			input: ModifyListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "shape-1",
				Action:           "remove",
				ParagraphIndices: []int{-1},
			},
			wantErr: ErrInvalidParagraphIndex,
		},
		{
			name: "error: paragraph index out of range",
			input: ModifyListInput{
				PresentationID:   "test-presentation-id",
				ObjectID:         "shape-with-bullets",
				Action:           "remove",
				ParagraphIndices: []int{99},
			},
			presentation: createTestPresentation(),
			wantErr:      ErrInvalidParagraphIndex,
		},
		{
			name: "error: object not found",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "nonexistent",
				Action:         "remove",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrObjectNotFound,
		},
		{
			name: "error: table object",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "table-1",
				Action:         "remove",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "error: image object",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "image-1",
				Action:         "remove",
			},
			presentation: createTestPresentation(),
			wantErr:      ErrNotTextObject,
		},
		{
			name: "error: presentation not found",
			input: ModifyListInput{
				PresentationID: "nonexistent",
				ObjectID:       "shape-1",
				Action:         "remove",
			},
			getErr:  errors.New("404 Not Found"),
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "error: access denied",
			input: ModifyListInput{
				PresentationID: "forbidden",
				ObjectID:       "shape-1",
				Action:         "remove",
			},
			getErr:  errors.New("403 Forbidden"),
			wantErr: ErrAccessDenied,
		},
		{
			name: "error: batch update fails",
			input: ModifyListInput{
				PresentationID: "test-presentation-id",
				ObjectID:       "shape-with-bullets",
				Action:         "remove",
			},
			presentation:   createTestPresentation(),
			batchUpdateErr: errors.New("API error"),
			wantErr:        ErrModifyListFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequests []*slides.Request

			mockService := &mockSlidesService{
				GetPresentationFunc: func(_ context.Context, _ string) (*slides.Presentation, error) {
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					if tt.presentation != nil {
						return tt.presentation, nil
					}
					return nil, errors.New("not found")
				},
				BatchUpdateFunc: func(_ context.Context, _ string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedRequests = requests
					if tt.batchUpdateErr != nil {
						return nil, tt.batchUpdateErr
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(_ context.Context, _ oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.ModifyList(ctx, nil, tt.input)

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

func TestGetCurrentIndent(t *testing.T) {
	tests := []struct {
		name             string
		text             *slides.TextContent
		paragraphIndices []int
		want             float64
	}{
		{
			name: "text with indent",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{
						EndIndex: 10,
						ParagraphMarker: &slides.ParagraphMarker{
							Style: &slides.ParagraphStyle{
								IndentStart: &slides.Dimension{
									Magnitude: 36,
									Unit:      "PT",
								},
							},
						},
					},
				},
			},
			paragraphIndices: nil,
			want:             36,
		},
		{
			name: "text without indent",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{
						EndIndex: 10,
						ParagraphMarker: &slides.ParagraphMarker{
							Style: &slides.ParagraphStyle{},
						},
					},
				},
			},
			paragraphIndices: nil,
			want:             0,
		},
		{
			name:             "nil text",
			text:             nil,
			paragraphIndices: nil,
			want:             0,
		},
		{
			name: "empty text elements",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{},
			},
			paragraphIndices: nil,
			want:             0,
		},
		{
			name: "indent with EMU unit (ignored)",
			text: &slides.TextContent{
				TextElements: []*slides.TextElement{
					{
						EndIndex: 10,
						ParagraphMarker: &slides.ParagraphMarker{
							Style: &slides.ParagraphStyle{
								IndentStart: &slides.Dimension{
									Magnitude: 457200,
									Unit:      "EMU",
								},
							},
						},
					},
				},
			},
			paragraphIndices: nil,
			want:             0, // EMU is not PT, so it returns 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCurrentIndent(tt.text, tt.paragraphIndices)
			if got != tt.want {
				t.Errorf("getCurrentIndent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildModifyListRequests(t *testing.T) {
	textContent := &slides.TextContent{
		TextElements: []*slides.TextElement{
			{EndIndex: 10, ParagraphMarker: &slides.ParagraphMarker{}},
		},
	}

	tests := []struct {
		name             string
		input            ModifyListInput
		wantCount        int
		wantDescContains string
	}{
		{
			name: "bullet style only",
			input: ModifyListInput{
				ObjectID: "shape-1",
				Properties: &ListModifyProperties{
					BulletStyle: "DISC",
				},
			},
			wantCount:        1,
			wantDescContains: "bullet_style=",
		},
		{
			name: "number style only",
			input: ModifyListInput{
				ObjectID: "shape-1",
				Properties: &ListModifyProperties{
					NumberStyle: "DECIMAL",
				},
			},
			wantCount:        1,
			wantDescContains: "number_style=",
		},
		{
			name: "color only",
			input: ModifyListInput{
				ObjectID: "shape-1",
				Properties: &ListModifyProperties{
					Color: "#FF0000",
				},
			},
			wantCount:        1,
			wantDescContains: "color=#FF0000",
		},
		{
			name: "all properties",
			input: ModifyListInput{
				ObjectID: "shape-1",
				Properties: &ListModifyProperties{
					BulletStyle: "DISC",
					NumberStyle: "DECIMAL",
					Color:       "#FF0000",
				},
			},
			wantCount:        3, // bullet, number, color
			wantDescContains: "bullet_style=",
		},
		{
			name: "invalid color ignored",
			input: ModifyListInput{
				ObjectID: "shape-1",
				Properties: &ListModifyProperties{
					Color: "invalid",
				},
			},
			wantCount:        0,  // Invalid color produces no requests
			wantDescContains: "", // Invalid color is silently ignored, so no description for it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, desc := buildModifyListRequests(tt.input, textContent)
			if len(requests) != tt.wantCount {
				t.Errorf("buildModifyListRequests() got %d requests, want %d", len(requests), tt.wantCount)
			}
			if tt.wantDescContains != "" && !strings.Contains(desc, tt.wantDescContains) {
				t.Errorf("description %q does not contain %q", desc, tt.wantDescContains)
			}
		})
	}
}

func TestBuildRemoveListRequests(t *testing.T) {
	textContent := &slides.TextContent{
		TextElements: []*slides.TextElement{
			{EndIndex: 10, ParagraphMarker: &slides.ParagraphMarker{}},
		},
	}

	input := ModifyListInput{
		ObjectID: "shape-1",
	}

	requests, desc := buildRemoveListRequests(input, textContent)

	if len(requests) != 1 {
		t.Errorf("expected 1 request, got %d", len(requests))
	}

	if requests[0].DeleteParagraphBullets == nil {
		t.Error("expected DeleteParagraphBullets request")
	}

	if requests[0].DeleteParagraphBullets.ObjectId != "shape-1" {
		t.Errorf("expected object_id 'shape-1', got '%s'", requests[0].DeleteParagraphBullets.ObjectId)
	}

	if desc != "Removed list formatting (converted to plain text)" {
		t.Errorf("unexpected description: %s", desc)
	}
}

func TestBuildIndentListRequests(t *testing.T) {
	textContent := &slides.TextContent{
		TextElements: []*slides.TextElement{
			{
				EndIndex: 10,
				ParagraphMarker: &slides.ParagraphMarker{
					Style: &slides.ParagraphStyle{
						IndentStart: &slides.Dimension{
							Magnitude: 18,
							Unit:      "PT",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name             string
		increase         bool
		wantIndent       float64
		wantDescContains string
	}{
		{
			name:             "increase indentation",
			increase:         true,
			wantIndent:       36, // 18 + 18
			wantDescContains: "Increased indentation to 36",
		},
		{
			name:             "decrease indentation",
			increase:         false,
			wantIndent:       0, // 18 - 18
			wantDescContains: "Decreased indentation to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ModifyListInput{
				ObjectID: "shape-1",
			}

			requests, desc := buildIndentListRequests(input, textContent, tt.increase)

			if len(requests) != 1 {
				t.Errorf("expected 1 request, got %d", len(requests))
			}

			req := requests[0].UpdateParagraphStyle
			if req == nil {
				t.Fatal("expected UpdateParagraphStyle request")
			}

			if req.Style.IndentStart.Magnitude != tt.wantIndent {
				t.Errorf("expected indent %f, got %f", tt.wantIndent, req.Style.IndentStart.Magnitude)
			}

			if req.Style.IndentStart.Unit != "PT" {
				t.Errorf("expected unit 'PT', got '%s'", req.Style.IndentStart.Unit)
			}

			if !strings.Contains(desc, tt.wantDescContains) {
				t.Errorf("description %q does not contain %q", desc, tt.wantDescContains)
			}
		})
	}
}

func TestBuildIndentListRequestsMinimumZero(t *testing.T) {
	// Test that indent doesn't go below 0
	textContent := &slides.TextContent{
		TextElements: []*slides.TextElement{
			{
				EndIndex: 10,
				ParagraphMarker: &slides.ParagraphMarker{
					Style: &slides.ParagraphStyle{
						IndentStart: &slides.Dimension{
							Magnitude: 5, // Less than default increment (18)
							Unit:      "PT",
						},
					},
				},
			},
		},
	}

	input := ModifyListInput{
		ObjectID: "shape-1",
	}

	requests, _ := buildIndentListRequests(input, textContent, false)

	req := requests[0].UpdateParagraphStyle
	if req.Style.IndentStart.Magnitude != 0 {
		t.Errorf("expected indent 0 (not negative), got %f", req.Style.IndentStart.Magnitude)
	}
}
