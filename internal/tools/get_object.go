package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for get_object tool.
var (
	ErrObjectNotFound = errors.New("object not found")
)

// GetObjectInput represents the input for the get_object tool.
type GetObjectInput struct {
	PresentationID string `json:"presentation_id"`
	ObjectID       string `json:"object_id"`
}

// GetObjectOutput represents the output of the get_object tool.
type GetObjectOutput struct {
	PresentationID string         `json:"presentation_id"`
	ObjectID       string         `json:"object_id"`
	ObjectType     string         `json:"object_type"`
	SlideIndex     int            `json:"slide_index"` // 1-based index of containing slide
	Position       *Position      `json:"position,omitempty"`
	Size           *Size          `json:"size,omitempty"`
	Shape          *ShapeDetails  `json:"shape,omitempty"`
	Image          *ImageDetails  `json:"image,omitempty"`
	Table          *TableDetails  `json:"table,omitempty"`
	Video          *VideoDetails  `json:"video,omitempty"`
	Line           *LineDetails   `json:"line,omitempty"`
	Group          *GroupDetails  `json:"group,omitempty"`
	Chart          *ChartDetails  `json:"chart,omitempty"`
	WordArt        *WordArtDetails `json:"word_art,omitempty"`
}

// ShapeDetails contains detailed information about a shape.
type ShapeDetails struct {
	ShapeType       string             `json:"shape_type"`
	Text            string             `json:"text,omitempty"`
	TextStyle       *TextStyleDetails  `json:"text_style,omitempty"`
	Fill            *FillDetails       `json:"fill,omitempty"`
	Outline         *OutlineDetails    `json:"outline,omitempty"`
	PlaceholderType string             `json:"placeholder_type,omitempty"`
}

// TextStyleDetails contains text styling information.
type TextStyleDetails struct {
	FontFamily  string `json:"font_family,omitempty"`
	FontSize    *float64 `json:"font_size,omitempty"` // in points
	Bold        *bool  `json:"bold,omitempty"`
	Italic      *bool  `json:"italic,omitempty"`
	Underline   *bool  `json:"underline,omitempty"`
	Color       string `json:"color,omitempty"` // hex format
	LinkURL     string `json:"link_url,omitempty"`
}

// FillDetails contains fill information for shapes.
type FillDetails struct {
	Type        string `json:"type"` // SOLID, GRADIENT, etc.
	SolidColor  string `json:"solid_color,omitempty"` // hex format
}

// OutlineDetails contains outline information for shapes.
type OutlineDetails struct {
	Color     string  `json:"color,omitempty"` // hex format
	Weight    float64 `json:"weight,omitempty"` // in points
	DashStyle string  `json:"dash_style,omitempty"`
}

// ImageDetails contains detailed information about an image.
type ImageDetails struct {
	ContentURL   string        `json:"content_url,omitempty"`
	SourceURL    string        `json:"source_url,omitempty"`
	Crop         *CropDetails  `json:"crop,omitempty"`
	Brightness   float64       `json:"brightness,omitempty"`
	Contrast     float64       `json:"contrast,omitempty"`
	Transparency float64       `json:"transparency,omitempty"`
	Recolor      string        `json:"recolor,omitempty"`
}

// CropDetails contains crop information for images.
type CropDetails struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

// TableDetails contains detailed information about a table.
type TableDetails struct {
	Rows    int              `json:"rows"`
	Columns int              `json:"columns"`
	Cells   [][]CellDetails  `json:"cells"`
}

// CellDetails contains information about a single table cell.
type CellDetails struct {
	Row        int    `json:"row"`
	Column     int    `json:"column"`
	Text       string `json:"text,omitempty"`
	RowSpan    int    `json:"row_span,omitempty"`
	ColumnSpan int    `json:"column_span,omitempty"`
	Background string `json:"background,omitempty"` // hex color
}

// VideoDetails contains detailed information about a video.
type VideoDetails struct {
	VideoID   string  `json:"video_id,omitempty"`
	Source    string  `json:"source"` // YOUTUBE, DRIVE, etc.
	URL       string  `json:"url,omitempty"`
	StartTime float64 `json:"start_time,omitempty"` // seconds
	EndTime   float64 `json:"end_time,omitempty"`   // seconds
	Autoplay  bool    `json:"autoplay"`
	Mute      bool    `json:"mute"`
}

