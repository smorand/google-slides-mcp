package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for set_background tool.
var (
	ErrSetBackgroundFailed    = errors.New("failed to set background")
	ErrInvalidBackgroundType  = errors.New("invalid background type")
	ErrMissingBackgroundColor = errors.New("color is required for solid background")
	ErrMissingGradientColors  = errors.New("start_color and end_color are required for gradient background")
	ErrInvalidGradientAngle   = errors.New("gradient angle must be between 0 and 360")
)

// SetBackgroundInput represents the input for the set_background tool.
type SetBackgroundInput struct {
	PresentationID string `json:"presentation_id"`           // Required
	Scope          string `json:"scope"`                     // Required: "slide" or "all"
	SlideIndex     int    `json:"slide_index,omitempty"`     // 1-based, required when scope is "slide"
	SlideID        string `json:"slide_id,omitempty"`        // Alternative to slide_index
	BackgroundType string `json:"background_type"`           // Required: "solid", "image", or "gradient"

	// For solid background
	Color string `json:"color,omitempty"` // Hex color (e.g., "#FF0000")

	// For image background
	ImageBase64 string `json:"image_base64,omitempty"` // Base64 encoded image data

	// For gradient background
	StartColor string   `json:"start_color,omitempty"` // Hex color for gradient start
	EndColor   string   `json:"end_color,omitempty"`   // Hex color for gradient end
	Angle      *float64 `json:"angle,omitempty"`       // Degrees (0-360), default 0 (left to right)
}

// SetBackgroundOutput represents the output of the set_background tool.
type SetBackgroundOutput struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message"`
	AffectedSlides []string `json:"affected_slides"` // Slide IDs that were modified
}

