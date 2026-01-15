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

// Sample PNG image bytes (1x1 red pixel)
var testPNGBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 image
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xFE,
	0xD4, 0xAA, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// Sample JPEG image bytes (minimal valid JPEG)
var testJPEGBytes = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, // SOI and APP0
	0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
	0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9, // EOI
}

// Sample GIF image bytes (minimal valid GIF)
var testGIFBytes = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, // GIF89a
	0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
	0x3B, // GIF trailer
}

// Sample WebP image bytes (minimal header)
var testWebPBytes = []byte{
	0x52, 0x49, 0x46, 0x46, // RIFF
	0x00, 0x00, 0x00, 0x00, // Size (placeholder)
	0x57, 0x45, 0x42, 0x50, // WEBP
	0x56, 0x50, 0x38, 0x20, // VP8
}

// Sample BMP image bytes (minimal header)
var testBMPBytes = []byte{
	0x42, 0x4D, // BM
	0x36, 0x00, 0x00, 0x00, // File size
	0x00, 0x00, 0x00, 0x00, // Reserved
	0x36, 0x00, 0x00, 0x00, // Data offset
}

func TestAddImage_Success(t *testing.T) {
	var capturedRequests []*slides.Request
	var capturedFileName string
	var capturedMimeType string
	var capturedFileID string

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
			capturedFileName = name
			capturedMimeType = mimeType
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	// Set fixed time for deterministic object IDs
	originalTimeFunc := imageTimeNowFunc
	imageTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	defer func() { imageTimeNowFunc = originalTimeFunc }()

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	output, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    imageBase64,
		Position:       &PositionInput{X: 100, Y: 50},
		Size: &ImageSizeInput{
			Width:  ptrFloat64(200),
			Height: ptrFloat64(150),
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectID == "" {
		t.Error("expected non-empty object ID")
	}

	// Verify file was uploaded with correct name and mime type
	if capturedFileName == "" {
		t.Error("expected file upload to be called")
	}
	if capturedMimeType != "image/png" {
		t.Errorf("expected mime type 'image/png', got '%s'", capturedMimeType)
	}

	// Verify file was made public
	if capturedFileID != "uploaded-file-123" {
		t.Errorf("expected MakeFilePublic to be called with 'uploaded-file-123', got '%s'", capturedFileID)
	}

	// Verify CreateImage request was made
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.CreateImage == nil {
		t.Fatal("expected CreateImage request")
	}

	// Verify position
	transform := req.CreateImage.ElementProperties.Transform
	if transform == nil {
		t.Fatal("expected transform to be set")
	}

	expectedX := pointsToEMU(100)
	expectedY := pointsToEMU(50)
	if transform.TranslateX != expectedX || transform.TranslateY != expectedY {
		t.Errorf("expected position (%f, %f), got (%f, %f)", expectedX, expectedY, transform.TranslateX, transform.TranslateY)
	}

	// Verify size
	size := req.CreateImage.ElementProperties.Size
	if size == nil {
		t.Fatal("expected size to be set")
	}

	expectedWidth := pointsToEMU(200)
	expectedHeight := pointsToEMU(150)
	if size.Width.Magnitude != expectedWidth || size.Height.Magnitude != expectedHeight {
		t.Errorf("expected size (%f x %f), got (%f x %f)", expectedWidth, expectedHeight, size.Width.Magnitude, size.Height.Magnitude)
	}
}

func TestAddImage_BySlideID(t *testing.T) {
	var capturedSlideID string

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
					{ObjectId: "slide-abc"},
					{ObjectId: "slide-3"},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) > 0 && requests[0].CreateImage != nil {
				capturedSlideID = requests[0].CreateImage.ElementProperties.PageObjectId
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideID:        "slide-abc",
		ImageBase64:    imageBase64,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSlideID != "slide-abc" {
		t.Errorf("expected slide ID 'slide-abc', got '%s'", capturedSlideID)
	}
}

func TestAddImage_DefaultPosition(t *testing.T) {
	var capturedTransform *slides.AffineTransform

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) > 0 && requests[0].CreateImage != nil {
				capturedTransform = requests[0].CreateImage.ElementProperties.Transform
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    imageBase64,
		// No position specified - should use nil (API default)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When no position is specified, transform should be nil (API decides placement)
	if capturedTransform != nil {
		t.Error("expected no transform when position not specified")
	}
}

func TestAddImage_OnlyWidth(t *testing.T) {
	var capturedSize *slides.Size

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) > 0 && requests[0].CreateImage != nil {
				capturedSize = requests[0].CreateImage.ElementProperties.Size
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    imageBase64,
		Size: &ImageSizeInput{
			Width: ptrFloat64(300),
			// Height omitted - should preserve aspect ratio
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSize == nil {
		t.Fatal("expected size to be set")
	}

	if capturedSize.Width == nil {
		t.Error("expected width to be set")
	}

	if capturedSize.Height != nil {
		t.Error("expected height to be nil when only width is specified")
	}
}

func TestAddImage_OnlyHeight(t *testing.T) {
	var capturedSize *slides.Size

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			if len(requests) > 0 && requests[0].CreateImage != nil {
				capturedSize = requests[0].CreateImage.ElementProperties.Size
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	imageBase64 := base64.StdEncoding.EncodeToString(testPNGBytes)

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    imageBase64,
		Size: &ImageSizeInput{
			// Width omitted
			Height: ptrFloat64(200),
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSize == nil {
		t.Fatal("expected size to be set")
	}

	if capturedSize.Width != nil {
		t.Error("expected width to be nil when only height is specified")
	}

	if capturedSize.Height == nil {
		t.Error("expected height to be set")
	}
}

func TestAddImage_InvalidPresentationID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "", // Empty
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for empty presentation_id")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestAddImage_InvalidSlideReference(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     0, // Invalid (should be 1-based or use SlideID)
		SlideID:        "", // Also not provided
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for invalid slide reference")
	}

	if !errors.Is(err, ErrInvalidSlideReference) {
		t.Errorf("expected ErrInvalidSlideReference, got %v", err)
	}
}

func TestAddImage_EmptyImageData(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    "", // Empty
	})

	if err == nil {
		t.Fatal("expected error for empty image data")
	}

	if !errors.Is(err, ErrInvalidImageData) {
		t.Errorf("expected ErrInvalidImageData, got %v", err)
	}
}

