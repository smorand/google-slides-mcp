package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for add_video tool.
var (
	ErrAddVideoFailed       = errors.New("failed to add video")
	ErrInvalidVideoSource   = errors.New("invalid video source: must be 'youtube' or 'drive'")
	ErrInvalidVideoID       = errors.New("invalid video ID: video_id is required")
	ErrInvalidVideoSize     = errors.New("size must have positive width and height")
	ErrInvalidVideoPosition = errors.New("position coordinates must be non-negative")
	ErrInvalidVideoTime     = errors.New("invalid video time: must be non-negative")
	ErrInvalidVideoTimeRange = errors.New("invalid video time range: end_time must be greater than start_time")
)

// AddVideoInput represents the input for the add_video tool.
type AddVideoInput struct {
	PresentationID string              `json:"presentation_id"`
	SlideIndex     int                 `json:"slide_index,omitempty"` // 1-based index
	SlideID        string              `json:"slide_id,omitempty"`    // Alternative to slide_index
	VideoSource    string              `json:"video_source"`          // "youtube" or "drive"
	VideoID        string              `json:"video_id"`              // YouTube video ID or Drive file ID
	Position       *PositionInput      `json:"position,omitempty"`    // Position in points (default: 0, 0)
	Size           *SizeInput          `json:"size,omitempty"`        // Size in points (optional)
	StartTime      *float64            `json:"start_time,omitempty"`  // Start time in seconds (optional)
	EndTime        *float64            `json:"end_time,omitempty"`    // End time in seconds (optional)
	Autoplay       bool                `json:"autoplay"`              // Auto-play video (default: false)
	Mute           bool                `json:"mute"`                  // Mute video (default: false)
}

// AddVideoOutput represents the output of the add_video tool.
type AddVideoOutput struct {
	ObjectID string `json:"object_id"`
}

// videoTimeNowFunc allows overriding the time function for tests.
var videoTimeNowFunc = time.Now

// generateVideoObjectID generates a unique object ID for a new video element.
func generateVideoObjectID() string {
	return fmt.Sprintf("video_%d", videoTimeNowFunc().UnixNano())
}

// AddVideo adds a video to a slide.
func (t *Tools) AddVideo(ctx context.Context, tokenSource oauth2.TokenSource, input AddVideoInput) (*AddVideoOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	// Validate video source
	videoSource := strings.ToUpper(input.VideoSource)
	if videoSource != "YOUTUBE" && videoSource != "DRIVE" {
		return nil, ErrInvalidVideoSource
	}

	if input.VideoID == "" {
		return nil, ErrInvalidVideoID
	}

	// Validate size if provided
	if input.Size != nil {
		if input.Size.Width <= 0 || input.Size.Height <= 0 {
			return nil, ErrInvalidVideoSize
		}
	}

	// Validate position if provided
	if input.Position != nil {
		if input.Position.X < 0 || input.Position.Y < 0 {
			return nil, ErrInvalidVideoPosition
		}
	}

	// Validate time values if provided
	if input.StartTime != nil && *input.StartTime < 0 {
		return nil, fmt.Errorf("%w: start_time cannot be negative", ErrInvalidVideoTime)
	}
	if input.EndTime != nil && *input.EndTime < 0 {
		return nil, fmt.Errorf("%w: end_time cannot be negative", ErrInvalidVideoTime)
	}
	if input.StartTime != nil && input.EndTime != nil && *input.EndTime <= *input.StartTime {
		return nil, ErrInvalidVideoTimeRange
	}

	t.config.Logger.Info("adding video to slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.String("video_source", videoSource),
		slog.String("video_id", input.VideoID),
	)

	// Create slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find the target slide
	presentation, err := slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrSlidesAPIError, err)
	}

	// Find the target slide
	slideID, _, err := findSlide(presentation, input.SlideIndex, input.SlideID)
	if err != nil {
		return nil, err
	}

	// Generate a unique object ID for the video
	objectID := generateVideoObjectID()

	// Build the request to create the video
	requests := buildVideoRequests(objectID, slideID, videoSource, input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrAddVideoFailed, err)
	}

	output := &AddVideoOutput{
		ObjectID: objectID,
	}

	t.config.Logger.Info("video added successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.String("video_source", videoSource),
		slog.String("video_id", input.VideoID),
	)

	return output, nil
}

// buildVideoRequests creates the batch update requests to add a video.
func buildVideoRequests(objectID, slideID, videoSource string, input AddVideoInput) []*slides.Request {
	createVideoRequest := &slides.CreateVideoRequest{
		ObjectId: objectID,
		Source:   videoSource,
		Id:       input.VideoID,
		ElementProperties: &slides.PageElementProperties{
			PageObjectId: slideID,
		},
	}

	// Set position
	if input.Position != nil {
		createVideoRequest.ElementProperties.Transform = &slides.AffineTransform{
			ScaleX:     1,
			ScaleY:     1,
			TranslateX: pointsToEMU(input.Position.X),
			TranslateY: pointsToEMU(input.Position.Y),
			Unit:       "EMU",
		}
	}

	// Set size if provided
	if input.Size != nil {
		createVideoRequest.ElementProperties.Size = &slides.Size{
			Width: &slides.Dimension{
				Magnitude: pointsToEMU(input.Size.Width),
				Unit:      "EMU",
			},
			Height: &slides.Dimension{
				Magnitude: pointsToEMU(input.Size.Height),
				Unit:      "EMU",
			},
		}
	}

	requests := []*slides.Request{
		{
			CreateVideo: createVideoRequest,
		},
	}

	// Add video properties update if we have any video-specific properties
	if input.StartTime != nil || input.EndTime != nil || input.Autoplay || input.Mute {
		videoPropertiesRequest := buildVideoPropertiesRequest(objectID, input)
		if videoPropertiesRequest != nil {
			requests = append(requests, videoPropertiesRequest)
		}
	}

	return requests
}

// buildVideoPropertiesRequest creates an UpdateVideoPropertiesRequest for video playback settings.
func buildVideoPropertiesRequest(objectID string, input AddVideoInput) *slides.Request {
	videoProperties := &slides.VideoProperties{}
	var fields []string

	// Set start time (convert seconds to milliseconds)
	if input.StartTime != nil {
		videoProperties.Start = int64(*input.StartTime * 1000)
		fields = append(fields, "start")
	}

	// Set end time (convert seconds to milliseconds)
	if input.EndTime != nil {
		videoProperties.End = int64(*input.EndTime * 1000)
		fields = append(fields, "end")
	}

	// Set autoplay
	if input.Autoplay {
		videoProperties.AutoPlay = true
		fields = append(fields, "autoPlay")
	}

	// Set mute
	if input.Mute {
		videoProperties.Mute = true
		fields = append(fields, "mute")
	}

	if len(fields) == 0 {
		return nil
	}

	return &slides.Request{
		UpdateVideoProperties: &slides.UpdateVideoPropertiesRequest{
			ObjectId:        objectID,
			VideoProperties: videoProperties,
			Fields:          strings.Join(fields, ","),
		},
	}
}
