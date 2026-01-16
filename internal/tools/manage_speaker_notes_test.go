package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Helper to create a presentation with speaker notes
func createPresentationWithSpeakerNotes(slideID, notesShapeID, notesText string) *slides.Presentation {
	return &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{
				ObjectId: slideID,
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: notesShapeID,
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{
										Type: "BODY",
									},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{
												TextRun: &slides.TextRun{
													Content: notesText,
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
	}
}

// Helper to create a presentation without speaker notes
func createPresentationWithoutSpeakerNotes(slideID string) *slides.Presentation {
	return &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{
				ObjectId:        slideID,
				SlideProperties: &slides.SlideProperties{},
			},
		},
	}
}

func TestManageSpeakerNotes_Get(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		input        ManageSpeakerNotesInput
		presentation *slides.Presentation
		wantOutput   *ManageSpeakerNotesOutput
		wantErr      error
	}{
		{
			name: "get speaker notes by slide index",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "get",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "These are speaker notes"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "get",
				NotesContent: "These are speaker notes",
			},
		},
		{
			name: "get speaker notes by slide ID",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideID:        "slide-1",
				Action:         "get",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Notes by ID"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "get",
				NotesContent: "Notes by ID",
			},
		},
		{
			name: "get empty speaker notes",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "get",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", ""),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "get",
				NotesContent: "",
			},
		},
		{
			name: "get speaker notes - no notes page",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "get",
			},
			presentation: createPresentationWithoutSpeakerNotes("slide-1"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "get",
				NotesContent: "",
			},
		},
		{
			name: "get speaker notes - action case insensitive",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "GET",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Test notes"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "get",
				NotesContent: "Test notes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == tt.input.PresentationID {
						return tt.presentation, nil
					}
					return nil, errors.New("not found")
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.ManageSpeakerNotes(ctx, nil, tt.input)

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

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output.SlideID != tt.wantOutput.SlideID {
				t.Errorf("SlideID = %v, want %v", output.SlideID, tt.wantOutput.SlideID)
			}
			if output.SlideIndex != tt.wantOutput.SlideIndex {
				t.Errorf("SlideIndex = %v, want %v", output.SlideIndex, tt.wantOutput.SlideIndex)
			}
			if output.Action != tt.wantOutput.Action {
				t.Errorf("Action = %v, want %v", output.Action, tt.wantOutput.Action)
			}
			if output.NotesContent != tt.wantOutput.NotesContent {
				t.Errorf("NotesContent = %q, want %q", output.NotesContent, tt.wantOutput.NotesContent)
			}
		})
	}
}

func TestManageSpeakerNotes_Set(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          ManageSpeakerNotesInput
		presentation   *slides.Presentation
		batchUpdateErr error
		wantOutput     *ManageSpeakerNotesOutput
		wantErr        error
	}{
		{
			name: "set speaker notes - replace existing",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "set",
				NotesText:      "New speaker notes",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Old notes"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "set",
				NotesContent: "New speaker notes",
			},
		},
		{
			name: "set speaker notes - empty to new",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "set",
				NotesText:      "First notes",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", ""),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "set",
				NotesContent: "First notes",
			},
		},
		{
			name: "set speaker notes - no notes placeholder",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "set",
				NotesText:      "Notes",
			},
			presentation: createPresentationWithoutSpeakerNotes("slide-1"),
			wantErr:      ErrNotesShapeNotFound,
		},
		{
			name: "set speaker notes - batch update error",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "set",
				NotesText:      "Notes",
			},
			presentation:   createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Old"),
			batchUpdateErr: errors.New("API error"),
			wantErr:        ErrManageSpeakerNotesFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.ManageSpeakerNotes(ctx, nil, tt.input)

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

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output.SlideID != tt.wantOutput.SlideID {
				t.Errorf("SlideID = %v, want %v", output.SlideID, tt.wantOutput.SlideID)
			}
			if output.SlideIndex != tt.wantOutput.SlideIndex {
				t.Errorf("SlideIndex = %v, want %v", output.SlideIndex, tt.wantOutput.SlideIndex)
			}
			if output.Action != tt.wantOutput.Action {
				t.Errorf("Action = %v, want %v", output.Action, tt.wantOutput.Action)
			}
			if output.NotesContent != tt.wantOutput.NotesContent {
				t.Errorf("NotesContent = %q, want %q", output.NotesContent, tt.wantOutput.NotesContent)
			}
		})
	}
}