func TestAddImage_InvalidBase64(t *testing.T) {
	mockSlides := &mockSlidesService{}
	mockDrive := &mockDriveService{}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    "not-valid-base64!!!", // Invalid base64
	})

	if err == nil {
		t.Fatal("expected error for invalid base64")
	}

	if !errors.Is(err, ErrInvalidImageData) {
		t.Errorf("expected ErrInvalidImageData, got %v", err)
	}
}

func TestAddImage_UnknownImageFormat(t *testing.T) {
	mockSlides := &mockSlidesService{}
	mockDrive := &mockDriveService{}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockDrive, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, driveFactory)
	tokenSource := &mockTokenSource{}

	// Random bytes that don't match any known format
	unknownBytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	imageBase64 := base64.StdEncoding.EncodeToString(unknownBytes)

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    imageBase64,
	})

	if err == nil {
		t.Fatal("expected error for unknown image format")
	}

	if !errors.Is(err, ErrInvalidImageData) {
		t.Errorf("expected ErrInvalidImageData, got %v", err)
	}
}

func TestAddImage_InvalidSize_NegativeWidth(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
		Size: &ImageSizeInput{
			Width: ptrFloat64(-100), // Negative
		},
	})

	if err == nil {
		t.Fatal("expected error for negative width")
	}

	if !errors.Is(err, ErrInvalidImageSize) {
		t.Errorf("expected ErrInvalidImageSize, got %v", err)
	}
}

func TestAddImage_InvalidSize_ZeroHeight(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
		Size: &ImageSizeInput{
			Height: ptrFloat64(0), // Zero
		},
	})

	if err == nil {
		t.Fatal("expected error for zero height")
	}

	if !errors.Is(err, ErrInvalidImageSize) {
		t.Errorf("expected ErrInvalidImageSize, got %v", err)
	}
}

func TestAddImage_InvalidPosition_NegativeX(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
		Position: &PositionInput{
			X: -50, // Negative
			Y: 100,
		},
	})

	if err == nil {
		t.Fatal("expected error for negative X position")
	}

	if !errors.Is(err, ErrInvalidImagePosition) {
		t.Errorf("expected ErrInvalidImagePosition, got %v", err)
	}
}

