package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestSearchText(t *testing.T) {
	ctx := context.Background()

	// Helper to create a presentation with text shapes
	createPresentation := func(slideTexts map[string][]string) *slides.Presentation {
		presentation := &slides.Presentation{
			PresentationId: "test-presentation",
			Slides:         make([]*slides.Page, 0),
		}

		slideIdx := 1
		for slideID, texts := range slideTexts {
			slide := &slides.Page{
				ObjectId:     slideID,
				PageElements: make([]*slides.PageElement, 0),
			}

			for i, text := range texts {
				slide.PageElements = append(slide.PageElements, &slides.PageElement{
					ObjectId: slideID + "-shape-" + string(rune('a'+i)),
					Shape: &slides.Shape{
						ShapeType: "TEXT_BOX",
						Text: &slides.TextContent{
							TextElements: []*slides.TextElement{
								{TextRun: &slides.TextRun{Content: text}},
							},
						},
					},
				})
			}

			presentation.Slides = append(presentation.Slides, slide)
			slideIdx++
		}

		return presentation
	}

	tests := []struct {
		name            string
		input           SearchTextInput
		presentation    *slides.Presentation
		wantMatches     int
		wantSlides      int
		wantErr         error
	}{
		{
			name: "find single match",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "hello",
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"Hello world", "Goodbye"},
			}),
			wantMatches: 1,
			wantSlides:  1,
		},
		{
			name: "find multiple matches on same slide",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "test",
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"This is a test", "Another test here"},
			}),
			wantMatches: 2,
			wantSlides:  1,
		},
		{
			name: "find matches across multiple slides",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "hello",
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"Hello world"},
				"slide-2": {"Hello again"},
			}),
			wantMatches: 2,
			wantSlides:  2,
		},
		{
			name: "case insensitive search (default)",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "HELLO",
				CaseSensitive:  false,
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"Hello world", "HELLO there", "hello again"},
			}),
			wantMatches: 3,
			wantSlides:  1,
		},
		{
			name: "case sensitive search",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "Hello",
				CaseSensitive:  true,
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"Hello world", "HELLO there", "hello again"},
			}),
			wantMatches: 1,
			wantSlides:  1,
		},
		{
			name: "no matches found",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "xyz",
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"Hello world"},
			}),
			wantMatches: 0,
			wantSlides:  0,
		},
		{
			name: "multiple matches in same text",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "the",
			},
			presentation: createPresentation(map[string][]string{
				"slide-1": {"The quick brown fox jumps over the lazy dog"},
			}),
			wantMatches: 2, // "The" and "the"
			wantSlides:  1,
		},
		{
			name: "empty presentation",
			input: SearchTextInput{
				PresentationID: "test-presentation",
				Query:          "hello",
			},
			presentation: &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{},
			},
			wantMatches: 0,
			wantSlides:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return tt.presentation, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			})

			output, err := tools.SearchText(ctx, nil, tt.input)

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

			if output.TotalMatches != tt.wantMatches {
				t.Errorf("TotalMatches = %d, want %d", output.TotalMatches, tt.wantMatches)
			}
			if len(output.Results) != tt.wantSlides {
				t.Errorf("len(Results) = %d, want %d slides", len(output.Results), tt.wantSlides)
			}
		})
	}
}

