package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestCreateShape(t *testing.T) {
	// Save original time function and restore after test
	originalTimeFunc := shapeTimeNowFunc
	shapeTimeNowFunc = func() time.Time {
		return time.Unix(1234567890, 123456789)
	}
	defer func() {
		shapeTimeNowFunc = originalTimeFunc
	}()

	tests := []struct {
		name           string
		input          CreateShapeInput
		mockService    func() *mockSlidesService
		wantErr        error
		wantErrContain string
		wantObjectID   bool
	}{
		{
			name: "creates rectangle at specified position with slide_index",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 100, Y: 50},
				Size:           &SizeInput{Width: 300, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) < 1 {
							t.Error("expected at least 1 request (CreateShape)")
						}
						if requests[0].CreateShape == nil {
							t.Error("first request should be CreateShape")
						}
						if requests[0].CreateShape.ShapeType != "RECTANGLE" {
							t.Errorf("expected RECTANGLE, got %s", requests[0].CreateShape.ShapeType)
						}
						if requests[0].CreateShape.ElementProperties.PageObjectId != "slide-1" {
							t.Errorf("expected page object id 'slide-1', got %s", requests[0].CreateShape.ElementProperties.PageObjectId)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates ellipse with slide_id",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideID:        "custom-slide",
				ShapeType:      "ELLIPSE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 200, Height: 200},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "custom-slide"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if requests[0].CreateShape.ShapeType != "ELLIPSE" {
							t.Errorf("expected ELLIPSE, got %s", requests[0].CreateShape.ShapeType)
						}
						if requests[0].CreateShape.ElementProperties.PageObjectId != "custom-slide" {
							t.Errorf("expected page object id 'custom-slide', got %s", requests[0].CreateShape.ElementProperties.PageObjectId)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates shape with fill color",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "STAR_5",
				Position:       &PositionInput{X: 100, Y: 100},
				Size:           &SizeInput{Width: 150, Height: 150},
				FillColor:      "#FF0000",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) != 2 {
							t.Errorf("expected 2 requests, got %d", len(requests))
						}
						if requests[1].UpdateShapeProperties == nil {
							t.Error("second request should be UpdateShapeProperties")
						} else {
							props := requests[1].UpdateShapeProperties.ShapeProperties
							if props.ShapeBackgroundFill == nil {
								t.Error("expected ShapeBackgroundFill to be set")
							} else if props.ShapeBackgroundFill.SolidFill == nil {
								t.Error("expected SolidFill to be set")
							} else if props.ShapeBackgroundFill.SolidFill.Color.RgbColor.Red != 1.0 {
								t.Errorf("expected red=1.0, got %f", props.ShapeBackgroundFill.SolidFill.Color.RgbColor.Red)
							}
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates shape with transparent fill",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "DIAMOND",
				Position:       &PositionInput{X: 50, Y: 50},
				Size:           &SizeInput{Width: 100, Height: 100},
				FillColor:      "transparent",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) != 2 {
							t.Errorf("expected 2 requests, got %d", len(requests))
						}
						if requests[1].UpdateShapeProperties == nil {
							t.Error("second request should be UpdateShapeProperties")
						} else {
							props := requests[1].UpdateShapeProperties.ShapeProperties
							if props.ShapeBackgroundFill == nil {
								t.Error("expected ShapeBackgroundFill to be set")
							} else if props.ShapeBackgroundFill.PropertyState != "NOT_RENDERED" {
								t.Errorf("expected PropertyState NOT_RENDERED, got %s", props.ShapeBackgroundFill.PropertyState)
							}
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates shape with outline color and weight",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "TRIANGLE",
				Position:       &PositionInput{X: 200, Y: 100},
				Size:           &SizeInput{Width: 120, Height: 100},
				OutlineColor:   "#0000FF",
				OutlineWeight:  ptrFloat64(3.0),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) != 2 {
							t.Errorf("expected 2 requests, got %d", len(requests))
						}
						if requests[1].UpdateShapeProperties == nil {
							t.Error("second request should be UpdateShapeProperties")
						} else {
							props := requests[1].UpdateShapeProperties.ShapeProperties
							if props.Outline == nil {
								t.Error("expected Outline to be set")
							} else {
								if props.Outline.OutlineFill == nil || props.Outline.OutlineFill.SolidFill == nil {
									t.Error("expected OutlineFill.SolidFill to be set")
								} else if props.Outline.OutlineFill.SolidFill.Color.RgbColor.Blue != 1.0 {
									t.Errorf("expected blue=1.0, got %f", props.Outline.OutlineFill.SolidFill.Color.RgbColor.Blue)
								}
								if props.Outline.Weight == nil {
									t.Error("expected Weight to be set")
								} else if props.Outline.Weight.Magnitude != 3.0 {
									t.Errorf("expected weight 3.0, got %f", props.Outline.Weight.Magnitude)
								}
							}
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates shape with transparent outline",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "PENTAGON",
				Position:       &PositionInput{X: 100, Y: 100},
				Size:           &SizeInput{Width: 100, Height: 100},
				OutlineColor:   "transparent",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) != 2 {
							t.Errorf("expected 2 requests, got %d", len(requests))
						}
						if requests[1].UpdateShapeProperties != nil {
							props := requests[1].UpdateShapeProperties.ShapeProperties
							if props.Outline == nil {
								t.Error("expected Outline to be set")
							} else if props.Outline.PropertyState != "NOT_RENDERED" {
								t.Errorf("expected PropertyState NOT_RENDERED, got %s", props.Outline.PropertyState)
							}
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates shape with fill and outline",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "HEXAGON",
				Position:       &PositionInput{X: 50, Y: 50},
				Size:           &SizeInput{Width: 200, Height: 200},
				FillColor:      "#00FF00",
				OutlineColor:   "#FF0000",
				OutlineWeight:  ptrFloat64(2.0),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) != 2 {
							t.Errorf("expected 2 requests, got %d", len(requests))
						}
						if requests[1].UpdateShapeProperties != nil {
							props := requests[1].UpdateShapeProperties.ShapeProperties
							// Check fill
							if props.ShapeBackgroundFill == nil || props.ShapeBackgroundFill.SolidFill == nil {
								t.Error("expected ShapeBackgroundFill.SolidFill to be set")
							} else if props.ShapeBackgroundFill.SolidFill.Color.RgbColor.Green != 1.0 {
								t.Errorf("expected fill green=1.0, got %f", props.ShapeBackgroundFill.SolidFill.Color.RgbColor.Green)
							}
							// Check outline
							if props.Outline == nil {
								t.Error("expected Outline to be set")
							} else if props.Outline.OutlineFill == nil {
								t.Error("expected OutlineFill to be set")
							}
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "converts points to EMU correctly",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "ROUND_RECTANGLE",
				Position:       &PositionInput{X: 100, Y: 50},
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						shape := requests[0].CreateShape
						expectedWidth := 200 * 12700.0
						expectedHeight := 100 * 12700.0
						expectedX := 100 * 12700.0
						expectedY := 50 * 12700.0

						if shape.ElementProperties.Size.Width.Magnitude != expectedWidth {
							t.Errorf("expected width %f EMU, got %f", expectedWidth, shape.ElementProperties.Size.Width.Magnitude)
						}
						if shape.ElementProperties.Size.Height.Magnitude != expectedHeight {
							t.Errorf("expected height %f EMU, got %f", expectedHeight, shape.ElementProperties.Size.Height.Magnitude)
						}
						if shape.ElementProperties.Transform.TranslateX != expectedX {
							t.Errorf("expected X %f EMU, got %f", expectedX, shape.ElementProperties.Transform.TranslateX)
						}
						if shape.ElementProperties.Transform.TranslateY != expectedY {
							t.Errorf("expected Y %f EMU, got %f", expectedY, shape.ElementProperties.Transform.TranslateY)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "defaults position to 0,0 when not provided",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "CHEVRON",
				Size:           &SizeInput{Width: 100, Height: 50},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						shape := requests[0].CreateShape
						if shape.ElementProperties.Transform.TranslateX != 0 {
							t.Errorf("expected X 0, got %f", shape.ElementProperties.Transform.TranslateX)
						}
						if shape.ElementProperties.Transform.TranslateY != 0 {
							t.Errorf("expected Y 0, got %f", shape.ElementProperties.Transform.TranslateY)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "normalizes shape type to uppercase",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "arrow_right",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 50},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if requests[0].CreateShape.ShapeType != "ARROW_RIGHT" {
							t.Errorf("expected ARROW_RIGHT, got %s", requests[0].CreateShape.ShapeType)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates various shape types",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "FLOWCHART_DECISION",
				Position:       &PositionInput{X: 100, Y: 100},
				Size:           &SizeInput{Width: 150, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if requests[0].CreateShape.ShapeType != "FLOWCHART_DECISION" {
							t.Errorf("expected FLOWCHART_DECISION, got %s", requests[0].CreateShape.ShapeType)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		// Error cases
		{
			name: "returns error for empty presentation_id",
			input: CreateShapeInput{
				SlideIndex: 1,
				ShapeType:  "RECTANGLE",
				Position:   &PositionInput{X: 0, Y: 0},
				Size:       &SizeInput{Width: 100, Height: 100},
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "returns error when neither slide_index nor slide_id provided",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			wantErr: ErrInvalidSlideReference,
		},
		{
			name: "returns error for empty shape_type",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			wantErr: ErrInvalidShapeType,
		},
		{
			name: "returns error for invalid shape_type",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "INVALID_SHAPE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			wantErr:        ErrInvalidShapeType,
			wantErrContain: "INVALID_SHAPE",
		},
		{
			name: "returns error when size is missing",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error for zero width",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 0, Height: 100},
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error for negative height",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: -50},
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error for zero outline weight",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
				OutlineWeight:  ptrFloat64(0),
			},
			wantErr: ErrInvalidOutlineWeight,
		},
		{
			name: "returns error for negative outline weight",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
				OutlineWeight:  ptrFloat64(-1.5),
			},
			wantErr: ErrInvalidOutlineWeight,
		},
		{
			name: "returns error for slide not found",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     5,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
				}
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "returns error when presentation not found",
			input: CreateShapeInput{
				PresentationID: "non-existent",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("googleapi: Error 404: File not found")
					},
				}
			},
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "returns error when access denied",
			input: CreateShapeInput{
				PresentationID: "forbidden",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("googleapi: Error 403: Forbidden")
					},
				}
			},
			wantErr: ErrAccessDenied,
		},
		{
			name: "returns error when batch update fails",
			input: CreateShapeInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				ShapeType:      "RECTANGLE",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 100, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("API error")
					},
				}
			},
			wantErr: ErrCreateShapeFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var slidesFactory SlidesServiceFactory
			if tt.mockService != nil {
				mock := tt.mockService()
				slidesFactory = func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
					return mock, nil
				}
			}

			tools := NewTools(DefaultToolsConfig(), slidesFactory)
			output, err := tools.CreateShape(context.Background(), nil, tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				if tt.wantErrContain != "" && !containsString(err.Error(), tt.wantErrContain) {
					t.Errorf("expected error to contain '%s', got '%s'", tt.wantErrContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantObjectID && output.ObjectID == "" {
				t.Error("expected object_id to be set")
			}

			if tt.wantObjectID && output.ObjectID != "" {
				expectedID := "shape_1234567890123456789"
				if output.ObjectID != expectedID {
					t.Errorf("expected object_id %s, got %s", expectedID, output.ObjectID)
				}
			}
		})
	}
}

