package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestDuplicateSlide_Success(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			// Check that it's a DuplicateObject request
			if len(requests) == 1 && requests[0].DuplicateObject != nil {
				return &slides.BatchUpdatePresentationResponse{
					Replies: []*slides.Response{
						{
							DuplicateObject: &slides.DuplicateObjectResponse{
								ObjectId: "slide-1-copy",
							},
						},
					},
				}, nil
			}
			// Move request (UpdateSlidesPosition)
			if len(requests) == 1 && requests[0].UpdateSlidesPosition != nil {
				return &slides.BatchUpdatePresentationResponse{}, nil
			}
			return nil, errors.New("unexpected request")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DuplicateSlide(context.Background(), tokenSource, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SlideID != "slide-1-copy" {
		t.Errorf("expected slide ID 'slide-1-copy', got '%s'", output.SlideID)
	}
	// Default position is after source slide (index 1) = position 2
	if output.SlideIndex != 2 {
		t.Errorf("expected slide index 2, got %d", output.SlideIndex)
	}
}

func TestDuplicateSlide_BySlideID(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) == 1 && requests[0].DuplicateObject != nil {
				if requests[0].DuplicateObject.ObjectId != "slide-2" {
					t.Errorf("expected to duplicate 'slide-2', got '%s'", requests[0].DuplicateObject.ObjectId)
				}
				return &slides.BatchUpdatePresentationResponse{
					Replies: []*slides.Response{
						{
							DuplicateObject: &slides.DuplicateObjectResponse{
								ObjectId: "slide-2-copy",
							},
						},
					},
				}, nil
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DuplicateSlide(context.Background(), tokenSource, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideID:        "slide-2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SlideID != "slide-2-copy" {
		t.Errorf("expected slide ID 'slide-2-copy', got '%s'", output.SlideID)
	}
	// Default position is after source slide (index 2) = position 3
	if output.SlideIndex != 3 {
		t.Errorf("expected slide index 3, got %d", output.SlideIndex)
	}
}

func TestDuplicateSlide_SlideIDTakesPrecedence(t *testing.T) {
	var duplicatedSlideID string

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) == 1 && requests[0].DuplicateObject != nil {
				duplicatedSlideID = requests[0].DuplicateObject.ObjectId
				return &slides.BatchUpdatePresentationResponse{
					Replies: []*slides.Response{
						{
							DuplicateObject: &slides.DuplicateObjectResponse{
								ObjectId: "slide-3-copy",
							},
						},
					},
				}, nil
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Both SlideIndex and SlideID provided - SlideID should take precedence
	output, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,       // Would be slide-1
		SlideID:        "slide-3", // slide-3 takes precedence
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if duplicatedSlideID != "slide-3" {
		t.Errorf("expected to duplicate 'slide-3', got '%s'", duplicatedSlideID)
	}

	if output.SlideID != "slide-3-copy" {
		t.Errorf("expected slide ID 'slide-3-copy', got '%s'", output.SlideID)
	}
}

func TestDuplicateSlide_WithInsertAt(t *testing.T) {
	tests := []struct {
		name              string
		sourceSlideIndex  int
		insertAt          int
		numExistingSlides int
		expectedPosition  int // 1-based output position
		needsMove         bool
	}{
		{
			name:              "insert at same position as default (no move needed)",
			sourceSlideIndex:  1,
			insertAt:          2, // After slide-1 (default)
			numExistingSlides: 3,
			expectedPosition:  2,
			needsMove:         false,
		},
		{
			name:              "insert at beginning",
			sourceSlideIndex:  2,
			insertAt:          1,
			numExistingSlides: 3,
			expectedPosition:  1,
			needsMove:         true,
		},
		{
			name:              "insert at end",
			sourceSlideIndex:  1,
			insertAt:          4, // After all 3 slides
			numExistingSlides: 3,
			expectedPosition:  4,
			needsMove:         true,
		},
		{
			name:              "insert_at clamped to max",
			sourceSlideIndex:  1,
			insertAt:          100, // Way beyond
			numExistingSlides: 3,
			expectedPosition:  4, // Clamped to end
			needsMove:         true,
		},
		{
			name:              "insert at zero means default (after source)",
			sourceSlideIndex:  2,
			insertAt:          0,
			numExistingSlides: 3,
			expectedPosition:  3, // After slide-2
			needsMove:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchCallCount := 0

			existingSlides := make([]*slides.Page, tt.numExistingSlides)
			for i := 0; i < tt.numExistingSlides; i++ {
				existingSlides[i] = &slides.Page{ObjectId: "slide-" + string(rune('1'+i))}
			}

			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return &slides.Presentation{
						PresentationId: presentationID,
						Slides:         existingSlides,
					}, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					batchCallCount++
					if len(requests) == 1 && requests[0].DuplicateObject != nil {
						return &slides.BatchUpdatePresentationResponse{
							Replies: []*slides.Response{
								{
									DuplicateObject: &slides.DuplicateObjectResponse{
										ObjectId: "new-slide",
									},
								},
							},
						}, nil
					}
					if len(requests) == 1 && requests[0].UpdateSlidesPosition != nil {
						return &slides.BatchUpdatePresentationResponse{}, nil
					}
					return nil, errors.New("unexpected request")
				},
			}

			factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			}

			tools := NewTools(DefaultToolsConfig(), factory)

			output, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
				PresentationID: "test-presentation",
				SlideIndex:     tt.sourceSlideIndex,
				InsertAt:       tt.insertAt,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.SlideIndex != tt.expectedPosition {
				t.Errorf("expected slide index %d, got %d", tt.expectedPosition, output.SlideIndex)
			}

			expectedCalls := 1
			if tt.needsMove {
				expectedCalls = 2
			}
			if batchCallCount != expectedCalls {
				t.Errorf("expected %d batch call(s), got %d", expectedCalls, batchCallCount)
			}
		})
	}
}

