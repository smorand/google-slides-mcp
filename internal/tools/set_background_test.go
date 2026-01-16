package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/slides/v1"
)

func TestSetBackground_SolidColor_SingleSlide(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
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

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     2,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}

	if len(output.AffectedSlides) != 1 {
		t.Errorf("expected 1 affected slide, got %d", len(output.AffectedSlides))
	}

	if output.AffectedSlides[0] != "slide-2" {
		t.Errorf("expected affected slide 'slide-2', got '%s'", output.AffectedSlides[0])
	}

	// Verify the request
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.UpdatePageProperties == nil {
		t.Fatal("expected UpdatePageProperties request")
	}

	if req.UpdatePageProperties.ObjectId != "slide-2" {
		t.Errorf("expected ObjectId 'slide-2', got '%s'", req.UpdatePageProperties.ObjectId)
	}

	bgFill := req.UpdatePageProperties.PageProperties.PageBackgroundFill
	if bgFill == nil || bgFill.SolidFill == nil {
		t.Fatal("expected SolidFill background")
	}

	// Verify color (red = 1.0, green = 0.0, blue = 0.0)
	rgb := bgFill.SolidFill.Color.RgbColor
	if rgb.Red != 1.0 || rgb.Green != 0.0 || rgb.Blue != 0.0 {
		t.Errorf("expected red color, got R=%.2f G=%.2f B=%.2f", rgb.Red, rgb.Green, rgb.Blue)
	}
}

func TestSetBackground_SolidColor_AllSlides(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
					{ObjectId: "slide-3"},
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

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "all",
		BackgroundType: "solid",
		Color:          "#00FF00",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}

	if len(output.AffectedSlides) != 3 {
		t.Errorf("expected 3 affected slides, got %d", len(output.AffectedSlides))
	}

	// Verify 3 update requests were created
	if len(capturedRequests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(capturedRequests))
	}

	// Verify each request targets the correct slide
	expectedSlides := []string{"slide-1", "slide-2", "slide-3"}
	for i, req := range capturedRequests {
		if req.UpdatePageProperties.ObjectId != expectedSlides[i] {
			t.Errorf("request %d: expected ObjectId '%s', got '%s'", i, expectedSlides[i], req.UpdatePageProperties.ObjectId)
		}

		// Verify color (green)
		rgb := req.UpdatePageProperties.PageProperties.PageBackgroundFill.SolidFill.Color.RgbColor
		if rgb.Red != 0.0 || rgb.Green != 1.0 || rgb.Blue != 0.0 {
			t.Errorf("request %d: expected green color, got R=%.2f G=%.2f B=%.2f", i, rgb.Red, rgb.Green, rgb.Blue)
		}
	}
}

func TestSetBackground_Image_SingleSlide(t *testing.T) {
	var capturedUploadName string
	var capturedUploadMimeType string
	var capturedFileID string
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			capturedUploadName = name
			capturedUploadMimeType = mimeType
			return &drive.File{Id: "uploaded-bg-123"}, nil
		},
		MakeFilePublicFunc: func(ctx context.Context, fileID string) error {
			capturedFileID = fileID
			return nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	// Set fixed time for deterministic file names
	originalTimeFunc := backgroundTimeNowFunc
	backgroundTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	defer func() { backgroundTimeNowFunc = originalTimeFunc }()

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "image",
		ImageBase64:    imageBase64,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}

	// Verify image upload
	if capturedUploadName == "" {
		t.Error("expected file upload to be called")
	}
	if capturedUploadMimeType != "image/png" {
		t.Errorf("expected mime type 'image/png', got '%s'", capturedUploadMimeType)
	}
	if capturedFileID != "uploaded-bg-123" {
		t.Errorf("expected MakeFilePublic to be called with 'uploaded-bg-123', got '%s'", capturedFileID)
	}

	// Verify request uses StretchedPictureFill
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	bgFill := req.UpdatePageProperties.PageProperties.PageBackgroundFill
	if bgFill == nil || bgFill.StretchedPictureFill == nil {
		t.Fatal("expected StretchedPictureFill background")
	}

	expectedURL := "https://drive.google.com/uc?id=uploaded-bg-123&export=download"
	if bgFill.StretchedPictureFill.ContentUrl != expectedURL {
		t.Errorf("expected ContentUrl '%s', got '%s'", expectedURL, bgFill.StretchedPictureFill.ContentUrl)
	}
}