func TestManageSpeakerNotes_Append(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		input        ManageSpeakerNotesInput
		presentation *slides.Presentation
		wantOutput   *ManageSpeakerNotesOutput
		wantErr      error
	}{
		{
			name: "append to existing notes",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "append",
				NotesText:      " - additional notes",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Existing notes"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "append",
				NotesContent: "Existing notes - additional notes",
			},
		},
		{
			name: "append to empty notes",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "append",
				NotesText:      "First notes",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", ""),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "append",
				NotesContent: "First notes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == tt.input.PresentationID {
						return tt.presentation, nil
					}
					return nil, errors.New("not found")
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.ManageSpeakerNotes(ctx, nil, tt.input)

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

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output.SlideID != tt.wantOutput.SlideID {
				t.Errorf("SlideID = %v, want %v", output.SlideID, tt.wantOutput.SlideID)
			}
			if output.SlideIndex != tt.wantOutput.SlideIndex {
				t.Errorf("SlideIndex = %v, want %v", output.SlideIndex, tt.wantOutput.SlideIndex)
			}
			if output.Action != tt.wantOutput.Action {
				t.Errorf("Action = %v, want %v", output.Action, tt.wantOutput.Action)
			}
			if output.NotesContent != tt.wantOutput.NotesContent {
				t.Errorf("NotesContent = %q, want %q", output.NotesContent, tt.wantOutput.NotesContent)
			}
		})
	}
}

func TestManageSpeakerNotes_Clear(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		input        ManageSpeakerNotesInput
		presentation *slides.Presentation
		wantOutput   *ManageSpeakerNotesOutput
		wantErr      error
	}{
		{
			name: "clear existing notes",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "clear",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Notes to clear"),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "clear",
				NotesContent: "",
			},
		},
		{
			name: "clear already empty notes",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Action:         "clear",
			},
			presentation: createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", ""),
			wantOutput: &ManageSpeakerNotesOutput{
				SlideID:      "slide-1",
				SlideIndex:   1,
				Action:       "clear",
				NotesContent: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if presentationID == tt.input.PresentationID {
						return tt.presentation, nil
					}
					return nil, errors.New("not found")
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.ManageSpeakerNotes(ctx, nil, tt.input)

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

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output.SlideID != tt.wantOutput.SlideID {
				t.Errorf("SlideID = %v, want %v", output.SlideID, tt.wantOutput.SlideID)
			}
			if output.SlideIndex != tt.wantOutput.SlideIndex {
				t.Errorf("SlideIndex = %v, want %v", output.SlideIndex, tt.wantOutput.SlideIndex)
			}
			if output.Action != tt.wantOutput.Action {
				t.Errorf("Action = %v, want %v", output.Action, tt.wantOutput.Action)
			}
			if output.NotesContent != tt.wantOutput.NotesContent {
				t.Errorf("NotesContent = %q, want %q", output.NotesContent, tt.wantOutput.NotesContent)
			}
		})
	}
}

