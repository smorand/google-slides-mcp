package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestReorderSlides_SingleSlide_ByIndex(t *testing.T) {
	var capturedSlideIDs []string
	var capturedInsertionIndex int64

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
					{ObjectId: "slide-4"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) != 1 || requests[0].UpdateSlidesPosition == nil {
				t.Fatal("expected UpdateSlidesPosition request")
			}
			capturedSlideIDs = requests[0].UpdateSlidesPosition.SlideObjectIds
			capturedInsertionIndex = requests[0].UpdateSlidesPosition.InsertionIndex
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	// Keep track of call count to return reordered state on second call
	callCount := 0
	mockService.GetPresentationFunc = func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
		callCount++
		if callCount == 1 {
			// Initial state
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
					{ObjectId: "slide-4"},
				},
			}, nil
		}
		// After reorder: slide-3 moved to position 1
		return &slides.Presentation{
			PresentationId: presentationID,
			Slides: []*slides.Page{
				{ObjectId: "slide-3"},
				{ObjectId: "slide-1"},
				{ObjectId: "slide-2"},
				{ObjectId: "slide-4"},
			},
		}, nil
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{3}, // Move slide 3
		InsertAt:       1,        // To position 1
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedSlideIDs) != 1 || capturedSlideIDs[0] != "slide-3" {
		t.Errorf("expected to move slide-3, got %v", capturedSlideIDs)
	}

	if capturedInsertionIndex != 0 { // insert_at 1 becomes index 0
		t.Errorf("expected insertion index 0, got %d", capturedInsertionIndex)
	}

	if len(output.NewOrder) != 4 {
		t.Errorf("expected 4 slides in new order, got %d", len(output.NewOrder))
	}

	// Verify new order
	if output.NewOrder[0].SlideID != "slide-3" {
		t.Errorf("expected slide-3 at position 1, got %s", output.NewOrder[0].SlideID)
	}
}

func TestReorderSlides_SingleSlide_ByID(t *testing.T) {
	var capturedSlideIDs []string

	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
			if callCount == 1 {
				return &slides.Presentation{
					PresentationId: presentationID,
					Slides: []*slides.Page{
						{ObjectId: "slide-a"},
						{ObjectId: "slide-b"},
						{ObjectId: "slide-c"},
					},
				}, nil
			}
			// After reorder: slide-c moved to position 2
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-a"},
					{ObjectId: "slide-c"},
					{ObjectId: "slide-b"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedSlideIDs = requests[0].UpdateSlidesPosition.SlideObjectIds
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIDs:       []string{"slide-c"}, // Move slide-c
		InsertAt:       2,                   // To position 2
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedSlideIDs) != 1 || capturedSlideIDs[0] != "slide-c" {
		t.Errorf("expected to move slide-c, got %v", capturedSlideIDs)
	}

	if len(output.NewOrder) != 3 {
		t.Errorf("expected 3 slides in new order, got %d", len(output.NewOrder))
	}
}

func TestReorderSlides_MultipleSlides_ByIndex(t *testing.T) {
	var capturedSlideIDs []string

	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
			if callCount == 1 {
				return &slides.Presentation{
					PresentationId: presentationID,
					Slides: []*slides.Page{
						{ObjectId: "slide-1"},
						{ObjectId: "slide-2"},
						{ObjectId: "slide-3"},
						{ObjectId: "slide-4"},
						{ObjectId: "slide-5"},
					},
				}, nil
			}
			// After reorder: slides 2 and 4 moved to position 5
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-3"},
					{ObjectId: "slide-5"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-4"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedSlideIDs = requests[0].UpdateSlidesPosition.SlideObjectIds
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{2, 4}, // Move slides 2 and 4
		InsertAt:       5,           // To the end
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedSlideIDs) != 2 {
		t.Errorf("expected to move 2 slides, got %d", len(capturedSlideIDs))
	}

	if capturedSlideIDs[0] != "slide-2" || capturedSlideIDs[1] != "slide-4" {
		t.Errorf("expected [slide-2, slide-4], got %v", capturedSlideIDs)
	}

	if len(output.NewOrder) != 5 {
		t.Errorf("expected 5 slides in new order, got %d", len(output.NewOrder))
	}
}

func TestReorderSlides_MultipleSlides_ByID(t *testing.T) {
	var capturedSlideIDs []string

	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-x"},
					{ObjectId: "slide-y"},
					{ObjectId: "slide-z"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedSlideIDs = requests[0].UpdateSlidesPosition.SlideObjectIds
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIDs:       []string{"slide-y", "slide-z"},
		InsertAt:       1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedSlideIDs) != 2 {
		t.Errorf("expected 2 slides, got %d", len(capturedSlideIDs))
	}

	if capturedSlideIDs[0] != "slide-y" || capturedSlideIDs[1] != "slide-z" {
		t.Errorf("expected [slide-y, slide-z], got %v", capturedSlideIDs)
	}
}

func TestReorderSlides_MoveToSamePosition(t *testing.T) {
	// Moving a slide to its current position should still succeed
	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
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
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{2}, // Move slide 2
		InsertAt:       2,        // To position 2 (same position)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.NewOrder) != 3 {
		t.Errorf("expected 3 slides, got %d", len(output.NewOrder))
	}
}

func TestReorderSlides_InsertAtBeyondEnd(t *testing.T) {
	// insert_at beyond the number of slides should clamp to end
	var capturedInsertionIndex int64

	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
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
			capturedInsertionIndex = requests[0].UpdateSlidesPosition.InsertionIndex
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       100, // Way beyond the 3 slides
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should clamp to index 2 (position 3, which is the last position)
	if capturedInsertionIndex != 2 {
		t.Errorf("expected insertion index to be clamped to 2, got %d", capturedInsertionIndex)
	}
}

