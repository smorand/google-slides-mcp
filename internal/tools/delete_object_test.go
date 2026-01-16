package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestDeleteObject_SingleObject_Success(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
							{ObjectId: "shape-3"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if presentationID != "test-pres-id" {
				t.Errorf("expected presentation ID 'test-pres-id', got '%s'", presentationID)
			}
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.DeleteObject(context.Background(), tokenSource, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "shape-2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	if capturedRequests[0].DeleteObject == nil {
		t.Fatal("expected DeleteObject request")
	}

	if capturedRequests[0].DeleteObject.ObjectId != "shape-2" {
		t.Errorf("expected object ID 'shape-2', got '%s'", capturedRequests[0].DeleteObject.ObjectId)
	}

	if output.DeletedCount != 1 {
		t.Errorf("expected deleted count 1, got %d", output.DeletedCount)
	}

	if len(output.DeletedIDs) != 1 || output.DeletedIDs[0] != "shape-2" {
		t.Errorf("expected deleted IDs ['shape-2'], got %v", output.DeletedIDs)
	}
}

func TestDeleteObject_MultipleObjects_Success(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
							{ObjectId: "shape-3"},
							{ObjectId: "image-1"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		Multiple:       []string{"shape-1", "shape-3", "image-1"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(capturedRequests))
	}

	for i, req := range capturedRequests {
		if req.DeleteObject == nil {
			t.Errorf("request %d: expected DeleteObject request", i)
		}
	}

	if output.DeletedCount != 3 {
		t.Errorf("expected deleted count 3, got %d", output.DeletedCount)
	}

	if len(output.DeletedIDs) != 3 {
		t.Errorf("expected 3 deleted IDs, got %d", len(output.DeletedIDs))
	}
}

func TestDeleteObject_SingleAndMultiple_Combined(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
							{ObjectId: "shape-3"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Provide both ObjectID and Multiple - should delete all unique IDs
	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "shape-1",
		Multiple:       []string{"shape-2", "shape-3"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.DeletedCount != 3 {
		t.Errorf("expected deleted count 3, got %d", output.DeletedCount)
	}

	// Verify correct number of delete requests were sent
	if len(capturedRequests) != 3 {
		t.Errorf("expected 3 delete requests, got %d", len(capturedRequests))
	}
}

func TestDeleteObject_Deduplication(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Provide duplicates - should deduplicate
	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "shape-1",
		Multiple:       []string{"shape-1", "shape-2", "shape-2", "shape-1"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only send 2 delete requests (deduplicated)
	if len(capturedRequests) != 2 {
		t.Errorf("expected 2 requests (deduplicated), got %d", len(capturedRequests))
	}

	if output.DeletedCount != 2 {
		t.Errorf("expected deleted count 2, got %d", output.DeletedCount)
	}
}

func TestDeleteObject_NestedInGroup(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "nested-shape-1"},
										{ObjectId: "nested-shape-2"},
									},
								},
							},
							{ObjectId: "shape-outside"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Delete a nested object
	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "nested-shape-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	if capturedRequests[0].DeleteObject.ObjectId != "nested-shape-1" {
		t.Errorf("expected to delete 'nested-shape-1', got '%s'", capturedRequests[0].DeleteObject.ObjectId)
	}

	if output.DeletedCount != 1 {
		t.Errorf("expected deleted count 1, got %d", output.DeletedCount)
	}
}

func TestDeleteObject_PartialNotFound(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Try to delete existing and non-existing objects
	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		Multiple:       []string{"shape-1", "nonexistent-1", "shape-2", "nonexistent-2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only delete existing objects
	if len(capturedRequests) != 2 {
		t.Errorf("expected 2 requests (only existing objects), got %d", len(capturedRequests))
	}

	if output.DeletedCount != 2 {
		t.Errorf("expected deleted count 2, got %d", output.DeletedCount)
	}

	if len(output.NotFoundIDs) != 2 {
		t.Errorf("expected 2 not found IDs, got %d", len(output.NotFoundIDs))
	}

	// Check that not found IDs are correct
	notFoundSet := make(map[string]bool)
	for _, id := range output.NotFoundIDs {
		notFoundSet[id] = true
	}
	if !notFoundSet["nonexistent-1"] || !notFoundSet["nonexistent-2"] {
		t.Errorf("expected not found IDs to contain 'nonexistent-1' and 'nonexistent-2', got %v", output.NotFoundIDs)
	}
}

func TestDeleteObject_AllNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
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

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		Multiple:       []string{"nonexistent-1", "nonexistent-2"},
	})

	if err == nil {
		t.Fatal("expected error when all objects not found")
	}

	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestDeleteObject_SingleObjectNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
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

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "nonexistent",
	})

	if err == nil {
		t.Fatal("expected error when object not found")
	}

	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestDeleteObject_MissingPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		ObjectID: "shape-1",
	})

	if err == nil {
		t.Fatal("expected error for missing presentation ID")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestDeleteObject_NoObjectsSpecified(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		// Neither ObjectID nor Multiple provided
	})

	if err == nil {
		t.Fatal("expected error when no objects specified")
	}

	if !errors.Is(err, ErrNoObjectsToDelete) {
		t.Errorf("expected ErrNoObjectsToDelete, got %v", err)
	}
}