// SetBackground sets the background for one or all slides.
func (t *Tools) SetBackground(ctx context.Context, tokenSource oauth2.TokenSource, input SetBackgroundInput) (*SetBackgroundOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	// Normalize scope
	scope := strings.ToLower(strings.TrimSpace(input.Scope))
	if scope != "slide" && scope != "all" {
		return nil, fmt.Errorf("%w: scope must be 'slide' or 'all', got '%s'", ErrInvalidScope, input.Scope)
	}

	// Normalize background type
	bgType := strings.ToLower(strings.TrimSpace(input.BackgroundType))
	if bgType != "solid" && bgType != "image" && bgType != "gradient" {
		return nil, fmt.Errorf("%w: background_type must be 'solid', 'image', or 'gradient', got '%s'", ErrInvalidBackgroundType, input.BackgroundType)
	}

	// Validate scope-specific parameters
	if scope == "slide" && input.SlideIndex == 0 && input.SlideID == "" {
		return nil, fmt.Errorf("%w: slide_index or slide_id is required when scope is 'slide'", ErrInvalidSlideReference)
	}

	// Validate background type-specific parameters
	switch bgType {
	case "solid":
		if input.Color == "" {
			return nil, ErrMissingBackgroundColor
		}
		if parseHexColor(input.Color) == nil {
			return nil, fmt.Errorf("%w: invalid color format '%s'", ErrMissingBackgroundColor, input.Color)
		}
	case "image":
		if input.ImageBase64 == "" {
			return nil, fmt.Errorf("%w: image_base64 is required for image background", ErrInvalidImageData)
		}
	case "gradient":
		if input.StartColor == "" || input.EndColor == "" {
			return nil, ErrMissingGradientColors
		}
		if parseHexColor(input.StartColor) == nil {
			return nil, fmt.Errorf("%w: invalid start_color format '%s'", ErrMissingGradientColors, input.StartColor)
		}
		if parseHexColor(input.EndColor) == nil {
			return nil, fmt.Errorf("%w: invalid end_color format '%s'", ErrMissingGradientColors, input.EndColor)
		}
		if input.Angle != nil && (*input.Angle < 0 || *input.Angle > 360) {
			return nil, ErrInvalidGradientAngle
		}
	}

	t.config.Logger.Info("setting background",
		slog.String("presentation_id", input.PresentationID),
		slog.String("scope", scope),
		slog.String("background_type", bgType),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation
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

	// Determine which slides to update
	var targetSlideIDs []string
	if scope == "all" {
		for _, slide := range presentation.Slides {
			targetSlideIDs = append(targetSlideIDs, slide.ObjectId)
		}
	} else {
		// Find the specific slide
		slideID, _, err := findSlide(presentation, input.SlideIndex, input.SlideID)
		if err != nil {
			return nil, err
		}
		targetSlideIDs = []string{slideID}
	}

	// Build background property based on type
	var pageBackgroundFill *slides.PageBackgroundFill
	var driveFileID string

	switch bgType {
	case "solid":
		rgb := parseHexColor(input.Color)
		pageBackgroundFill = &slides.PageBackgroundFill{
			SolidFill: &slides.SolidFill{
				Color: &slides.OpaqueColor{
					RgbColor: rgb,
				},
			},
		}
	case "image":
		// Decode and upload image
		imageData, err := base64.StdEncoding.DecodeString(input.ImageBase64)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidImageData, err)
		}

		mimeType := detectImageMimeType(imageData)
		if mimeType == "" {
			return nil, fmt.Errorf("%w: unable to detect image format", ErrInvalidImageData)
		}

		// Create Drive service to upload image
		driveService, err := t.driveServiceFactory(ctx, tokenSource)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
		}

		// Upload image to Drive
		fileName := generateBackgroundFileName()
		uploadedFile, err := driveService.UploadFile(ctx, fileName, mimeType, bytes.NewReader(imageData))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrImageUploadFailed, err)
		}
		driveFileID = uploadedFile.Id

		// Make the file publicly accessible so Slides can read it
		err = driveService.MakeFilePublic(ctx, driveFileID)
		if err != nil {
			t.config.Logger.Warn("failed to make background image public",
				slog.String("file_id", driveFileID),
				slog.String("error", err.Error()),
			)
		}

		imageURL := fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download", driveFileID)
		pageBackgroundFill = &slides.PageBackgroundFill{
			StretchedPictureFill: &slides.StretchedPictureFill{
				ContentUrl: imageURL,
			},
		}
	case "gradient":
		// Build gradient stops
		angle := 0.0
		if input.Angle != nil {
			angle = *input.Angle
		}

		startRgb := parseHexColor(input.StartColor)
		endRgb := parseHexColor(input.EndColor)

		// Note: Google Slides API doesn't directly support gradient backgrounds via UpdatePageProperties.
		// The StretchedPictureFill and SolidFill are the only supported fill types.
		// Gradient backgrounds can only be applied through the UI, not the API.
		//
		// However, we can work around this by creating a gradient image and using it as
		// a stretched picture fill. For now, we'll return an error explaining the limitation.
		//
		// Alternative approach: we could generate a gradient PNG and upload it, then use
		// StretchedPictureFill. Let's implement that approach.

		// Generate gradient image
		gradientImageData, err := generateGradientImage(startRgb, endRgb, angle)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to generate gradient image: %v", ErrSetBackgroundFailed, err)
		}

		// Upload gradient image to Drive
		driveService, err := t.driveServiceFactory(ctx, tokenSource)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
		}

		fileName := generateBackgroundFileName()
		uploadedFile, err := driveService.UploadFile(ctx, fileName, "image/png", bytes.NewReader(gradientImageData))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrImageUploadFailed, err)
		}
		driveFileID = uploadedFile.Id

		// Make the file publicly accessible
		err = driveService.MakeFilePublic(ctx, driveFileID)
		if err != nil {
			t.config.Logger.Warn("failed to make gradient image public",
				slog.String("file_id", driveFileID),
				slog.String("error", err.Error()),
			)
		}

		imageURL := fmt.Sprintf("https://drive.google.com/uc?id=%s&export=download", driveFileID)
		pageBackgroundFill = &slides.PageBackgroundFill{
			StretchedPictureFill: &slides.StretchedPictureFill{
				ContentUrl: imageURL,
			},
		}
	}

	// Build update requests for each target slide
	var requests []*slides.Request
	for _, slideID := range targetSlideIDs {
		requests = append(requests, &slides.Request{
			UpdatePageProperties: &slides.UpdatePagePropertiesRequest{
				ObjectId: slideID,
				PageProperties: &slides.PageProperties{
					PageBackgroundFill: pageBackgroundFill,
				},
				Fields: "pageBackgroundFill",
			},
		})
	}

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrSetBackgroundFailed, err)
	}

	// Build success message
	var message string
	switch bgType {
	case "solid":
		message = fmt.Sprintf("Solid background (%s) applied successfully", input.Color)
	case "image":
		message = "Image background applied successfully"
	case "gradient":
		message = fmt.Sprintf("Gradient background (%s to %s) applied successfully", input.StartColor, input.EndColor)
	}

	if scope == "all" {
		message += fmt.Sprintf(" to all %d slides", len(targetSlideIDs))
	} else {
		message += " to slide"
	}

	output := &SetBackgroundOutput{
		Success:        true,
		Message:        message,
		AffectedSlides: targetSlideIDs,
	}

	t.config.Logger.Info("background set successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("background_type", bgType),
		slog.Int("slides_affected", len(targetSlideIDs)),
	)

	return output, nil
}