func TestReorderSlides_MissingPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		SlideIndices: []int{1},
		InsertAt:     2,
	})

	if err == nil {
		t.Fatal("expected error for missing presentation ID")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestReorderSlides_MissingSlidesToMove(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		InsertAt:       2,
		// Neither SlideIndices nor SlideIDs provided
	})

	if err == nil {
		t.Fatal("expected error for missing slides to move")
	}

	if !errors.Is(err, ErrNoSlidesToMove) {
		t.Errorf("expected ErrNoSlidesToMove, got %v", err)
	}
}

func TestReorderSlides_InvalidInsertAt_Zero(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       0, // Invalid
	})

	if err == nil {
		t.Fatal("expected error for insert_at = 0")
	}

	if !errors.Is(err, ErrInvalidInsertAt) {
		t.Errorf("expected ErrInvalidInsertAt, got %v", err)
	}
}

func TestReorderSlides_InvalidInsertAt_Negative(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       -5, // Invalid
	})

	if err == nil {
		t.Fatal("expected error for negative insert_at")
	}

	if !errors.Is(err, ErrInvalidInsertAt) {
		t.Errorf("expected ErrInvalidInsertAt, got %v", err)
	}
}

func TestReorderSlides_SlideIndexOutOfRange(t *testing.T) {
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

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{5}, // Out of range (only 3 slides)
		InsertAt:       1,
	})

	if err == nil {
		t.Fatal("expected error for out of range slide index")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestReorderSlides_SlideIDNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIDs:       []string{"nonexistent-slide"},
		InsertAt:       1,
	})

	if err == nil {
		t.Fatal("expected error for nonexistent slide ID")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestReorderSlides_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "nonexistent-pres",
		SlideIndices:   []int{1},
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for nonexistent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestReorderSlides_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: forbidden")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "restricted-pres",
		SlideIndices:   []int{1},
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestReorderSlides_BatchUpdateFails(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("batch update failed")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrReorderSlidesFailed) {
		t.Errorf("expected ErrReorderSlidesFailed, got %v", err)
	}
}

func TestReorderSlides_BatchUpdateAccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("googleapi: Error 403: permission denied")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for batch update permission denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestReorderSlides_ServiceCreationFails(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for service creation failure")
	}

	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestReorderSlides_FetchAfterReorderFails(t *testing.T) {
	// When fetching the updated presentation fails, we should still return success
	// but with an empty new order

	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
			if callCount == 1 {
				return &slides.Presentation{
					PresentationId: presentationID,
					Slides: []*slides.Page{
						{ObjectId: "slide-1"},
						{ObjectId: "slide-2"},
					},
				}, nil
			}
			// Second call (to get updated state) fails
			return nil, errors.New("network error")
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},
		InsertAt:       2,
	})

	// Should not fail - the reorder succeeded
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// But new order should be empty since we couldn't fetch updated state
	if len(output.NewOrder) != 0 {
		t.Errorf("expected empty new order when fetch fails, got %d items", len(output.NewOrder))
	}
}

func TestReorderSlides_NewOrderOutput(t *testing.T) {
	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
			if callCount == 1 {
				return &slides.Presentation{
					PresentationId: presentationID,
					Slides: []*slides.Page{
						{ObjectId: "slide-a"},
						{ObjectId: "slide-b"},
						{ObjectId: "slide-c"},
					},
				}, nil
			}
			// After reorder
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-c"},
					{ObjectId: "slide-a"},
					{ObjectId: "slide-b"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIDs:       []string{"slide-c"},
		InsertAt:       1,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify new order structure
	expected := []struct {
		index   int
		slideID string
	}{
		{1, "slide-c"},
		{2, "slide-a"},
		{3, "slide-b"},
	}

	for i, exp := range expected {
		if output.NewOrder[i].Index != exp.index {
			t.Errorf("slide %d: expected index %d, got %d", i, exp.index, output.NewOrder[i].Index)
		}
		if output.NewOrder[i].SlideID != exp.slideID {
			t.Errorf("slide %d: expected ID %s, got %s", i, exp.slideID, output.NewOrder[i].SlideID)
		}
	}
}

func TestReorderSlides_EmptySlideIndicesArray(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{}, // Empty array
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for empty slide indices array")
	}

	if !errors.Is(err, ErrNoSlidesToMove) {
		t.Errorf("expected ErrNoSlidesToMove, got %v", err)
	}
}

func TestReorderSlides_EmptySlideIDsArray(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIDs:       []string{}, // Empty array
		InsertAt:       2,
	})

	if err == nil {
		t.Fatal("expected error for empty slide IDs array")
	}

	if !errors.Is(err, ErrNoSlidesToMove) {
		t.Errorf("expected ErrNoSlidesToMove, got %v", err)
	}
}

func TestReorderSlides_SlideIDsTakePrecedence(t *testing.T) {
	// When both SlideIndices and SlideIDs are provided, SlideIDs should be used
	var capturedSlideIDs []string

	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			callCount++
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
			capturedSlideIDs = requests[0].UpdateSlidesPosition.SlideObjectIds
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.ReorderSlides(context.Background(), &mockTokenSource{}, ReorderSlidesInput{
		PresentationID: "test-pres-id",
		SlideIndices:   []int{1},                // Would use slide-1
		SlideIDs:       []string{"slide-3"},     // Should use slide-3 instead
		InsertAt:       2,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedSlideIDs) != 1 || capturedSlideIDs[0] != "slide-3" {
		t.Errorf("expected SlideIDs to take precedence and use slide-3, got %v", capturedSlideIDs)
	}
}
