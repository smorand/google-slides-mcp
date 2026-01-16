package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestAddVideo_YouTube_Success(t *testing.T) {
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	// Set fixed time for deterministic object IDs
	originalTimeFunc := videoTimeNowFunc
	videoTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	defer func() { videoTimeNowFunc = originalTimeFunc }()

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	output, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		Position:       &PositionInput{X: 100, Y: 50},
		Size:           &SizeInput{Width: 400, Height: 225},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectID == "" {
		t.Error("expected non-empty object ID")
	}

	// Verify CreateVideo request was made
	if len(capturedRequests) < 1 {
		t.Fatal("expected at least 1 request")
	}

	req := capturedRequests[0]
	if req.CreateVideo == nil {
		t.Fatal("expected CreateVideo request")
	}

	// Verify video source and ID
	if req.CreateVideo.Source != "YOUTUBE" {
		t.Errorf("expected video source 'YOUTUBE', got '%s'", req.CreateVideo.Source)
	}
	if req.CreateVideo.Id != "dQw4w9WgXcQ" {
		t.Errorf("expected video ID 'dQw4w9WgXcQ', got '%s'", req.CreateVideo.Id)
	}

	// Verify position
	transform := req.CreateVideo.ElementProperties.Transform
	if transform == nil {
		t.Fatal("expected transform to be set")
	}

	expectedX := pointsToEMU(100)
	expectedY := pointsToEMU(50)
	if transform.TranslateX != expectedX || transform.TranslateY != expectedY {
		t.Errorf("expected position (%f, %f), got (%f, %f)", expectedX, expectedY, transform.TranslateX, transform.TranslateY)
	}

	// Verify size
	size := req.CreateVideo.ElementProperties.Size
	if size == nil {
		t.Fatal("expected size to be set")
	}

	expectedWidth := pointsToEMU(400)
	expectedHeight := pointsToEMU(225)
	if size.Width.Magnitude != expectedWidth || size.Height.Magnitude != expectedHeight {
		t.Errorf("expected size (%f x %f), got (%f x %f)", expectedWidth, expectedHeight, size.Width.Magnitude, size.Height.Magnitude)
	}
}

func TestAddVideo_Drive_Success(t *testing.T) {
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	// Set fixed time for deterministic object IDs
	originalTimeFunc := videoTimeNowFunc
	videoTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	defer func() { videoTimeNowFunc = originalTimeFunc }()

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	output, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "drive",
		VideoID:        "drive-file-id-123",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectID == "" {
		t.Error("expected non-empty object ID")
	}

	// Verify CreateVideo request was made
	if len(capturedRequests) < 1 {
		t.Fatal("expected at least 1 request")
	}

	req := capturedRequests[0]
	if req.CreateVideo == nil {
		t.Fatal("expected CreateVideo request")
	}

	// Verify video source and ID
	if req.CreateVideo.Source != "DRIVE" {
		t.Errorf("expected video source 'DRIVE', got '%s'", req.CreateVideo.Source)
	}
	if req.CreateVideo.Id != "drive-file-id-123" {
		t.Errorf("expected video ID 'drive-file-id-123', got '%s'", req.CreateVideo.Id)
	}
}

func TestAddVideo_WithStartEndTimes(t *testing.T) {
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	startTime := 30.0  // 30 seconds
	endTime := 120.0   // 2 minutes

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		StartTime:      &startTime,
		EndTime:        &endTime,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have CreateVideo and UpdateVideoProperties requests
	if len(capturedRequests) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(capturedRequests))
	}

	// Find UpdateVideoProperties request
	var updateReq *slides.UpdateVideoPropertiesRequest
	for _, req := range capturedRequests {
		if req.UpdateVideoProperties != nil {
			updateReq = req.UpdateVideoProperties
			break
		}
	}

	if updateReq == nil {
		t.Fatal("expected UpdateVideoProperties request")
	}

	// Verify start and end times are in milliseconds
	expectedStart := int64(30000)  // 30 seconds * 1000
	expectedEnd := int64(120000)   // 120 seconds * 1000

	if updateReq.VideoProperties.Start != expectedStart {
		t.Errorf("expected start time %d ms, got %d ms", expectedStart, updateReq.VideoProperties.Start)
	}
	if updateReq.VideoProperties.End != expectedEnd {
		t.Errorf("expected end time %d ms, got %d ms", expectedEnd, updateReq.VideoProperties.End)
	}
}

func TestAddVideo_WithAutoplayAndMute(t *testing.T) {
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

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		Autoplay:       true,
		Mute:           true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have CreateVideo and UpdateVideoProperties requests
	if len(capturedRequests) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(capturedRequests))
	}

	// Find UpdateVideoProperties request
	var updateReq *slides.UpdateVideoPropertiesRequest
	for _, req := range capturedRequests {
		if req.UpdateVideoProperties != nil {
			updateReq = req.UpdateVideoProperties
			break
		}
	}

	if updateReq == nil {
		t.Fatal("expected UpdateVideoProperties request")
	}

	// Verify autoplay and mute settings
	if !updateReq.VideoProperties.AutoPlay {
		t.Error("expected AutoPlay to be true")
	}
	if !updateReq.VideoProperties.Mute {
		t.Error("expected Mute to be true")
	}
}