func TestManageSpeakerNotes_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	tools := NewTools(DefaultToolsConfig(), nil)

	tests := []struct {
		name    string
		input   ManageSpeakerNotesInput
		wantErr error
	}{
		{
			name: "missing presentation_id",
			input: ManageSpeakerNotesInput{
				SlideIndex: 1,
				Action:     "get",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing slide reference",
			input: ManageSpeakerNotesInput{
				PresentationID: "test",
				Action:         "get",
			},
			wantErr: ErrInvalidSlideReference,
		},
		{
			name: "invalid action",
			input: ManageSpeakerNotesInput{
				PresentationID: "test",
				SlideIndex:     1,
				Action:         "invalid",
			},
			wantErr: ErrInvalidSpeakerNotesAction,
		},
		{
			name: "missing notes_text for set",
			input: ManageSpeakerNotesInput{
				PresentationID: "test",
				SlideIndex:     1,
				Action:         "set",
			},
			wantErr: ErrNotesTextRequired,
		},
		{
			name: "missing notes_text for append",
			input: ManageSpeakerNotesInput{
				PresentationID: "test",
				SlideIndex:     1,
				Action:         "append",
			},
			wantErr: ErrNotesTextRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.ManageSpeakerNotes(ctx, nil, tt.input)
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

func TestManageSpeakerNotes_PresentationErrors(t *testing.T) {
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

			_, err := tools.ManageSpeakerNotes(ctx, nil, ManageSpeakerNotesInput{
				PresentationID: "test",
				SlideIndex:     1,
				Action:         "get",
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

func TestManageSpeakerNotes_SlideNotFound(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{ObjectId: "slide-1"},
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

	tests := []struct {
		name  string
		input ManageSpeakerNotesInput
	}{
		{
			name: "slide index out of range - too high",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     5,
				Action:         "get",
			},
		},
		{
			name: "slide index out of range - zero",
			input: ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideID:        "nonexistent-slide",
				Action:         "get",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.ManageSpeakerNotes(ctx, nil, tt.input)

			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !errors.Is(err, ErrSlideNotFound) {
				t.Errorf("expected ErrSlideNotFound, got %v", err)
			}
		})
	}
}

func TestManageSpeakerNotes_MultipleSlides(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-1",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Notes for slide 1"}},
										},
									},
								},
							},
						},
					},
				},
			},
			{
				ObjectId: "slide-2",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-2",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Notes for slide 2"}},
										},
									},
								},
							},
						},
					},
				},
			},
			{
				ObjectId: "slide-3",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-3",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Notes for slide 3"}},
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
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	tests := []struct {
		name       string
		slideIndex int
		slideID    string
		wantNotes  string
		wantIndex  int
	}{
		{
			name:       "get first slide notes by index",
			slideIndex: 1,
			wantNotes:  "Notes for slide 1",
			wantIndex:  1,
		},
		{
			name:       "get second slide notes by index",
			slideIndex: 2,
			wantNotes:  "Notes for slide 2",
			wantIndex:  2,
		},
		{
			name:       "get third slide notes by index",
			slideIndex: 3,
			wantNotes:  "Notes for slide 3",
			wantIndex:  3,
		},
		{
			name:      "get second slide notes by ID",
			slideID:   "slide-2",
			wantNotes: "Notes for slide 2",
			wantIndex: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tools.ManageSpeakerNotes(ctx, nil, ManageSpeakerNotesInput{
				PresentationID: "test-presentation",
				SlideIndex:     tt.slideIndex,
				SlideID:        tt.slideID,
				Action:         "get",
			})

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output.NotesContent != tt.wantNotes {
				t.Errorf("NotesContent = %q, want %q", output.NotesContent, tt.wantNotes)
			}
			if output.SlideIndex != tt.wantIndex {
				t.Errorf("SlideIndex = %d, want %d", output.SlideIndex, tt.wantIndex)
			}
		})
	}
}

