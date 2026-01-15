package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestDeleteSlide_ByIndex_Success(t *testing.T) {
	var capturedObjectID string

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if presentationID != "test-pres-id" {
				t.Errorf("expected presentation ID 'test-pres-id', got '%s'", presentationID)
			}
			if len(requests) != 1 {
				t.Errorf("expected 1 request, got %d", len(requests))
			}
			if requests[0].DeleteObject == nil {
				t.Fatal("expected DeleteObject request")
			}
			capturedObjectID = requests[0].DeleteObject.ObjectId
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DeleteSlide(context.Background(), tokenSource, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     2, // Delete second slide
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedObjectID != "slide-2" {
		t.Errorf("expected to delete 'slide-2', got '%s'", capturedObjectID)
	}

	if output.DeletedSlideID != "slide-2" {
		t.Errorf("expected deleted slide ID 'slide-2', got '%s'", output.DeletedSlideID)
	}

	if output.RemainingSlideCount != 2 {
		t.Errorf("expected remaining slide count 2, got %d", output.RemainingSlideCount)
	}
}

func TestDeleteSlide_ByID_Success(t *testing.T) {
	var capturedObjectID string

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-abc"},
					{ObjectId: "slide-xyz"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedObjectID = requests[0].DeleteObject.ObjectId
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideID:        "slide-xyz",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedObjectID != "slide-xyz" {
		t.Errorf("expected to delete 'slide-xyz', got '%s'", capturedObjectID)
	}

	if output.DeletedSlideID != "slide-xyz" {
		t.Errorf("expected deleted slide ID 'slide-xyz', got '%s'", output.DeletedSlideID)
	}

	if output.RemainingSlideCount != 1 {
		t.Errorf("expected remaining slide count 1, got %d", output.RemainingSlideCount)
	}
}

func TestDeleteSlide_FirstSlide(t *testing.T) {
	var capturedObjectID string

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "first-slide"},
					{ObjectId: "second-slide"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedObjectID = requests[0].DeleteObject.ObjectId
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1, // Delete first slide
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedObjectID != "first-slide" {
		t.Errorf("expected to delete 'first-slide', got '%s'", capturedObjectID)
	}

	if output.DeletedSlideID != "first-slide" {
		t.Errorf("expected deleted slide ID 'first-slide', got '%s'", output.DeletedSlideID)
	}
}

func TestDeleteSlide_LastSlideByIndex(t *testing.T) {
	var capturedObjectID string

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
			capturedObjectID = requests[0].DeleteObject.ObjectId
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     3, // Delete last slide
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedObjectID != "slide-3" {
		t.Errorf("expected to delete 'slide-3', got '%s'", capturedObjectID)
	}

	if output.RemainingSlideCount != 2 {
		t.Errorf("expected remaining slide count 2, got %d", output.RemainingSlideCount)
	}
}

func TestDeleteSlide_CannotDeleteLastRemaining(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "only-slide"},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error when deleting last slide")
	}

	if !errors.Is(err, ErrLastSlideDelete) {
		t.Errorf("expected ErrLastSlideDelete, got %v", err)
	}
}

func TestDeleteSlide_CannotDeleteFromEmptyPresentation(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{}, // Empty
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error when deleting from empty presentation")
	}

	if !errors.Is(err, ErrLastSlideDelete) {
		t.Errorf("expected ErrLastSlideDelete, got %v", err)
	}
}

func TestDeleteSlide_IndexOutOfRange(t *testing.T) {
	tests := []struct {
		name       string
		slideIndex int
		numSlides  int
	}{
		{"index too high", 5, 3},
		{"index zero", 0, 3}, // This is caught by validation as missing reference
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existingSlides := make([]*slides.Page, tt.numSlides)
			for i := 0; i < tt.numSlides; i++ {
				existingSlides[i] = &slides.Page{ObjectId: "slide"}
			}

			mockService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return &slides.Presentation{
						PresentationId: presentationID,
						Slides:         existingSlides,
					}, nil
				},
			}

			factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockService, nil
			}

			tools := NewTools(DefaultToolsConfig(), factory)

			_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
				PresentationID: "test-pres-id",
				SlideIndex:     tt.slideIndex,
			})

			if err == nil {
				t.Fatal("expected error for out of range index")
			}

			// Zero index should be invalid reference, others should be not found
			if tt.slideIndex == 0 {
				if !errors.Is(err, ErrInvalidSlideReference) {
					t.Errorf("expected ErrInvalidSlideReference, got %v", err)
				}
			} else {
				if !errors.Is(err, ErrSlideNotFound) {
					t.Errorf("expected ErrSlideNotFound, got %v", err)
				}
			}
		})
	}
}

func TestDeleteSlide_SlideIDNotFound(t *testing.T) {
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

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideID:        "nonexistent-slide",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent slide ID")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestDeleteSlide_MissingPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		SlideIndex: 1,
	})

	if err == nil {
		t.Fatal("expected error for missing presentation ID")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestDeleteSlide_MissingSlideReference(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		// Neither SlideIndex nor SlideID provided
	})

	if err == nil {
		t.Fatal("expected error for missing slide reference")
	}

	if !errors.Is(err, ErrInvalidSlideReference) {
		t.Errorf("expected ErrInvalidSlideReference, got %v", err)
	}
}

func TestDeleteSlide_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "nonexistent-pres",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for nonexistent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestDeleteSlide_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: forbidden")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "restricted-pres",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDeleteSlide_BatchUpdateFails(t *testing.T) {
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

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrDeleteSlideFailed) {
		t.Errorf("expected ErrDeleteSlideFailed, got %v", err)
	}
}

func TestDeleteSlide_ServiceCreationFails(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for service creation failure")
	}

	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestDeleteSlide_BatchUpdateAccessDenied(t *testing.T) {
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

	_, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1,
	})

	if err == nil {
		t.Fatal("expected error for batch update permission denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDeleteSlide_SlideIDTakesPrecedence(t *testing.T) {
	var capturedObjectID string

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
			capturedObjectID = requests[0].DeleteObject.ObjectId
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Provide both SlideID and SlideIndex - SlideID should take precedence
	output, err := tools.DeleteSlide(context.Background(), &mockTokenSource{}, DeleteSlideInput{
		PresentationID: "test-pres-id",
		SlideIndex:     1,        // Would delete slide-1
		SlideID:        "slide-3", // Should delete slide-3 instead
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedObjectID != "slide-3" {
		t.Errorf("expected SlideID to take precedence and delete 'slide-3', got '%s'", capturedObjectID)
	}

	if output.DeletedSlideID != "slide-3" {
		t.Errorf("expected deleted slide ID 'slide-3', got '%s'", output.DeletedSlideID)
	}
}
