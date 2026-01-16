package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestTransformObject(t *testing.T) {
	// Setup common variables
	ctx := context.Background()
	presentationID := "test-presentation-id"
	objectID := "shape-id-1"
	slideID := "slide-id-1"

	// Helper to create a basic shape element
	createBaseElement := func() *slides.PageElement {
		return &slides.PageElement{
			ObjectId: objectID,
			Size: &slides.Size{
				Width:  &slides.Dimension{Magnitude: pointsToEMU(100), Unit: "EMU"},
				Height: &slides.Dimension{Magnitude: pointsToEMU(50), Unit: "EMU"},
			},
			Transform: &slides.AffineTransform{
				ScaleX:     1,
				ScaleY:     1,
				TranslateX: pointsToEMU(10),
				TranslateY: pointsToEMU(20),
				Unit:       "EMU",
			},
		}
	}

	tests := []struct {
		name           string
		input          TransformObjectInput
		setupElement   func() *slides.PageElement
		setupMocks     func(*mockSlidesService)
		expectedErr    error
		validateReqs   func(*testing.T, []*slides.Request)
		expectedOutput func(*testing.T, *TransformObjectOutput)
	}{
		{
			name: "Success - Move Only",
			input: TransformObjectInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Position:       &PositionInput{X: 50, Y: 60},
			},
			setupElement: createBaseElement,
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				transform := reqs[0].UpdatePageElementTransform
				require.NotNil(t, transform)
				assert.Equal(t, "ABSOLUTE", transform.ApplyMode)
				assert.Equal(t, pointsToEMU(50), transform.Transform.TranslateX)
				assert.Equal(t, pointsToEMU(60), transform.Transform.TranslateY)
				// Scale should remain 1
				assert.Equal(t, 1.0, transform.Transform.ScaleX)
				assert.Equal(t, 1.0, transform.Transform.ScaleY)
			},
			expectedOutput: func(t *testing.T, out *TransformObjectOutput) {
				assert.Equal(t, 50.0, out.Position.X)
				assert.Equal(t, 60.0, out.Position.Y)
				assert.Equal(t, 100.0, out.Size.Width)
				assert.Equal(t, 50.0, out.Size.Height)
			},
		},
		{
			name: "Success - Resize Width Only (Proportional)",
			input: TransformObjectInput{
				PresentationID:      presentationID,
				ObjectID:            objectID,
				Size:                &SizeInput{Width: 200}, // Double width
				ScaleProportionally: true,
			},
			setupElement: createBaseElement,
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				transform := reqs[0].UpdatePageElementTransform.Transform
				// Scale should be 2.0 for both X and Y
				assert.InDelta(t, 2.0, transform.ScaleX, 0.001)
				assert.InDelta(t, 2.0, transform.ScaleY, 0.001)
			},
			expectedOutput: func(t *testing.T, out *TransformObjectOutput) {
				assert.Equal(t, 200.0, out.Size.Width)
				assert.Equal(t, 100.0, out.Size.Height) // Height doubled too
			},
		},
		{
			name: "Success - Resize Width Only (Non-Proportional)",
			input: TransformObjectInput{
				PresentationID:      presentationID,
				ObjectID:            objectID,
				Size:                &SizeInput{Width: 200},
				ScaleProportionally: false,
			},
			setupElement: createBaseElement,
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				transform := reqs[0].UpdatePageElementTransform.Transform
				assert.InDelta(t, 2.0, transform.ScaleX, 0.001)
				assert.InDelta(t, 1.0, transform.ScaleY, 0.001) // Y remains 1.0
			},
			expectedOutput: func(t *testing.T, out *TransformObjectOutput) {
				assert.Equal(t, 200.0, out.Size.Width)
				assert.Equal(t, 50.0, out.Size.Height)
			},
		},
		{
			name: "Success - Rotate",
			input: TransformObjectInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Rotation:       float64PtrTransform(90),
			},
			setupElement: createBaseElement,
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				transform := reqs[0].UpdatePageElementTransform.Transform
				// 90 degrees rotation matrix:
				// cos(90)=0, sin(90)=1
				// ScaleX = 1*0 = 0
				// ShearY = 1*1 = 1
				// ShearX = -1*1 = -1
				// ScaleY = 1*0 = 0
				assert.InDelta(t, 0.0, transform.ScaleX, 0.001)
				assert.InDelta(t, 1.0, transform.ShearY, 0.001)
				assert.InDelta(t, -1.0, transform.ShearX, 0.001)
				assert.InDelta(t, 0.0, transform.ScaleY, 0.001)
			},
			expectedOutput: func(t *testing.T, out *TransformObjectOutput) {
				assert.InDelta(t, 90.0, out.Rotation, 0.001)
			},
		},
		{
			name: "Success - Rotate and Move",
			input: TransformObjectInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Position:       &PositionInput{X: 100, Y: 100},
				Rotation:       float64PtrTransform(180),
			},
			setupElement: createBaseElement,
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				transform := reqs[0].UpdatePageElementTransform.Transform
				// 180 degrees: cos=-1, sin=0
				// ScaleX = -1, ScaleY = -1
				assert.InDelta(t, -1.0, transform.ScaleX, 0.001)
				assert.InDelta(t, -1.0, transform.ScaleY, 0.001)
				assert.InDelta(t, 0.0, transform.ShearX, 0.001)
				assert.InDelta(t, 0.0, transform.ShearY, 0.001)
				
				assert.Equal(t, pointsToEMU(100), transform.TranslateX)
				assert.Equal(t, pointsToEMU(100), transform.TranslateY)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSlides := &mockSlidesService{}
			// Setup GetPresentation to return our element
			mockSlides.GetPresentationFunc = func(ctx context.Context, id string) (*slides.Presentation, error) {
				element := tt.setupElement()
				return &slides.Presentation{
					PresentationId: presentationID,
					Slides: []*slides.Page{
						{
							ObjectId:     slideID,
							PageElements: []*slides.PageElement{element},
						},
					},
				}, nil
			}
			
			if tt.setupMocks != nil {
				tt.setupMocks(mockSlides)
			}

			slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlides, nil
			}

			tool := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
			
			// Capture requests
			var capturedReqs []*slides.Request
			if mockSlides.BatchUpdateFunc != nil {
				originalBatchUpdate := mockSlides.BatchUpdateFunc
				mockSlides.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedReqs = reqs
					return originalBatchUpdate(ctx, id, reqs)
				}
			}

			output, err := tool.TransformObject(ctx, nil, tt.input)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			
			if tt.validateReqs != nil {
				tt.validateReqs(t, capturedReqs)
			}
			
			if tt.expectedOutput != nil {
				tt.expectedOutput(t, output)
			}
		})
	}
}

// Reusing boolPtrLocal/float64PtrLocal from modify_shape_test.go if needed, 
// or define locally if they are not exported (they are not).
func float64PtrTransform(v float64) *float64 {
	return &v
}