// backgroundTimeNowFunc allows overriding the time function for tests.
var backgroundTimeNowFunc = imageTimeNowFunc

// generateBackgroundFileName generates a unique file name for the uploaded background image.
func generateBackgroundFileName() string {
	return fmt.Sprintf("slides_background_%d.png", backgroundTimeNowFunc().UnixNano())
}

// generateGradientImage creates a PNG image with a linear gradient.
// The angle is in degrees (0 = left to right, 90 = top to bottom).
func generateGradientImage(startColor, endColor *slides.RgbColor, angle float64) ([]byte, error) {
	// For simplicity, we generate a small gradient image (100x100 pixels)
	// that will be stretched to fill the slide background.
	// This approach works because the API stretches the image to fit.

	width := 100
	height := 100

	// Convert RgbColor (0-1 range) to 0-255 range
	startR := uint8(startColor.Red * 255)
	startG := uint8(startColor.Green * 255)
	startB := uint8(startColor.Blue * 255)
	endR := uint8(endColor.Red * 255)
	endG := uint8(endColor.Green * 255)
	endB := uint8(endColor.Blue * 255)

	// Create PNG image data manually using a simple approach
	// For now, we'll create a simple horizontal or vertical gradient based on angle
	// More complex angles would require rotation math, but we'll support the main cases

	// Normalize angle to 0-360
	for angle < 0 {
		angle += 360
	}
	for angle >= 360 {
		angle -= 360
	}

	// Determine gradient direction
	// 0 degrees = left to right
	// 90 degrees = top to bottom
	// 180 degrees = right to left
	// 270 degrees = bottom to top
	horizontal := angle < 45 || angle >= 315 || (angle >= 135 && angle < 225)
	reversed := (angle >= 135 && angle < 315)

	if angle >= 45 && angle < 135 {
		// Top to bottom
		horizontal = false
		reversed = false
	} else if angle >= 225 && angle < 315 {
		// Bottom to top
		horizontal = false
		reversed = true
	} else if angle >= 135 && angle < 225 {
		// Right to left
		horizontal = true
		reversed = true
	}

	// Generate raw RGBA pixel data
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var t float64
			if horizontal {
				t = float64(x) / float64(width-1)
			} else {
				t = float64(y) / float64(height-1)
			}

			if reversed {
				t = 1 - t
			}

			// Interpolate colors
			r := uint8(float64(startR)*(1-t) + float64(endR)*t)
			g := uint8(float64(startG)*(1-t) + float64(endG)*t)
			b := uint8(float64(startB)*(1-t) + float64(endB)*t)

			idx := (y*width + x) * 4
			pixels[idx] = r
			pixels[idx+1] = g
			pixels[idx+2] = b
			pixels[idx+3] = 255 // Alpha
		}
	}

	// Encode as PNG using a minimal PNG encoder
	return encodePNG(width, height, pixels)
}

