package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestChangeZOrder(t *testing.T) {
	tests := []struct {
		name           string
		input          ChangeZOrderInput
		mockPresentation *slides.Presentation
		mockBatchErr   error
		mockGetErr     error
		wantErr        error
		wantAction     string
		wantNewZOrder  int
	}{
		{
			name: "bring_to_front success",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-2",
				Action:         "bring_to_front",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
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
			},
			wantAction:    "bring_to_front",
			wantNewZOrder: 2, // Moved to front (last position in array after re-fetch simulation)
		},
		{
			name: "send_to_back success",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-2",
				Action:         "send_to_back",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
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
			},
			wantAction:    "send_to_back",
			wantNewZOrder: 0, // Moved to back (first position)
		},
		{
			name: "bring_forward success",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				Action:         "bring_forward",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
						},
					},
				},
			},
			wantAction:    "bring_forward",
			wantNewZOrder: 1, // Moved forward one position
		},
		{
			name: "send_backward success",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-2",
				Action:         "send_backward",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
							{ObjectId: "shape-2"},
						},
					},
				},
			},
			wantAction:    "send_backward",
			wantNewZOrder: 0, // Moved backward one position
		},
		{
			name: "uppercase action accepted",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				Action:         "BRING_TO_FRONT",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
				},
			},
			wantAction:    "bring_to_front",
			wantNewZOrder: 0,
		},
		{
			name: "empty presentation_id",
			input: ChangeZOrderInput{
				PresentationID: "",
				ObjectID:       "shape-1",
				Action:         "bring_to_front",
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "empty object_id",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "",
				Action:         "bring_to_front",
			},
			wantErr: ErrObjectNotFound,
		},
		{
			name: "empty action",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				Action:         "",
			},
			wantErr: ErrInvalidZOrderAction,
		},
		{
			name: "invalid action",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				Action:         "invalid_action",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
				},
			},
			wantErr: ErrInvalidZOrderAction,
		},
		{
			name: "object not found",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "nonexistent",
				Action:         "bring_to_front",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
				},
			},
			wantErr: ErrObjectNotFound,
		},
		{
			name: "object in group not allowed",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "grouped-shape",
				Action:         "bring_to_front",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "grouped-shape"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: ErrObjectInGroup,
		},
		{
			name: "presentation not found",
			input: ChangeZOrderInput{
				PresentationID: "nonexistent",
				ObjectID:       "shape-1",
				Action:         "bring_to_front",
			},
			mockGetErr: errors.New("notFound: presentation not found"),
			wantErr:    ErrPresentationNotFound,
		},
		{
			name: "access denied",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				Action:         "bring_to_front",
			},
			mockGetErr: errors.New("forbidden: access denied"),
			wantErr:    ErrAccessDenied,
		},
		{
			name: "batch update fails",
			input: ChangeZOrderInput{
				PresentationID: "pres-1",
				ObjectID:       "shape-1",
				Action:         "bring_to_front",
			},
			mockPresentation: &slides.Presentation{
				PresentationId: "pres-1",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1"},
						},
					},
				},
			},
			mockBatchErr: errors.New("API error"),
			wantErr:      ErrChangeZOrderFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track BatchUpdate requests
			var capturedRequests []*slides.Request
			batchUpdateCalled := false

			// Setup mock
			mockSlides := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if tt.mockGetErr != nil {
						return nil, tt.mockGetErr
					}
					// For the second call (after BatchUpdate), simulate z-order change
					if batchUpdateCalled && tt.mockPresentation != nil {
						return simulateZOrderChange(tt.mockPresentation, tt.input.ObjectID, tt.input.Action)
					}
					return tt.mockPresentation, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					batchUpdateCalled = true
					capturedRequests = requests
					if tt.mockBatchErr != nil {
						return nil, tt.mockBatchErr
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlides, nil
			})

			output, err := tools.ChangeZOrder(context.Background(), nil, tt.input)

			// Check error
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error containing %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error containing %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check output
			if output.ObjectID != tt.input.ObjectID {
				t.Errorf("output.ObjectID = %s, want %s", output.ObjectID, tt.input.ObjectID)
			}
			if output.Action != tt.wantAction {
				t.Errorf("output.Action = %s, want %s", output.Action, tt.wantAction)
			}
			if output.NewZOrder != tt.wantNewZOrder {
				t.Errorf("output.NewZOrder = %d, want %d", output.NewZOrder, tt.wantNewZOrder)
			}

			// Verify BatchUpdate was called with correct operation
			if len(capturedRequests) != 1 {
				t.Errorf("expected 1 request, got %d", len(capturedRequests))
				return
			}
			req := capturedRequests[0]
			if req.UpdatePageElementsZOrder == nil {
				t.Error("expected UpdatePageElementsZOrder request")
				return
			}
			if len(req.UpdatePageElementsZOrder.PageElementObjectIds) != 1 ||
				req.UpdatePageElementsZOrder.PageElementObjectIds[0] != tt.input.ObjectID {
				t.Errorf("expected object ID %s in request", tt.input.ObjectID)
			}
		})
	}
}

