package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestAddTextBox(t *testing.T) {
	tests := []struct {
		name           string
		input          AddTextBoxInput
		mockService    func() *mockSlidesService
		wantErr        error
		wantErrContain string
		wantObjectID   bool
	}{
		{
			name: "creates text box at specified position with slide_index",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Hello World",
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
						// Verify the requests
						if len(requests) < 2 {
							t.Error("expected at least 2 requests (CreateShape and InsertText)")
						}
						if requests[0].CreateShape == nil {
							t.Error("first request should be CreateShape")
						}
						if requests[0].CreateShape.ShapeType != "TEXT_BOX" {
							t.Errorf("expected TEXT_BOX, got %s", requests[0].CreateShape.ShapeType)
						}
						if requests[1].InsertText == nil {
							t.Error("second request should be InsertText")
						}
						if requests[1].InsertText.Text != "Hello World" {
							t.Errorf("expected 'Hello World', got %s", requests[1].InsertText.Text)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "creates text box with slide_id",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideID:        "slide-1",
				Text:           "Test Text",
				Position:       &PositionInput{X: 0, Y: 0},
				Size:           &SizeInput{Width: 200, Height: 50},
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
						// Verify the page object ID
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
			name: "applies text styling options",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Styled Text",
				Position:       &PositionInput{X: 100, Y: 100},
				Size:           &SizeInput{Width: 200, Height: 50},
				Style: &TextStyleInput{
					FontFamily: "Arial",
					FontSize:   24,
					Bold:       true,
					Italic:     true,
					Color:      "#FF0000",
				},
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
						// Should have 3 requests: CreateShape, InsertText, UpdateTextStyle
						if len(requests) != 3 {
							t.Errorf("expected 3 requests, got %d", len(requests))
						}
						if requests[2].UpdateTextStyle == nil {
							t.Error("third request should be UpdateTextStyle")
						} else {
							style := requests[2].UpdateTextStyle.Style
							if style.FontFamily != "Arial" {
								t.Errorf("expected font family Arial, got %s", style.FontFamily)
							}
							if style.FontSize.Magnitude != 24 {
								t.Errorf("expected font size 24, got %f", style.FontSize.Magnitude)
							}
							if !style.Bold {
								t.Error("expected bold to be true")
							}
							if !style.Italic {
								t.Error("expected italic to be true")
							}
							if style.ForegroundColor == nil || style.ForegroundColor.OpaqueColor == nil {
								t.Error("expected foreground color to be set")
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
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Test",
				Position:       &PositionInput{X: 100, Y: 50},       // 100 points, 50 points
				Size:           &SizeInput{Width: 200, Height: 100}, // 200 points, 100 points
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
						// 1 point = 12700 EMU
						expectedWidth := 200 * 12700.0  // 2540000
						expectedHeight := 100 * 12700.0 // 1270000
						expectedX := 100 * 12700.0      // 1270000
						expectedY := 50 * 12700.0       // 635000

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
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Test",
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
						if shape.ElementProperties.Transform.TranslateX != 0 {
							t.Errorf("expected X to be 0, got %f", shape.ElementProperties.Transform.TranslateX)
						}
						if shape.ElementProperties.Transform.TranslateY != 0 {
							t.Errorf("expected Y to be 0, got %f", shape.ElementProperties.Transform.TranslateY)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
		},
		{
			name: "returns error when presentation_id is empty",
			input: AddTextBoxInput{
				PresentationID: "",
				SlideIndex:     1,
				Text:           "Test",
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "returns error when neither slide_index nor slide_id provided",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				Text:           "Test",
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidSlideReference,
		},
		{
			name: "returns error when text is empty",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "",
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidText,
		},
		{
			name: "returns error when size is not provided",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Test",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error when size width is zero",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Test",
				Size:           &SizeInput{Width: 0, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error when size height is zero",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Test",
				Size:           &SizeInput{Width: 200, Height: 0},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error when presentation not found",
			input: AddTextBoxInput{
				PresentationID: "nonexistent",
				SlideIndex:     1,
				Text:           "Test",
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("404 Not Found")
					},
				}
			},
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "returns error when access denied",
			input: AddTextBoxInput{
				PresentationID: "forbidden",
				SlideIndex:     1,
				Text:           "Test",
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("403 Forbidden")
					},
				}
			},
			wantErr: ErrAccessDenied,
		},
		{
			name: "returns error when slide_index out of range",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     5,
				Text:           "Test",
				Size:           &SizeInput{Width: 200, Height: 100},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
								{ObjectId: "slide-2"},
							},
						}, nil
					},
				}
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "returns error when slide_id not found",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideID:        "nonexistent-slide",
				Text:           "Test",
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
				}
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "returns error when batch update fails",
			input: AddTextBoxInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Text:           "Test",
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
						return nil, errors.New("batch update failed")
					},
				}
			},
			wantErr: ErrAddTextBoxFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override time function for consistent object IDs
			origTimeNowFunc := timeNowFunc
			timeNowFunc = func() time.Time {
				return time.Date(2024, 1, 15, 10, 0, 0, 123456789, time.UTC)
			}
			defer func() { timeNowFunc = origTimeNowFunc }()

			mockSvc := tt.mockService()
			factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSvc, nil
			}
			tools := NewTools(DefaultToolsConfig(), factory)

			output, err := tools.AddTextBox(context.Background(), nil, tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantObjectID && output.ObjectID == "" {
				t.Error("expected object_id to be set")
			}

			// Verify object ID format
			if tt.wantObjectID && len(output.ObjectID) > 0 {
				if output.ObjectID[:8] != "textbox_" {
					t.Errorf("expected object_id to start with 'textbox_', got %s", output.ObjectID)
				}
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name      string
		hex       string
		wantR     float64
		wantG     float64
		wantB     float64
		wantValid bool
	}{
		{
			name:      "parses red",
			hex:       "#FF0000",
			wantR:     1.0,
			wantG:     0.0,
			wantB:     0.0,
			wantValid: true,
		},
		{
			name:      "parses green",
			hex:       "#00FF00",
			wantR:     0.0,
			wantG:     1.0,
			wantB:     0.0,
			wantValid: true,
		},
		{
			name:      "parses blue",
			hex:       "#0000FF",
			wantR:     0.0,
			wantG:     0.0,
			wantB:     1.0,
			wantValid: true,
		},
		{
			name:      "parses without hash",
			hex:       "FF0000",
			wantR:     1.0,
			wantG:     0.0,
			wantB:     0.0,
			wantValid: true,
		},
		{
			name:      "parses mixed color",
			hex:       "#7F7F7F",
			wantR:     127.0 / 255.0,
			wantG:     127.0 / 255.0,
			wantB:     127.0 / 255.0,
			wantValid: true,
		},
		{
			name:      "parses lowercase hex",
			hex:       "#ff0000",
			wantR:     1.0,
			wantG:     0.0,
			wantB:     0.0,
			wantValid: true,
		},
		{
			name:      "returns nil for short hex",
			hex:       "#FFF",
			wantValid: false,
		},
		{
			name:      "returns nil for invalid hex",
			hex:       "#GGGGGG",
			wantValid: false,
		},
		{
			name:      "returns nil for empty string",
			hex:       "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgb := parseHexColor(tt.hex)

			if tt.wantValid {
				if rgb == nil {
					t.Fatal("expected valid RGB color, got nil")
				}
				if rgb.Red != tt.wantR {
					t.Errorf("expected red %f, got %f", tt.wantR, rgb.Red)
				}
				if rgb.Green != tt.wantG {
					t.Errorf("expected green %f, got %f", tt.wantG, rgb.Green)
				}
				if rgb.Blue != tt.wantB {
					t.Errorf("expected blue %f, got %f", tt.wantB, rgb.Blue)
				}
			} else {
				if rgb != nil {
					t.Errorf("expected nil, got %+v", rgb)
				}
			}
		})
	}
}

