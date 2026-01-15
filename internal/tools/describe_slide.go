package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for describe_slide tool.
var (
	ErrInvalidSlideReference = errors.New("either slide_index or slide_id must be provided")
	ErrSlideNotFound         = errors.New("slide not found")
)

// DescribeSlideInput represents the input for the describe_slide tool.
type DescribeSlideInput struct {
	PresentationID string `json:"presentation_id"`
	SlideIndex     int    `json:"slide_index,omitempty"`     // 1-based index
	SlideID        string `json:"slide_id,omitempty"`        // Alternative to slide_index
}

// DescribeSlideOutput represents the output of the describe_slide tool.
type DescribeSlideOutput struct {
	PresentationID  string             `json:"presentation_id"`
	SlideID         string             `json:"slide_id"`
	SlideIndex      int                `json:"slide_index"`
	Title           string             `json:"title,omitempty"`
	LayoutType      string             `json:"layout_type,omitempty"`
	PageSize        *PageSize          `json:"page_size,omitempty"`
	Objects         []ObjectDescription `json:"objects"`
	LayoutDescription string           `json:"layout_description"`
	ScreenshotBase64  string           `json:"screenshot_base64,omitempty"`
	SpeakerNotes      string           `json:"speaker_notes,omitempty"`
}

// ObjectDescription provides detailed information about a page element.
type ObjectDescription struct {
	ObjectID       string         `json:"object_id"`
	ObjectType     string         `json:"object_type"`
	Position       *Position      `json:"position,omitempty"`
	Size           *Size          `json:"size,omitempty"`
	ContentSummary string         `json:"content_summary,omitempty"`
	ZOrder         int            `json:"z_order"`
	Children       []ObjectDescription `json:"children,omitempty"` // For groups
}

// Position represents the x, y coordinates in points.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Size represents width and height in points.
type Size struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// DescribeSlide returns detailed human-readable description of a slide.
func (t *Tools) DescribeSlide(ctx context.Context, tokenSource oauth2.TokenSource, input DescribeSlideInput) (*DescribeSlideOutput, error) {
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	t.config.Logger.Info("describing slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
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

	// Find the target slide
	var targetSlide *slides.Page
	var slideIndex int

	if input.SlideID != "" {
		// Find by slide_id
		for i, slide := range presentation.Slides {
			if slide.ObjectId == input.SlideID {
				targetSlide = slide
				slideIndex = i + 1 // 1-based
				break
			}
		}
	} else {
		// Find by slide_index (1-based)
		if input.SlideIndex < 1 || input.SlideIndex > len(presentation.Slides) {
			return nil, fmt.Errorf("%w: slide index %d out of range (1-%d)", ErrSlideNotFound, input.SlideIndex, len(presentation.Slides))
		}
		targetSlide = presentation.Slides[input.SlideIndex-1]
		slideIndex = input.SlideIndex
	}

	if targetSlide == nil {
		return nil, fmt.Errorf("%w: slide_id '%s' not found", ErrSlideNotFound, input.SlideID)
	}

	// Build output
	output := &DescribeSlideOutput{
		PresentationID: presentation.PresentationId,
		SlideID:        targetSlide.ObjectId,
		SlideIndex:     slideIndex,
	}

	// Get page size
	if presentation.PageSize != nil {
		output.PageSize = &PageSize{}
		if presentation.PageSize.Width != nil {
			output.PageSize.Width = &Dimension{
				Magnitude: presentation.PageSize.Width.Magnitude,
				Unit:      presentation.PageSize.Width.Unit,
			}
		}
		if presentation.PageSize.Height != nil {
			output.PageSize.Height = &Dimension{
				Magnitude: presentation.PageSize.Height.Magnitude,
				Unit:      presentation.PageSize.Height.Unit,
			}
		}
	}

	// Get layout type
	output.LayoutType = getLayoutType(targetSlide, presentation.Layouts)

	// Extract slide title
	output.Title = extractSlideTitle(targetSlide)

	// Extract speaker notes
	output.SpeakerNotes = extractSpeakerNotes(targetSlide)

	// Extract object descriptions
	output.Objects = extractObjectDescriptions(targetSlide.PageElements)

	// Generate layout description
	output.LayoutDescription = generateLayoutDescription(output.Objects, output.PageSize, output.Title)

	// Get slide screenshot
	thumbnail, err := slidesService.GetThumbnail(ctx, input.PresentationID, targetSlide.ObjectId)
	if err == nil && thumbnail != nil {
		thumbnailData, err := fetchThumbnailImage(ctx, thumbnail.ContentUrl)
		if err != nil {
			t.config.Logger.Warn("failed to fetch screenshot",
				slog.Int("slide_index", slideIndex),
				slog.Any("error", err),
			)
		} else {
			output.ScreenshotBase64 = base64.StdEncoding.EncodeToString(thumbnailData)
		}
	} else if err != nil {
		t.config.Logger.Warn("failed to get thumbnail for screenshot",
			slog.Int("slide_index", slideIndex),
			slog.Any("error", err),
		)
	}

	t.config.Logger.Info("slide described successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", slideIndex),
		slog.Int("object_count", len(output.Objects)),
	)

	return output, nil
}

