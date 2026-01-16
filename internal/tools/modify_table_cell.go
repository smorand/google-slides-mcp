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

// Sentinel errors for modify_table_cell tool.
var (
	ErrModifyTableCellFailed    = errors.New("failed to modify table cell")
	ErrInvalidCellIndex         = errors.New("invalid cell index")
	ErrNoCellModification       = errors.New("no modification specified")
	ErrInvalidHorizontalAlign   = errors.New("invalid horizontal alignment")
	ErrInvalidVerticalAlign     = errors.New("invalid vertical alignment")
)

// validHorizontalAlignments maps alignment names to their normalized form.
var validHorizontalAlignments = map[string]string{
	"START":     "START",
	"CENTER":    "CENTER",
	"END":       "END",
	"JUSTIFIED": "JUSTIFIED",
	"start":     "START",
	"center":    "CENTER",
	"end":       "END",
	"justified": "JUSTIFIED",
}

// validVerticalAlignments maps alignment names to their normalized form.
var validVerticalAlignments = map[string]string{
	"TOP":    "TOP",
	"MIDDLE": "MIDDLE",
	"BOTTOM": "BOTTOM",
	"top":    "TOP",
	"middle": "MIDDLE",
	"bottom": "BOTTOM",
}

// ModifyTableCellInput represents the input for the modify_table_cell tool.
type ModifyTableCellInput struct {
	PresentationID string                `json:"presentation_id"`
	ObjectID       string                `json:"object_id"` // Table object ID
	Row            int                   `json:"row"`       // 0-based row index
	Column         int                   `json:"column"`    // 0-based column index
	Text           *string               `json:"text,omitempty"`
	Style          *TableCellStyleInput  `json:"style,omitempty"`
	Alignment      *TableCellAlignInput  `json:"alignment,omitempty"`
}

// TableCellStyleInput represents text styling options for a table cell.
type TableCellStyleInput struct {
	FontFamily      string `json:"font_family,omitempty"`
	FontSize        int    `json:"font_size,omitempty"`         // In points
	Bold            *bool  `json:"bold,omitempty"`
	Italic          *bool  `json:"italic,omitempty"`
	Underline       *bool  `json:"underline,omitempty"`
	Strikethrough   *bool  `json:"strikethrough,omitempty"`
	ForegroundColor string `json:"foreground_color,omitempty"` // Hex color
	BackgroundColor string `json:"background_color,omitempty"` // Hex color
}

// TableCellAlignInput represents alignment options for a table cell.
type TableCellAlignInput struct {
	Horizontal string `json:"horizontal,omitempty"` // START, CENTER, END, JUSTIFIED
	Vertical   string `json:"vertical,omitempty"`   // TOP, MIDDLE, BOTTOM
}

// ModifyTableCellOutput represents the output of the modify_table_cell tool.
type ModifyTableCellOutput struct {
	ObjectID          string   `json:"object_id"`
	Row               int      `json:"row"`
	Column            int      `json:"column"`
	ModifiedProperties []string `json:"modified_properties"`
}

