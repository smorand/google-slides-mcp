package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestModifyVideo_StartEndTimes_Success(t *testing.T) {
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
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
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

	startTime := 30.0
	endTime := 120.0

	output, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectID != "video-1" {
		t.Errorf("expected object ID 'video-1', got '%s'", output.ObjectID)
	}

	// Verify modified properties
	if len(output.ModifiedProperties) != 2 {
		t.Errorf("expected 2 modified properties, got %d", len(output.ModifiedProperties))
	}

	// Verify UpdateVideoProperties request
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.UpdateVideoProperties == nil {
		t.Fatal("expected UpdateVideoProperties request")
	}

	// Verify start and end times are in milliseconds
	expectedStart := int64(30000)
	expectedEnd := int64(120000)

	if req.UpdateVideoProperties.VideoProperties.Start != expectedStart {
		t.Errorf("expected start time %d ms, got %d ms", expectedStart, req.UpdateVideoProperties.VideoProperties.Start)
	}
	if req.UpdateVideoProperties.VideoProperties.End != expectedEnd {
		t.Errorf("expected end time %d ms, got %d ms", expectedEnd, req.UpdateVideoProperties.VideoProperties.End)
	}
}

func TestModifyVideo_AutoplayAndMute_Success(t *testing.T) {
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
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
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

	autoplay := true
	mute := true

	output, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
			Mute:     &mute,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify modified properties contain autoplay and mute
	found := make(map[string]bool)
	for _, prop := range output.ModifiedProperties {
		found[prop] = true
	}
	if !found["autoplay"] {
		t.Error("expected 'autoplay' in modified properties")
	}
	if !found["mute"] {
		t.Error("expected 'mute' in modified properties")
	}

	// Verify UpdateVideoProperties request
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.UpdateVideoProperties == nil {
		t.Fatal("expected UpdateVideoProperties request")
	}

	if !req.UpdateVideoProperties.VideoProperties.AutoPlay {
		t.Error("expected AutoPlay to be true")
	}
	if !req.UpdateVideoProperties.VideoProperties.Mute {
		t.Error("expected Mute to be true")
	}
}

func TestModifyVideo_DisableAutoplayAndMute_Success(t *testing.T) {
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
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
									VideoProperties: &slides.VideoProperties{
										AutoPlay: true,
										Mute:     true,
									},
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

	autoplay := false
	mute := false

	output, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
			Mute:     &mute,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.ModifiedProperties) != 2 {
		t.Errorf("expected 2 modified properties, got %d", len(output.ModifiedProperties))
	}

	// Verify UpdateVideoProperties request sets values to false
	req := capturedRequests[0]
	if req.UpdateVideoProperties.VideoProperties.AutoPlay {
		t.Error("expected AutoPlay to be false")
	}
	if req.UpdateVideoProperties.VideoProperties.Mute {
		t.Error("expected Mute to be false")
	}
}

func TestModifyVideo_PositionAndSize_Success(t *testing.T) {
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
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
								},
								Transform: &slides.AffineTransform{
									ScaleX:     1.0,
									ScaleY:     1.0,
									TranslateX: 0,
									TranslateY: 0,
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: pointsToEMU(400), Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: pointsToEMU(225), Unit: "EMU"},
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

	output, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Position: &PositionInput{X: 100, Y: 50},
			Size:     &SizeInput{Width: 500, Height: 300},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify modified properties
	found := make(map[string]bool)
	for _, prop := range output.ModifiedProperties {
		found[prop] = true
	}
	if !found["position"] {
		t.Error("expected 'position' in modified properties")
	}
	if !found["size"] {
		t.Error("expected 'size' in modified properties")
	}

	// Verify UpdatePageElementTransformRequest
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.UpdatePageElementTransform == nil {
		t.Fatal("expected UpdatePageElementTransform request")
	}

	if req.UpdatePageElementTransform.ApplyMode != "ABSOLUTE" {
		t.Errorf("expected ApplyMode 'ABSOLUTE', got '%s'", req.UpdatePageElementTransform.ApplyMode)
	}

	// Verify position in EMU
	expectedX := pointsToEMU(100)
	expectedY := pointsToEMU(50)

	if req.UpdatePageElementTransform.Transform.TranslateX != expectedX {
		t.Errorf("expected TranslateX %f, got %f", expectedX, req.UpdatePageElementTransform.Transform.TranslateX)
	}
	if req.UpdatePageElementTransform.Transform.TranslateY != expectedY {
		t.Errorf("expected TranslateY %f, got %f", expectedY, req.UpdatePageElementTransform.Transform.TranslateY)
	}
}