func TestManageSpeakerNotes_BatchUpdateForbidden(t *testing.T) {
	ctx := context.Background()

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return createPresentationWithSpeakerNotes("slide-1", "notes-shape-1", "Old notes"), nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("403 forbidden")
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	_, err := tools.ManageSpeakerNotes(ctx, nil, ManageSpeakerNotesInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		Action:         "set",
		NotesText:      "New notes",
	})

	if err == nil {
		t.Error("expected error, got nil")
		return
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestBuildSpeakerNotesRequests(t *testing.T) {
	tests := []struct {
		name         string
		shapeID      string
		action       string
		notesText    string
		currentNotes string
		wantExpected string
		wantReqCount int
	}{
		{
			name:         "set replaces existing notes",
			shapeID:      "notes-1",
			action:       "set",
			notesText:    "New notes",
			currentNotes: "Old notes",
			wantExpected: "New notes",
			wantReqCount: 2, // Delete + Insert
		},
		{
			name:         "set on empty notes",
			shapeID:      "notes-1",
			action:       "set",
			notesText:    "New notes",
			currentNotes: "",
			wantExpected: "New notes",
			wantReqCount: 1, // Only Insert
		},
		{
			name:         "append to existing",
			shapeID:      "notes-1",
			action:       "append",
			notesText:    " more",
			currentNotes: "Start",
			wantExpected: "Start more",
			wantReqCount: 1, // Only Insert
		},
		{
			name:         "append to empty",
			shapeID:      "notes-1",
			action:       "append",
			notesText:    "First",
			currentNotes: "",
			wantExpected: "First",
			wantReqCount: 1, // Only Insert
		},
		{
			name:         "clear non-empty",
			shapeID:      "notes-1",
			action:       "clear",
			notesText:    "",
			currentNotes: "To clear",
			wantExpected: "",
			wantReqCount: 1, // Only Delete
		},
		{
			name:         "clear empty",
			shapeID:      "notes-1",
			action:       "clear",
			notesText:    "",
			currentNotes: "",
			wantExpected: "",
			wantReqCount: 0, // No requests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, expectedNotes := buildSpeakerNotesRequests(tt.shapeID, tt.action, tt.notesText, tt.currentNotes)

			if expectedNotes != tt.wantExpected {
				t.Errorf("expectedNotes = %q, want %q", expectedNotes, tt.wantExpected)
			}
			if len(requests) != tt.wantReqCount {
				t.Errorf("got %d requests, want %d", len(requests), tt.wantReqCount)
			}
		})
	}
}

func TestFindSpeakerNotesShape(t *testing.T) {
	tests := []struct {
		name         string
		slide        *slides.Page
		wantShapeID  string
		wantText     string
	}{
		{
			name:        "nil slide",
			slide:       nil,
			wantShapeID: "",
			wantText:    "",
		},
		{
			name: "no slide properties",
			slide: &slides.Page{
				ObjectId: "slide-1",
			},
			wantShapeID: "",
			wantText:    "",
		},
		{
			name: "no notes page",
			slide: &slides.Page{
				ObjectId:        "slide-1",
				SlideProperties: &slides.SlideProperties{},
			},
			wantShapeID: "",
			wantText:    "",
		},
		{
			name: "with BODY placeholder",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-shape",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Notes text"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantShapeID: "notes-shape",
			wantText:    "Notes text",
		},
		{
			name: "fallback to shape without placeholder",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "other-placeholder",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "SLIDE_IMAGE"},
								},
							},
							{
								ObjectId: "notes-shape",
								Shape: &slides.Shape{
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Fallback notes"}},
										},
									},
								},
							},
						},
					},
				},
			},
			wantShapeID: "notes-shape",
			wantText:    "Fallback notes",
		},
		{
			name: "BODY placeholder with empty text",
			slide: &slides.Page{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-shape",
								Shape: &slides.Shape{
									Placeholder: &slides.Placeholder{Type: "BODY"},
									Text:        nil,
								},
							},
						},
					},
				},
			},
			wantShapeID: "notes-shape",
			wantText:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shapeID, text := findSpeakerNotesShape(tt.slide)

			if shapeID != tt.wantShapeID {
				t.Errorf("shapeID = %q, want %q", shapeID, tt.wantShapeID)
			}
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}