// extractObjectDescriptions extracts detailed descriptions from page elements.
func extractObjectDescriptions(elements []*slides.PageElement) []ObjectDescription {
	var descriptions []ObjectDescription

	for i, element := range elements {
		if element == nil {
			continue
		}

		desc := ObjectDescription{
			ObjectID:   element.ObjectId,
			ObjectType: determineObjectType(element),
			ZOrder:     i, // Z-order based on position in the array
		}

		// Extract position
		if element.Transform != nil {
			desc.Position = &Position{
				X: emuToPoints(element.Transform.TranslateX),
				Y: emuToPoints(element.Transform.TranslateY),
			}
		}

		// Extract size
		if element.Size != nil {
			desc.Size = &Size{}
			if element.Size.Width != nil {
				desc.Size.Width = convertToPoints(element.Size.Width)
			}
			if element.Size.Height != nil {
				desc.Size.Height = convertToPoints(element.Size.Height)
			}
		}

		// Extract content summary
		desc.ContentSummary = extractContentSummary(element)

		// Handle groups recursively
		if element.ElementGroup != nil {
			desc.Children = extractObjectDescriptions(element.ElementGroup.Children)
		}

		descriptions = append(descriptions, desc)
	}

	return descriptions
}

// extractContentSummary extracts a summary of the element's content.
func extractContentSummary(element *slides.PageElement) string {
	if element == nil {
		return ""
	}

	switch {
	case element.Shape != nil:
		if element.Shape.Text != nil {
			text := extractTextFromTextContent(element.Shape.Text)
			return truncateText(text, 100)
		}
		if element.Shape.Placeholder != nil {
			return fmt.Sprintf("[%s placeholder]", element.Shape.Placeholder.Type)
		}
		return ""

	case element.Image != nil:
		summary := "Image"
		if element.Image.ContentUrl != "" {
			summary = "Image (external)"
		}
		return summary

	case element.Video != nil:
		if element.Video.Id != "" {
			if element.Video.Source == "YOUTUBE" {
				return fmt.Sprintf("YouTube video: %s", element.Video.Id)
			}
			return fmt.Sprintf("Video: %s", element.Video.Id)
		}
		return "Video"

	case element.Table != nil:
		rows := len(element.Table.TableRows)
		cols := 0
		if rows > 0 && len(element.Table.TableRows[0].TableCells) > 0 {
			cols = len(element.Table.TableRows[0].TableCells)
		}
		return fmt.Sprintf("Table (%dx%d)", rows, cols)

	case element.Line != nil:
		lineType := "Line"
		if element.Line.LineType != "" {
			lineType = element.Line.LineType
		}
		return lineType

	case element.SheetsChart != nil:
		return "Sheets chart"

	case element.WordArt != nil:
		return "Word art"

	case element.ElementGroup != nil:
		return fmt.Sprintf("Group (%d items)", len(element.ElementGroup.Children))

	default:
		return ""
	}
}

// truncateText truncates text to a maximum length, adding ellipsis if needed.
func truncateText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	// Replace newlines with spaces for a cleaner summary
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", "")

	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// emuToPoints converts EMU (English Metric Units) to points.
// 1 point = 12700 EMU
const emuPerPoint = 12700.0

func emuToPoints(emu float64) float64 {
	return emu / emuPerPoint
}