func TestModifyVideo_AllProperties_Success(t *testing.T) {
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
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
								},
								Transform: &slides.AffineTransform{
									ScaleX:     1.0,
									ScaleY:     1.0,
									TranslateX: 0,
									TranslateY: 0,
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: pointsToEMU(400), Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: pointsToEMU(225), Unit: "EMU"},
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

	startTime := 10.0
	endTime := 60.0
	autoplay := true
	mute := false

	output, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Position:  &PositionInput{X: 100, Y: 50},
			Size:      &SizeInput{Width: 500, Height: 300},
			StartTime: &startTime,
			EndTime:   &endTime,
			Autoplay:  &autoplay,
			Mute:      &mute,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 6 modified properties
	if len(output.ModifiedProperties) != 6 {
		t.Errorf("expected 6 modified properties, got %d: %v", len(output.ModifiedProperties), output.ModifiedProperties)
	}

	// Should have 2 requests: transform and video properties
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(capturedRequests))
	}

	// Verify first request is transform
	if capturedRequests[0].UpdatePageElementTransform == nil {
		t.Error("expected first request to be UpdatePageElementTransform")
	}

	// Verify second request is video properties
	if capturedRequests[1].UpdateVideoProperties == nil {
		t.Error("expected second request to be UpdateVideoProperties")
	}
}

func TestModifyVideo_OnlyPositionPreservesCurrentValues(t *testing.T) {
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
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
								},
								Transform: &slides.AffineTransform{
									ScaleX:     2.0,
									ScaleY:     1.5,
									TranslateX: 100,
									TranslateY: 200,
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: pointsToEMU(400), Unit: "EMU"},
									Height: &slides.Dimension{Magnitude: pointsToEMU(225), Unit: "EMU"},
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

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Position: &PositionInput{X: 150, Y: 75},
			// No size change - should preserve existing scale
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	transform := capturedRequests[0].UpdatePageElementTransform.Transform

	// Position should be the new values
	expectedX := pointsToEMU(150)
	expectedY := pointsToEMU(75)
	if transform.TranslateX != expectedX || transform.TranslateY != expectedY {
		t.Errorf("expected position (%f, %f), got (%f, %f)", expectedX, expectedY, transform.TranslateX, transform.TranslateY)
	}

	// Scale should be preserved
	if transform.ScaleX != 2.0 {
		t.Errorf("expected ScaleX 2.0, got %f", transform.ScaleX)
	}
	if transform.ScaleY != 1.5 {
		t.Errorf("expected ScaleY 1.5, got %f", transform.ScaleY)
	}
}

func TestModifyVideo_InvalidPresentationID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	startTime := 30.0

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "", // Empty
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			StartTime: &startTime,
		},
	})

	if err == nil {
		t.Fatal("expected error for empty presentation_id")
	}

	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestModifyVideo_InvalidObjectID(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	startTime := 30.0

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "", // Empty
		Properties: &VideoModifyProperties{
			StartTime: &startTime,
		},
	})

	if err == nil {
		t.Fatal("expected error for empty object_id")
	}

	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestModifyVideo_NilProperties(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties:     nil, // Nil properties
	})

	if err == nil {
		t.Fatal("expected error for nil properties")
	}

	if !errors.Is(err, ErrNoVideoProperties) {
		t.Errorf("expected ErrNoVideoProperties, got %v", err)
	}
}

