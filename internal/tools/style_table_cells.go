package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for style_table_cells tool.
var (
	ErrStyleTableCellsFailed = errors.New("failed to style table cells")
	ErrInvalidCellSelector   = errors.New("invalid cell selector")
	ErrNoCellStyle           = errors.New("no style specified")
)

// StyleTableCellsInput represents the input for the style_table_cells tool.
type StyleTableCellsInput struct {
	PresentationID string                `json:"presentation_id"`
	ObjectID       string                `json:"object_id"` // Table object ID
	Cells          CellSelector          `json:"cells"`     // Cell selection: array of {row, column}, "all", "row:N", "column:N"
	Style          *TableCellsStyleInput `json:"style"`     // Style to apply
}

// CellSelector represents different ways to select cells.
// Can be "all", "row:N", "column:N", or an array of {row, column} objects.
type CellSelector struct {
	All       bool                  `json:"-"` // Select all cells
	Row       *int                  `json:"-"` // Select a specific row (0-based)
	Column    *int                  `json:"-"` // Select a specific column (0-based)
	Positions []CellPosition        `json:"-"` // Select specific cells
	Raw       interface{}           `json:"-"` // Raw input for error messages
}

// CellPosition represents a single cell position.
type CellPosition struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// TableCellsStyleInput represents style options for table cells.
type TableCellsStyleInput struct {
	BackgroundColor string               `json:"background_color,omitempty"` // Hex color
	BorderTop       *TableBorderInput    `json:"border_top,omitempty"`
	BorderBottom    *TableBorderInput    `json:"border_bottom,omitempty"`
	BorderLeft      *TableBorderInput    `json:"border_left,omitempty"`
	BorderRight     *TableBorderInput    `json:"border_right,omitempty"`
}

// TableBorderInput represents a single border style.
type TableBorderInput struct {
	Color     string  `json:"color,omitempty"`      // Hex color
	Width     float64 `json:"width,omitempty"`      // Width in points
	DashStyle string  `json:"dash_style,omitempty"` // SOLID, DOT, DASH, DASH_DOT, LONG_DASH, LONG_DASH_DOT
}

// StyleTableCellsOutput represents the output of the style_table_cells tool.
type StyleTableCellsOutput struct {
	ObjectID      string   `json:"object_id"`
	CellsAffected int      `json:"cells_affected"`
	AppliedStyles []string `json:"applied_styles"`
}

// validDashStyles maps dash style names to their normalized form.
var validDashStyles = map[string]string{
	"SOLID":         "SOLID",
	"DOT":           "DOT",
	"DASH":          "DASH",
	"DASH_DOT":      "DASH_DOT",
	"LONG_DASH":     "LONG_DASH",
	"LONG_DASH_DOT": "LONG_DASH_DOT",
	"solid":         "SOLID",
	"dot":           "DOT",
	"dash":          "DASH",
	"dash_dot":      "DASH_DOT",
	"long_dash":     "LONG_DASH",
	"long_dash_dot": "LONG_DASH_DOT",
}

// ParseCellSelector parses the cell selector from various formats.
func ParseCellSelector(input interface{}) (CellSelector, error) {
	selector := CellSelector{Raw: input}

	switch v := input.(type) {
	case string:
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "all" {
			selector.All = true
			return selector, nil
		}
		// Check for "row:N" format
		if strings.HasPrefix(v, "row:") {
			numStr := strings.TrimPrefix(v, "row:")
			num, err := strconv.Atoi(numStr)
			if err != nil || num < 0 {
				return selector, fmt.Errorf("%w: invalid row number in '%s'", ErrInvalidCellSelector, input)
			}
			selector.Row = &num
			return selector, nil
		}
		// Check for "column:N" format
		if strings.HasPrefix(v, "column:") {
			numStr := strings.TrimPrefix(v, "column:")
			num, err := strconv.Atoi(numStr)
			if err != nil || num < 0 {
				return selector, fmt.Errorf("%w: invalid column number in '%s'", ErrInvalidCellSelector, input)
			}
			selector.Column = &num
			return selector, nil
		}
		return selector, fmt.Errorf("%w: '%s' (expected 'all', 'row:N', 'column:N', or array of positions)", ErrInvalidCellSelector, input)

	case []interface{}:
		positions := make([]CellPosition, 0, len(v))
		for i, item := range v {
			pos, ok := item.(map[string]interface{})
			if !ok {
				return selector, fmt.Errorf("%w: element %d is not a valid cell position object", ErrInvalidCellSelector, i)
			}
			rowVal, hasRow := pos["row"]
			colVal, hasCol := pos["column"]
			if !hasRow || !hasCol {
				return selector, fmt.Errorf("%w: element %d missing 'row' or 'column' field", ErrInvalidCellSelector, i)
			}
			row, rowOk := toInt(rowVal)
			col, colOk := toInt(colVal)
			if !rowOk || !colOk || row < 0 || col < 0 {
				return selector, fmt.Errorf("%w: element %d has invalid row or column value", ErrInvalidCellSelector, i)
			}
			positions = append(positions, CellPosition{Row: row, Column: col})
		}
		if len(positions) == 0 {
			return selector, fmt.Errorf("%w: cell positions array is empty", ErrInvalidCellSelector)
		}
		selector.Positions = positions
		return selector, nil

	case []CellPosition:
		if len(v) == 0 {
			return selector, fmt.Errorf("%w: cell positions array is empty", ErrInvalidCellSelector)
		}
		selector.Positions = v
		return selector, nil

	default:
		return selector, fmt.Errorf("%w: expected string or array, got %T", ErrInvalidCellSelector, input)
	}
}

