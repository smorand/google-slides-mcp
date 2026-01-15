package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestModifyText(t *testing.T) {
	ctx := context.Background()

	// Helper to create a presentation with a text shape
	createPresentationWithTextShape := func(objectID, text string) *slides.Presentation {
		return &slides.Presentation{
			PresentationId: "test-presentation",
			Slides: []*slides.Page{
				{
					ObjectId: "slide-1",
					PageElements: []*slides.PageElement{
						{
							ObjectId: objectID,
							Shape: &slides.Shape{
								ShapeType: "TEXT_BOX",
								Text: &slides.TextContent{
									TextElements: []*slides.TextElement{
										{
											TextRun: &slides.TextRun{
												Content: text,
											},
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
		input          ModifyTextInput
		presentation   *slides.Presentation
		batchUpdateErr error
		wantOutput     *ModifyTextOutput
		wantErr        error
		wantErrMsg     string
	}{
		{
			name: "replace all text",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "New text",
			},
			presentation: createPresentationWithTextShape("shape-1", "Old text"),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "New text",
				Action:      "replace",
			},
		},
		{
			name: "replace partial text with indices",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "REPLACED",
				StartIndex:     intPtr(5),
				EndIndex:       intPtr(10),
			},
			presentation: createPresentationWithTextShape("shape-1", "Hello World!"),
			// "Hello World!" -> indices 5-10 is " Worl" -> result is "HelloREPLACEDd!"
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "HelloREPLACEDd!",
				Action:      "replace",
			},
		},
		{
			name: "append text",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "append",
				Text:           " appended",
			},
			presentation: createPresentationWithTextShape("shape-1", "Hello"),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "Hello appended",
				Action:      "append",
			},
		},
		{
			name: "prepend text",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "prepend",
				Text:           "Start: ",
			},
			presentation: createPresentationWithTextShape("shape-1", "Hello"),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "Start: Hello",
				Action:      "prepend",
			},
		},
		{
			name: "delete all text",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "delete",
			},
			presentation: createPresentationWithTextShape("shape-1", "Text to delete"),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "",
				Action:      "delete",
			},
		},
		{
			name: "delete empty text (no-op)",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "delete",
			},
			presentation: createPresentationWithTextShape("shape-1", ""),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "",
				Action:      "delete",
			},
		},
		{
			name: "replace with empty text (clears)",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "",
			},
			presentation: createPresentationWithTextShape("shape-1", "Original"),
			wantErr:      ErrTextRequired,
		},
		{
			name: "append to empty text",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "append",
				Text:           "New content",
			},
			presentation: createPresentationWithTextShape("shape-1", ""),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "New content",
				Action:      "append",
			},
		},
		{
			name: "batch update failure",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "New text",
			},
			presentation:   createPresentationWithTextShape("shape-1", "Old text"),
			batchUpdateErr: errors.New("API error"),
			wantErr:        ErrModifyTextFailed,
		},
		{
			name: "object not found",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "nonexistent",
				Action:         "replace",
				Text:           "New text",
			},
			presentation: createPresentationWithTextShape("shape-1", "Old text"),
			wantErr:      ErrObjectNotFound,
		},
		{
			name: "replace partial at end of text",
			input: ModifyTextInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "END",
				StartIndex:     intPtr(5),
				EndIndex:       intPtr(100), // Beyond text length
			},
			presentation: createPresentationWithTextShape("shape-1", "Hello World"),
			wantOutput: &ModifyTextOutput{
				ObjectID:    "shape-1",
				UpdatedText: "HelloEND",
				Action:      "replace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock slides service
			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == tt.input.PresentationID {
						return tt.presentation, nil
					}
					return nil, errors.New("not found")
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					if tt.batchUpdateErr != nil {
						return nil, tt.batchUpdateErr
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			// Create tools instance with mock
			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			// Execute
			output, err := tools.ModifyText(ctx, nil, tt.input)

			// Check error
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if tt.wantErrMsg != "" {
				if err == nil || err.Error() != tt.wantErrMsg {
					t.Errorf("expected error message %q, got %v", tt.wantErrMsg, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check output
			if tt.wantOutput != nil {
				if output.ObjectID != tt.wantOutput.ObjectID {
					t.Errorf("ObjectID = %v, want %v", output.ObjectID, tt.wantOutput.ObjectID)
				}
				if output.UpdatedText != tt.wantOutput.UpdatedText {
					t.Errorf("UpdatedText = %q, want %q", output.UpdatedText, tt.wantOutput.UpdatedText)
				}
				if output.Action != tt.wantOutput.Action {
					t.Errorf("Action = %v, want %v", output.Action, tt.wantOutput.Action)
				}
			}
		})
	}
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
	return &i
}

func TestModifyText_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	tools := NewTools(DefaultToolsConfig(), nil)

	tests := []struct {
		name    string
		input   ModifyTextInput
		wantErr error
	}{
		{
			name: "missing presentation_id",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "replace",
				Text:     "New text",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing object_id",
			input: ModifyTextInput{
				PresentationID: "test",
				Action:         "replace",
				Text:           "New text",
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "invalid action",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "invalid",
				Text:           "New text",
			},
			wantErr: ErrInvalidAction,
		},
		{
			name: "missing text for replace",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "replace",
			},
			wantErr: ErrTextRequired,
		},
		{
			name: "missing text for append",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "append",
			},
			wantErr: ErrTextRequired,
		},
		{
			name: "missing text for prepend",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "prepend",
			},
			wantErr: ErrTextRequired,
		},
		{
			name: "negative start_index",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "New",
				StartIndex:     intPtr(-1),
			},
			wantErr: ErrInvalidTextRange,
		},
		{
			name: "negative end_index",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "New",
				EndIndex:       intPtr(-1),
			},
			wantErr: ErrInvalidTextRange,
		},
		{
			name: "start_index greater than end_index",
			input: ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "New",
				StartIndex:     intPtr(10),
				EndIndex:       intPtr(5),
			},
			wantErr: ErrInvalidTextRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.ModifyText(ctx, nil, tt.input)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestModifyText_PresentationErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		getPresentErr error
		wantErr       error
	}{
		{
			name:          "presentation not found",
			getPresentErr: errors.New("404 not found"),
			wantErr:       ErrPresentationNotFound,
		},
		{
			name:          "access denied",
			getPresentErr: errors.New("403 forbidden"),
			wantErr:       ErrAccessDenied,
		},
		{
			name:          "api error",
			getPresentErr: errors.New("internal server error"),
			wantErr:       ErrSlidesAPIError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return nil, tt.getPresentErr
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			_, err := tools.ModifyText(ctx, nil, ModifyTextInput{
				PresentationID: "test",
				ObjectID:       "shape-1",
				Action:         "replace",
				Text:           "New text",
			})

			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestModifyText_TableNotSupported(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				PageElements: []*slides.PageElement{
					{
						ObjectId: "table-1",
						Table: &slides.Table{
							Rows:    3,
							Columns: 3,
						},
					},
				},
			},
		},
	}

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	_, err := tools.ModifyText(ctx, nil, ModifyTextInput{
		PresentationID: "test-presentation",
		ObjectID:       "table-1",
		Action:         "replace",
		Text:           "New text",
	})

	if err == nil {
		t.Error("expected error for table, got nil")
		return
	}
	if !errors.Is(err, ErrNotTextObject) {
		t.Errorf("expected ErrNotTextObject, got %v", err)
	}
}