// convertToPoints converts a Dimension to points.
func convertToPoints(dim *slides.Dimension) float64 {
	if dim == nil {
		return 0
	}

	switch dim.Unit {
	case "PT":
		return dim.Magnitude
	case "EMU":
		return dim.Magnitude / emuPerPoint
	default:
		return dim.Magnitude
	}
}

// generateLayoutDescription creates a human-readable description of the slide layout.
func generateLayoutDescription(objects []ObjectDescription, pageSize *PageSize, title string) string {
	if len(objects) == 0 {
		return "Empty slide with no objects"
	}

	var parts []string

	// Determine page dimensions for layout analysis
	var pageWidth, pageHeight float64 = 720, 405 // Default slide size in points
	if pageSize != nil && pageSize.Width != nil && pageSize.Height != nil {
		pageWidth = pageSize.Width.Magnitude
		pageHeight = pageSize.Height.Magnitude
	}

	// Categorize objects by position
	var titleObjects, topObjects, centerObjects, bottomObjects, leftObjects, rightObjects []ObjectDescription

	for _, obj := range objects {
		if obj.Position == nil || obj.Size == nil {
			continue
		}

		// Calculate center position of object
		centerX := obj.Position.X + obj.Size.Width/2
		centerY := obj.Position.Y + obj.Size.Height/2

		// Determine vertical position (top third, middle, bottom third)
		if centerY < pageHeight/3 {
			// Check if it's a title-like position (top center, wide)
			if obj.Size.Width > pageWidth*0.5 && centerX > pageWidth*0.25 && centerX < pageWidth*0.75 {
				titleObjects = append(titleObjects, obj)
			} else {
				topObjects = append(topObjects, obj)
			}
		} else if centerY > pageHeight*2/3 {
			bottomObjects = append(bottomObjects, obj)
		} else {
			// Middle area - check horizontal position
			if centerX < pageWidth/3 {
				leftObjects = append(leftObjects, obj)
			} else if centerX > pageWidth*2/3 {
				rightObjects = append(rightObjects, obj)
			} else {
				centerObjects = append(centerObjects, obj)
			}
		}
	}

	// Build description
	if title != "" {
		parts = append(parts, fmt.Sprintf("Title at top: \"%s\"", truncateText(title, 50)))
	} else if len(titleObjects) > 0 {
		parts = append(parts, fmt.Sprintf("%d element(s) in title area at top", len(titleObjects)))
	}

	if len(topObjects) > 0 {
		parts = append(parts, fmt.Sprintf("%d element(s) at top", len(topObjects)))
	}

	if len(leftObjects) > 0 && len(rightObjects) > 0 {
		parts = append(parts, fmt.Sprintf("Two-column layout: %d element(s) on left, %d on right", len(leftObjects), len(rightObjects)))
	} else if len(leftObjects) > 0 {
		parts = append(parts, fmt.Sprintf("%d element(s) on left side", len(leftObjects)))
	} else if len(rightObjects) > 0 {
		parts = append(parts, fmt.Sprintf("%d element(s) on right side", len(rightObjects)))
	}

	if len(centerObjects) > 0 {
		parts = append(parts, fmt.Sprintf("%d element(s) in center", len(centerObjects)))
	}

	if len(bottomObjects) > 0 {
		parts = append(parts, fmt.Sprintf("%d element(s) at bottom", len(bottomObjects)))
	}

	// Add object type summary
	typeCounts := make(map[string]int)
	for _, obj := range objects {
		typeCounts[obj.ObjectType]++
	}

	var typeDescriptions []string
	for objType, count := range typeCounts {
		if count == 1 {
			typeDescriptions = append(typeDescriptions, fmt.Sprintf("1 %s", strings.ToLower(objType)))
		} else {
			typeDescriptions = append(typeDescriptions, fmt.Sprintf("%d %ss", count, strings.ToLower(objType)))
		}
	}

	// Sort for consistent output
	sort.Strings(typeDescriptions)

	if len(typeDescriptions) > 0 {
		parts = append(parts, fmt.Sprintf("Contains: %s", strings.Join(typeDescriptions, ", ")))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("Slide with %d objects", len(objects))
	}

	return strings.Join(parts, ". ")
}