// LineDetails contains detailed information about a line.
type LineDetails struct {
	LineType   string          `json:"line_type"`
	StartArrow string          `json:"start_arrow,omitempty"`
	EndArrow   string          `json:"end_arrow,omitempty"`
	Color      string          `json:"color,omitempty"`
	Weight     float64         `json:"weight,omitempty"` // in points
	DashStyle  string          `json:"dash_style,omitempty"`
	StartPoint *Position       `json:"start_point,omitempty"`
	EndPoint   *Position       `json:"end_point,omitempty"`
}

// GroupDetails contains detailed information about a group.
type GroupDetails struct {
	ChildCount int      `json:"child_count"`
	ChildIDs   []string `json:"child_ids"`
}

// ChartDetails contains detailed information about a Sheets chart.
type ChartDetails struct {
	SpreadsheetID string `json:"spreadsheet_id,omitempty"`
	ChartID       int64  `json:"chart_id,omitempty"`
}

// WordArtDetails contains detailed information about word art.
type WordArtDetails struct {
	RenderedText string `json:"rendered_text,omitempty"`
}

// GetObject returns detailed information about a specific object.
func (t *Tools) GetObject(ctx context.Context, tokenSource oauth2.TokenSource, input GetObjectInput) (*GetObjectOutput, error) {
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrObjectNotFound)
	}

	t.config.Logger.Info("getting object details",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
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

	// Find the object in slides
	var targetElement *slides.PageElement
	var slideIndex int

	for slideIdx, slide := range presentation.Slides {
		element := findElementByID(slide.PageElements, input.ObjectID)
		if element != nil {
			targetElement = element
			slideIndex = slideIdx + 1 // 1-based
			break
		}
	}

	if targetElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Build output
	output := &GetObjectOutput{
		PresentationID: presentation.PresentationId,
		ObjectID:       targetElement.ObjectId,
		ObjectType:     determineObjectType(targetElement),
		SlideIndex:     slideIndex,
	}

	// Extract position
	if targetElement.Transform != nil {
		output.Position = &Position{
			X: emuToPoints(targetElement.Transform.TranslateX),
			Y: emuToPoints(targetElement.Transform.TranslateY),
		}
	}

	// Extract size
	if targetElement.Size != nil {
		output.Size = &Size{}
		if targetElement.Size.Width != nil {
			output.Size.Width = convertToPoints(targetElement.Size.Width)
		}
		if targetElement.Size.Height != nil {
			output.Size.Height = convertToPoints(targetElement.Size.Height)
		}
	}

	// Extract type-specific details
	switch {
	case targetElement.Shape != nil:
		output.Shape = extractShapeDetails(targetElement.Shape)
	case targetElement.Image != nil:
		output.Image = extractImageDetails(targetElement.Image)
	case targetElement.Table != nil:
		output.Table = extractTableDetails(targetElement.Table)
	case targetElement.Video != nil:
		output.Video = extractVideoDetails(targetElement.Video)
	case targetElement.Line != nil:
		output.Line = extractLineDetails(targetElement)
	case targetElement.ElementGroup != nil:
		output.Group = extractGroupDetails(targetElement.ElementGroup)
	case targetElement.SheetsChart != nil:
		output.Chart = extractChartDetails(targetElement.SheetsChart)
	case targetElement.WordArt != nil:
		output.WordArt = extractWordArtDetails(targetElement.WordArt)
	}

	t.config.Logger.Info("object details retrieved successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("object_type", output.ObjectType),
	)

	return output, nil
}

// findElementByID searches for an element by ID in a list of page elements.
func findElementByID(elements []*slides.PageElement, objectID string) *slides.PageElement {
	for _, element := range elements {
		if element == nil {
			continue
		}
		if element.ObjectId == objectID {
			return element
		}
		// Search in groups recursively
		if element.ElementGroup != nil {
			found := findElementByID(element.ElementGroup.Children, objectID)
			if found != nil {
				return found
			}
		}
	}
	return nil
}