func TestPointsToEMU(t *testing.T) {
	tests := []struct {
		name    string
		points  float64
		wantEMU float64
	}{
		{
			name:    "converts 1 point",
			points:  1,
			wantEMU: 12700,
		},
		{
			name:    "converts 100 points",
			points:  100,
			wantEMU: 1270000,
		},
		{
			name:    "converts 720 points (slide width)",
			points:  720,
			wantEMU: 9144000,
		},
		{
			name:    "converts 0 points",
			points:  0,
			wantEMU: 0,
		},
		{
			name:    "converts fractional points",
			points:  0.5,
			wantEMU: 6350,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pointsToEMU(tt.points)
			if got != tt.wantEMU {
				t.Errorf("pointsToEMU(%f) = %f, want %f", tt.points, got, tt.wantEMU)
			}
		})
	}
}

func TestFindSlide(t *testing.T) {
	presentation := &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{ObjectId: "slide-1"},
			{ObjectId: "slide-2"},
			{ObjectId: "slide-3"},
		},
	}

	tests := []struct {
		name       string
		slideIndex int
		slideID    string
		wantID     string
		wantIndex  int
		wantErr    error
	}{
		{
			name:       "finds slide by index 1",
			slideIndex: 1,
			wantID:     "slide-1",
			wantIndex:  1,
		},
		{
			name:       "finds slide by index 3",
			slideIndex: 3,
			wantID:     "slide-3",
			wantIndex:  3,
		},
		{
			name:      "finds slide by ID",
			slideID:   "slide-2",
			wantID:    "slide-2",
			wantIndex: 2,
		},
		{
			name:       "prefers slide_id over slide_index",
			slideIndex: 1,
			slideID:    "slide-3",
			wantID:     "slide-3",
			wantIndex:  3,
		},
		{
			name:       "returns error for index 0",
			slideIndex: 0,
			wantErr:    ErrSlideNotFound,
		},
		{
			name:       "returns error for index out of range",
			slideIndex: 5,
			wantErr:    ErrSlideNotFound,
		},
		{
			name:    "returns error for nonexistent slide_id",
			slideID: "nonexistent",
			wantErr: ErrSlideNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotIndex, err := findSlide(presentation, tt.slideIndex, tt.slideID)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotID != tt.wantID {
				t.Errorf("expected ID %s, got %s", tt.wantID, gotID)
			}

			if gotIndex != tt.wantIndex {
				t.Errorf("expected index %d, got %d", tt.wantIndex, gotIndex)
			}
		})
	}
}