// encodePNG encodes raw RGBA pixel data as a PNG image.
// This is a minimal implementation that creates a valid PNG without external dependencies.
func encodePNG(width, height int, pixels []byte) ([]byte, error) {
	var buf bytes.Buffer

	// PNG signature
	buf.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	// IHDR chunk
	ihdrData := make([]byte, 13)
	// Width (4 bytes big-endian)
	ihdrData[0] = byte(width >> 24)
	ihdrData[1] = byte(width >> 16)
	ihdrData[2] = byte(width >> 8)
	ihdrData[3] = byte(width)
	// Height (4 bytes big-endian)
	ihdrData[4] = byte(height >> 24)
	ihdrData[5] = byte(height >> 16)
	ihdrData[6] = byte(height >> 8)
	ihdrData[7] = byte(height)
	// Bit depth (8), Color type (6 = RGBA), Compression (0), Filter (0), Interlace (0)
	ihdrData[8] = 8  // bit depth
	ihdrData[9] = 6  // color type: RGBA
	ihdrData[10] = 0 // compression
	ihdrData[11] = 0 // filter
	ihdrData[12] = 0 // interlace

	writeChunk(&buf, "IHDR", ihdrData)

	// IDAT chunk (image data)
	// Add filter byte (0 = None) at the start of each row
	rawData := make([]byte, height*(1+width*4))
	for y := 0; y < height; y++ {
		rawData[y*(1+width*4)] = 0 // Filter byte
		copy(rawData[y*(1+width*4)+1:], pixels[y*width*4:(y+1)*width*4])
	}

	// Compress using zlib (deflate)
	compressedData, err := compressZlib(rawData)
	if err != nil {
		return nil, err
	}

	writeChunk(&buf, "IDAT", compressedData)

	// IEND chunk
	writeChunk(&buf, "IEND", nil)

	return buf.Bytes(), nil
}

// writeChunk writes a PNG chunk to the buffer.
func writeChunk(buf *bytes.Buffer, chunkType string, data []byte) {
	length := len(data)

	// Length (4 bytes big-endian)
	buf.WriteByte(byte(length >> 24))
	buf.WriteByte(byte(length >> 16))
	buf.WriteByte(byte(length >> 8))
	buf.WriteByte(byte(length))

	// Chunk type (4 bytes)
	buf.WriteString(chunkType)

	// Data
	if data != nil {
		buf.Write(data)
	}

	// CRC (4 bytes) - CRC32 of chunk type + data
	crcData := make([]byte, 4+len(data))
	copy(crcData[:4], chunkType)
	copy(crcData[4:], data)
	crc := crc32PNG(crcData)
	buf.WriteByte(byte(crc >> 24))
	buf.WriteByte(byte(crc >> 16))
	buf.WriteByte(byte(crc >> 8))
	buf.WriteByte(byte(crc))
}

// crc32PNG calculates CRC32 using the PNG polynomial.
func crc32PNG(data []byte) uint32 {
	// PNG uses CRC-32-IEEE (same as zlib)
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc = crc32Table[(crc^uint32(b))&0xFF] ^ (crc >> 8)
	}
	return crc ^ 0xFFFFFFFF
}

