package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestModifyImage_Success(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image: &slides.Image{
									ContentUrl: "https://example.com/image.png",
								},
								Transform: &slides.AffineTransform{
									ScaleX:     1,
									ScaleY:     1,
									TranslateX: 100,
									TranslateY: 50,
									Unit:       "EMU",
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 200 * 12700, Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: 150 * 12700, Unit: "EMU"},
								},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectID != "image-1" {
		t.Errorf("expected object ID 'image-1', got '%s'", output.ObjectID)
	}

	if len(output.ModifiedProperties) != 1 || output.ModifiedProperties[0] != "brightness" {
		t.Errorf("expected modified properties [brightness], got %v", output.ModifiedProperties)
	}

	// Verify UpdateImageProperties request was made
	found := false
	for _, req := range capturedRequests {
		if req.UpdateImageProperties != nil {
			found = true
			if req.UpdateImageProperties.ObjectId != "image-1" {
				t.Errorf("expected object ID 'image-1', got '%s'", req.UpdateImageProperties.ObjectId)
			}
			if req.UpdateImageProperties.ImageProperties.Brightness != 0.5 {
				t.Errorf("expected brightness 0.5, got %f", req.UpdateImageProperties.ImageProperties.Brightness)
			}
		}
	}
	if !found {
		t.Error("expected UpdateImageProperties request")
	}
}

func TestModifyImage_Position(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
								Transform: &slides.AffineTransform{
									ScaleX:     1,
									ScaleY:     1,
									TranslateX: 100 * 12700,
									TranslateY: 50 * 12700,
									Unit:       "EMU",
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 200 * 12700, Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: 150 * 12700, Unit: "EMU"},
								},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Position: &PositionInput{X: 200, Y: 100},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.ModifiedProperties) != 1 || output.ModifiedProperties[0] != "position" {
		t.Errorf("expected modified properties [position], got %v", output.ModifiedProperties)
	}

	// Verify UpdatePageElementTransform request
	found := false
	for _, req := range capturedRequests {
		if req.UpdatePageElementTransform != nil {
			found = true
			transform := req.UpdatePageElementTransform.Transform
			expectedX := pointsToEMU(200)
			expectedY := pointsToEMU(100)
			if transform.TranslateX != expectedX || transform.TranslateY != expectedY {
				t.Errorf("expected position (%f, %f), got (%f, %f)", expectedX, expectedY, transform.TranslateX, transform.TranslateY)
			}
			if req.UpdatePageElementTransform.ApplyMode != "ABSOLUTE" {
				t.Errorf("expected apply mode ABSOLUTE, got %s", req.UpdatePageElementTransform.ApplyMode)
			}
		}
	}
	if !found {
		t.Error("expected UpdatePageElementTransform request")
	}
}

func TestModifyImage_Size(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
								Transform: &slides.AffineTransform{
									ScaleX:     1,
									ScaleY:     1,
									TranslateX: 100 * 12700,
									TranslateY: 50 * 12700,
									Unit:       "EMU",
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 200 * 12700, Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: 150 * 12700, Unit: "EMU"},
								},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Size: &SizeInput{Width: 400, Height: 300},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.ModifiedProperties) != 1 || output.ModifiedProperties[0] != "size" {
		t.Errorf("expected modified properties [size], got %v", output.ModifiedProperties)
	}

	// Verify UpdatePageElementTransform request with scale changes
	found := false
	for _, req := range capturedRequests {
		if req.UpdatePageElementTransform != nil {
			found = true
			// Size doubled (200->400, 150->300), so scale should be 2x
			transform := req.UpdatePageElementTransform.Transform
			expectedScaleX := 2.0
			expectedScaleY := 2.0
			if transform.ScaleX != expectedScaleX || transform.ScaleY != expectedScaleY {
				t.Errorf("expected scale (%f, %f), got (%f, %f)", expectedScaleX, expectedScaleY, transform.ScaleX, transform.ScaleY)
			}
		}
	}
	if !found {
		t.Error("expected UpdatePageElementTransform request")
	}
}