// toInt converts various numeric types to int.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	default:
		return 0, false
	}
}

// StyleTableCells applies visual styling to table cells.
func (t *Tools) StyleTableCells(ctx context.Context, tokenSource oauth2.TokenSource, input StyleTableCellsInput) (*StyleTableCellsOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}
	if input.Style == nil {
		return nil, fmt.Errorf("%w: style is required", ErrNoCellStyle)
	}

	// Check that at least one style property is provided
	if !hasAnyStyle(input.Style) {
		return nil, fmt.Errorf("%w: at least one style property must be specified", ErrNoCellStyle)
	}

	// Validate border dash styles if provided
	if err := validateBorderStyles(input.Style); err != nil {
		return nil, err
	}

	t.config.Logger.Info("styling table cells",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
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

	// Resolve cell positions from selector
	positions, err := resolveCellPositions(input.Cells, tableRows, tableCols)
	if err != nil {
		return nil, err
	}

	// Build the requests
	requests, appliedStyles := buildStyleTableCellsRequests(input.ObjectID, positions, input.Style)

	if len(requests) == 0 {
		return nil, fmt.Errorf("%w: no valid style requests generated", ErrStyleTableCellsFailed)
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
		return nil, fmt.Errorf("%w: %v", ErrStyleTableCellsFailed, err)
	}

	output := &StyleTableCellsOutput{
		ObjectID:      input.ObjectID,
		CellsAffected: len(positions),
		AppliedStyles: appliedStyles,
	}

	t.config.Logger.Info("table cells styled successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.Int("cells_affected", output.CellsAffected),
		slog.Int("styles_applied", len(output.AppliedStyles)),
	)

	return output, nil
}

// hasAnyStyle checks if at least one style property is specified.
func hasAnyStyle(style *TableCellsStyleInput) bool {
	if style == nil {
		return false
	}
	if style.BackgroundColor != "" {
		return true
	}
	if style.BorderTop != nil {
		return true
	}
	if style.BorderBottom != nil {
		return true
	}
	if style.BorderLeft != nil {
		return true
	}
	if style.BorderRight != nil {
		return true
	}
	return false
}

// validateBorderStyles validates dash styles for all borders.
func validateBorderStyles(style *TableCellsStyleInput) error {
	borders := []*TableBorderInput{
		style.BorderTop,
		style.BorderBottom,
		style.BorderLeft,
		style.BorderRight,
	}
	borderNames := []string{"border_top", "border_bottom", "border_left", "border_right"}

	for i, border := range borders {
		if border != nil && border.DashStyle != "" {
			if _, ok := validDashStyles[border.DashStyle]; !ok {
				return fmt.Errorf("%w: invalid dash_style '%s' for %s (expected SOLID, DOT, DASH, DASH_DOT, LONG_DASH, LONG_DASH_DOT)", ErrStyleTableCellsFailed, border.DashStyle, borderNames[i])
			}
		}
	}
	return nil
}

// resolveCellPositions converts a CellSelector into concrete cell positions.
func resolveCellPositions(selector CellSelector, tableRows, tableCols int) ([]CellPosition, error) {
	var positions []CellPosition

	if selector.All {
		// All cells in the table
		for row := 0; row < tableRows; row++ {
			for col := 0; col < tableCols; col++ {
				positions = append(positions, CellPosition{Row: row, Column: col})
			}
		}
		return positions, nil
	}

	if selector.Row != nil {
		// All cells in a specific row
		rowIdx := *selector.Row
		if rowIdx >= tableRows {
			return nil, fmt.Errorf("%w: row %d is out of range (table has %d rows)", ErrInvalidCellSelector, rowIdx, tableRows)
		}
		for col := 0; col < tableCols; col++ {
			positions = append(positions, CellPosition{Row: rowIdx, Column: col})
		}
		return positions, nil
	}

	if selector.Column != nil {
		// All cells in a specific column
		colIdx := *selector.Column
		if colIdx >= tableCols {
			return nil, fmt.Errorf("%w: column %d is out of range (table has %d columns)", ErrInvalidCellSelector, colIdx, tableCols)
		}
		for row := 0; row < tableRows; row++ {
			positions = append(positions, CellPosition{Row: row, Column: colIdx})
		}
		return positions, nil
	}

	if len(selector.Positions) > 0 {
		// Specific cell positions
		for _, pos := range selector.Positions {
			if pos.Row >= tableRows {
				return nil, fmt.Errorf("%w: row %d is out of range (table has %d rows)", ErrInvalidCellSelector, pos.Row, tableRows)
			}
			if pos.Column >= tableCols {
				return nil, fmt.Errorf("%w: column %d is out of range (table has %d columns)", ErrInvalidCellSelector, pos.Column, tableCols)
			}
			positions = append(positions, pos)
		}
		return positions, nil
	}

	return nil, fmt.Errorf("%w: no valid cell selection", ErrInvalidCellSelector)
}