// crc32Table is the precomputed CRC32 table for PNG.
var crc32Table = [256]uint32{
	0x00000000, 0x77073096, 0xEE0E612C, 0x990951BA, 0x076DC419, 0x706AF48F, 0xE963A535, 0x9E6495A3,
	0x0EDB8832, 0x79DCB8A4, 0xE0D5E91E, 0x97D2D988, 0x09B64C2B, 0x7EB17CBD, 0xE7B82D07, 0x90BF1D91,
	0x1DB71064, 0x6AB020F2, 0xF3B97148, 0x84BE41DE, 0x1ADAD47D, 0x6DDDE4EB, 0xF4D4B551, 0x83D385C7,
	0x136C9856, 0x646BA8C0, 0xFD62F97A, 0x8A65C9EC, 0x14015C4F, 0x63066CD9, 0xFA0F3D63, 0x8D080DF5,
	0x3B6E20C8, 0x4C69105E, 0xD56041E4, 0xA2677172, 0x3C03E4D1, 0x4B04D447, 0xD20D85FD, 0xA50AB56B,
	0x35B5A8FA, 0x42B2986C, 0xDBBBC9D6, 0xACBCF940, 0x32D86CE3, 0x45DF5C75, 0xDCD60DCF, 0xABD13D59,
	0x26D930AC, 0x51DE003A, 0xC8D75180, 0xBFD06116, 0x21B4F4B5, 0x56B3C423, 0xCFBA9599, 0xB8BDA50F,
	0x2802B89E, 0x5F058808, 0xC60CD9B2, 0xB10BE924, 0x2F6F7C87, 0x58684C11, 0xC1611DAB, 0xB6662D3D,
	0x76DC4190, 0x01DB7106, 0x98D220BC, 0xEFD5102A, 0x71B18589, 0x06B6B51F, 0x9FBFE4A5, 0xE8B8D433,
	0x7807C9A2, 0x0F00F934, 0x9609A88E, 0xE10E9818, 0x7F6A0DBB, 0x086D3D2D, 0x91646C97, 0xE6635C01,
	0x6B6B51F4, 0x1C6C6162, 0x856530D8, 0xF262004E, 0x6C0695ED, 0x1B01A57B, 0x8208F4C1, 0xF50FC457,
	0x65B0D9C6, 0x12B7E950, 0x8BBEB8EA, 0xFCB9887C, 0x62DD1DDF, 0x15DA2D49, 0x8CD37CF3, 0xFBD44C65,
	0x4DB26158, 0x3AB551CE, 0xA3BC0074, 0xD4BB30E2, 0x4ADFA541, 0x3DD895D7, 0xA4D1C46D, 0xD3D6F4FB,
	0x4369E96A, 0x346ED9FC, 0xAD678846, 0xDA60B8D0, 0x44042D73, 0x33031DE5, 0xAA0A4C5F, 0xDD0D7CC9,
	0x5005713C, 0x270241AA, 0xBE0B1010, 0xC90C2086, 0x5768B525, 0x206F85B3, 0xB966D409, 0xCE61E49F,
	0x5EDEF90E, 0x29D9C998, 0xB0D09822, 0xC7D7A8B4, 0x59B33D17, 0x2EB40D81, 0xB7BD5C3B, 0xC0BA6CAD,
	0xEDB88320, 0x9ABFB3B6, 0x03B6E20C, 0x74B1D29A, 0xEAD54739, 0x9DD277AF, 0x04DB2615, 0x73DC1683,
	0xE3630B12, 0x94643B84, 0x0D6D6A3E, 0x7A6A5AA8, 0xE40ECF0B, 0x9309FF9D, 0x0A00AE27, 0x7D079EB1,
	0xF00F9344, 0x8708A3D2, 0x1E01F268, 0x6906C2FE, 0xF762575D, 0x806567CB, 0x196C3671, 0x6E6B06E7,
	0xFED41B76, 0x89D32BE0, 0x10DA7A5A, 0x67DD4ACC, 0xF9B9DF6F, 0x8EBEEFF9, 0x17B7BE43, 0x60B08ED5,
	0xD6D6A3E8, 0xA1D1937E, 0x38D8C2C4, 0x4FDFF252, 0xD1BB67F1, 0xA6BC5767, 0x3FB506DD, 0x48B2364B,
	0xD80D2BDA, 0xAF0A1B4C, 0x36034AF6, 0x41047A60, 0xDF60EFC3, 0xA867DF55, 0x316E8EEF, 0x4669BE79,
	0xCB61B38C, 0xBC66831A, 0x256FD2A0, 0x5268E236, 0xCC0C7795, 0xBB0B4703, 0x220216B9, 0x5505262F,
	0xC5BA3BBE, 0xB2BD0B28, 0x2BB45A92, 0x5CB36A04, 0xC2D7FFA7, 0xB5D0CF31, 0x2CD99E8B, 0x5BDEAE1D,
	0x9B64C2B0, 0xEC63F226, 0x756AA39C, 0x026D930A, 0x9C0906A9, 0xEB0E363F, 0x72076785, 0x05005713,
	0x95BF4A82, 0xE2B87A14, 0x7BB12BAE, 0x0CB61B38, 0x92D28E9B, 0xE5D5BE0D, 0x7CDCEFB7, 0x0BDBDF21,
	0x86D3D2D4, 0xF1D4E242, 0x68DDB3F8, 0x1FDA836E, 0x81BE16CD, 0xF6B9265B, 0x6FB077E1, 0x18B74777,
	0x88085AE6, 0xFF0F6A70, 0x66063BCA, 0x11010B5C, 0x8F659EFF, 0xF862AE69, 0x616BFFD3, 0x166CCF45,
	0xA00AE278, 0xD70DD2EE, 0x4E048354, 0x3903B3C2, 0xA7672661, 0xD06016F7, 0x4969474D, 0x3E6E77DB,
	0xAED16A4A, 0xD9D65ADC, 0x40DF0B66, 0x37D83BF0, 0xA9BCAE53, 0xDEBB9EC5, 0x47B2CF7F, 0x30B5FFE9,
	0xBDBDF21C, 0xCABAC28A, 0x53B39330, 0x24B4A3A6, 0xBAD03605, 0xCDD706B3, 0x54DE5729, 0x23D967BF,
	0xB3667A2E, 0xC4614AB8, 0x5D681B02, 0x2A6F2B94, 0xB40BBE37, 0xC30C8EA1, 0x5A05DF1B, 0x2D02EF8D,
}