func TestModifyImage_Crop(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	top := 0.1
	bottom := 0.2
	left := 0.05
	right := 0.15

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Crop: &CropInput{
				Top:    &top,
				Bottom: &bottom,
				Left:   &left,
				Right:  &right,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify crop is in modified properties
	found := false
	for _, prop := range output.ModifiedProperties {
		if prop == "crop" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'crop' in modified properties, got %v", output.ModifiedProperties)
	}

	// Verify UpdateImageProperties request with crop
	for _, req := range capturedRequests {
		if req.UpdateImageProperties != nil {
			crop := req.UpdateImageProperties.ImageProperties.CropProperties
			if crop == nil {
				t.Fatal("expected crop properties")
			}
			if crop.TopOffset != 0.1 {
				t.Errorf("expected top offset 0.1, got %f", crop.TopOffset)
			}
			if crop.BottomOffset != 0.2 {
				t.Errorf("expected bottom offset 0.2, got %f", crop.BottomOffset)
			}
			if crop.LeftOffset != 0.05 {
				t.Errorf("expected left offset 0.05, got %f", crop.LeftOffset)
			}
			if crop.RightOffset != 0.15 {
				t.Errorf("expected right offset 0.15, got %f", crop.RightOffset)
			}
		}
	}
}

func TestModifyImage_BrightnessContrast(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.3
	contrast := -0.2

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
			Contrast:   &contrast,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify modified properties
	hasBrightness := false
	hasContrast := false
	for _, prop := range output.ModifiedProperties {
		if prop == "brightness" {
			hasBrightness = true
		}
		if prop == "contrast" {
			hasContrast = true
		}
	}
	if !hasBrightness || !hasContrast {
		t.Errorf("expected brightness and contrast in modified properties, got %v", output.ModifiedProperties)
	}

	// Verify UpdateImageProperties request
	for _, req := range capturedRequests {
		if req.UpdateImageProperties != nil {
			if req.UpdateImageProperties.ImageProperties.Brightness != 0.3 {
				t.Errorf("expected brightness 0.3, got %f", req.UpdateImageProperties.ImageProperties.Brightness)
			}
			if req.UpdateImageProperties.ImageProperties.Contrast != -0.2 {
				t.Errorf("expected contrast -0.2, got %f", req.UpdateImageProperties.ImageProperties.Contrast)
			}
		}
	}
}

func TestModifyImage_Transparency(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	transparency := 0.5

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Transparency: &transparency,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify transparency in modified properties
	found := false
	for _, prop := range output.ModifiedProperties {
		if prop == "transparency" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'transparency' in modified properties, got %v", output.ModifiedProperties)
	}

	// Verify UpdateImageProperties request
	for _, req := range capturedRequests {
		if req.UpdateImageProperties != nil {
			if req.UpdateImageProperties.ImageProperties.Transparency != 0.5 {
				t.Errorf("expected transparency 0.5, got %f", req.UpdateImageProperties.ImageProperties.Transparency)
			}
		}
	}
}

func TestModifyImage_Recolor(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	recolor := "GRAYSCALE"

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Recolor: &recolor,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify recolor in modified properties
	found := false
	for _, prop := range output.ModifiedProperties {
		if prop == "recolor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'recolor' in modified properties, got %v", output.ModifiedProperties)
	}

	// Verify UpdateImageProperties request
	for _, req := range capturedRequests {
		if req.UpdateImageProperties != nil {
			if req.UpdateImageProperties.ImageProperties.Recolor == nil {
				t.Fatal("expected recolor to be set")
			}
			if req.UpdateImageProperties.ImageProperties.Recolor.Name != "GRAYSCALE" {
				t.Errorf("expected recolor 'GRAYSCALE', got '%s'", req.UpdateImageProperties.ImageProperties.Recolor.Name)
			}
		}
	}
}

func TestModifyImage_RecolorRemove(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	recolor := "none"

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Recolor: &recolor,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.ModifiedProperties) == 0 {
		t.Error("expected at least one modified property")
	}

	// Verify UpdateImageProperties request removes recolor
	for _, req := range capturedRequests {
		if req.UpdateImageProperties != nil {
			if req.UpdateImageProperties.ImageProperties.Recolor != nil {
				t.Error("expected recolor to be nil for removal")
			}
			// Fields should still include "recolor" to trigger the update
			if req.UpdateImageProperties.Fields == "" {
				t.Error("expected fields to be set")
			}
		}
	}
}