// buildStyleTableCellsRequests creates the batch update requests for styling table cells.
func buildStyleTableCellsRequests(tableObjectID string, positions []CellPosition, style *TableCellsStyleInput) ([]*slides.Request, []string) {
	var requests []*slides.Request
	var appliedStyles []string

	// Apply background color using UpdateTableCellPropertiesRequest
	if style.BackgroundColor != "" {
		color := parseHexColor(style.BackgroundColor)
		if color != nil {
			// Create one request per cell for background color
			for _, pos := range positions {
				requests = append(requests, &slides.Request{
					UpdateTableCellProperties: &slides.UpdateTableCellPropertiesRequest{
						ObjectId: tableObjectID,
						TableRange: &slides.TableRange{
							Location: &slides.TableCellLocation{
								RowIndex:    int64(pos.Row),
								ColumnIndex: int64(pos.Column),
							},
							RowSpan:    1,
							ColumnSpan: 1,
						},
						TableCellProperties: &slides.TableCellProperties{
							TableCellBackgroundFill: &slides.TableCellBackgroundFill{
								SolidFill: &slides.SolidFill{
									Color: &slides.OpaqueColor{
										RgbColor: color,
									},
								},
							},
						},
						Fields: "tableCellBackgroundFill.solidFill.color",
					},
				})
			}
			appliedStyles = append(appliedStyles, fmt.Sprintf("background_color=%s", style.BackgroundColor))
		}
	}

	// Apply borders using UpdateTableBorderPropertiesRequest
	borderPositions := map[string]*TableBorderInput{
		"TOP":    style.BorderTop,
		"BOTTOM": style.BorderBottom,
		"LEFT":   style.BorderLeft,
		"RIGHT":  style.BorderRight,
	}

	for borderPos, border := range borderPositions {
		if border == nil {
			continue
		}
		borderRequests, borderStyle := buildBorderRequests(tableObjectID, positions, borderPos, border)
		requests = append(requests, borderRequests...)
		if borderStyle != "" {
			appliedStyles = append(appliedStyles, borderStyle)
		}
	}

	return requests, appliedStyles
}

// buildBorderRequests creates the border update requests for a specific border position.
func buildBorderRequests(tableObjectID string, positions []CellPosition, borderPosition string, border *TableBorderInput) ([]*slides.Request, string) {
	var requests []*slides.Request
	var styleDesc strings.Builder

	styleDesc.WriteString(fmt.Sprintf("border_%s", strings.ToLower(borderPosition)))

	// Build border properties
	borderProps := &slides.TableBorderProperties{}
	var fields []string

	// Color
	if border.Color != "" {
		color := parseHexColor(border.Color)
		if color != nil {
			borderProps.TableBorderFill = &slides.TableBorderFill{
				SolidFill: &slides.SolidFill{
					Color: &slides.OpaqueColor{
						RgbColor: color,
					},
				},
			}
			fields = append(fields, "tableBorderFill.solidFill.color")
			styleDesc.WriteString(fmt.Sprintf("(color=%s", border.Color))
		}
	}

	// Width
	if border.Width > 0 {
		borderProps.Weight = &slides.Dimension{
			Magnitude: border.Width,
			Unit:      "PT",
		}
		fields = append(fields, "weight")
		if strings.Contains(styleDesc.String(), "(") {
			styleDesc.WriteString(fmt.Sprintf(",width=%.1f", border.Width))
		} else {
			styleDesc.WriteString(fmt.Sprintf("(width=%.1f", border.Width))
		}
	}

	// Dash style
	if border.DashStyle != "" {
		normalizedDash := validDashStyles[border.DashStyle]
		borderProps.DashStyle = normalizedDash
		fields = append(fields, "dashStyle")
		if strings.Contains(styleDesc.String(), "(") {
			styleDesc.WriteString(fmt.Sprintf(",dash=%s", normalizedDash))
		} else {
			styleDesc.WriteString(fmt.Sprintf("(dash=%s", normalizedDash))
		}
	}

	if strings.Contains(styleDesc.String(), "(") {
		styleDesc.WriteString(")")
	}

	if len(fields) == 0 {
		return nil, ""
	}

	// Create one request per cell position for this border
	for _, pos := range positions {
		requests = append(requests, &slides.Request{
			UpdateTableBorderProperties: &slides.UpdateTableBorderPropertiesRequest{
				ObjectId: tableObjectID,
				TableRange: &slides.TableRange{
					Location: &slides.TableCellLocation{
						RowIndex:    int64(pos.Row),
						ColumnIndex: int64(pos.Column),
					},
					RowSpan:    1,
					ColumnSpan: 1,
				},
				BorderPosition:        borderPosition,
				TableBorderProperties: borderProps,
				Fields:                strings.Join(fields, ","),
			},
		})
	}

	return requests, styleDesc.String()
}