// compressZlib compresses data using zlib (deflate with zlib wrapper).
func compressZlib(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	// Zlib header
	// CMF: deflate method (8), 32K window (7) => 0x78
	// FLG: check bits to make CMF*256+FLG divisible by 31 => 0x01 for FCHECK
	cmf := byte(0x78)
	flg := byte(0x01)
	buf.WriteByte(cmf)
	buf.WriteByte(flg)

	// Deflate compressed data (using store - no compression for simplicity)
	// For a proper implementation, we'd use compress/zlib, but we want to avoid imports
	// Let's use stored blocks (no compression)
	compressedData := deflateStore(data)
	buf.Write(compressedData)

	// Adler-32 checksum of uncompressed data
	checksum := adler32(data)
	buf.WriteByte(byte(checksum >> 24))
	buf.WriteByte(byte(checksum >> 16))
	buf.WriteByte(byte(checksum >> 8))
	buf.WriteByte(byte(checksum))

	return buf.Bytes(), nil
}

// deflateStore creates stored (non-compressed) deflate blocks.
func deflateStore(data []byte) []byte {
	var buf bytes.Buffer

	remaining := len(data)
	offset := 0

	for remaining > 0 {
		// Maximum block size is 65535 bytes
		blockSize := remaining
		if blockSize > 65535 {
			blockSize = 65535
		}

		isFinal := remaining <= 65535

		// Block header byte: BFINAL (1 bit) + BTYPE (2 bits: 00 = stored)
		header := byte(0)
		if isFinal {
			header = 1 // BFINAL = 1
		}
		buf.WriteByte(header)

		// LEN and NLEN (little-endian)
		buf.WriteByte(byte(blockSize & 0xFF))
		buf.WriteByte(byte(blockSize >> 8))
		nlen := ^uint16(blockSize)
		buf.WriteByte(byte(nlen & 0xFF))
		buf.WriteByte(byte(nlen >> 8))

		// Data
		buf.Write(data[offset : offset+blockSize])

		offset += blockSize
		remaining -= blockSize
	}

	return buf.Bytes()
}

// adler32 calculates the Adler-32 checksum.
func adler32(data []byte) uint32 {
	a := uint32(1)
	b := uint32(0)

	for _, d := range data {
		a = (a + uint32(d)) % 65521
		b = (b + a) % 65521
	}

	return (b << 16) | a
}