func TestBuildTextStyleRequest(t *testing.T) {
	tests := []struct {
		name       string
		style      *TextStyleInput
		wantFields string
		wantNil    bool
	}{
		{
			name:    "returns nil for nil style",
			style:   nil,
			wantNil: true,
		},
		{
			name:    "returns nil for empty style",
			style:   &TextStyleInput{},
			wantNil: true,
		},
		{
			name: "includes font family",
			style: &TextStyleInput{
				FontFamily: "Arial",
			},
			wantFields: "fontFamily",
		},
		{
			name: "includes font size",
			style: &TextStyleInput{
				FontSize: 24,
			},
			wantFields: "fontSize",
		},
		{
			name: "includes bold",
			style: &TextStyleInput{
				Bold: true,
			},
			wantFields: "bold",
		},
		{
			name: "includes italic",
			style: &TextStyleInput{
				Italic: true,
			},
			wantFields: "italic",
		},
		{
			name: "includes foreground color",
			style: &TextStyleInput{
				Color: "#FF0000",
			},
			wantFields: "foregroundColor",
		},
		{
			name: "includes all fields",
			style: &TextStyleInput{
				FontFamily: "Arial",
				FontSize:   24,
				Bold:       true,
				Italic:     true,
				Color:      "#FF0000",
			},
			wantFields: "fontFamily,fontSize,bold,italic,foregroundColor",
		},
		{
			name: "ignores invalid color",
			style: &TextStyleInput{
				FontFamily: "Arial",
				Color:      "invalid",
			},
			wantFields: "fontFamily",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := buildTextStyleRequest("test-object", tt.style)

			if tt.wantNil {
				if req != nil {
					t.Errorf("expected nil, got %+v", req)
				}
				return
			}

			if req == nil {
				t.Fatal("expected non-nil request")
			}

			if req.UpdateTextStyle == nil {
				t.Fatal("expected UpdateTextStyle to be set")
			}

			if req.UpdateTextStyle.Fields != tt.wantFields {
				t.Errorf("expected fields '%s', got '%s'", tt.wantFields, req.UpdateTextStyle.Fields)
			}
		})
	}
}
