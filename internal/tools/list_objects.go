package tools

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// ListObjectsInput represents the input for the list_objects tool.
type ListObjectsInput struct {
	PresentationID string   `json:"presentation_id"`
	SlideIndices   []int    `json:"slide_indices,omitempty"`   // 1-based indices, optional - default all slides
	ObjectTypes    []string `json:"object_types,omitempty"`    // Filter by type: SHAPE, IMAGE, TABLE, VIDEO, LINE, etc.
}

// ListObjectsOutput represents the output of the list_objects tool.
type ListObjectsOutput struct {
	PresentationID string         `json:"presentation_id"`
	Objects        []ObjectListing `json:"objects"`
	TotalCount     int            `json:"total_count"`
	FilteredBy     *FilterInfo    `json:"filtered_by,omitempty"`
}

// ObjectListing provides information about an object for listing purposes.
type ObjectListing struct {
	SlideIndex     int       `json:"slide_index"`      // 1-based
	ObjectID       string    `json:"object_id"`
	ObjectType     string    `json:"object_type"`
	Position       *Position `json:"position,omitempty"`
	Size           *Size     `json:"size,omitempty"`
	ZOrder         int       `json:"z_order"`
	ContentPreview string    `json:"content_preview,omitempty"` // First 100 chars for text objects
}

// FilterInfo describes the filters applied to the listing.
type FilterInfo struct {
	SlideIndices []int    `json:"slide_indices,omitempty"`
	ObjectTypes  []string `json:"object_types,omitempty"`
}

// ListObjects lists all objects on slides with optional filtering.
func (t *Tools) ListObjects(ctx context.Context, tokenSource oauth2.TokenSource, input ListObjectsInput) (*ListObjectsOutput, error) {
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	t.config.Logger.Info("listing objects",
		slog.String("presentation_id", input.PresentationID),
		slog.Any("slide_indices", input.SlideIndices),
		slog.Any("object_types", input.ObjectTypes),
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

	// Build output
	output := &ListObjectsOutput{
		PresentationID: presentation.PresentationId,
		Objects:        []ObjectListing{},
	}

	// Add filter info if filters were applied
	if len(input.SlideIndices) > 0 || len(input.ObjectTypes) > 0 {
		output.FilteredBy = &FilterInfo{
			SlideIndices: input.SlideIndices,
			ObjectTypes:  input.ObjectTypes,
		}
	}

	// Build set of allowed slide indices (1-based)
	allowedSlideIndices := make(map[int]bool)
	if len(input.SlideIndices) > 0 {
		for _, idx := range input.SlideIndices {
			allowedSlideIndices[idx] = true
		}
	}

	// Build set of allowed object types (case-insensitive)
	allowedTypes := make(map[string]bool)
	if len(input.ObjectTypes) > 0 {
		for _, objType := range input.ObjectTypes {
			allowedTypes[objType] = true
		}
	}

	// Process each slide
	for slideIdx, slide := range presentation.Slides {
		slideIndex := slideIdx + 1 // 1-based

		// Check slide index filter
		if len(allowedSlideIndices) > 0 && !allowedSlideIndices[slideIndex] {
			continue
		}

		// Extract objects from this slide
		objects := extractObjectListings(slide.PageElements, slideIndex, allowedTypes)
		output.Objects = append(output.Objects, objects...)
	}

	output.TotalCount = len(output.Objects)

	t.config.Logger.Info("objects listed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("total_count", output.TotalCount),
	)

	return output, nil
}

// extractObjectListings extracts object listings from page elements.
func extractObjectListings(elements []*slides.PageElement, slideIndex int, allowedTypes map[string]bool) []ObjectListing {
	var listings []ObjectListing

	for zOrder, element := range elements {
		if element == nil {
			continue
		}

		// Get object type
		objectType := determineObjectType(element)

		// Check type filter
		if len(allowedTypes) > 0 && !allowedTypes[objectType] {
			// Also skip children if this is a group
			continue
		}

		listing := ObjectListing{
			SlideIndex: slideIndex,
			ObjectID:   element.ObjectId,
			ObjectType: objectType,
			ZOrder:     zOrder,
		}

		// Extract position
		if element.Transform != nil {
			listing.Position = &Position{
				X: emuToPoints(element.Transform.TranslateX),
				Y: emuToPoints(element.Transform.TranslateY),
			}
		}

		// Extract size
		if element.Size != nil {
			listing.Size = &Size{}
			if element.Size.Width != nil {
				listing.Size.Width = convertToPoints(element.Size.Width)
			}
			if element.Size.Height != nil {
				listing.Size.Height = convertToPoints(element.Size.Height)
			}
		}

		// Extract content preview for text objects
		listing.ContentPreview = extractContentPreview(element)

		listings = append(listings, listing)

		// Process groups recursively
		if element.ElementGroup != nil {
			childListings := extractObjectListingsFromGroup(element.ElementGroup.Children, slideIndex, zOrder, allowedTypes)
			listings = append(listings, childListings...)
		}
	}

	return listings
}

// extractObjectListingsFromGroup extracts object listings from group children.
func extractObjectListingsFromGroup(elements []*slides.PageElement, slideIndex int, parentZOrder int, allowedTypes map[string]bool) []ObjectListing {
	var listings []ObjectListing

	for childIdx, element := range elements {
		if element == nil {
			continue
		}

		// Get object type
		objectType := determineObjectType(element)

		// Check type filter
		if len(allowedTypes) > 0 && !allowedTypes[objectType] {
			continue
		}

		listing := ObjectListing{
			SlideIndex: slideIndex,
			ObjectID:   element.ObjectId,
			ObjectType: objectType,
			ZOrder:     parentZOrder*1000 + childIdx, // Nested z-order
		}

		// Extract position
		if element.Transform != nil {
			listing.Position = &Position{
				X: emuToPoints(element.Transform.TranslateX),
				Y: emuToPoints(element.Transform.TranslateY),
			}
		}

		// Extract size
		if element.Size != nil {
			listing.Size = &Size{}
			if element.Size.Width != nil {
				listing.Size.Width = convertToPoints(element.Size.Width)
			}
			if element.Size.Height != nil {
				listing.Size.Height = convertToPoints(element.Size.Height)
			}
		}

		// Extract content preview for text objects
		listing.ContentPreview = extractContentPreview(element)

		listings = append(listings, listing)

		// Process nested groups recursively
		if element.ElementGroup != nil {
			childListings := extractObjectListingsFromGroup(element.ElementGroup.Children, slideIndex, listing.ZOrder, allowedTypes)
			listings = append(listings, childListings...)
		}
	}

	return listings
}

// extractContentPreview extracts a preview of text content (first 100 chars).
func extractContentPreview(element *slides.PageElement) string {
	if element == nil {
		return ""
	}

	var text string

	switch {
	case element.Shape != nil && element.Shape.Text != nil:
		text = extractTextFromTextContent(element.Shape.Text)
	case element.Table != nil:
		// For tables, extract first cell content as preview
		text = extractFirstTableCellContent(element.Table)
	default:
		return ""
	}

	return truncateText(text, 100)
}

// extractFirstTableCellContent extracts text from the first non-empty cell.
func extractFirstTableCellContent(table *slides.Table) string {
	if table == nil || len(table.TableRows) == 0 {
		return ""
	}

	for _, row := range table.TableRows {
		if row == nil {
			continue
		}
		for _, cell := range row.TableCells {
			if cell == nil || cell.Text == nil {
				continue
			}
			text := extractTextFromTextContent(cell.Text)
			if text != "" {
				return text
			}
		}
	}

	return ""
}