func TestAddVideo_BySlideID(t *testing.T) {
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
			if len(requests) > 0 && requests[0].CreateVideo != nil {
				capturedSlideID = requests[0].CreateVideo.ElementProperties.PageObjectId
			}
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideID:        "slide-abc",
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSlideID != "slide-abc" {
		t.Errorf("expected slide ID 'slide-abc', got '%s'", capturedSlideID)
	}
}

func TestAddVideo_NoPositionNoSize(t *testing.T) {
	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
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

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		// No position or size specified
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have CreateVideo request (no video properties update)
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.CreateVideo == nil {
		t.Fatal("expected CreateVideo request")
	}

	// Verify no position transform when not specified
	if req.CreateVideo.ElementProperties.Transform != nil {
		t.Error("expected no transform when position not specified")
	}

	// Verify no size when not specified
	if req.CreateVideo.ElementProperties.Size != nil {
		t.Error("expected no size when size not specified")
	}
}

func TestAddVideo_InvalidPresentationID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "", // Empty
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation_id")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestAddVideo_InvalidSlideReference(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     0,  // Invalid (should be 1-based or use SlideID)
		SlideID:        "", // Also not provided
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err == nil {
		t.Fatal("expected error for invalid slide reference")
	}

	if !errors.Is(err, ErrInvalidSlideReference) {
		t.Errorf("expected ErrInvalidSlideReference, got %v", err)
	}
}

func TestAddVideo_InvalidVideoSource(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "vimeo", // Invalid - must be 'youtube' or 'drive'
		VideoID:        "123456",
	})

	if err == nil {
		t.Fatal("expected error for invalid video source")
	}

	if !errors.Is(err, ErrInvalidVideoSource) {
		t.Errorf("expected ErrInvalidVideoSource, got %v", err)
	}
}

func TestAddVideo_EmptyVideoID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "", // Empty
	})

	if err == nil {
		t.Fatal("expected error for empty video ID")
	}

	if !errors.Is(err, ErrInvalidVideoID) {
		t.Errorf("expected ErrInvalidVideoID, got %v", err)
	}
}

func TestAddVideo_InvalidSize_NegativeWidth(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		Size:           &SizeInput{Width: -100, Height: 200}, // Negative width
	})

	if err == nil {
		t.Fatal("expected error for negative width")
	}

	if !errors.Is(err, ErrInvalidVideoSize) {
		t.Errorf("expected ErrInvalidVideoSize, got %v", err)
	}
}

func TestAddVideo_InvalidSize_ZeroHeight(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		Size:           &SizeInput{Width: 200, Height: 0}, // Zero height
	})

	if err == nil {
		t.Fatal("expected error for zero height")
	}

	if !errors.Is(err, ErrInvalidVideoSize) {
		t.Errorf("expected ErrInvalidVideoSize, got %v", err)
	}
}

func TestAddVideo_InvalidPosition_NegativeX(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		Position:       &PositionInput{X: -50, Y: 100}, // Negative X
	})

	if err == nil {
		t.Fatal("expected error for negative X position")
	}

	if !errors.Is(err, ErrInvalidVideoPosition) {
		t.Errorf("expected ErrInvalidVideoPosition, got %v", err)
	}
}

func TestAddVideo_InvalidStartTime_Negative(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	startTime := -5.0

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		StartTime:      &startTime,
	})

	if err == nil {
		t.Fatal("expected error for negative start time")
	}

	if !errors.Is(err, ErrInvalidVideoTime) {
		t.Errorf("expected ErrInvalidVideoTime, got %v", err)
	}
}

func TestAddVideo_InvalidEndTime_Negative(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	endTime := -10.0

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		EndTime:        &endTime,
	})

	if err == nil {
		t.Fatal("expected error for negative end time")
	}

	if !errors.Is(err, ErrInvalidVideoTime) {
		t.Errorf("expected ErrInvalidVideoTime, got %v", err)
	}
}

func TestAddVideo_InvalidTimeRange_EndBeforeStart(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	startTime := 60.0
	endTime := 30.0 // End before start

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		StartTime:      &startTime,
		EndTime:        &endTime,
	})

	if err == nil {
		t.Fatal("expected error for end time before start time")
	}

	if !errors.Is(err, ErrInvalidVideoTimeRange) {
		t.Errorf("expected ErrInvalidVideoTimeRange, got %v", err)
	}
}