func TestModifyImage_MultipleProperties(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
								Transform: &slides.AffineTransform{
									ScaleX:     1,
									ScaleY:     1,
									TranslateX: 100 * 12700,
									TranslateY: 50 * 12700,
									Unit:       "EMU",
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 200 * 12700, Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: 150 * 12700, Unit: "EMU"},
								},
							},
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.3
	transparency := 0.2

	output, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Position:     &PositionInput{X: 300, Y: 200},
			Brightness:   &brightness,
			Transparency: &transparency,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have position, brightness, transparency
	if len(output.ModifiedProperties) < 3 {
		t.Errorf("expected at least 3 modified properties, got %d: %v", len(output.ModifiedProperties), output.ModifiedProperties)
	}

	// Should have both transform and image properties requests
	hasTransform := false
	hasImageProps := false
	for _, req := range capturedRequests {
		if req.UpdatePageElementTransform != nil {
			hasTransform = true
		}
		if req.UpdateImageProperties != nil {
			hasImageProps = true
		}
	}

	if !hasTransform {
		t.Error("expected UpdatePageElementTransform request for position")
	}
	if !hasImageProps {
		t.Error("expected UpdateImageProperties request for brightness and transparency")
	}
}

func TestModifyImage_NotFound(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId:     "slide-1",
						PageElements: []*slides.PageElement{},
					},
				},
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "nonexistent-image",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for nonexistent object")
	}
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestModifyImage_NotAnImage(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-1",
								Shape: &slides.Shape{
									ShapeType: "RECTANGLE",
								},
							},
						},
					},
				},
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "shape-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for non-image object")
	}
	if !errors.Is(err, ErrNotImageObject) {
		t.Errorf("expected ErrNotImageObject, got %v", err)
	}
}

func TestModifyImage_NoProperties(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties:     nil,
	})

	if err == nil {
		t.Fatal("expected error for nil properties")
	}
	if !errors.Is(err, ErrNoImageProperties) {
		t.Errorf("expected ErrNoImageProperties, got %v", err)
	}
}

func TestModifyImage_EmptyProperties(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties:     &ImageModifyProperties{}, // All nil
	})

	if err == nil {
		t.Fatal("expected error for empty properties")
	}
	if !errors.Is(err, ErrNoImageProperties) {
		t.Errorf("expected ErrNoImageProperties, got %v", err)
	}
}

func TestModifyImage_InvalidBrightness(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	testCases := []struct {
		name       string
		brightness float64
	}{
		{"too low", -1.5},
		{"too high", 1.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			brightness := tc.brightness
			_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
				PresentationID: "test-presentation",
				ObjectID:       "image-1",
				Properties: &ImageModifyProperties{
					Brightness: &brightness,
				},
			})

			if err == nil {
				t.Fatal("expected error for invalid brightness")
			}
			if !errors.Is(err, ErrInvalidBrightnessValue) {
				t.Errorf("expected ErrInvalidBrightnessValue, got %v", err)
			}
		})
	}
}

func TestModifyImage_InvalidContrast(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	testCases := []struct {
		name     string
		contrast float64
	}{
		{"too low", -1.5},
		{"too high", 1.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contrast := tc.contrast
			_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
				PresentationID: "test-presentation",
				ObjectID:       "image-1",
				Properties: &ImageModifyProperties{
					Contrast: &contrast,
				},
			})

			if err == nil {
				t.Fatal("expected error for invalid contrast")
			}
			if !errors.Is(err, ErrInvalidContrastValue) {
				t.Errorf("expected ErrInvalidContrastValue, got %v", err)
			}
		})
	}
}

func TestModifyImage_InvalidTransparency(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	testCases := []struct {
		name         string
		transparency float64
	}{
		{"negative", -0.1},
		{"too high", 1.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transparency := tc.transparency
			_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
				PresentationID: "test-presentation",
				ObjectID:       "image-1",
				Properties: &ImageModifyProperties{
					Transparency: &transparency,
				},
			})

			if err == nil {
				t.Fatal("expected error for invalid transparency")
			}
			if !errors.Is(err, ErrInvalidTransparency) {
				t.Errorf("expected ErrInvalidTransparency, got %v", err)
			}
		})
	}
}

