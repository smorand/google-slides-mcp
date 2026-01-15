package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for add_image tool.
var (
	ErrAddImageFailed       = errors.New("failed to add image")
	ErrInvalidImageData     = errors.New("invalid image data: base64 decoding failed")
	ErrImageUploadFailed    = errors.New("failed to upload image to Drive")
	ErrInvalidImageSize     = errors.New("size must have positive width and/or height")
	ErrInvalidImagePosition = errors.New("position coordinates must be non-negative")
)

// AddImageInput represents the input for the add_image tool.
type AddImageInput struct {
	PresentationID string           `json:"presentation_id"`
	SlideIndex     int              `json:"slide_index,omitempty"` // 1-based index
	SlideID        string           `json:"slide_id,omitempty"`    // Alternative to slide_index
	ImageBase64    string           `json:"image_base64"`          // Base64 encoded image data
	Position       *PositionInput   `json:"position,omitempty"`    // Position in points (default: 0, 0)
	Size           *ImageSizeInput  `json:"size,omitempty"`        // Size in points (optional)
}

// ImageSizeInput represents width and height for image sizing.
// If only one dimension is provided, aspect ratio is preserved.
type ImageSizeInput struct {
	Width  *float64 `json:"width,omitempty"`  // Width in points (optional)
	Height *float64 `json:"height,omitempty"` // Height in points (optional)
}

// AddImageOutput represents the output of the add_image tool.
type AddImageOutput struct {
	ObjectID string `json:"object_id"`
}

// AddImage adds an image to a slide.
func (t *Tools) AddImage(ctx context.Context, tokenSource oauth2.TokenSource, input AddImageInput) (*AddImageOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	if input.ImageBase64 == "" {
		return nil, fmt.Errorf("%w: image_base64 is required", ErrInvalidImageData)
	}

	// Validate size if provided
	if input.Size != nil {
		if (input.Size.Width != nil && *input.Size.Width <= 0) ||
			(input.Size.Height != nil && *input.Size.Height <= 0) {
			return nil, ErrInvalidImageSize
		}
	}

	// Validate position if provided
	if input.Position != nil {
		if input.Position.X < 0 || input.Position.Y < 0 {
			return nil, ErrInvalidImagePosition
		}
	}

	t.config.Logger.Info("adding image to slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.Int("image_data_length", len(input.ImageBase64)),
	)

	// Decode base64 image data
	imageData, err := base64.StdEncoding.DecodeString(input.ImageBase64)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidImageData, err)
	}

	// Detect image MIME type from magic bytes
	mimeType := detectImageMimeType(imageData)
	if mimeType == "" {
		return nil, fmt.Errorf("%w: unable to detect image format", ErrInvalidImageData)
	}

	// Create services
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
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

	// Upload image to Drive
	fileName := generateImageFileName()
	uploadedFile, err := driveService.UploadFile(ctx, fileName, mimeType, bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImageUploadFailed, err)
	}

	// Make the file publicly accessible so Slides can read it
	err = driveService.MakeFilePublic(ctx, uploadedFile.Id)
	if err != nil {
		t.config.Logger.Warn("failed to make image public, image may not display",
			slog.String("file_id", uploadedFile.Id),
			slog.String("error", err.Error()),
		)
	}

	// Generate a unique object ID for the image
	objectID := generateImageObjectID()

	// Build the request to create the image
	requests := buildImageRequests(objectID, slideID, uploadedFile.Id, input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrAddImageFailed, err)
	}

	output := &AddImageOutput{
		ObjectID: objectID,
	}

	t.config.Logger.Info("image added successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.String("drive_file_id", uploadedFile.Id),
	)

	return output, nil
}

// detectImageMimeType detects the MIME type from image magic bytes.
func detectImageMimeType(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// GIF: GIF87a or GIF89a
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	// BMP: BM
	if data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}

	return ""
}

// imageTimeNowFunc allows overriding the time function for tests.
var imageTimeNowFunc = time.Now

// generateImageFileName generates a unique file name for the uploaded image.
func generateImageFileName() string {
	return fmt.Sprintf("slides_image_%d", imageTimeNowFunc().UnixNano())
}

// generateImageObjectID generates a unique object ID for a new image element.
func generateImageObjectID() string {
	return fmt.Sprintf("image_%d", imageTimeNowFunc().UnixNano())
}

// buildImageRequests creates the batch update requests to add an image.
func buildImageRequests(objectID, slideID, driveFileID string, input AddImageInput) []*slides.Request {
	// Create the image URL from Drive file ID
	imageURL := fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download", driveFileID)

	createImageRequest := &slides.CreateImageRequest{
		ObjectId: objectID,
		Url:      imageURL,
		ElementProperties: &slides.PageElementProperties{
			PageObjectId: slideID,
		},
	}

	// Set position
	if input.Position != nil {
		createImageRequest.ElementProperties.Transform = &slides.AffineTransform{
			ScaleX:     1,
			ScaleY:     1,
			TranslateX: pointsToEMU(input.Position.X),
			TranslateY: pointsToEMU(input.Position.Y),
			Unit:       "EMU",
		}
	}

	// Set size if provided
	if input.Size != nil {
		size := &slides.Size{}
		hasSize := false

		if input.Size.Width != nil {
			size.Width = &slides.Dimension{
				Magnitude: pointsToEMU(*input.Size.Width),
				Unit:      "EMU",
			}
			hasSize = true
		}

		if input.Size.Height != nil {
			size.Height = &slides.Dimension{
				Magnitude: pointsToEMU(*input.Size.Height),
				Unit:      "EMU",
			}
			hasSize = true
		}

		if hasSize {
			createImageRequest.ElementProperties.Size = size
		}
	}

	return []*slides.Request{
		{
			CreateImage: createImageRequest,
		},
	}
}