func TestAddVideo_PresentationNotFound(t *testing.T) {
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

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "non-existent",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err == nil {
		t.Fatal("expected error for non-existent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestAddVideo_AccessDenied(t *testing.T) {
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

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "protected",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestAddVideo_SlideNotFound(t *testing.T) {
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

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     5, // Out of range
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err == nil {
		t.Fatal("expected error for slide not found")
	}

	if !errors.Is(err, ErrSlideNotFound) {
		t.Errorf("expected ErrSlideNotFound, got %v", err)
	}
}

func TestAddVideo_BatchUpdateFailed(t *testing.T) {
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

	tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrAddVideoFailed) {
		t.Errorf("expected ErrAddVideoFailed, got %v", err)
	}
}

func TestAddVideo_VideoSourceCaseInsensitive(t *testing.T) {
	tests := []struct {
		name           string
		videoSource    string
		expectedSource string
	}{
		{"lowercase youtube", "youtube", "YOUTUBE"},
		{"uppercase YOUTUBE", "YOUTUBE", "YOUTUBE"},
		{"mixed case YouTube", "YouTube", "YOUTUBE"},
		{"lowercase drive", "drive", "DRIVE"},
		{"uppercase DRIVE", "DRIVE", "DRIVE"},
		{"mixed case Drive", "Drive", "DRIVE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedSource string

			mockSlides := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return &slides.Presentation{
						PresentationId: "test-presentation",
						Slides:         []*slides.Page{{ObjectId: "slide-1"}},
					}, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					if len(requests) > 0 && requests[0].CreateVideo != nil {
						capturedSource = requests[0].CreateVideo.Source
					}
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlides, nil
			}

			tools := NewToolsWithDrive(DefaultToolsConfig(), slidesFactory, nil)
			tokenSource := &mockTokenSource{}

			_, err := tools.AddVideo(context.Background(), tokenSource, AddVideoInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				VideoSource:    tt.videoSource,
				VideoID:        "video-id",
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if capturedSource != tt.expectedSource {
				t.Errorf("expected source '%s', got '%s'", tt.expectedSource, capturedSource)
			}
		})
	}
}

// Tests for helper functions
func TestGenerateVideoObjectID(t *testing.T) {
	originalTimeFunc := videoTimeNowFunc
	videoTimeNowFunc = func() time.Time {
		return time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	}
	defer func() { videoTimeNowFunc = originalTimeFunc }()

	objectID := generateVideoObjectID()
	if objectID == "" {
		t.Error("expected non-empty object ID")
	}

	// Verify format contains expected prefix
	expected := "video_"
	if len(objectID) < len(expected) || objectID[:len(expected)] != expected {
		t.Errorf("expected object ID to start with '%s', got: %s", expected, objectID)
	}
}

func TestBuildVideoRequests_NoVideoProperties(t *testing.T) {
	input := AddVideoInput{
		PresentationID: "test",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		// No start/end times, no autoplay, no mute
	}

	requests := buildVideoRequests("video-1", "slide-1", "YOUTUBE", input)

	// Should only have CreateVideo request
	if len(requests) != 1 {
		t.Errorf("expected 1 request, got %d", len(requests))
	}

	if requests[0].CreateVideo == nil {
		t.Error("expected CreateVideo request")
	}
}

func TestBuildVideoRequests_WithAllVideoProperties(t *testing.T) {
	startTime := 10.0
	endTime := 60.0

	input := AddVideoInput{
		PresentationID: "test",
		SlideIndex:     1,
		VideoSource:    "youtube",
		VideoID:        "dQw4w9WgXcQ",
		StartTime:      &startTime,
		EndTime:        &endTime,
		Autoplay:       true,
		Mute:           true,
	}

	requests := buildVideoRequests("video-1", "slide-1", "YOUTUBE", input)

	// Should have CreateVideo and UpdateVideoProperties requests
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	if requests[0].CreateVideo == nil {
		t.Error("expected CreateVideo request as first request")
	}

	if requests[1].UpdateVideoProperties == nil {
		t.Fatal("expected UpdateVideoProperties request as second request")
	}

	updateReq := requests[1].UpdateVideoProperties
	if updateReq.VideoProperties.Start != 10000 {
		t.Errorf("expected start time 10000ms, got %d", updateReq.VideoProperties.Start)
	}
	if updateReq.VideoProperties.End != 60000 {
		t.Errorf("expected end time 60000ms, got %d", updateReq.VideoProperties.End)
	}
	if !updateReq.VideoProperties.AutoPlay {
		t.Error("expected AutoPlay to be true")
	}
	if !updateReq.VideoProperties.Mute {
		t.Error("expected Mute to be true")
	}
}

func TestBuildVideoPropertiesRequest_NoProperties(t *testing.T) {
	input := AddVideoInput{
		// No start/end times, autoplay false, mute false
	}

	req := buildVideoPropertiesRequest("video-1", input)

	if req != nil {
		t.Error("expected nil request when no properties to set")
	}
}

func TestBuildVideoPropertiesRequest_OnlyAutoplay(t *testing.T) {
	input := AddVideoInput{
		Autoplay: true,
	}

	req := buildVideoPropertiesRequest("video-1", input)

	if req == nil {
		t.Fatal("expected non-nil request")
	}

	if req.UpdateVideoProperties == nil {
		t.Fatal("expected UpdateVideoProperties request")
	}

	if !req.UpdateVideoProperties.VideoProperties.AutoPlay {
		t.Error("expected AutoPlay to be true")
	}

	// Verify fields mask
	if req.UpdateVideoProperties.Fields != "autoPlay" {
		t.Errorf("expected fields 'autoPlay', got '%s'", req.UpdateVideoProperties.Fields)
	}
}