// extractShapeDetails extracts detailed information from a shape.
func extractShapeDetails(shape *slides.Shape) *ShapeDetails {
	if shape == nil {
		return nil
	}

	details := &ShapeDetails{
		ShapeType: shape.ShapeType,
	}

	// Extract placeholder type
	if shape.Placeholder != nil {
		details.PlaceholderType = shape.Placeholder.Type
	}

	// Extract text content
	if shape.Text != nil {
		details.Text = extractTextFromTextContent(shape.Text)
		details.TextStyle = extractTextStyle(shape.Text)
	}

	// Extract shape properties (fill, outline)
	if shape.ShapeProperties != nil {
		details.Fill = extractFillDetails(shape.ShapeProperties.ShapeBackgroundFill)
		details.Outline = extractOutlineDetails(shape.ShapeProperties.Outline)
	}

	return details
}

// extractTextStyle extracts text styling from text content.
func extractTextStyle(textContent *slides.TextContent) *TextStyleDetails {
	if textContent == nil || len(textContent.TextElements) == 0 {
		return nil
	}

	// Get style from first text run
	for _, element := range textContent.TextElements {
		if element.TextRun != nil && element.TextRun.Style != nil {
			style := element.TextRun.Style
			details := &TextStyleDetails{}

			if style.FontFamily != "" {
				details.FontFamily = style.FontFamily
			}
			if style.FontSize != nil {
				magnitude := style.FontSize.Magnitude
				details.FontSize = &magnitude
			}
			if style.Bold {
				bold := style.Bold
				details.Bold = &bold
			}
			if style.Italic {
				italic := style.Italic
				details.Italic = &italic
			}
			if style.Underline {
				underline := style.Underline
				details.Underline = &underline
			}
			if style.ForegroundColor != nil && style.ForegroundColor.OpaqueColor != nil {
				details.Color = extractColor(style.ForegroundColor.OpaqueColor)
			}
			if style.Link != nil && style.Link.Url != "" {
				details.LinkURL = style.Link.Url
			}

			return details
		}
	}

	return nil
}

// extractFillDetails extracts fill details from shape background fill.
func extractFillDetails(fill *slides.ShapeBackgroundFill) *FillDetails {
	if fill == nil {
		return nil
	}

	details := &FillDetails{}

	if fill.SolidFill != nil {
		details.Type = "SOLID"
		if fill.SolidFill.Color != nil {
			details.SolidColor = extractColor(fill.SolidFill.Color)
		}
	}

	return details
}

// extractOutlineDetails extracts outline details.
func extractOutlineDetails(outline *slides.Outline) *OutlineDetails {
	if outline == nil {
		return nil
	}

	details := &OutlineDetails{}

	if outline.OutlineFill != nil && outline.OutlineFill.SolidFill != nil {
		if outline.OutlineFill.SolidFill.Color != nil {
			details.Color = extractColor(outline.OutlineFill.SolidFill.Color)
		}
	}

	if outline.Weight != nil {
		details.Weight = convertToPoints(outline.Weight)
	}

	if outline.DashStyle != "" {
		details.DashStyle = outline.DashStyle
	}

	return details
}

// extractColor extracts a color as a hex string.
func extractColor(color *slides.OpaqueColor) string {
	if color == nil {
		return ""
	}

	if color.RgbColor != nil {
		r := int(color.RgbColor.Red * 255)
		g := int(color.RgbColor.Green * 255)
		b := int(color.RgbColor.Blue * 255)
		return fmt.Sprintf("#%02X%02X%02X", r, g, b)
	}

	if color.ThemeColor != "" {
		return fmt.Sprintf("theme:%s", color.ThemeColor)
	}

	return ""
}

// extractImageDetails extracts detailed information from an image.
func extractImageDetails(image *slides.Image) *ImageDetails {
	if image == nil {
		return nil
	}

	details := &ImageDetails{
		ContentURL: image.ContentUrl,
		SourceURL:  image.SourceUrl,
	}

	// Extract image properties
	if image.ImageProperties != nil {
		props := image.ImageProperties

		details.Brightness = props.Brightness
		details.Contrast = props.Contrast
		details.Transparency = props.Transparency

		if props.Recolor != nil && props.Recolor.Name != "" {
			details.Recolor = props.Recolor.Name
		}

		if props.CropProperties != nil {
			details.Crop = &CropDetails{
				Top:    props.CropProperties.TopOffset,
				Bottom: props.CropProperties.BottomOffset,
				Left:   props.CropProperties.LeftOffset,
				Right:  props.CropProperties.RightOffset,
			}
		}
	}

	return details
}