func TestModifyText_ImageNotSupported(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
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
	}

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	_, err := tools.ModifyText(ctx, nil, ModifyTextInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Action:         "replace",
		Text:           "New text",
	})

	if err == nil {
		t.Error("expected error for image, got nil")
		return
	}
	if !errors.Is(err, ErrNotTextObject) {
		t.Errorf("expected ErrNotTextObject, got %v", err)
	}
}

func TestModifyText_DeleteWithIndices_IgnoresIndices(t *testing.T) {
	// Delete action does not use indices - it always deletes all text
	// This test verifies that indices are ignored for delete action
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
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
											Content: "Hello World",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	// Even with indices, delete should clear all text
	output, err := tools.ModifyText(ctx, nil, ModifyTextInput{
		PresentationID: "test-presentation",
		ObjectID:       "shape-1",
		Action:         "delete",
		StartIndex:     intPtr(0),
		EndIndex:       intPtr(5),
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if output.UpdatedText != "" {
		t.Errorf("expected empty text after delete, got %q", output.UpdatedText)
	}
}

func TestBuildModifyTextRequests(t *testing.T) {
	tests := []struct {
		name         string
		input        ModifyTextInput
		currentText  string
		wantExpected string
		wantReqCount int
	}{
		{
			name: "replace all",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "replace",
				Text:     "New",
			},
			currentText:  "Old",
			wantExpected: "New",
			wantReqCount: 2, // Delete + Insert
		},
		{
			name: "replace empty",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "replace",
				Text:     "New",
			},
			currentText:  "",
			wantExpected: "New",
			wantReqCount: 1, // Only Insert (no delete needed)
		},
		{
			name: "replace partial",
			input: ModifyTextInput{
				ObjectID:   "shape-1",
				Action:     "replace",
				Text:       "X",
				StartIndex: intPtr(2),
				EndIndex:   intPtr(4),
			},
			currentText:  "Hello",
			wantExpected: "HeXo",
			wantReqCount: 2, // Delete + Insert
		},
		{
			name: "replace partial same indices",
			input: ModifyTextInput{
				ObjectID:   "shape-1",
				Action:     "replace",
				Text:       "X",
				StartIndex: intPtr(2),
				EndIndex:   intPtr(2),
			},
			currentText:  "Hello",
			wantExpected: "HeXllo",
			wantReqCount: 1, // Only Insert (no delete when start==end)
		},
		{
			name: "append",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "append",
				Text:     " World",
			},
			currentText:  "Hello",
			wantExpected: "Hello World",
			wantReqCount: 1, // Only Insert
		},
		{
			name: "prepend",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "prepend",
				Text:     "Say ",
			},
			currentText:  "Hello",
			wantExpected: "Say Hello",
			wantReqCount: 1, // Only Insert
		},
		{
			name: "delete non-empty",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "delete",
			},
			currentText:  "Hello",
			wantExpected: "",
			wantReqCount: 1, // Only Delete
		},
		{
			name: "delete empty",
			input: ModifyTextInput{
				ObjectID: "shape-1",
				Action:   "delete",
			},
			currentText:  "",
			wantExpected: "",
			wantReqCount: 0, // No requests needed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, expectedText := buildModifyTextRequests(tt.input, tt.currentText)

			if expectedText != tt.wantExpected {
				t.Errorf("expectedText = %q, want %q", expectedText, tt.wantExpected)
			}
			if len(requests) != tt.wantReqCount {
				t.Errorf("got %d requests, want %d", len(requests), tt.wantReqCount)
			}
		})
	}
}