// ModifyTableCell modifies the content and styling of a table cell.
func (t *Tools) ModifyTableCell(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyTableCellInput) (*ModifyTableCellOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Validate cell indices
	if input.Row < 0 {
		return nil, fmt.Errorf("%w: row must be non-negative", ErrInvalidCellIndex)
	}
	if input.Column < 0 {
		return nil, fmt.Errorf("%w: column must be non-negative", ErrInvalidCellIndex)
	}

	// Check that at least one modification is provided
	if input.Text == nil && input.Style == nil && input.Alignment == nil {
		return nil, fmt.Errorf("%w: text, style, or alignment must be provided", ErrNoCellModification)
	}

	// Validate alignment values if provided
	if input.Alignment != nil {
		if input.Alignment.Horizontal != "" {
			normalized, ok := validHorizontalAlignments[input.Alignment.Horizontal]
			if !ok {
				return nil, fmt.Errorf("%w: must be START, CENTER, END, or JUSTIFIED", ErrInvalidHorizontalAlign)
			}
			input.Alignment.Horizontal = normalized
		}
		if input.Alignment.Vertical != "" {
			normalized, ok := validVerticalAlignments[input.Alignment.Vertical]
			if !ok {
				return nil, fmt.Errorf("%w: must be TOP, MIDDLE, or BOTTOM", ErrInvalidVerticalAlign)
			}
			input.Alignment.Vertical = normalized
		}
	}

	t.config.Logger.Info("modifying table cell",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.Int("row", input.Row),
		slog.Int("column", input.Column),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to validate the table
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

	// Find the table and validate it
	tableElement := findTableByID(presentation, input.ObjectID)
	if tableElement == nil {
		return nil, fmt.Errorf("%w: %s", ErrObjectNotFound, input.ObjectID)
	}

	// Verify it's a table
	if tableElement.Table == nil {
		return nil, fmt.Errorf("%w: object '%s' is not a table", ErrNotATable, input.ObjectID)
	}

	table := tableElement.Table
	tableRows := len(table.TableRows)
	tableCols := 0
	if tableRows > 0 && len(table.TableRows[0].TableCells) > 0 {
		tableCols = len(table.TableRows[0].TableCells)
	}

	// Validate cell indices are within bounds
	if input.Row >= tableRows {
		return nil, fmt.Errorf("%w: row %d is out of range (table has %d rows)", ErrInvalidCellIndex, input.Row, tableRows)
	}
	if input.Column >= tableCols {
		return nil, fmt.Errorf("%w: column %d is out of range (table has %d columns)", ErrInvalidCellIndex, input.Column, tableCols)
	}

	// Build the requests
	requests, modifiedProps := buildModifyTableCellRequests(input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrModifyTableCellFailed, err)
	}

	output := &ModifyTableCellOutput{
		ObjectID:          input.ObjectID,
		Row:               input.Row,
		Column:            input.Column,
		ModifiedProperties: modifiedProps,
	}

	t.config.Logger.Info("table cell modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.Int("row", output.Row),
		slog.Int("column", output.Column),
		slog.Int("modified_properties_count", len(output.ModifiedProperties)),
	)

	return output, nil
}

// buildModifyTableCellRequests creates the batch update requests for table cell modification.
func buildModifyTableCellRequests(input ModifyTableCellInput) ([]*slides.Request, []string) {
	var requests []*slides.Request
	var modifiedProps []string

	cellLocation := &slides.TableCellLocation{
		RowIndex:    int64(input.Row),
		ColumnIndex: int64(input.Column),
	}

	// Handle text modification
	if input.Text != nil {
		// First, delete existing text
		requests = append(requests, &slides.Request{
			DeleteText: &slides.DeleteTextRequest{
				ObjectId:     input.ObjectID,
				CellLocation: cellLocation,
				TextRange: &slides.Range{
					Type: "ALL",
				},
			},
		})

		// Then insert new text (if non-empty)
		if *input.Text != "" {
			requests = append(requests, &slides.Request{
				InsertText: &slides.InsertTextRequest{
					ObjectId:       input.ObjectID,
					CellLocation:   cellLocation,
					Text:           *input.Text,
					InsertionIndex: 0,
				},
			})
		}
		modifiedProps = append(modifiedProps, "text")
	}

	// Handle text styling
	if input.Style != nil {
		styleRequest, styleFields := buildTableCellStyleRequest(input.ObjectID, cellLocation, input.Style)
		if styleRequest != nil {
			requests = append(requests, styleRequest)
			modifiedProps = append(modifiedProps, styleFields...)
		}
	}

	// Handle horizontal alignment (paragraph style)
	if input.Alignment != nil && input.Alignment.Horizontal != "" {
		requests = append(requests, &slides.Request{
			UpdateParagraphStyle: &slides.UpdateParagraphStyleRequest{
				ObjectId:     input.ObjectID,
				CellLocation: cellLocation,
				Style: &slides.ParagraphStyle{
					Alignment: input.Alignment.Horizontal,
				},
				TextRange: &slides.Range{
					Type: "ALL",
				},
				Fields: "alignment",
			},
		})
		modifiedProps = append(modifiedProps, fmt.Sprintf("horizontal_alignment=%s", input.Alignment.Horizontal))
	}

	// Handle vertical alignment (table cell properties)
	if input.Alignment != nil && input.Alignment.Vertical != "" {
		requests = append(requests, &slides.Request{
			UpdateTableCellProperties: &slides.UpdateTableCellPropertiesRequest{
				ObjectId: input.ObjectID,
				TableRange: &slides.TableRange{
					Location: cellLocation,
					RowSpan:    1,
					ColumnSpan: 1,
				},
				TableCellProperties: &slides.TableCellProperties{
					ContentAlignment: input.Alignment.Vertical,
				},
				Fields: "contentAlignment",
			},
		})
		modifiedProps = append(modifiedProps, fmt.Sprintf("vertical_alignment=%s", input.Alignment.Vertical))
	}

	return requests, modifiedProps
}

