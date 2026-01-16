package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for manage_hyperlinks tool.
var (
	ErrManageHyperlinksFailed = errors.New("failed to manage hyperlinks")
	ErrInvalidHyperlinkAction = errors.New("invalid action: must be 'list', 'add', or 'remove'")
	ErrInvalidHyperlinkURL    = errors.New("url is required for add action")
	ErrNoHyperlinkToRemove    = errors.New("no hyperlink found at specified range")
)

// ManageHyperlinksInput represents the input for the manage_hyperlinks tool.
type ManageHyperlinksInput struct {
	PresentationID string `json:"presentation_id"`
	Action         string `json:"action"` // "list", "add", "remove"

	// For list action
	Scope    string `json:"scope,omitempty"`     // "all", "slide", "object" - default "all"
	SlideID  string `json:"slide_id,omitempty"`  // Required when scope is "slide"
	ObjectID string `json:"object_id,omitempty"` // Required when scope is "object" or for add/remove

	// For add/remove actions on text
	StartIndex *int `json:"start_index,omitempty"` // For text link range
	EndIndex   *int `json:"end_index,omitempty"`   // For text link range

	// For add action
	URL string `json:"url,omitempty"` // External URL, internal slide link, or presentation link
}

// ManageHyperlinksOutput represents the output of the manage_hyperlinks tool.
type ManageHyperlinksOutput struct {
	PresentationID string          `json:"presentation_id"`
	Action         string          `json:"action"`
	Links          []HyperlinkInfo `json:"links,omitempty"`  // For list action
	Success        bool            `json:"success,omitempty"`
	Message        string          `json:"message,omitempty"`
}