func TestDeleteObject_EmptyMultiple(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		Multiple:       []string{}, // Empty array
	})

	if err == nil {
		t.Fatal("expected error when no objects specified")
	}

	if !errors.Is(err, ErrNoObjectsToDelete) {
		t.Errorf("expected ErrNoObjectsToDelete, got %v", err)
	}
}

func TestDeleteObject_EmptyStringsInMultiple(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		Multiple:       []string{"", "", ""}, // Only empty strings
	})

	if err == nil {
		t.Fatal("expected error when only empty strings in multiple")
	}

	if !errors.Is(err, ErrNoObjectsToDelete) {
		t.Errorf("expected ErrNoObjectsToDelete, got %v", err)
	}
}

func TestDeleteObject_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "nonexistent-pres",
		ObjectID:       "shape-1",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestDeleteObject_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: forbidden")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "restricted-pres",
		ObjectID:       "shape-1",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDeleteObject_BatchUpdateFails(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
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

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "shape-1",
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrDeleteObjectFailed) {
		t.Errorf("expected ErrDeleteObjectFailed, got %v", err)
	}
}

func TestDeleteObject_BatchUpdateAccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
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

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "shape-1",
	})

	if err == nil {
		t.Fatal("expected error for batch update permission denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDeleteObject_ServiceCreationFails(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	_, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "shape-1",
	})

	if err == nil {
		t.Fatal("expected error for service creation failure")
	}

	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestDeleteObject_DeleteSlide(t *testing.T) {
	// Test that slides can also be deleted via delete_object
	var capturedRequests []*slides.Request

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
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		ObjectID:       "slide-2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	if capturedRequests[0].DeleteObject.ObjectId != "slide-2" {
		t.Errorf("expected to delete 'slide-2', got '%s'", capturedRequests[0].DeleteObject.ObjectId)
	}

	if output.DeletedCount != 1 {
		t.Errorf("expected deleted count 1, got %d", output.DeletedCount)
	}
}

func TestDeleteObject_ObjectsAcrossMultipleSlides(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1-1"},
							{ObjectId: "shape-1-2"},
						},
					},
					{
						ObjectId: "slide-2",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-2-1"},
							{ObjectId: "shape-2-2"},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)

	// Delete objects from different slides
	output, err := tools.DeleteObject(context.Background(), &mockTokenSource{}, DeleteObjectInput{
		PresentationID: "test-pres-id",
		Multiple:       []string{"shape-1-1", "shape-2-2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(capturedRequests))
	}

	if output.DeletedCount != 2 {
		t.Errorf("expected deleted count 2, got %d", output.DeletedCount)
	}
}

// Test helper functions
func TestCollectObjectIDsToDelete(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)

	tests := []struct {
		name     string
		input    DeleteObjectInput
		expected []string
	}{
		{
			name:     "single object",
			input:    DeleteObjectInput{ObjectID: "shape-1"},
			expected: []string{"shape-1"},
		},
		{
			name:     "multiple objects",
			input:    DeleteObjectInput{Multiple: []string{"a", "b", "c"}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "combined",
			input:    DeleteObjectInput{ObjectID: "single", Multiple: []string{"multi-1", "multi-2"}},
			expected: []string{"single", "multi-1", "multi-2"},
		},
		{
			name:     "with duplicates",
			input:    DeleteObjectInput{ObjectID: "a", Multiple: []string{"a", "b", "a", "c", "b"}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty strings filtered",
			input:    DeleteObjectInput{Multiple: []string{"", "valid", "", ""}},
			expected: []string{"valid"},
		},
		{
			name:     "empty input",
			input:    DeleteObjectInput{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.collectObjectIDsToDelete(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d elements, got %d", len(tt.expected), len(result))
				return
			}

			// For dedup test, order might vary, so check content
			if tt.name == "with duplicates" {
				resultSet := make(map[string]bool)
				for _, id := range result {
					resultSet[id] = true
				}
				for _, expected := range tt.expected {
					if !resultSet[expected] {
						t.Errorf("expected %s in result", expected)
					}
				}
			} else {
				for i, expected := range tt.expected {
					if result[i] != expected {
						t.Errorf("at index %d: expected %s, got %s", i, expected, result[i])
					}
				}
			}
		})
	}
}