func TestBuildCreateShapeRequests(t *testing.T) {
	tests := []struct {
		name           string
		objectID       string
		slideID        string
		shapeType      string
		input          CreateShapeInput
		wantNumRequests int
		verifyRequests func(t *testing.T, requests []*slides.Request)
	}{
		{
			name:      "basic shape without styling",
			objectID:  "shape-1",
			slideID:   "slide-1",
			shapeType: "RECTANGLE",
			input: CreateShapeInput{
				Position: &PositionInput{X: 100, Y: 50},
				Size:     &SizeInput{Width: 200, Height: 100},
			},
			wantNumRequests: 1,
			verifyRequests: func(t *testing.T, requests []*slides.Request) {
				req := requests[0].CreateShape
				if req.ObjectId != "shape-1" {
					t.Errorf("expected object id 'shape-1', got '%s'", req.ObjectId)
				}
				if req.ShapeType != "RECTANGLE" {
					t.Errorf("expected shape type 'RECTANGLE', got '%s'", req.ShapeType)
				}
				if req.ElementProperties.PageObjectId != "slide-1" {
					t.Errorf("expected page object id 'slide-1', got '%s'", req.ElementProperties.PageObjectId)
				}
			},
		},
		{
			name:      "shape with fill color",
			objectID:  "shape-2",
			slideID:   "slide-1",
			shapeType: "ELLIPSE",
			input: CreateShapeInput{
				Position:  &PositionInput{X: 0, Y: 0},
				Size:      &SizeInput{Width: 100, Height: 100},
				FillColor: "#FF0000",
			},
			wantNumRequests: 2,
			verifyRequests: func(t *testing.T, requests []*slides.Request) {
				if requests[1].UpdateShapeProperties == nil {
					t.Error("expected UpdateShapeProperties request")
				}
			},
		},
		{
			name:      "shape with outline only",
			objectID:  "shape-3",
			slideID:   "slide-1",
			shapeType: "TRIANGLE",
			input: CreateShapeInput{
				Position:     &PositionInput{X: 0, Y: 0},
				Size:         &SizeInput{Width: 100, Height: 100},
				OutlineColor: "#0000FF",
				OutlineWeight: ptrFloat64(2.0),
			},
			wantNumRequests: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildCreateShapeRequests(tt.objectID, tt.slideID, tt.shapeType, tt.input)

			if len(requests) != tt.wantNumRequests {
				t.Errorf("expected %d requests, got %d", tt.wantNumRequests, len(requests))
			}

			if tt.verifyRequests != nil {
				tt.verifyRequests(t, requests)
			}
		})
	}
}