// simulateZOrderChange simulates z-order change by reordering elements in the mock response.
func simulateZOrderChange(pres *slides.Presentation, objectID, action string) (*slides.Presentation, error) {
	// Deep copy to avoid modifying original
	result := &slides.Presentation{
		PresentationId: pres.PresentationId,
		Slides:         make([]*slides.Page, len(pres.Slides)),
	}

	for i, slide := range pres.Slides {
		newSlide := &slides.Page{
			ObjectId:     slide.ObjectId,
			PageElements: make([]*slides.PageElement, len(slide.PageElements)),
		}
		copy(newSlide.PageElements, slide.PageElements)

		// Find and reorder the object
		objectIdx := -1
		for j, elem := range newSlide.PageElements {
			if elem.ObjectId == objectID {
				objectIdx = j
				break
			}
		}

		if objectIdx >= 0 {
			elem := newSlide.PageElements[objectIdx]
			// Remove from current position
			newSlide.PageElements = append(newSlide.PageElements[:objectIdx], newSlide.PageElements[objectIdx+1:]...)

			switch action {
			case "bring_to_front", "BRING_TO_FRONT":
				newSlide.PageElements = append(newSlide.PageElements, elem)
			case "send_to_back", "SEND_TO_BACK":
				newSlide.PageElements = append([]*slides.PageElement{elem}, newSlide.PageElements...)
			case "bring_forward", "BRING_FORWARD":
				// Insert one position forward (if possible)
				newPos := objectIdx + 1
				if newPos > len(newSlide.PageElements) {
					newPos = len(newSlide.PageElements)
				}
				newSlide.PageElements = append(newSlide.PageElements[:newPos], append([]*slides.PageElement{elem}, newSlide.PageElements[newPos:]...)...)
			case "send_backward", "SEND_BACKWARD":
				// Insert one position backward (if possible)
				newPos := objectIdx - 1
				if newPos < 0 {
					newPos = 0
				}
				newSlide.PageElements = append(newSlide.PageElements[:newPos], append([]*slides.PageElement{elem}, newSlide.PageElements[newPos:]...)...)
			}
		}

		result.Slides[i] = newSlide
	}

	return result, nil
}

func TestFindElementAndCheckGroup(t *testing.T) {
	tests := []struct {
		name        string
		elements    []*slides.PageElement
		objectID    string
		wantFound   bool
		wantInGroup bool
	}{
		{
			name: "find at top level",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1"},
				{ObjectId: "shape-2"},
			},
			objectID:    "shape-2",
			wantFound:   true,
			wantInGroup: false,
		},
		{
			name: "find in group",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1"},
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{ObjectId: "grouped-shape"},
						},
					},
				},
			},
			objectID:    "grouped-shape",
			wantFound:   true,
			wantInGroup: true,
		},
		{
			name: "find in nested group",
			elements: []*slides.PageElement{
				{
					ObjectId: "group-1",
					ElementGroup: &slides.Group{
						Children: []*slides.PageElement{
							{
								ObjectId: "group-2",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "deeply-nested"},
									},
								},
							},
						},
					},
				},
			},
			objectID:    "deeply-nested",
			wantFound:   true,
			wantInGroup: true,
		},
		{
			name: "not found",
			elements: []*slides.PageElement{
				{ObjectId: "shape-1"},
			},
			objectID:    "nonexistent",
			wantFound:   false,
			wantInGroup: false,
		},
		{
			name:        "empty elements",
			elements:    []*slides.PageElement{},
			objectID:    "shape-1",
			wantFound:   false,
			wantInGroup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, inGroup := findElementAndCheckGroup(tt.elements, tt.objectID)
			if (found != nil) != tt.wantFound {
				t.Errorf("found = %v, wantFound = %v", found != nil, tt.wantFound)
			}
			if inGroup != tt.wantInGroup {
				t.Errorf("inGroup = %v, wantInGroup = %v", inGroup, tt.wantInGroup)
			}
		})
	}
}