func TestSearchText_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	tools := NewTools(DefaultToolsConfig(), nil)

	tests := []struct {
		name    string
		input   SearchTextInput
		wantErr error
	}{
		{
			name: "missing presentation_id",
			input: SearchTextInput{
				Query: "hello",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "missing query",
			input: SearchTextInput{
				PresentationID: "test",
			},
			wantErr: ErrInvalidQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.SearchText(ctx, nil, tt.input)
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

func TestSearchText_PresentationErrors(t *testing.T) {
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

			_, err := tools.SearchText(ctx, nil, SearchTextInput{
				PresentationID: "test",
				Query:          "hello",
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

func TestSearchText_TableSearch(t *testing.T) {
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
							Rows:    2,
							Columns: 2,
							TableRows: []*slides.TableRow{
								{
									TableCells: []*slides.TableCell{
										{
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "Hello cell"}},
												},
											},
										},
										{
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "World cell"}},
												},
											},
										},
									},
								},
								{
									TableCells: []*slides.TableCell{
										{
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "Another hello"}},
												},
											},
										},
										{
											Text: &slides.TextContent{
												TextElements: []*slides.TextElement{
													{TextRun: &slides.TextRun{Content: "Goodbye"}},
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

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	output, err := tools.SearchText(ctx, nil, SearchTextInput{
		PresentationID: "test-presentation",
		Query:          "hello",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if output.TotalMatches != 2 {
		t.Errorf("TotalMatches = %d, want 2 (both cells with 'hello')", output.TotalMatches)
	}

	// Check that table cell positions are in object IDs
	if len(output.Results) > 0 && len(output.Results[0].Matches) > 0 {
		firstMatch := output.Results[0].Matches[0]
		if firstMatch.ObjectType != "TABLE_CELL" {
			t.Errorf("ObjectType = %s, want TABLE_CELL", firstMatch.ObjectType)
		}
		// Should include row,col position like "table-1[0,0]"
		if !contains(firstMatch.ObjectID, "[") {
			t.Errorf("ObjectID = %s, should contain cell position", firstMatch.ObjectID)
		}
	}
}

func TestSearchText_GroupedElements(t *testing.T) {
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
												{TextRun: &slides.TextRun{Content: "Hello from group"}},
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
	}

	tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	})

	output, err := tools.SearchText(ctx, nil, SearchTextInput{
		PresentationID: "test-presentation",
		Query:          "hello",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if output.TotalMatches != 1 {
		t.Errorf("TotalMatches = %d, want 1 (nested element)", output.TotalMatches)
	}

	if len(output.Results) > 0 && len(output.Results[0].Matches) > 0 {
		if output.Results[0].Matches[0].ObjectID != "nested-shape" {
			t.Errorf("ObjectID = %s, want nested-shape", output.Results[0].Matches[0].ObjectID)
		}
	}
}

func TestSearchText_SpeakerNotes(t *testing.T) {
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
									{TextRun: &slides.TextRun{Content: "Main content"}},
								},
							},
						},
					},
				},
				SlideProperties: &slides.SlideProperties{
					NotesPage: &slides.Page{
						PageElements: []*slides.PageElement{
							{
								ObjectId: "notes-shape",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Hello in speaker notes"}},
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

	output, err := tools.SearchText(ctx, nil, SearchTextInput{
		PresentationID: "test-presentation",
		Query:          "hello",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if output.TotalMatches != 1 {
		t.Errorf("TotalMatches = %d, want 1 (from speaker notes)", output.TotalMatches)
	}

	// Check that the match is marked as from speaker notes
	if len(output.Results) > 0 && len(output.Results[0].Matches) > 0 {
		match := output.Results[0].Matches[0]
		if !contains(match.ObjectType, "SPEAKER_NOTES") {
			t.Errorf("ObjectType = %s, should contain SPEAKER_NOTES prefix", match.ObjectType)
		}
	}
}

func TestSearchText_ContextExtraction(t *testing.T) {
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
									{TextRun: &slides.TextRun{Content: "This is some text before the keyword TARGET and some text after the keyword"}},
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

	output, err := tools.SearchText(ctx, nil, SearchTextInput{
		PresentationID: "test-presentation",
		Query:          "TARGET",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if output.TotalMatches != 1 {
		t.Errorf("TotalMatches = %d, want 1", output.TotalMatches)
		return
	}

	match := output.Results[0].Matches[0]

	// Context should include text before and after
	if !contains(match.TextContext, "keyword") {
		t.Errorf("TextContext = %q, should contain 'keyword' (context word)", match.TextContext)
	}
	if !contains(match.TextContext, "TARGET") {
		t.Errorf("TextContext = %q, should contain 'TARGET'", match.TextContext)
	}

	// Check start index
	expectedIndex := 37 // Position of "TARGET" in the text
	if match.StartIndex != expectedIndex {
		t.Errorf("StartIndex = %d, want %d", match.StartIndex, expectedIndex)
	}
}

func TestSearchText_OverlappingMatches(t *testing.T) {
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
									{TextRun: &slides.TextRun{Content: "aaa"}},
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

	output, err := tools.SearchText(ctx, nil, SearchTextInput{
		PresentationID: "test-presentation",
		Query:          "aa",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// "aaa" contains "aa" at positions 0 and 1 (overlapping)
	if output.TotalMatches != 2 {
		t.Errorf("TotalMatches = %d, want 2 (overlapping matches)", output.TotalMatches)
	}

	// Check indices
	if len(output.Results) > 0 && len(output.Results[0].Matches) >= 2 {
		if output.Results[0].Matches[0].StartIndex != 0 {
			t.Errorf("First match StartIndex = %d, want 0", output.Results[0].Matches[0].StartIndex)
		}
		if output.Results[0].Matches[1].StartIndex != 1 {
			t.Errorf("Second match StartIndex = %d, want 1", output.Results[0].Matches[1].StartIndex)
		}
	}
}

func TestFindMatchesInText(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		query         string
		caseSensitive bool
		wantCount     int
		wantIndices   []int
	}{
		{
			name:          "simple match",
			text:          "Hello world",
			query:         "world",
			caseSensitive: false,
			wantCount:     1,
			wantIndices:   []int{6},
		},
		{
			name:          "case insensitive",
			text:          "Hello WORLD world World",
			query:         "world",
			caseSensitive: false,
			wantCount:     3,
			wantIndices:   []int{6, 12, 18},
		},
		{
			name:          "case sensitive",
			text:          "Hello WORLD world World",
			query:         "world",
			caseSensitive: true,
			wantCount:     1,
			wantIndices:   []int{12},
		},
		{
			name:          "no match",
			text:          "Hello world",
			query:         "xyz",
			caseSensitive: false,
			wantCount:     0,
			wantIndices:   []int{},
		},
		{
			name:          "empty text",
			text:          "",
			query:         "hello",
			caseSensitive: false,
			wantCount:     0,
			wantIndices:   []int{},
		},
		{
			name:          "empty query",
			text:          "Hello",
			query:         "",
			caseSensitive: false,
			wantCount:     0,
			wantIndices:   []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := findMatchesInText(tt.text, tt.query, tt.caseSensitive)

			if len(matches) != tt.wantCount {
				t.Errorf("got %d matches, want %d", len(matches), tt.wantCount)
			}

			for i, wantIdx := range tt.wantIndices {
				if i < len(matches) && matches[i].startIndex != wantIdx {
					t.Errorf("match %d: startIndex = %d, want %d", i, matches[i].startIndex, wantIdx)
				}
			}
		})
	}
}

func TestExtractContext(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		matchStart   int
		matchLen     int
		contextChars int
		wantPrefix   string // Expected to start with
		wantSuffix   string // Expected to end with
		wantContains string // Expected to contain
	}{
		{
			name:         "match at start",
			text:         "Hello world, how are you?",
			matchStart:   0,
			matchLen:     5,
			contextChars: 10,
			wantPrefix:   "Hello", // No ellipsis at start
			wantSuffix:   "...",   // Truncated at end
			wantContains: "Hello world",
		},
		{
			name:         "match at end",
			text:         "Hello world, how are you?",
			matchStart:   21,
			matchLen:     4,
			contextChars: 10,
			wantPrefix:   "...",  // Truncated at start
			wantSuffix:   "you?", // No ellipsis at end
			wantContains: "you?",
		},
		{
			name:         "match in middle",
			text:         "This is a test string for context extraction",
			matchStart:   10,
			matchLen:     4,
			contextChars: 5,
			wantPrefix:   "...",
			wantSuffix:   "...",
			wantContains: "test",
		},
		{
			name:         "short text no ellipsis",
			text:         "Hello",
			matchStart:   0,
			matchLen:     5,
			contextChars: 50,
			wantPrefix:   "Hello",
			wantSuffix:   "Hello",
			wantContains: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContext(tt.text, tt.matchStart, tt.matchLen, tt.contextChars)

			if !hasPrefix(result, tt.wantPrefix) {
				t.Errorf("result = %q, want prefix %q", result, tt.wantPrefix)
			}
			if !hasSuffix(result, tt.wantSuffix) {
				t.Errorf("result = %q, want suffix %q", result, tt.wantSuffix)
			}
			if !contains(result, tt.wantContains) {
				t.Errorf("result = %q, should contain %q", result, tt.wantContains)
			}
		})
	}
}

// Note: contains and findSubstring helpers are declared in search_presentations_test.go

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