func TestBuildShapePropertiesRequest(t *testing.T) {
	tests := []struct {
		name         string
		objectID     string
		input        CreateShapeInput
		wantNil      bool
		verifyRequest func(t *testing.T, req *slides.Request)
	}{
		{
			name:     "returns nil when no styling provided",
			objectID: "shape-1",
			input: CreateShapeInput{
				Position: &PositionInput{X: 0, Y: 0},
				Size:     &SizeInput{Width: 100, Height: 100},
			},
			wantNil: true,
		},
		{
			name:     "builds fill color request",
			objectID: "shape-2",
			input: CreateShapeInput{
				Position:  &PositionInput{X: 0, Y: 0},
				Size:      &SizeInput{Width: 100, Height: 100},
				FillColor: "#00FF00",
			},
			verifyRequest: func(t *testing.T, req *slides.Request) {
				if req.UpdateShapeProperties.ObjectId != "shape-2" {
					t.Errorf("expected object id 'shape-2', got '%s'", req.UpdateShapeProperties.ObjectId)
				}
				props := req.UpdateShapeProperties.ShapeProperties
				if props.ShapeBackgroundFill == nil {
					t.Error("expected ShapeBackgroundFill to be set")
				}
				if props.ShapeBackgroundFill.SolidFill.Color.RgbColor.Green != 1.0 {
					t.Errorf("expected green=1.0, got %f", props.ShapeBackgroundFill.SolidFill.Color.RgbColor.Green)
				}
			},
		},
		{
			name:     "builds transparent fill request",
			objectID: "shape-3",
			input: CreateShapeInput{
				Position:  &PositionInput{X: 0, Y: 0},
				Size:      &SizeInput{Width: 100, Height: 100},
				FillColor: "transparent",
			},
			verifyRequest: func(t *testing.T, req *slides.Request) {
				props := req.UpdateShapeProperties.ShapeProperties
				if props.ShapeBackgroundFill.PropertyState != "NOT_RENDERED" {
					t.Errorf("expected PropertyState NOT_RENDERED, got %s", props.ShapeBackgroundFill.PropertyState)
				}
			},
		},
		{
			name:     "builds outline request",
			objectID: "shape-4",
			input: CreateShapeInput{
				Position:      &PositionInput{X: 0, Y: 0},
				Size:          &SizeInput{Width: 100, Height: 100},
				OutlineColor:  "#FF0000",
				OutlineWeight: ptrFloat64(1.5),
			},
			verifyRequest: func(t *testing.T, req *slides.Request) {
				props := req.UpdateShapeProperties.ShapeProperties
				if props.Outline == nil {
					t.Error("expected Outline to be set")
				}
				if props.Outline.Weight.Magnitude != 1.5 {
					t.Errorf("expected weight 1.5, got %f", props.Outline.Weight.Magnitude)
				}
				if props.Outline.OutlineFill.SolidFill.Color.RgbColor.Red != 1.0 {
					t.Errorf("expected red=1.0, got %f", props.Outline.OutlineFill.SolidFill.Color.RgbColor.Red)
				}
			},
		},
		{
			name:     "ignores invalid fill color",
			objectID: "shape-5",
			input: CreateShapeInput{
				Position:  &PositionInput{X: 0, Y: 0},
				Size:      &SizeInput{Width: 100, Height: 100},
				FillColor: "invalid",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := buildShapePropertiesRequest(tt.objectID, tt.input)

			if tt.wantNil {
				if req != nil {
					t.Error("expected nil request")
				}
				return
			}

			if req == nil {
				t.Error("expected non-nil request")
				return
			}

			if tt.verifyRequest != nil {
				tt.verifyRequest(t, req)
			}
		})
	}
}

