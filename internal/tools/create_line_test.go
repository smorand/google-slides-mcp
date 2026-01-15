package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestCreateLine(t *testing.T) {
	// Setup common variables
	ctx := context.Background()
	presentationID := "test-presentation-id"
	slideID := "slide-id-1"

	tests := []struct {
		name           string
		input          CreateLineInput
		setupMocks     func(*mockSlidesService)
		expectedErr    error
		validateReqs   func(*testing.T, []*slides.Request)
	}{
		{
			name: "Success - Straight Line Positive Slope",
			input: CreateLineInput{
				PresentationID: presentationID,
				SlideIndex:     1,
				StartPoint:     &Point{X: 10, Y: 10},
				EndPoint:       &Point{X: 110, Y: 60},
				LineType:       "STRAIGHT",
			},
			setupMocks: func(m *mockSlidesService) {
				m.GetPresentationFunc = func(ctx context.Context, id string) (*slides.Presentation, error) {
					return &slides.Presentation{
						Slides: []*slides.Page{
							{ObjectId: slideID},
						},
					}, nil
				}
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1) // Only create, no update props
				create := reqs[0].CreateLine
				require.NotNil(t, create)
				assert.Equal(t, "STRAIGHT", create.Category)
				
				// width = 100, height = 50
				// x2 > x1 (110 > 10) -> ScaleX = 1
				// y2 > y1 (60 > 10) -> ScaleY = 1
				
				assert.Equal(t, pointsToEMU(100), create.ElementProperties.Size.Width.Magnitude)
				assert.Equal(t, pointsToEMU(50), create.ElementProperties.Size.Height.Magnitude)
				
				assert.Equal(t, 1.0, create.ElementProperties.Transform.ScaleX)
				assert.Equal(t, 1.0, create.ElementProperties.Transform.ScaleY)
				assert.Equal(t, pointsToEMU(10), create.ElementProperties.Transform.TranslateX)
				assert.Equal(t, pointsToEMU(10), create.ElementProperties.Transform.TranslateY)
			},
		},
		{
			name: "Success - Straight Line Negative Slope (Flip Y)",
			input: CreateLineInput{
				PresentationID: presentationID,
				SlideIndex:     1,
				StartPoint:     &Point{X: 10, Y: 60},
				EndPoint:       &Point{X: 110, Y: 10},
			},
			setupMocks: func(m *mockSlidesService) {
				m.GetPresentationFunc = func(ctx context.Context, id string) (*slides.Presentation, error) {
					return &slides.Presentation{
						Slides: []*slides.Page{
							{ObjectId: slideID},
						},
					}, nil
				}
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 1)
				create := reqs[0].CreateLine
				
				// width = 100, height = 50
				// x2 > x1 (110 > 10) -> ScaleX = 1
				// y2 < y1 (10 < 60) -> ScaleY = -1
				
				assert.Equal(t, pointsToEMU(100), create.ElementProperties.Size.Width.Magnitude)
				assert.Equal(t, pointsToEMU(50), create.ElementProperties.Size.Height.Magnitude)
				
				assert.Equal(t, 1.0, create.ElementProperties.Transform.ScaleX)
				assert.Equal(t, -1.0, create.ElementProperties.Transform.ScaleY)
				assert.Equal(t, pointsToEMU(10), create.ElementProperties.Transform.TranslateX)
				assert.Equal(t, pointsToEMU(60), create.ElementProperties.Transform.TranslateY)
			},
		},
		{
			name: "Success - Straight Line Negative Slope (Flip X)",
			input: CreateLineInput{
				PresentationID: presentationID,
				SlideIndex:     1,
				StartPoint:     &Point{X: 110, Y: 10},
				EndPoint:       &Point{X: 10, Y: 60},
			},
			setupMocks: func(m *mockSlidesService) {
				m.GetPresentationFunc = func(ctx context.Context, id string) (*slides.Presentation, error) {
					return &slides.Presentation{
						Slides: []*slides.Page{
							{ObjectId: slideID},
						},
					}, nil
				}
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				create := reqs[0].CreateLine
				
				// x2 < x1 (10 < 110) -> ScaleX = -1
				// y2 > y1 (60 > 10) -> ScaleY = 1
				
				assert.Equal(t, -1.0, create.ElementProperties.Transform.ScaleX)
				assert.Equal(t, 1.0, create.ElementProperties.Transform.ScaleY)
				assert.Equal(t, pointsToEMU(110), create.ElementProperties.Transform.TranslateX)
				assert.Equal(t, pointsToEMU(10), create.ElementProperties.Transform.TranslateY)
			},
		},
		{
			name: "Success - Straight Line Negative Slope (Flip Both)",
			input: CreateLineInput{
				PresentationID: presentationID,
				SlideIndex:     1,
				StartPoint:     &Point{X: 110, Y: 60},
				EndPoint:       &Point{X: 10, Y: 10},
			},
			setupMocks: func(m *mockSlidesService) {
				m.GetPresentationFunc = func(ctx context.Context, id string) (*slides.Presentation, error) {
					return &slides.Presentation{
						Slides: []*slides.Page{
							{ObjectId: slideID},
						},
					}, nil
				}
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				create := reqs[0].CreateLine
				
				// x2 < x1 -> ScaleX = -1
				// y2 < y1 -> ScaleY = -1
				
				assert.Equal(t, -1.0, create.ElementProperties.Transform.ScaleX)
				assert.Equal(t, -1.0, create.ElementProperties.Transform.ScaleY)
				assert.Equal(t, pointsToEMU(110), create.ElementProperties.Transform.TranslateX)
				assert.Equal(t, pointsToEMU(60), create.ElementProperties.Transform.TranslateY)
			},
		},
		{
			name: "Success - With Styling and Arrows",
			input: CreateLineInput{
				PresentationID: presentationID,
				SlideIndex:     1,
				StartPoint:     &Point{X: 10, Y: 10},
				EndPoint:       &Point{X: 100, Y: 10},
				LineType:       "ELBOW",
				StartArrow:     "ARROW",
				EndArrow:       "DIAMOND",
				LineColor:      "#FF0000",
				LineWeight:     2.0,
				LineDash:       "DASH",
			},
			setupMocks: func(m *mockSlidesService) {
				m.GetPresentationFunc = func(ctx context.Context, id string) (*slides.Presentation, error) {
					return &slides.Presentation{
						Slides: []*slides.Page{
							{ObjectId: slideID},
						},
					}, nil
				}
				m.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				}
			},
			validateReqs: func(t *testing.T, reqs []*slides.Request) {
				require.Len(t, reqs, 2)
				
				// Check Create
				create := reqs[0].CreateLine
				assert.Equal(t, "BENT", create.Category)
				
				// Check Update
				update := reqs[1].UpdateLineProperties
				require.NotNil(t, update)
				
				// Verify properties are set correctly (StartArrow/EndArrow directly on LineProperties)
				assert.Equal(t, "FILL_ARROW", update.LineProperties.StartArrow)
				assert.Equal(t, "FILL_DIAMOND", update.LineProperties.EndArrow)
				assert.Equal(t, 2.0, update.LineProperties.Weight.Magnitude)
				assert.Equal(t, "DASH", update.LineProperties.DashStyle)
				assert.NotNil(t, update.LineProperties.LineFill.SolidFill.Color.RgbColor)
				assert.Equal(t, 1.0, update.LineProperties.LineFill.SolidFill.Color.RgbColor.Red)
				
				// Verify fields
				assert.Contains(t, update.Fields, "startArrow")
				assert.Contains(t, update.Fields, "endArrow")
				assert.Contains(t, update.Fields, "weight")
				assert.Contains(t, update.Fields, "dashStyle")
				assert.Contains(t, update.Fields, "lineFill.solidFill.color")
			},
		},
		{
			name: "Validation Error - Missing Points",
			input: CreateLineInput{
				PresentationID: presentationID,
				SlideIndex:     1,
				StartPoint:     nil,
			},
			expectedErr: ErrInvalidPoints,
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
			
			// Capture requests in the mock
			var capturedReqs []*slides.Request
			if mockSlides.BatchUpdateFunc != nil {
				originalBatchUpdate := mockSlides.BatchUpdateFunc
				mockSlides.BatchUpdateFunc = func(ctx context.Context, id string, reqs []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedReqs = reqs
					return originalBatchUpdate(ctx, id, reqs)
				}
			}

			output, err := tool.CreateLine(ctx, nil, tt.input)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, output.ObjectID)

			if tt.validateReqs != nil {
				tt.validateReqs(t, capturedReqs)
			}
		})
	}
}