// HyperlinkInfo represents information about a hyperlink.
type HyperlinkInfo struct {
	SlideIndex int    `json:"slide_index"` // 1-based
	SlideID    string `json:"slide_id"`
	ObjectID   string `json:"object_id"`
	ObjectType string `json:"object_type"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	URL        string `json:"url,omitempty"`       // External URL
	SlideLink  string `json:"slide_link,omitempty"` // Internal slide ID link
	LinkType   string `json:"link_type"`           // "external", "internal_slide", "internal_position"
	Text       string `json:"text"`                // The linked text
}

// ManageHyperlinks manages hyperlinks in a presentation.
func (t *Tools) ManageHyperlinks(ctx context.Context, tokenSource oauth2.TokenSource, input ManageHyperlinksInput) (*ManageHyperlinksOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "list" && action != "add" && action != "remove" {
		return nil, fmt.Errorf("%w: got '%s'", ErrInvalidHyperlinkAction, input.Action)
	}

	t.config.Logger.Info("managing hyperlinks",
		slog.String("presentation_id", input.PresentationID),
		slog.String("action", action),
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

	switch action {
	case "list":
		return t.listHyperlinks(ctx, presentation, input)
	case "add":
		return t.addHyperlink(ctx, slidesService, presentation, input)
	case "remove":
		return t.removeHyperlink(ctx, slidesService, presentation, input)
	default:
		return nil, fmt.Errorf("%w: got '%s'", ErrInvalidHyperlinkAction, action)
	}
}

// listHyperlinks lists hyperlinks in the presentation.
func (t *Tools) listHyperlinks(_ context.Context, presentation *slides.Presentation, input ManageHyperlinksInput) (*ManageHyperlinksOutput, error) {
	scope := strings.ToLower(strings.TrimSpace(input.Scope))
	if scope == "" {
		scope = "all"
	}

	if scope != "all" && scope != "slide" && scope != "object" {
		return nil, fmt.Errorf("%w: scope must be 'all', 'slide', or 'object'", ErrInvalidScope)
	}

	// Validate scope-specific parameters
	if scope == "slide" && input.SlideID == "" {
		return nil, fmt.Errorf("%w: slide_id is required when scope is 'slide'", ErrInvalidSlideReference)
	}
	if scope == "object" && input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required when scope is 'object'", ErrInvalidObjectID)
	}

	var links []HyperlinkInfo

	for slideIdx, slide := range presentation.Slides {
		// Apply slide filter
		if scope == "slide" && slide.ObjectId != input.SlideID {
			continue
		}

		slideLinks := extractLinksFromSlide(slide, slideIdx+1, input.ObjectID)
		links = append(links, slideLinks...)

		// If we found the specific slide, no need to continue
		if scope == "slide" {
			break
		}
	}

	// If scope is object but no links found, check if object exists
	if scope == "object" && len(links) == 0 {
		found := false
		for _, slide := range presentation.Slides {
			if findElementByID(slide.PageElements, input.ObjectID) != nil {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
		}
	}

	output := &ManageHyperlinksOutput{
		PresentationID: input.PresentationID,
		Action:         "list",
		Links:          links,
		Success:        true,
		Message:        fmt.Sprintf("Found %d hyperlink(s)", len(links)),
	}

	t.config.Logger.Info("hyperlinks listed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("count", len(links)),
	)

	return output, nil
}

// extractLinksFromSlide extracts all hyperlinks from a slide.
func extractLinksFromSlide(slide *slides.Page, slideIndex int, filterObjectID string) []HyperlinkInfo {
	var links []HyperlinkInfo

	if slide == nil {
		return links
	}

	links = append(links, extractLinksFromElements(slide.PageElements, slideIndex, slide.ObjectId, filterObjectID)...)
	return links
}

// extractLinksFromElements extracts hyperlinks from page elements.
func extractLinksFromElements(elements []*slides.PageElement, slideIndex int, slideID string, filterObjectID string) []HyperlinkInfo {
	var links []HyperlinkInfo

	for _, element := range elements {
		if element == nil {
			continue
		}

		// Apply object filter
		if filterObjectID != "" && element.ObjectId != filterObjectID {
			// Check in groups
			if element.ElementGroup != nil {
				groupLinks := extractLinksFromElements(element.ElementGroup.Children, slideIndex, slideID, filterObjectID)
				links = append(links, groupLinks...)
			}
			continue
		}

		// Extract links from shape text
		if element.Shape != nil && element.Shape.Text != nil {
			shapeLinks := extractLinksFromTextContent(element.Shape.Text, slideIndex, slideID, element.ObjectId, determineObjectType(element))
			links = append(links, shapeLinks...)
		}

		// Extract links from table cells
		if element.Table != nil {
			tableLinks := extractLinksFromTable(element.Table, slideIndex, slideID, element.ObjectId)
			links = append(links, tableLinks...)
		}

		// Extract links from groups recursively (if no filter or object not found yet)
		if element.ElementGroup != nil && filterObjectID == "" {
			groupLinks := extractLinksFromElements(element.ElementGroup.Children, slideIndex, slideID, filterObjectID)
			links = append(links, groupLinks...)
		}

		// Check if shape/image itself has a link (Link property on whole shape via ShapeProperties)
		if element.Shape != nil && element.Shape.ShapeProperties != nil && element.Shape.ShapeProperties.Link != nil {
			link := buildLinkInfo(element.Shape.ShapeProperties.Link, slideIndex, slideID, element.ObjectId, determineObjectType(element), 0, -1, "")
			if link != nil {
				links = append(links, *link)
			}
		}
		if element.Image != nil && element.Image.ImageProperties != nil && element.Image.ImageProperties.Link != nil {
			link := buildLinkInfo(element.Image.ImageProperties.Link, slideIndex, slideID, element.ObjectId, "IMAGE", 0, -1, "")
			if link != nil {
				links = append(links, *link)
			}
		}
	}

	return links
}

// extractLinksFromTextContent extracts hyperlinks from text content.
func extractLinksFromTextContent(textContent *slides.TextContent, slideIndex int, slideID, objectID, objectType string) []HyperlinkInfo {
	var links []HyperlinkInfo

	if textContent == nil || len(textContent.TextElements) == 0 {
		return links
	}

	var currentIdx int
	for _, textElement := range textContent.TextElements {
		if textElement == nil {
			continue
		}

		if textElement.TextRun != nil {
			textLen := len(textElement.TextRun.Content)

			// Check if this text run has a link
			if textElement.TextRun.Style != nil && textElement.TextRun.Style.Link != nil {
				link := buildLinkInfo(
					textElement.TextRun.Style.Link,
					slideIndex,
					slideID,
					objectID,
					objectType,
					currentIdx,
					currentIdx+textLen,
					textElement.TextRun.Content,
				)
				if link != nil {
					links = append(links, *link)
				}
			}
			currentIdx += textLen
		} else if textElement.ParagraphMarker != nil {
			currentIdx++ // Paragraph markers typically add 1 character (newline)
		}
	}

	return links
}

// extractLinksFromTable extracts hyperlinks from a table's cells.
func extractLinksFromTable(table *slides.Table, slideIndex int, slideID, tableID string) []HyperlinkInfo {
	var links []HyperlinkInfo

	if table == nil {
		return links
	}

	for rowIdx, row := range table.TableRows {
		if row == nil {
			continue
		}
		for colIdx, cell := range row.TableCells {
			if cell == nil || cell.Text == nil {
				continue
			}
			cellID := fmt.Sprintf("%s[%d,%d]", tableID, rowIdx, colIdx)
			cellLinks := extractLinksFromTextContent(cell.Text, slideIndex, slideID, cellID, "TABLE_CELL")
			links = append(links, cellLinks...)
		}
	}

	return links
}

// buildLinkInfo creates a HyperlinkInfo from a Link structure.
func buildLinkInfo(link *slides.Link, slideIndex int, slideID, objectID, objectType string, startIndex, endIndex int, text string) *HyperlinkInfo {
	if link == nil {
		return nil
	}

	info := &HyperlinkInfo{
		SlideIndex: slideIndex,
		SlideID:    slideID,
		ObjectID:   objectID,
		ObjectType: objectType,
		StartIndex: startIndex,
		EndIndex:   endIndex,
		Text:       strings.TrimSpace(text),
	}

	// Determine link type and set appropriate fields
	if link.Url != "" {
		info.URL = link.Url
		info.LinkType = "external"
	} else if link.SlideIndex != 0 || link.PageObjectId != "" {
		if link.PageObjectId != "" {
			info.SlideLink = link.PageObjectId
			info.LinkType = "internal_slide"
		} else {
			// SlideIndex is 0-based in API, but we use 1-based
			info.SlideLink = fmt.Sprintf("slide:%d", link.SlideIndex+1)
			info.LinkType = "internal_position"
		}
	} else if link.RelativeLink != "" {
		// Relative links like NEXT_SLIDE, PREVIOUS_SLIDE, FIRST_SLIDE, LAST_SLIDE
		info.SlideLink = link.RelativeLink
		info.LinkType = "internal_position"
	}

	// Skip if no link info found
	if info.URL == "" && info.SlideLink == "" {
		return nil
	}

	return info
}

// addHyperlink adds a hyperlink to text or an object.
func (t *Tools) addHyperlink(ctx context.Context, slidesService SlidesService, presentation *slides.Presentation, input ManageHyperlinksInput) (*ManageHyperlinksOutput, error) {
	// Validate input
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required for add action", ErrInvalidObjectID)
	}
	if input.URL == "" {
		return nil, ErrInvalidHyperlinkURL
	}

	// Find the target element
	var targetElement *slides.PageElement
	for _, slide := range presentation.Slides {
		element := findElementByID(slide.PageElements, input.ObjectID)
		if element != nil {
			targetElement = element
			break
		}
	}

	if targetElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Validate indices if provided
	if input.StartIndex != nil && *input.StartIndex < 0 {
		return nil, fmt.Errorf("%w: start_index cannot be negative", ErrInvalidTextRange)
	}
	if input.EndIndex != nil && *input.EndIndex < 0 {
		return nil, fmt.Errorf("%w: end_index cannot be negative", ErrInvalidTextRange)
	}
	if input.StartIndex != nil && input.EndIndex != nil && *input.StartIndex >= *input.EndIndex {
		return nil, fmt.Errorf("%w: start_index must be less than end_index", ErrInvalidTextRange)
	}

	// Build the appropriate request based on object type
	var requests []*slides.Request

	// Check if this is a text-based link (shape/text) or object link (image/shape)
	if targetElement.Shape != nil && targetElement.Shape.Text != nil && input.StartIndex != nil && input.EndIndex != nil {
		// Text link - use UpdateTextStyle
		link := buildLinkFromURL(input.URL)

		textRange := &slides.Range{
			Type: "FIXED_RANGE",
		}
		startIdx64 := int64(*input.StartIndex)
		endIdx64 := int64(*input.EndIndex)
		textRange.StartIndex = &startIdx64
		textRange.EndIndex = &endIdx64

		requests = append(requests, &slides.Request{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: input.ObjectID,
				Style: &slides.TextStyle{
					Link: link,
				},
				TextRange: textRange,
				Fields:    "link",
			},
		})
	} else if targetElement.Shape != nil {
		// Shape object link - use UpdateShapeProperties
		link := buildLinkFromURL(input.URL)
		requests = append(requests, &slides.Request{
			UpdateShapeProperties: &slides.UpdateShapePropertiesRequest{
				ObjectId: input.ObjectID,
				ShapeProperties: &slides.ShapeProperties{
					Link: link,
				},
				Fields: "link",
			},
		})
	} else if targetElement.Image != nil {
		// Image link - use UpdateImageProperties
		link := buildLinkFromURL(input.URL)
		requests = append(requests, &slides.Request{
			UpdateImageProperties: &slides.UpdateImagePropertiesRequest{
				ObjectId: input.ObjectID,
				ImageProperties: &slides.ImageProperties{
					Link: link,
				},
				Fields: "link",
			},
		})
	} else {
		return nil, fmt.Errorf("%w: cannot add hyperlink to this object type", ErrManageHyperlinksFailed)
	}

	// Execute batch update
	_, err := slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrManageHyperlinksFailed, err)
	}

	output := &ManageHyperlinksOutput{
		PresentationID: input.PresentationID,
		Action:         "add",
		Success:        true,
		Message:        fmt.Sprintf("Hyperlink added to object '%s'", input.ObjectID),
	}

	t.config.Logger.Info("hyperlink added successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("url", input.URL),
	)

	return output, nil
}

// buildLinkFromURL creates a Link structure from a URL string.
// Supports external URLs, internal slide links (#slide=1), and relative links.
func buildLinkFromURL(url string) *slides.Link {
	link := &slides.Link{}

	// Check for internal slide links
	if strings.HasPrefix(url, "#slide=") {
		// Parse slide index (1-based in user input)
		var slideIdx int
		if _, err := fmt.Sscanf(url, "#slide=%d", &slideIdx); err == nil && slideIdx > 0 {
			// API uses 0-based slide index
			link.SlideIndex = int64(slideIdx - 1)
			return link
		}
	}

	// Check for slide ID links
	if pageID, ok := strings.CutPrefix(url, "#slideId="); ok {
		link.PageObjectId = pageID
		return link
	}

	// Check for relative links
	relativeLinks := map[string]string{
		"#next":     "NEXT_SLIDE",
		"#previous": "PREVIOUS_SLIDE",
		"#first":    "FIRST_SLIDE",
		"#last":     "LAST_SLIDE",
	}
	if relLink, ok := relativeLinks[strings.ToLower(url)]; ok {
		link.RelativeLink = relLink
		return link
	}

	// External URL
	link.Url = url
	return link
}

// removeHyperlink removes a hyperlink from text or an object.
func (t *Tools) removeHyperlink(ctx context.Context, slidesService SlidesService, presentation *slides.Presentation, input ManageHyperlinksInput) (*ManageHyperlinksOutput, error) {
	// Validate input
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required for remove action", ErrInvalidObjectID)
	}

	// Find the target element
	var targetElement *slides.PageElement
	for _, slide := range presentation.Slides {
		element := findElementByID(slide.PageElements, input.ObjectID)
		if element != nil {
			targetElement = element
			break
		}
	}

	if targetElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Validate indices if provided for text
	if input.StartIndex != nil && *input.StartIndex < 0 {
		return nil, fmt.Errorf("%w: start_index cannot be negative", ErrInvalidTextRange)
	}
	if input.EndIndex != nil && *input.EndIndex < 0 {
		return nil, fmt.Errorf("%w: end_index cannot be negative", ErrInvalidTextRange)
	}
	if input.StartIndex != nil && input.EndIndex != nil && *input.StartIndex >= *input.EndIndex {
		return nil, fmt.Errorf("%w: start_index must be less than end_index", ErrInvalidTextRange)
	}

	// Build the appropriate request based on object type
	var requests []*slides.Request

	// Check if this is a text-based link removal or object link removal
	if targetElement.Shape != nil && targetElement.Shape.Text != nil && input.StartIndex != nil && input.EndIndex != nil {
		// Text link - use UpdateTextStyle with nil link to remove
		textRange := &slides.Range{
			Type: "FIXED_RANGE",
		}
		startIdx64 := int64(*input.StartIndex)
		endIdx64 := int64(*input.EndIndex)
		textRange.StartIndex = &startIdx64
		textRange.EndIndex = &endIdx64

		// Setting link field to empty removes the link
		requests = append(requests, &slides.Request{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: input.ObjectID,
				Style: &slides.TextStyle{
					// Empty link removes the hyperlink
					Link: nil,
				},
				TextRange: textRange,
				Fields:    "link",
			},
		})
	} else if targetElement.Shape != nil {
		// Shape object link - remove using UpdateShapeProperties
		requests = append(requests, &slides.Request{
			UpdateShapeProperties: &slides.UpdateShapePropertiesRequest{
				ObjectId: input.ObjectID,
				ShapeProperties: &slides.ShapeProperties{
					Link: nil,
				},
				Fields: "link",
			},
		})
	} else if targetElement.Image != nil {
		// Image link - remove using UpdateImageProperties
		requests = append(requests, &slides.Request{
			UpdateImageProperties: &slides.UpdateImagePropertiesRequest{
				ObjectId: input.ObjectID,
				ImageProperties: &slides.ImageProperties{
					Link: nil,
				},
				Fields: "link",
			},
		})
	} else {
		return nil, fmt.Errorf("%w: cannot remove hyperlink from this object type", ErrManageHyperlinksFailed)
	}

	// Execute batch update
	_, err := slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrManageHyperlinksFailed, err)
	}

	output := &ManageHyperlinksOutput{
		PresentationID: input.PresentationID,
		Action:         "remove",
		Success:        true,
		Message:        fmt.Sprintf("Hyperlink removed from object '%s'", input.ObjectID),
	}

	t.config.Logger.Info("hyperlink removed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
	)

	return output, nil
}