func TestDuplicateSlide_DuplicateLastSlide(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{
						DuplicateObject: &slides.DuplicateObjectResponse{
							ObjectId: "slide-3-copy",
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     3, // Last slide
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be position 4 (after last slide)
	if output.SlideIndex != 4 {
		t.Errorf("expected slide index 4, got %d", output.SlideIndex)
	}
}

func TestDuplicateSlide_SingleSlidePresentation(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "only-slide"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{
					{
						DuplicateObject: &slides.DuplicateObjectResponse{
							ObjectId: "only-slide-copy",
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SlideIndex != 2 {
		t.Errorf("expected slide index 2, got %d", output.SlideIndex)
	}

	if output.SlideID != "only-slide-copy" {
		t.Errorf("expected slide_id only-slide-copy, got %s", output.SlideID)
	}
}

func TestDuplicateSlide_MissingPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		SlideIndex: 1,
	})

	if err == nil {
		t.Fatal("expected error for missing presentation ID")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestDuplicateSlide_MissingSlideReference(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
	})

	if err == nil {
		t.Fatal("expected error for missing slide reference")
	}

	if !errors.Is(err, ErrInvalidSlideReference) {
		t.Errorf("expected ErrInvalidSlideReference, got %v", err)
	}
}

func TestDuplicateSlide_SlideIndexOutOfRange(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     10, // Out of range
	})

	if err == nil {
		t.Fatal("expected error for slide index out of range")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestDuplicateSlide_SlideIDNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideID:        "nonexistent-slide",
	})

	if err == nil {
		t.Fatal("expected error for slide ID not found")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestDuplicateSlide_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "nonexistent",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for presentation not found")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestDuplicateSlide_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: forbidden")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "forbidden-presentation",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDuplicateSlide_BatchUpdateFails(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("internal error")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrDuplicateSlideFailed) {
		t.Errorf("expected ErrDuplicateSlideFailed, got %v", err)
	}
}

func TestDuplicateSlide_NoSlideIDInResponse(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{}, // No replies
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for no slide ID in response")
	}

	if !errors.Is(err, ErrDuplicateSlideFailed) {
		t.Errorf("expected ErrDuplicateSlideFailed, got %v", err)
	}
}

func TestDuplicateSlide_ServiceCreationFails(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for service creation failure")
	}

	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestDuplicateSlide_MoveFailure(t *testing.T) {
	// Test that if move fails, we still return the duplicated slide in its default position
	batchCallCount := 0

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			batchCallCount++
			if batchCallCount == 1 && requests[0].DuplicateObject != nil {
				// First call: duplicate succeeds
				return &slides.BatchUpdatePresentationResponse{
					Replies: []*slides.Response{
						{
							DuplicateObject: &slides.DuplicateObjectResponse{
								ObjectId: "slide-1-copy",
							},
						},
					},
				}, nil
			}
			if batchCallCount == 2 && requests[0].UpdateSlidesPosition != nil {
				// Second call: move fails
				return nil, errors.New("move failed")
			}
			return nil, errors.New("unexpected request")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DuplicateSlide(context.Background(), &mockTokenSource{}, DuplicateSlideInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		InsertAt:       4, // Request move to end
	})

	// Should not fail - slide was duplicated
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return the slide in its default position (after source)
	if output.SlideIndex != 2 {
		t.Errorf("expected slide_index 2 (default position after move failure), got %d", output.SlideIndex)
	}

	if output.SlideID != "slide-1-copy" {
		t.Errorf("expected slide_id slide-1-copy, got %s", output.SlideID)
	}

	// Should have been called twice (duplicate + move attempt)
	if batchCallCount != 2 {
		t.Errorf("expected 2 batch update calls, got %d", batchCallCount)
	}
}
