package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestModifyShape(t *testing.T) {
	// Setup common variables
	ctx := context.Background()
	presentationID := "test-presentation-id"
	objectID := "shape-id-1"

	tests := []struct {
		name           string
		input          ModifyShapeInput
		setupMocks     func(*mockSlidesService)
		expectedErr    error
		validateReqs   func(*testing.T, []*slides.Request)
	}{
		{
			name: "Success - Modify Fill Color",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties: &ShapeProperties{
					FillColor: "#FF0000",
				},
			},
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				update := reqs[0].UpdateShapeProperties
				require.NotNil(t, update)
				assert.Equal(t, objectID, update.ObjectId)
				assert.Contains(t, update.Fields, "shapeBackgroundFill")
				
				fill := update.ShapeProperties.ShapeBackgroundFill
				require.NotNil(t, fill)
				require.NotNil(t, fill.SolidFill)
				require.NotNil(t, fill.SolidFill.Color)
				assert.Equal(t, 1.0, fill.SolidFill.Color.RgbColor.Red)
			},
		},
		{
			name: "Success - Transparent Fill",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties: &ShapeProperties{
					FillColor: "transparent",
				},
			},
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				update := reqs[0].UpdateShapeProperties
				fill := update.ShapeProperties.ShapeBackgroundFill
				assert.Equal(t, "NOT_RENDERED", fill.PropertyState)
			},
		},
		{
			name: "Success - Modify Outline",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties: &ShapeProperties{
					OutlineColor:  "#00FF00",
					OutlineWeight: float64PtrLocal(3.0),
					OutlineDash:   "DOT",
				},
			},
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				update := reqs[0].UpdateShapeProperties
				outline := update.ShapeProperties.Outline
				
				require.NotNil(t, outline)
				assert.Contains(t, update.Fields, "outline.outlineFill")
				assert.Contains(t, update.Fields, "outline.weight")
				assert.Contains(t, update.Fields, "outline.dashStyle")
				
				assert.Equal(t, 1.0, outline.OutlineFill.SolidFill.Color.RgbColor.Green)
				assert.Equal(t, 3.0, outline.Weight.Magnitude)
				assert.Equal(t, "DOT", outline.DashStyle)
			},
		},
		{
			name: "Success - Transparent Outline",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties: &ShapeProperties{
					OutlineColor: "transparent",
				},
			},
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				update := reqs[0].UpdateShapeProperties
				outline := update.ShapeProperties.Outline
				
				assert.Equal(t, "NOT_RENDERED", outline.PropertyState)
				assert.Contains(t, update.Fields, "outline.propertyState")
			},
		},
		{
			name: "Success - Enable Shadow",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties: &ShapeProperties{
					Shadow: boolPtrLocal(true),
				},
			},
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				update := reqs[0].UpdateShapeProperties
				assert.Contains(t, update.Fields, "shadow")
				assert.Equal(t, "OUTER", update.ShapeProperties.Shadow.Type)
			},
		},
		{
			name: "Success - Disable Shadow",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties: &ShapeProperties{
					Shadow: boolPtrLocal(false),
				},
			},
			setupMocks: func(m *mockSlidesService) {
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				update := reqs[0].UpdateShapeProperties
				assert.Contains(t, update.Fields, "shadow")
				assert.Equal(t, "NOT_RENDERED", update.ShapeProperties.Shadow.PropertyState)
			},
		},
		{
			name: "Error - No Properties",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties:     nil,
			},
			expectedErr: ErrNoProperties,
		},
		{
			name: "Error - Empty Properties",
			input: ModifyShapeInput{
				PresentationID: presentationID,
				ObjectID:       objectID,
				Properties:     &ShapeProperties{},
			},
			expectedErr: ErrNoProperties,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSlides := &mockSlidesService{}
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

			output, err := tool.ModifyShape(ctx, nil, tt.input)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, objectID, output.ObjectID)
			
			if tt.validateReqs != nil {
				tt.validateReqs(t, capturedReqs)
			}
		})
	}
}

func float64PtrLocal(v float64) *float64 {
	return &v
}

func boolPtrLocal(v bool) *bool {
	return &v
}