// extractTableDetails extracts detailed information from a table.
func extractTableDetails(table *slides.Table) *TableDetails {
	if table == nil {
		return nil
	}

	rows := len(table.TableRows)
	cols := 0
	if rows > 0 && len(table.TableRows[0].TableCells) > 0 {
		cols = len(table.TableRows[0].TableCells)
	}

	details := &TableDetails{
		Rows:    rows,
		Columns: cols,
		Cells:   make([][]CellDetails, rows),
	}

	for rowIdx, row := range table.TableRows {
		if row == nil {
			continue
		}
		details.Cells[rowIdx] = make([]CellDetails, len(row.TableCells))

		for colIdx, cell := range row.TableCells {
			if cell == nil {
				continue
			}

			cellDetails := CellDetails{
				Row:        rowIdx,
				Column:     colIdx,
				RowSpan:    int(cell.RowSpan),
				ColumnSpan: int(cell.ColumnSpan),
			}

			if cell.Text != nil {
				cellDetails.Text = extractTextFromTextContent(cell.Text)
			}

			if cell.TableCellProperties != nil && cell.TableCellProperties.TableCellBackgroundFill != nil {
				fill := cell.TableCellProperties.TableCellBackgroundFill
				if fill.SolidFill != nil && fill.SolidFill.Color != nil {
					cellDetails.Background = extractColor(fill.SolidFill.Color)
				}
			}

			details.Cells[rowIdx][colIdx] = cellDetails
		}
	}

	return details
}

// extractVideoDetails extracts detailed information from a video.
func extractVideoDetails(video *slides.Video) *VideoDetails {
	if video == nil {
		return nil
	}

	details := &VideoDetails{
		VideoID: video.Id,
		Source:  video.Source,
		URL:     video.Url,
	}

	if video.VideoProperties != nil {
		props := video.VideoProperties

		if props.Start != 0 {
			details.StartTime = float64(props.Start) / 1000.0 // Convert ms to seconds
		}
		if props.End != 0 {
			details.EndTime = float64(props.End) / 1000.0 // Convert ms to seconds
		}
		details.Autoplay = props.AutoPlay
		details.Mute = props.Mute
	}

	return details
}

// extractLineDetails extracts detailed information from a line.
func extractLineDetails(element *slides.PageElement) *LineDetails {
	if element == nil || element.Line == nil {
		return nil
	}

	line := element.Line
	details := &LineDetails{
		LineType: line.LineType,
	}

	if line.LineProperties != nil {
		props := line.LineProperties

		if props.StartArrow != "" {
			details.StartArrow = props.StartArrow
		}
		if props.EndArrow != "" {
			details.EndArrow = props.EndArrow
		}
		if props.DashStyle != "" {
			details.DashStyle = props.DashStyle
		}
		if props.LineFill != nil && props.LineFill.SolidFill != nil {
			if props.LineFill.SolidFill.Color != nil {
				details.Color = extractColor(props.LineFill.SolidFill.Color)
			}
		}
		if props.Weight != nil {
			details.Weight = convertToPoints(props.Weight)
		}
	}

	// Extract start and end points from line category
	if line.LineCategory == "STRAIGHT" || line.LineCategory == "BENT" || line.LineCategory == "CURVED" {
		// Points would need to be calculated from transform, which is complex
		// For now, we just report the line type
	}

	return details
}

// extractGroupDetails extracts detailed information from a group.
func extractGroupDetails(group *slides.Group) *GroupDetails {
	if group == nil {
		return nil
	}

	childIDs := make([]string, 0, len(group.Children))
	for _, child := range group.Children {
		if child != nil {
			childIDs = append(childIDs, child.ObjectId)
		}
	}

	return &GroupDetails{
		ChildCount: len(group.Children),
		ChildIDs:   childIDs,
	}
}

// extractChartDetails extracts detailed information from a Sheets chart.
func extractChartDetails(chart *slides.SheetsChart) *ChartDetails {
	if chart == nil {
		return nil
	}

	return &ChartDetails{
		SpreadsheetID: chart.SpreadsheetId,
		ChartID:       chart.ChartId,
	}
}

// extractWordArtDetails extracts detailed information from word art.
func extractWordArtDetails(wordArt *slides.WordArt) *WordArtDetails {
	if wordArt == nil {
		return nil
	}

	return &WordArtDetails{
		RenderedText: wordArt.RenderedText,
	}
}