func TestModifyImage_InvalidCrop(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	testCases := []struct {
		name string
		crop *CropInput
	}{
		{"top negative", &CropInput{Top: ptrFloat64(-0.1)}},
		{"top too high", &CropInput{Top: ptrFloat64(1.5)}},
		{"bottom negative", &CropInput{Bottom: ptrFloat64(-0.1)}},
		{"left negative", &CropInput{Left: ptrFloat64(-0.1)}},
		{"right too high", &CropInput{Right: ptrFloat64(1.2)}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
				PresentationID: "test-presentation",
				ObjectID:       "image-1",
				Properties: &ImageModifyProperties{
					Crop: tc.crop,
				},
			})

			if err == nil {
				t.Fatal("expected error for invalid crop")
			}
			if !errors.Is(err, ErrInvalidCropValue) {
				t.Errorf("expected ErrInvalidCropValue, got %v", err)
			}
		})
	}
}

func TestModifyImage_PresentationNotFound(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("404 not found")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "nonexistent",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for nonexistent presentation")
	}
	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestModifyImage_AccessDenied(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("403 forbidden")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "restricted",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestModifyImage_BatchUpdateError(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image:    &slides.Image{},
							},
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("batch update failed")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}
	if !errors.Is(err, ErrModifyImageFailed) {
		t.Errorf("expected ErrModifyImageFailed, got %v", err)
	}
}

func TestModifyImage_MissingPresentationID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "",
		ObjectID:       "image-1",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for missing presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestModifyImage_MissingObjectID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	brightness := 0.5
	_, err := tools.ModifyImage(context.Background(), tokenSource, ModifyImageInput{
		PresentationID: "test-presentation",
		ObjectID:       "",
		Properties: &ImageModifyProperties{
			Brightness: &brightness,
		},
	})

	if err == nil {
		t.Fatal("expected error for missing object ID")
	}
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

// Helper function tests

func TestValidateCropValues(t *testing.T) {
	testCases := []struct {
		name      string
		crop      *CropInput
		expectErr bool
	}{
		{"valid all", &CropInput{Top: ptrFloat64(0.1), Bottom: ptrFloat64(0.2), Left: ptrFloat64(0.3), Right: ptrFloat64(0.4)}, false},
		{"valid zero", &CropInput{Top: ptrFloat64(0)}, false},
		{"valid one", &CropInput{Top: ptrFloat64(1)}, false},
		{"invalid top negative", &CropInput{Top: ptrFloat64(-0.1)}, true},
		{"invalid top high", &CropInput{Top: ptrFloat64(1.1)}, true},
		{"invalid bottom negative", &CropInput{Bottom: ptrFloat64(-0.5)}, true},
		{"invalid left high", &CropInput{Left: ptrFloat64(2)}, true},
		{"invalid right negative", &CropInput{Right: ptrFloat64(-1)}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCropValues(tc.crop)
			if tc.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestHasImagePropertiesToModify(t *testing.T) {
	testCases := []struct {
		name     string
		props    *ImageModifyProperties
		expected bool
	}{
		{"nil", nil, false},
		{"empty", &ImageModifyProperties{}, false},
		{"position only", &ImageModifyProperties{Position: &PositionInput{X: 100, Y: 50}}, true},
		{"size only", &ImageModifyProperties{Size: &SizeInput{Width: 200, Height: 150}}, true},
		{"brightness only", &ImageModifyProperties{Brightness: ptrFloat64(0.5)}, true},
		{"contrast only", &ImageModifyProperties{Contrast: ptrFloat64(-0.3)}, true},
		{"transparency only", &ImageModifyProperties{Transparency: ptrFloat64(0.2)}, true},
		{"crop only", &ImageModifyProperties{Crop: &CropInput{Top: ptrFloat64(0.1)}}, true},
		{"recolor only", &ImageModifyProperties{Recolor: ptrString("GRAYSCALE")}, true},
		{"multiple", &ImageModifyProperties{Position: &PositionInput{X: 100, Y: 50}, Brightness: ptrFloat64(0.5)}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasImagePropertiesToModify(tc.props)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// Helper to create string pointer
func ptrString(s string) *string {
	return &s
}