func TestSetBackground_Gradient_SingleSlide(t *testing.T) {
	var capturedUploadMimeType string
	var capturedRequests []*slides.Request
	var uploadedImageData []byte

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			capturedUploadMimeType = mimeType
			// Read the content to verify it's a valid PNG
			data, _ := io.ReadAll(content)
			uploadedImageData = data
			return &drive.File{Id: "uploaded-gradient-123"}, nil
		},
		MakeFilePublicFunc: func(ctx context.Context, fileID string) error {
			return nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	angle := 90.0 // Top to bottom
	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "gradient",
		StartColor:     "#FF0000",
		EndColor:       "#0000FF",
		Angle:          &angle,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}

	// Verify gradient image was uploaded as PNG
	if capturedUploadMimeType != "image/png" {
		t.Errorf("expected mime type 'image/png', got '%s'", capturedUploadMimeType)
	}

	// Verify uploaded data is a valid PNG (starts with PNG signature)
	if len(uploadedImageData) < 8 {
		t.Error("expected uploaded image data to be a valid PNG")
	} else {
		pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		for i := 0; i < 8; i++ {
			if uploadedImageData[i] != pngSignature[i] {
				t.Error("expected uploaded image to have PNG signature")
				break
			}
		}
	}

	// Verify request uses StretchedPictureFill
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	bgFill := req.UpdatePageProperties.PageProperties.PageBackgroundFill
	if bgFill == nil || bgFill.StretchedPictureFill == nil {
		t.Fatal("expected StretchedPictureFill background for gradient")
	}
}

func TestSetBackground_Gradient_AllSlides(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
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

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-gradient-all"}, nil
		},
		MakeFilePublicFunc: func(ctx context.Context, fileID string) error {
			return nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "all",
		BackgroundType: "gradient",
		StartColor:     "#FFFFFF",
		EndColor:       "#000000",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}

	if len(output.AffectedSlides) != 2 {
		t.Errorf("expected 2 affected slides, got %d", len(output.AffectedSlides))
	}

	// Verify 2 update requests were created
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(capturedRequests))
	}
}

func TestSetBackground_BySlideID(t *testing.T) {
	var capturedObjectID string

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-custom-id"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) > 0 {
				capturedObjectID = requests[0].UpdatePageProperties.ObjectId
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideID:        "slide-custom-id",
		BackgroundType: "solid",
		Color:          "#0000FF",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedObjectID != "slide-custom-id" {
		t.Errorf("expected ObjectId 'slide-custom-id', got '%s'", capturedObjectID)
	}

	if len(output.AffectedSlides) != 1 || output.AffectedSlides[0] != "slide-custom-id" {
		t.Errorf("expected affected slide 'slide-custom-id', got %v", output.AffectedSlides)
	}
}

func TestSetBackground_InvalidPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation_id")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestSetBackground_InvalidScope(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "invalid",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for invalid scope")
	}

	if !errors.Is(err, ErrInvalidScope) {
		t.Errorf("expected ErrInvalidScope, got %v", err)
	}
}

func TestSetBackground_InvalidBackgroundType(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "invalid",
	})

	if err == nil {
		t.Fatal("expected error for invalid background_type")
	}

	if !errors.Is(err, ErrInvalidBackgroundType) {
		t.Errorf("expected ErrInvalidBackgroundType, got %v", err)
	}
}

func TestSetBackground_MissingSlideReference(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     0, // Invalid
		SlideID:        "", // Also not provided
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for missing slide reference")
	}

	if !errors.Is(err, ErrInvalidSlideReference) {
		t.Errorf("expected ErrInvalidSlideReference, got %v", err)
	}
}

func TestSetBackground_MissingSolidColor(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "", // Missing
	})

	if err == nil {
		t.Fatal("expected error for missing color")
	}

	if !errors.Is(err, ErrMissingBackgroundColor) {
		t.Errorf("expected ErrMissingBackgroundColor, got %v", err)
	}
}

func TestSetBackground_InvalidSolidColor(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "not-a-color",
	})

	if err == nil {
		t.Fatal("expected error for invalid color format")
	}

	if !errors.Is(err, ErrMissingBackgroundColor) {
		t.Errorf("expected ErrMissingBackgroundColor, got %v", err)
	}
}

func TestSetBackground_MissingImageData(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "image",
		ImageBase64:    "", // Missing
	})

	if err == nil {
		t.Fatal("expected error for missing image data")
	}

	if !errors.Is(err, ErrInvalidImageData) {
		t.Errorf("expected ErrInvalidImageData, got %v", err)
	}
}