func TestModifyText_BatchUpdateForbidden(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
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
		},
	}

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("403 forbidden")
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	_, err := tools.ModifyText(ctx, nil, ModifyTextInput{
		PresentationID: "test-presentation",
		ObjectID:       "shape-1",
		Action:         "replace",
		Text:           "New text",
	})

	if err == nil {
		t.Error("expected error, got nil")
		return
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestModifyText_ObjectInGroup(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				PageElements: []*slides.PageElement{
					{
						ObjectId: "group-1",
						ElementGroup: &slides.Group{
							Children: []*slides.PageElement{
								{
									ObjectId: "nested-shape",
									Shape: &slides.Shape{
										ShapeType: "TEXT_BOX",
										Text: &slides.TextContent{
											TextElements: []*slides.TextElement{
												{TextRun: &slides.TextRun{Content: "Nested text"}},
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
	}

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	output, err := tools.ModifyText(ctx, nil, ModifyTextInput{
		PresentationID: "test-presentation",
		ObjectID:       "nested-shape",
		Action:         "replace",
		Text:           "Updated nested text",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if output.ObjectID != "nested-shape" {
		t.Errorf("ObjectID = %v, want nested-shape", output.ObjectID)
	}
	if output.UpdatedText != "Updated nested text" {
		t.Errorf("UpdatedText = %q, want %q", output.UpdatedText, "Updated nested text")
	}
}
