package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for get_presentation tool.
var (
	ErrPresentationNotFound = errors.New("presentation not found")
	ErrAccessDenied         = errors.New("access denied to presentation")
	ErrSlidesAPIError       = errors.New("slides API error")
)

// GetPresentationInput represents the input for the get_presentation tool.
type GetPresentationInput struct {
	PresentationID    string `json:"presentation_id"`
	IncludeThumbnails bool   `json:"include_thumbnails,omitempty"`
}

// GetPresentationOutput represents the output of the get_presentation tool.
type GetPresentationOutput struct {
	PresentationID string       `json:"presentation_id"`
	Title          string       `json:"title"`
	Locale         string       `json:"locale,omitempty"`
	SlidesCount    int          `json:"slides_count"`
	PageSize       *PageSize    `json:"page_size,omitempty"`
	Slides         []SlideInfo  `json:"slides"`
	Masters        []MasterInfo `json:"masters,omitempty"`
	Layouts        []LayoutInfo `json:"layouts,omitempty"`
}

// PageSize represents the page dimensions.
type PageSize struct {
	Width  *Dimension `json:"width,omitempty"`
	Height *Dimension `json:"height,omitempty"`
}

// Dimension represents a dimension value.
type Dimension struct {
	Magnitude float64 `json:"magnitude"`
	Unit      string  `json:"unit"`
}

// SlideInfo represents information about a single slide.
type SlideInfo struct {
	Index           int          `json:"index"`
	ObjectID        string       `json:"object_id"`
	LayoutID        string       `json:"layout_id,omitempty"`
	LayoutName      string       `json:"layout_name,omitempty"`
	TextContent     []TextBlock  `json:"text_content,omitempty"`
	SpeakerNotes    string       `json:"speaker_notes,omitempty"`
	ObjectCount     int          `json:"object_count"`
	Objects         []ObjectInfo `json:"objects,omitempty"`
	ThumbnailBase64 string       `json:"thumbnail_base64,omitempty"`
}

// TextBlock represents a text element on a slide.
type TextBlock struct {
	ObjectID   string `json:"object_id"`
	ObjectType string `json:"object_type"`
	Text       string `json:"text"`
}

// ObjectInfo represents information about a page element.
type ObjectInfo struct {
	ObjectID   string `json:"object_id"`
	ObjectType string `json:"object_type"`
}

// MasterInfo represents information about a master slide.
type MasterInfo struct {
	ObjectID string `json:"object_id"`
	Name     string `json:"name,omitempty"`
}

// LayoutInfo represents information about a layout.
type LayoutInfo struct {
	ObjectID   string `json:"object_id"`
	Name       string `json:"name,omitempty"`
	MasterID   string `json:"master_id,omitempty"`
	LayoutType string `json:"layout_type,omitempty"`
}