func TestSetBackground_MissingGradientColors(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	// Missing both colors
	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "gradient",
		StartColor:     "",
		EndColor:       "",
	})

	if err == nil {
		t.Fatal("expected error for missing gradient colors")
	}

	if !errors.Is(err, ErrMissingGradientColors) {
		t.Errorf("expected ErrMissingGradientColors, got %v", err)
	}

	// Missing end color only
	_, err = tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "gradient",
		StartColor:     "#FF0000",
		EndColor:       "",
	})

	if err == nil {
		t.Fatal("expected error for missing end color")
	}

	if !errors.Is(err, ErrMissingGradientColors) {
		t.Errorf("expected ErrMissingGradientColors, got %v", err)
	}
}

func TestSetBackground_InvalidGradientAngle(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	angle := 400.0 // Out of range
	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "gradient",
		StartColor:     "#FF0000",
		EndColor:       "#0000FF",
		Angle:          &angle,
	})

	if err == nil {
		t.Fatal("expected error for invalid gradient angle")
	}

	if !errors.Is(err, ErrInvalidGradientAngle) {
		t.Errorf("expected ErrInvalidGradientAngle, got %v", err)
	}

	// Negative angle
	angle = -10.0
	_, err = tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "gradient",
		StartColor:     "#FF0000",
		EndColor:       "#0000FF",
		Angle:          &angle,
	})

	if err == nil {
		t.Fatal("expected error for negative gradient angle")
	}

	if !errors.Is(err, ErrInvalidGradientAngle) {
		t.Errorf("expected ErrInvalidGradientAngle, got %v", err)
	}
}

func TestSetBackground_PresentationNotFound(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("404 not found")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "non-existent",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for non-existent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestSetBackground_AccessDenied(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("403 forbidden")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "protected",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestSetBackground_SlideNotFound(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     5, // Out of range
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for slide not found")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestSetBackground_InvalidImageBase64(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			}, nil
		},
	}

	mockDrive := &mockDriveService{}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "image",
		ImageBase64:    "not-valid-base64!!!",
	})

	if err == nil {
		t.Fatal("expected error for invalid base64")
	}

	if !errors.Is(err, ErrInvalidImageData) {
		t.Errorf("expected ErrInvalidImageData, got %v", err)
	}
}

func TestSetBackground_UnknownImageFormat(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
			}, nil
		},
	}

	mockDrive := &mockDriveService{}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	// Random bytes that don't match any known image format
	unknownBytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	imageBase64 := base64.StdEncoding.EncodeToString(unknownBytes)

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "image",
		ImageBase64:    imageBase64,
	})

	if err == nil {
		t.Fatal("expected error for unknown image format")
	}

	if !errors.Is(err, ErrInvalidImageData) {
		t.Errorf("expected ErrInvalidImageData, got %v", err)
	}
}

func TestSetBackground_ImageUploadFailed(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return nil, errors.New("upload failed")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "image",
		ImageBase64:    imageBase64,
	})

	if err == nil {
		t.Fatal("expected error for upload failure")
	}

	if !errors.Is(err, ErrImageUploadFailed) {
		t.Errorf("expected ErrImageUploadFailed, got %v", err)
	}
}

func TestSetBackground_BatchUpdateFailed(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("batch update failed")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrSetBackgroundFailed) {
		t.Errorf("expected ErrSetBackgroundFailed, got %v", err)
	}
}

func TestSetBackground_MakePublicFailed_StillSucceeds(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-bg-123"}, nil
		},
		MakeFilePublicFunc: func(ctx context.Context, fileID string) error {
			return errors.New("permission denied")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	// MakePublic failure should be logged but not fail the operation
	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "image",
		ImageBase64:    imageBase64,
	})

	if err != nil {
		t.Fatalf("unexpected error (MakePublic failure should not fail the operation): %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}
}

func TestSetBackground_ScopeCaseInsensitive(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-2"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Test uppercase "ALL"
	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "ALL",
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err != nil {
		t.Fatalf("unexpected error with uppercase scope: %v", err)
	}

	if len(output.AffectedSlides) != 2 {
		t.Errorf("expected 2 affected slides, got %d", len(output.AffectedSlides))
	}

	// Test mixed case "Slide"
	output, err = tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "Slide",
		SlideIndex:     1,
		BackgroundType: "solid",
		Color:          "#FF0000",
	})

	if err != nil {
		t.Fatalf("unexpected error with mixed case scope: %v", err)
	}

	if len(output.AffectedSlides) != 1 {
		t.Errorf("expected 1 affected slide, got %d", len(output.AffectedSlides))
	}
}