func TestAddImage_PresentationNotFound(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("404 not found")
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

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "non-existent",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for non-existent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestAddImage_AccessDenied(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("403 forbidden")
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

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "protected",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestAddImage_SlideNotFound(t *testing.T) {
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

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     5, // Out of range
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for slide not found")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestAddImage_UploadFailed(t *testing.T) {
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

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for upload failure")
	}

	if !errors.Is(err, ErrImageUploadFailed) {
		t.Errorf("expected ErrImageUploadFailed, got %v", err)
	}
}

func TestAddImage_MakePublicFailed_StillSucceeds(t *testing.T) {
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
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	// MakePublic failure should be logged but not fail the operation
	output, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err != nil {
		t.Fatalf("unexpected error (MakePublic failure should not fail the operation): %v", err)
	}

	if output.ObjectID == "" {
		t.Error("expected non-empty object ID")
	}
}

func TestAddImage_BatchUpdateFailed(t *testing.T) {
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

	mockDrive := &mockDriveService{
		UploadFileFunc: func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
			return &drive.File{Id: "uploaded-file-123"}, nil
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

	_, err := tools.AddImage(context.Background(), tokenSource, AddImageInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		ImageBase64:    base64.StdEncoding.EncodeToString(testPNGBytes),
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrAddImageFailed) {
		t.Errorf("expected ErrAddImageFailed, got %v", err)
	}
}

// Helper function to create a pointer to float64
func ptrFloat64(f float64) *float64 {
	return &f
}

// Tests for detectImageMimeType helper function
func TestDetectImageMimeType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "PNG",
			data:     testPNGBytes,
			expected: "image/png",
		},
		{
			name:     "JPEG",
			data:     testJPEGBytes,
			expected: "image/jpeg",
		},
		{
			name:     "GIF",
			data:     testGIFBytes,
			expected: "image/gif",
		},
		{
			name:     "WebP",
			data:     testWebPBytes,
			expected: "image/webp",
		},
		{
			name:     "BMP",
			data:     testBMPBytes,
			expected: "image/bmp",
		},
		{
			name:     "Unknown format",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			expected: "",
		},
		{
			name:     "Too short",
			data:     []byte{0x89, 0x50},
			expected: "",
		},
		{
			name:     "Empty",
			data:     []byte{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectImageMimeType(tt.data)
			if result != tt.expected {
				t.Errorf("detectImageMimeType() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// Tests for generateImageFileName and generateImageObjectID
func TestGenerateImageFileName(t *testing.T) {
	originalTimeFunc := imageTimeNowFunc
	imageTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	}
	defer func() { imageTimeNowFunc = originalTimeFunc }()

	fileName := generateImageFileName()
	if fileName == "" {
		t.Error("expected non-empty file name")
	}

	// Verify format contains expected prefix
	if !contains(fileName, "slides_image_") {
		t.Errorf("expected file name to contain 'slides_image_', got: %s", fileName)
	}
}

func TestGenerateImageObjectID(t *testing.T) {
	originalTimeFunc := imageTimeNowFunc
	imageTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	}
	defer func() { imageTimeNowFunc = originalTimeFunc }()

	objectID := generateImageObjectID()
	if objectID == "" {
		t.Error("expected non-empty object ID")
	}

	// Verify format contains expected prefix
	if !contains(objectID, "image_") {
		t.Errorf("expected object ID to contain 'image_', got: %s", objectID)
	}
}

// Test for buildImageRequests
func TestBuildImageRequests(t *testing.T) {
	tests := []struct {
		name         string
		input        AddImageInput
		expectPos    bool
		expectSize   bool
		expectWidth  bool
		expectHeight bool
	}{
		{
			name: "No position, no size",
			input: AddImageInput{
				PresentationID: "test",
				SlideIndex:     1,
				ImageBase64:    "test",
			},
			expectPos:    false,
			expectSize:   false,
			expectWidth:  false,
			expectHeight: false,
		},
		{
			name: "With position, no size",
			input: AddImageInput{
				PresentationID: "test",
				SlideIndex:     1,
				ImageBase64:    "test",
				Position:       &PositionInput{X: 100, Y: 50},
			},
			expectPos:    true,
			expectSize:   false,
			expectWidth:  false,
			expectHeight: false,
		},
		{
			name: "With both dimensions",
			input: AddImageInput{
				PresentationID: "test",
				SlideIndex:     1,
				ImageBase64:    "test",
				Size: &ImageSizeInput{
					Width:  ptrFloat64(200),
					Height: ptrFloat64(150),
				},
			},
			expectPos:    false,
			expectSize:   true,
			expectWidth:  true,
			expectHeight: true,
		},
		{
			name: "Only width",
			input: AddImageInput{
				PresentationID: "test",
				SlideIndex:     1,
				ImageBase64:    "test",
				Size: &ImageSizeInput{
					Width: ptrFloat64(200),
				},
			},
			expectPos:    false,
			expectSize:   true,
			expectWidth:  true,
			expectHeight: false,
		},
		{
			name: "Only height",
			input: AddImageInput{
				PresentationID: "test",
				SlideIndex:     1,
				ImageBase64:    "test",
				Size: &ImageSizeInput{
					Height: ptrFloat64(150),
				},
			},
			expectPos:    false,
			expectSize:   true,
			expectWidth:  false,
			expectHeight: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildImageRequests("obj-1", "slide-1", "file-123", tt.input)

			if len(requests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(requests))
			}

			req := requests[0].CreateImage
			if req == nil {
				t.Fatal("expected CreateImage request")
			}

			// Verify position
			if tt.expectPos {
				if req.ElementProperties.Transform == nil {
					t.Error("expected transform to be set")
				}
			} else {
				if req.ElementProperties.Transform != nil {
					t.Error("expected transform to be nil")
				}
			}

			// Verify size
			if tt.expectSize {
				if req.ElementProperties.Size == nil {
					t.Error("expected size to be set")
				} else {
					if tt.expectWidth && req.ElementProperties.Size.Width == nil {
						t.Error("expected width to be set")
					}
					if !tt.expectWidth && req.ElementProperties.Size.Width != nil {
						t.Error("expected width to be nil")
					}
					if tt.expectHeight && req.ElementProperties.Size.Height == nil {
						t.Error("expected height to be set")
					}
					if !tt.expectHeight && req.ElementProperties.Size.Height != nil {
						t.Error("expected height to be nil")
					}
				}
			} else {
				if req.ElementProperties.Size != nil {
					t.Error("expected size to be nil")
				}
			}
		})
	}
}