func TestGenerateShapeObjectID(t *testing.T) {
	originalFunc := shapeTimeNowFunc
	defer func() {
		shapeTimeNowFunc = originalFunc
	}()

	shapeTimeNowFunc = func() time.Time {
		return time.Unix(1234567890, 987654321)
	}

	objectID := generateShapeObjectID()
	expected := "shape_1234567890987654321"

	if objectID != expected {
		t.Errorf("expected object ID %s, got %s", expected, objectID)
	}
}

func TestValidShapeTypes(t *testing.T) {
	// Test that all common shape types are valid
	commonShapes := []string{
		"RECTANGLE", "ROUND_RECTANGLE", "ELLIPSE", "TRIANGLE", "DIAMOND",
		"PENTAGON", "HEXAGON", "STAR_5", "STAR_4", "ARROW_RIGHT", "ARROW_LEFT",
		"ARROW_UP", "ARROW_DOWN", "CHEVRON", "HEART", "CLOUD", "CUBE",
		"FLOWCHART_PROCESS", "FLOWCHART_DECISION", "PLUS", "MINUS", "EQUAL",
	}

	for _, shape := range commonShapes {
		if !validShapeTypes[shape] {
			t.Errorf("expected shape type '%s' to be valid", shape)
		}
	}

	// Test invalid shapes
	invalidShapes := []string{"INVALID", "NOT_A_SHAPE", "CIRCLE", "SQUARE"}
	for _, shape := range invalidShapes {
		if validShapeTypes[shape] {
			t.Errorf("expected shape type '%s' to be invalid", shape)
		}
	}
}

// Note: ptrFloat64 and containsString are already defined in other test files in this package