func TestSetBackground_BackgroundTypeCaseInsensitive(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Test uppercase "SOLID"
	output, err := tools.SetBackground(context.Background(), tokenSource, SetBackgroundInput{
		PresentationID: "test-presentation",
		Scope:          "slide",
		SlideIndex:     1,
		BackgroundType: "SOLID",
		Color:          "#FF0000",
	})

	if err != nil {
		t.Fatalf("unexpected error with uppercase background_type: %v", err)
	}

	if !output.Success {
		t.Error("expected success to be true")
	}
}

// Test helper functions
func TestGenerateGradientImage(t *testing.T) {
	startRgb := &slides.RgbColor{Red: 1.0, Green: 0.0, Blue: 0.0} // Red
	endRgb := &slides.RgbColor{Red: 0.0, Green: 0.0, Blue: 1.0}   // Blue

	// Test horizontal gradient (0 degrees)
	imageData, err := generateGradientImage(startRgb, endRgb, 0)
	if err != nil {
		t.Fatalf("unexpected error generating gradient: %v", err)
	}

	if len(imageData) == 0 {
		t.Error("expected non-empty image data")
	}

	// Verify PNG signature
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i := 0; i < 8; i++ {
		if imageData[i] != pngSignature[i] {
			t.Errorf("expected PNG signature at position %d, got %x want %x", i, imageData[i], pngSignature[i])
		}
	}
}

func TestGenerateGradientImage_DifferentAngles(t *testing.T) {
	startRgb := &slides.RgbColor{Red: 1.0, Green: 0.0, Blue: 0.0}
	endRgb := &slides.RgbColor{Red: 0.0, Green: 1.0, Blue: 0.0}

	angles := []float64{0, 45, 90, 135, 180, 225, 270, 315, 360}

	for _, angle := range angles {
		imageData, err := generateGradientImage(startRgb, endRgb, angle)
		if err != nil {
			t.Errorf("unexpected error for angle %.0f: %v", angle, err)
			continue
		}

		if len(imageData) == 0 {
			t.Errorf("expected non-empty image data for angle %.0f", angle)
		}
	}
}

func TestEncodePNG(t *testing.T) {
	// Create a simple 2x2 red image
	pixels := []byte{
		255, 0, 0, 255, 255, 0, 0, 255, // Row 0: 2 red pixels
		255, 0, 0, 255, 255, 0, 0, 255, // Row 1: 2 red pixels
	}

	pngData, err := encodePNG(2, 2, pixels)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify PNG signature
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i := 0; i < 8; i++ {
		if pngData[i] != pngSignature[i] {
			t.Errorf("expected PNG signature at position %d, got %x want %x", i, pngData[i], pngSignature[i])
		}
	}

	// Verify IHDR chunk exists (right after signature)
	// Length bytes (4) + "IHDR" (4) = positions 8-15
	if string(pngData[12:16]) != "IHDR" {
		t.Errorf("expected IHDR chunk, got %s", string(pngData[12:16]))
	}
}

func TestAdler32(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected uint32
	}{
		{
			name:     "empty",
			data:     []byte{},
			expected: 1, // Adler-32 of empty data is 1
		},
		{
			name:     "single byte",
			data:     []byte{0},
			expected: 0x10001, // s1=1, s2=1 => 0x10001
		},
		{
			name:     "Wikipedia example",
			data:     []byte("Wikipedia"),
			expected: 0x11E60398, // Known value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adler32(tt.data)
			if result != tt.expected {
				t.Errorf("adler32(%v) = %#x, want %#x", tt.data, result, tt.expected)
			}
		})
	}
}

func TestGenerateBackgroundFileName(t *testing.T) {
	originalTimeFunc := backgroundTimeNowFunc
	backgroundTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	}
	defer func() { backgroundTimeNowFunc = originalTimeFunc }()

	fileName := generateBackgroundFileName()
	if fileName == "" {
		t.Error("expected non-empty file name")
	}

	if !contains(fileName, "slides_background_") {
		t.Errorf("expected file name to contain 'slides_background_', got: %s", fileName)
	}
}
