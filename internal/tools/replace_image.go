package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for replace_image tool.
var (
	ErrReplaceImageFailed = errors.New("failed to replace image")
)

// ReplaceImageInput represents the input for the replace_image tool.
type ReplaceImageInput struct {
	PresentationID string `json:"presentation_id"`
	ObjectID       string `json:"object_id"`
	ImageBase64    string `json:"image_base64"`
	PreserveSize   *bool  `json:"preserve_size,omitempty"` // Default true
}

// ReplaceImageOutput represents the output of the replace_image tool.
type ReplaceImageOutput struct {
	ObjectID     string `json:"object_id"`
	NewObjectID  string `json:"new_object_id,omitempty"` // Only set if object ID changed
	PreservedSize bool  `json:"preserved_size"`
}

// ReplaceImage replaces an existing image with a new one.
func (t *Tools) ReplaceImage(ctx context.Context, tokenSource oauth2.TokenSource, input ReplaceImageInput) (*ReplaceImageOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}
	if input.ImageBase64 == "" {
		return nil, fmt.Errorf("%w: image_base64 is required", ErrInvalidImageData)
	}

	// Default preserve_size to true
	preserveSize := true
	if input.PreserveSize != nil {
		preserveSize = *input.PreserveSize
	}

	t.config.Logger.Info("replacing image",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Bool("preserve_size", preserveSize),
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

	// Get the presentation to find the target image
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

	// Find the image object and its slide
	var targetElement *slides.PageElement
	var slideID string
	for _, slide := range presentation.Slides {
		element := findElementByID(slide.PageElements, input.ObjectID)
		if element != nil {
			targetElement = element
			slideID = slide.ObjectId
			break
		}
	}

	if targetElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Verify it's an image
	if targetElement.Image == nil {
		return nil, fmt.Errorf("%w: object '%s' is not an image (type: %s)", ErrNotImageObject, input.ObjectID, determineObjectType(targetElement))
	}

	// Upload the new image to Drive
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

	// Build the replacement requests
	requests, newObjectID := buildReplaceImageRequests(input.ObjectID, slideID, uploadedFile.Id, targetElement, preserveSize)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrReplaceImageFailed, err)
	}

	output := &ReplaceImageOutput{
		ObjectID:      input.ObjectID,
		PreservedSize: preserveSize,
	}

	// If the object ID changed (new image created), include it
	if newObjectID != input.ObjectID {
		output.NewObjectID = newObjectID
	}

	t.config.Logger.Info("image replaced successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("original_object_id", input.ObjectID),
		slog.String("new_object_id", newObjectID),
		slog.String("drive_file_id", uploadedFile.Id),
		slog.Bool("preserved_size", preserveSize),
	)

	return output, nil
}

// buildReplaceImageRequests creates the batch update requests to replace an image.
// The strategy is: delete the old image, create a new one at the same position/size.
func buildReplaceImageRequests(objectID, slideID, driveFileID string, oldElement *slides.PageElement, preserveSize bool) ([]*slides.Request, string) {
	// Generate a new object ID for the replacement image
	newObjectID := generateImageObjectID()

	// Create the image URL from Drive file ID
	imageURL := fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download", driveFileID)

	// Build the create image request with same position as the old element
	createImageRequest := &slides.CreateImageRequest{
		ObjectId: newObjectID,
		Url:      imageURL,
		ElementProperties: &slides.PageElementProperties{
			PageObjectId: slideID,
		},
	}

	// Copy transform (position) from old element
	if oldElement.Transform != nil {
		createImageRequest.ElementProperties.Transform = &slides.AffineTransform{
			ScaleX:     oldElement.Transform.ScaleX,
			ScaleY:     oldElement.Transform.ScaleY,
			TranslateX: oldElement.Transform.TranslateX,
			TranslateY: oldElement.Transform.TranslateY,
			ShearX:     oldElement.Transform.ShearX,
			ShearY:     oldElement.Transform.ShearY,
			Unit:       oldElement.Transform.Unit,
		}
		if createImageRequest.ElementProperties.Transform.Unit == "" {
			createImageRequest.ElementProperties.Transform.Unit = "EMU"
		}
	}

	// Copy size from old element if preserveSize is true
	if preserveSize && oldElement.Size != nil {
		createImageRequest.ElementProperties.Size = &slides.Size{}
		if oldElement.Size.Width != nil {
			createImageRequest.ElementProperties.Size.Width = &slides.Dimension{
				Magnitude: oldElement.Size.Width.Magnitude,
				Unit:      oldElement.Size.Width.Unit,
			}
		}
		if oldElement.Size.Height != nil {
			createImageRequest.ElementProperties.Size.Height = &slides.Dimension{
				Magnitude: oldElement.Size.Height.Magnitude,
				Unit:      oldElement.Size.Height.Unit,
			}
		}
	}

	// First delete the old image, then create the new one
	requests := []*slides.Request{
		{
			DeleteObject: &slides.DeleteObjectRequest{
				ObjectId: objectID,
			},
		},
		{
			CreateImage: createImageRequest,
		},
	}

	return requests, newObjectID
}