// buildTableCellStyleRequest creates the UpdateTextStyleRequest for table cell styling.
func buildTableCellStyleRequest(objectID string, cellLocation *slides.TableCellLocation, style *TableCellStyleInput) (*slides.Request, []string) {
	textStyle := &slides.TextStyle{}
	var fields []string
	var modifiedProps []string

	// Font family
	if style.FontFamily != "" {
		textStyle.FontFamily = style.FontFamily
		fields = append(fields, "fontFamily")
		modifiedProps = append(modifiedProps, fmt.Sprintf("font_family=%s", style.FontFamily))
	}

	// Font size
	if style.FontSize > 0 {
		textStyle.FontSize = &slides.Dimension{
			Magnitude: float64(style.FontSize),
			Unit:      "PT",
		}
		fields = append(fields, "fontSize")
		modifiedProps = append(modifiedProps, fmt.Sprintf("font_size=%d", style.FontSize))
	}

	// Bold
	if style.Bold != nil {
		textStyle.Bold = *style.Bold
		fields = append(fields, "bold")
		modifiedProps = append(modifiedProps, fmt.Sprintf("bold=%t", *style.Bold))
	}

	// Italic
	if style.Italic != nil {
		textStyle.Italic = *style.Italic
		fields = append(fields, "italic")
		modifiedProps = append(modifiedProps, fmt.Sprintf("italic=%t", *style.Italic))
	}

	// Underline
	if style.Underline != nil {
		textStyle.Underline = *style.Underline
		fields = append(fields, "underline")
		modifiedProps = append(modifiedProps, fmt.Sprintf("underline=%t", *style.Underline))
	}

	// Strikethrough
	if style.Strikethrough != nil {
		textStyle.Strikethrough = *style.Strikethrough
		fields = append(fields, "strikethrough")
		modifiedProps = append(modifiedProps, fmt.Sprintf("strikethrough=%t", *style.Strikethrough))
	}

	// Foreground color
	if style.ForegroundColor != "" {
		color := parseHexColor(style.ForegroundColor)
		if color != nil {
			textStyle.ForegroundColor = &slides.OptionalColor{
				OpaqueColor: &slides.OpaqueColor{
					RgbColor: color,
				},
			}
			fields = append(fields, "foregroundColor")
			modifiedProps = append(modifiedProps, fmt.Sprintf("foreground_color=%s", style.ForegroundColor))
		}
	}

	// Background color
	if style.BackgroundColor != "" {
		color := parseHexColor(style.BackgroundColor)
		if color != nil {
			textStyle.BackgroundColor = &slides.OptionalColor{
				OpaqueColor: &slides.OpaqueColor{
					RgbColor: color,
				},
			}
			fields = append(fields, "backgroundColor")
			modifiedProps = append(modifiedProps, fmt.Sprintf("background_color=%s", style.BackgroundColor))
		}
	}

	if len(fields) == 0 {
		return nil, nil
	}

	return &slides.Request{
		UpdateTextStyle: &slides.UpdateTextStyleRequest{
			ObjectId:     objectID,
			CellLocation: cellLocation,
			Style:        textStyle,
			TextRange: &slides.Range{
				Type: "ALL",
			},
			Fields: strings.Join(fields, ","),
		},
	}, modifiedProps
}