func TestModifyVideo_EmptyProperties(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties:     &VideoModifyProperties{}, // Empty properties
	})

	if err == nil {
		t.Fatal("expected error for empty properties")
	}

	if !errors.Is(err, ErrNoVideoProperties) {
		t.Errorf("expected ErrNoVideoProperties, got %v", err)
	}
}

func TestModifyVideo_InvalidNegativeStartTime(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	startTime := -5.0

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			StartTime: &startTime,
		},
	})

	if err == nil {
		t.Fatal("expected error for negative start time")
	}

	if !errors.Is(err, ErrInvalidVideoTime) {
		t.Errorf("expected ErrInvalidVideoTime, got %v", err)
	}
}

func TestModifyVideo_InvalidNegativeEndTime(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	endTime := -10.0

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			EndTime: &endTime,
		},
	})

	if err == nil {
		t.Fatal("expected error for negative end time")
	}

	if !errors.Is(err, ErrInvalidVideoTime) {
		t.Errorf("expected ErrInvalidVideoTime, got %v", err)
	}
}

func TestModifyVideo_InvalidTimeRange_EndBeforeStart(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	startTime := 60.0
	endTime := 30.0 // End before start

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	})

	if err == nil {
		t.Fatal("expected error for end time before start time")
	}

	if !errors.Is(err, ErrInvalidVideoTimeRange) {
		t.Errorf("expected ErrInvalidVideoTimeRange, got %v", err)
	}
}

func TestModifyVideo_InvalidSize_NegativeWidth(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Size: &SizeInput{Width: -100, Height: 200},
		},
	})

	if err == nil {
		t.Fatal("expected error for negative width")
	}

	if !errors.Is(err, ErrInvalidVideoSize) {
		t.Errorf("expected ErrInvalidVideoSize, got %v", err)
	}
}

func TestModifyVideo_InvalidSize_ZeroWidthAndHeight(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Size: &SizeInput{Width: 0, Height: 0},
		},
	})

	if err == nil {
		t.Fatal("expected error for zero width and height")
	}

	if !errors.Is(err, ErrInvalidVideoSize) {
		t.Errorf("expected ErrInvalidVideoSize, got %v", err)
	}
}

func TestModifyVideo_InvalidPosition_NegativeX(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Position: &PositionInput{X: -50, Y: 100},
		},
	})

	if err == nil {
		t.Fatal("expected error for negative X position")
	}

	if !errors.Is(err, ErrInvalidVideoPosition) {
		t.Errorf("expected ErrInvalidVideoPosition, got %v", err)
	}
}

func TestModifyVideo_InvalidPosition_NegativeY(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Position: &PositionInput{X: 100, Y: -50},
		},
	})

	if err == nil {
		t.Fatal("expected error for negative Y position")
	}

	if !errors.Is(err, ErrInvalidVideoPosition) {
		t.Errorf("expected ErrInvalidVideoPosition, got %v", err)
	}
}

func TestModifyVideo_PresentationNotFound(t *testing.T) {
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

	autoplay := true

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "non-existent",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
		},
	})

	if err == nil {
		t.Fatal("expected error for non-existent presentation")
	}

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestModifyVideo_AccessDenied(t *testing.T) {
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

	autoplay := true

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "protected",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
		},
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestModifyVideo_ObjectNotFound(t *testing.T) {
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

	autoplay := true

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "non-existent-video",
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
		},
	})

	if err == nil {
		t.Fatal("expected error for object not found")
	}

	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestModifyVideo_NotVideoObject(t *testing.T) {
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

	autoplay := true

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "shape-1", // Not a video
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
		},
	})

	if err == nil {
		t.Fatal("expected error for non-video object")
	}

	if !errors.Is(err, ErrNotVideoObject) {
		t.Errorf("expected ErrNotVideoObject, got %v", err)
	}
}