// GetPresentation loads a Google Slides presentation and returns its full structure.
func (t *Tools) GetPresentation(ctx context.Context, tokenSource oauth2.TokenSource, input GetPresentationInput) (*GetPresentationOutput, error) {
	if input.PresentationID == "" {
		return nil, errors.New("presentation_id is required")
	}

	t.config.Logger.Info("getting presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Bool("include_thumbnails", input.IncludeThumbnails),
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
	output := &GetPresentationOutput{
		PresentationID: presentation.PresentationId,
		Title:          presentation.Title,
		Locale:         presentation.Locale,
		SlidesCount:    len(presentation.Slides),
	}

	// Page size
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

	// Process slides
	output.Slides = make([]SlideInfo, len(presentation.Slides))
	for i, slide := range presentation.Slides {
		slideInfo := SlideInfo{
			Index:       i + 1, // 1-based index
			ObjectID:    slide.ObjectId,
			ObjectCount: len(slide.PageElements),
		}

		// Get layout info
		if slide.SlideProperties != nil {
			slideInfo.LayoutID = slide.SlideProperties.LayoutObjectId
			// Find layout name from layouts
			for _, layout := range presentation.Layouts {
				if layout.ObjectId == slide.SlideProperties.LayoutObjectId {
					if layout.LayoutProperties != nil {
						slideInfo.LayoutName = layout.LayoutProperties.DisplayName
					}
					break
				}
			}
		}

		// Extract text content and objects
		slideInfo.TextContent, slideInfo.Objects = extractPageContent(slide.PageElements)

		// Extract speaker notes
		slideInfo.SpeakerNotes = extractSpeakerNotes(slide)

		// Get thumbnail if requested
		if input.IncludeThumbnails {
			thumbnail, err := slidesService.GetThumbnail(ctx, input.PresentationID, slide.ObjectId)
			if err == nil && thumbnail != nil {
				// Fetch the thumbnail image
				thumbnailData, err := fetchThumbnailImage(ctx, thumbnail.ContentUrl)
				if err != nil {
					t.config.Logger.Warn("failed to fetch thumbnail",
						slog.Int("slide", i+1),
						slog.Any("error", err),
					)
				} else {
					slideInfo.ThumbnailBase64 = base64.StdEncoding.EncodeToString(thumbnailData)
				}
			} else if err != nil {
				t.config.Logger.Warn("failed to get thumbnail",
					slog.Int("slide", i+1),
					slog.Any("error", err),
				)
			}
		}

		output.Slides[i] = slideInfo
	}

	// Process masters
	if len(presentation.Masters) > 0 {
		output.Masters = make([]MasterInfo, len(presentation.Masters))
		for i, master := range presentation.Masters {
			output.Masters[i] = MasterInfo{
				ObjectID: master.ObjectId,
			}
			if master.MasterProperties != nil {
				output.Masters[i].Name = master.MasterProperties.DisplayName
			}
		}
	}

	// Process layouts
	if len(presentation.Layouts) > 0 {
		output.Layouts = make([]LayoutInfo, len(presentation.Layouts))
		for i, layout := range presentation.Layouts {
			output.Layouts[i] = LayoutInfo{
				ObjectID: layout.ObjectId,
			}
			if layout.LayoutProperties != nil {
				output.Layouts[i].Name = layout.LayoutProperties.DisplayName
				output.Layouts[i].MasterID = layout.LayoutProperties.MasterObjectId
				output.Layouts[i].LayoutType = layout.LayoutProperties.Name
			}
		}
	}

	t.config.Logger.Info("presentation loaded successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("title", output.Title),
		slog.Int("slides_count", output.SlidesCount),
	)

	return output, nil
}

// extractPageContent extracts text content and object info from page elements.
func extractPageContent(elements []*slides.PageElement) ([]TextBlock, []ObjectInfo) {
	var textBlocks []TextBlock
	var objects []ObjectInfo

	for _, element := range elements {
		if element == nil {
			continue
		}

		objectType := determineObjectType(element)
		objects = append(objects, ObjectInfo{
			ObjectID:   element.ObjectId,
			ObjectType: objectType,
		})

		// Extract text from shape
		if element.Shape != nil && element.Shape.Text != nil {
			text := extractTextFromTextContent(element.Shape.Text)
			if text != "" {
				textBlocks = append(textBlocks, TextBlock{
					ObjectID:   element.ObjectId,
					ObjectType: objectType,
					Text:       text,
				})
			}
		}

		// Extract text from table
		if element.Table != nil {
			text := extractTextFromTable(element.Table)
			if text != "" {
				textBlocks = append(textBlocks, TextBlock{
					ObjectID:   element.ObjectId,
					ObjectType: "TABLE",
					Text:       text,
				})
			}
		}

		// Process groups recursively
		if element.ElementGroup != nil {
			groupText, groupObjects := extractPageContent(element.ElementGroup.Children)
			textBlocks = append(textBlocks, groupText...)
			objects = append(objects, groupObjects...)
		}
	}

	return textBlocks, objects
}

// determineObjectType returns the type of a page element.
func determineObjectType(element *slides.PageElement) string {
	switch {
	case element.Shape != nil:
		if element.Shape.ShapeType != "" {
			return element.Shape.ShapeType
		}
		return "SHAPE"
	case element.Image != nil:
		return "IMAGE"
	case element.Video != nil:
		return "VIDEO"
	case element.Table != nil:
		return "TABLE"
	case element.Line != nil:
		return "LINE"
	case element.ElementGroup != nil:
		return "GROUP"
	case element.SheetsChart != nil:
		return "SHEETS_CHART"
	case element.WordArt != nil:
		return "WORD_ART"
	default:
		return "UNKNOWN"
	}
}

// extractTextFromTextContent extracts plain text from a TextContent structure.
func extractTextFromTextContent(textContent *slides.TextContent) string {
	if textContent == nil || len(textContent.TextElements) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, element := range textContent.TextElements {
		if element.TextRun != nil && element.TextRun.Content != "" {
			builder.WriteString(element.TextRun.Content)
		}
	}

	return strings.TrimSpace(builder.String())
}

// extractTextFromTable extracts text from all cells in a table.
func extractTextFromTable(table *slides.Table) string {
	if table == nil || len(table.TableRows) == 0 {
		return ""
	}

	var builder strings.Builder
	for rowIdx, row := range table.TableRows {
		if row == nil {
			continue
		}
		for colIdx, cell := range row.TableCells {
			if cell == nil || cell.Text == nil {
				continue
			}
			text := extractTextFromTextContent(cell.Text)
			if text != "" {
				if builder.Len() > 0 {
					builder.WriteString(" | ")
				}
				fmt.Fprintf(&builder, "[%d,%d]: %s", rowIdx, colIdx, text)
			}
		}
		if rowIdx < len(table.TableRows)-1 {
			builder.WriteString("\n")
		}
	}

	return strings.TrimSpace(builder.String())
}

// extractSpeakerNotes extracts speaker notes from a slide.
func extractSpeakerNotes(slide *slides.Page) string {
	if slide == nil || slide.SlideProperties == nil {
		return ""
	}

	notesPage := slide.SlideProperties.NotesPage
	if notesPage == nil || len(notesPage.PageElements) == 0 {
		return ""
	}

	// Look for the notes shape (usually has placeholder type BODY)
	for _, element := range notesPage.PageElements {
		if element.Shape != nil && element.Shape.Text != nil {
			// Check if this is the body placeholder (speaker notes)
			if element.Shape.Placeholder != nil &&
				element.Shape.Placeholder.Type == "BODY" {
				return extractTextFromTextContent(element.Shape.Text)
			}
		}
	}

	// Fallback: try to find any shape with text in the notes page
	for _, element := range notesPage.PageElements {
		if element.Shape != nil && element.Shape.Text != nil {
			text := extractTextFromTextContent(element.Shape.Text)
			if text != "" {
				return text
			}
		}
	}

	return ""
}

// fetchThumbnailImage fetches the thumbnail image from the URL.
func fetchThumbnailImage(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, errors.New("empty thumbnail URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch thumbnail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail: %w", err)
	}

	return data, nil
}

// isNotFoundError checks if an error indicates a resource was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "notFound") ||
		strings.Contains(errStr, "not found")
}

// isForbiddenError checks if an error indicates access was denied.
func isForbiddenError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "permission denied")
}