func TestModifyVideo_BatchUpdateFailed(t *testing.T) {
	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: "test-presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
								},
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

	autoplay := true

	_, err := tools.ModifyVideo(context.Background(), tokenSource, ModifyVideoInput{
		PresentationID: "test-presentation",
		ObjectID:       "video-1",
		Properties: &VideoModifyProperties{
			Autoplay: &autoplay,
		},
	})

	if err == nil {
		t.Fatal("expected error for batch update failure")
	}

	if !errors.Is(err, ErrModifyVideoFailed) {
		t.Errorf("expected ErrModifyVideoFailed, got %v", err)
	}
}

// Tests for helper functions
func TestValidateVideoModifyProperties_Valid(t *testing.T) {
	startTime := 30.0
	endTime := 60.0
	autoplay := true

	props := &VideoModifyProperties{
		Position:  &PositionInput{X: 100, Y: 50},
		Size:      &SizeInput{Width: 400, Height: 225},
		StartTime: &startTime,
		EndTime:   &endTime,
		Autoplay:  &autoplay,
	}

	err := validateVideoModifyProperties(props)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHasVideoPropertiesToModify_True(t *testing.T) {
	trueBool := true
	tests := []struct {
		name  string
		props *VideoModifyProperties
	}{
		{"position only", &VideoModifyProperties{Position: &PositionInput{X: 10, Y: 10}}},
		{"size only", &VideoModifyProperties{Size: &SizeInput{Width: 100, Height: 100}}},
		{"start time only", &VideoModifyProperties{StartTime: ptrFloat64(10)}},
		{"end time only", &VideoModifyProperties{EndTime: ptrFloat64(60)}},
		{"autoplay only", &VideoModifyProperties{Autoplay: &trueBool}},
		{"mute only", &VideoModifyProperties{Mute: &trueBool}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !hasVideoPropertiesToModify(tt.props) {
				t.Error("expected hasVideoPropertiesToModify to return true")
			}
		})
	}
}

func TestHasVideoPropertiesToModify_False(t *testing.T) {
	if hasVideoPropertiesToModify(nil) {
		t.Error("expected hasVideoPropertiesToModify(nil) to return false")
	}

	if hasVideoPropertiesToModify(&VideoModifyProperties{}) {
		t.Error("expected hasVideoPropertiesToModify({}) to return false")
	}
}

func TestBuildModifyVideoPropertiesRequest_NoFields(t *testing.T) {
	props := &VideoModifyProperties{}
	req, modifiedProps := buildModifyVideoPropertiesRequest("video-1", props)

	if req != nil {
		t.Error("expected nil request when no properties set")
	}
	if len(modifiedProps) != 0 {
		t.Error("expected empty modified properties when no properties set")
	}
}

func TestBuildModifyVideoPropertiesRequest_AllFields(t *testing.T) {
	startTime := 10.0
	endTime := 60.0
	autoplay := true
	mute := false

	props := &VideoModifyProperties{
		StartTime: &startTime,
		EndTime:   &endTime,
		Autoplay:  &autoplay,
		Mute:      &mute,
	}

	req, modifiedProps := buildModifyVideoPropertiesRequest("video-1", props)

	if req == nil {
		t.Fatal("expected non-nil request")
	}

	if req.UpdateVideoProperties == nil {
		t.Fatal("expected UpdateVideoProperties request")
	}

	// Verify fields
	if req.UpdateVideoProperties.ObjectId != "video-1" {
		t.Errorf("expected object ID 'video-1', got '%s'", req.UpdateVideoProperties.ObjectId)
	}

	// Verify values
	vp := req.UpdateVideoProperties.VideoProperties
	if vp.Start != 10000 {
		t.Errorf("expected Start 10000, got %d", vp.Start)
	}
	if vp.End != 60000 {
		t.Errorf("expected End 60000, got %d", vp.End)
	}
	if !vp.AutoPlay {
		t.Error("expected AutoPlay true")
	}
	if vp.Mute {
		t.Error("expected Mute false")
	}

	// Verify modified properties list
	if len(modifiedProps) != 4 {
		t.Errorf("expected 4 modified properties, got %d", len(modifiedProps))
	}
}
