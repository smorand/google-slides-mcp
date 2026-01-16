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

// Sentinel errors for merge_cells tool.
var (
	ErrMergeCellsFailed   = errors.New("failed to merge cells")
	ErrUnmergeCellsFailed = errors.New("failed to unmerge cells")
	ErrInvalidMergeAction = errors.New("invalid merge action")
	ErrInvalidMergeRange  = errors.New("invalid merge range")
)

// MergeCellsInput represents the input for the merge_cells tool.
type MergeCellsInput struct {
	PresentationID string `json:"presentation_id"`
	ObjectID       string `json:"object_id"`       // Table object ID
	Action         string `json:"action"`          // 'merge' | 'unmerge'

	// For 'merge' action - defines the rectangular range to merge
	StartRow    int `json:"start_row"`    // 0-based starting row index
	StartColumn int `json:"start_column"` // 0-based starting column index
	EndRow      int `json:"end_row"`      // 0-based ending row index (exclusive)
	EndColumn   int `json:"end_column"`   // 0-based ending column index (exclusive)

	// For 'unmerge' action - position of a merged cell to unmerge
	Row    int `json:"row,omitempty"`    // 0-based row index of merged cell
	Column int `json:"column,omitempty"` // 0-based column index of merged cell
}

// MergeCellsOutput represents the output of the merge_cells tool.
type MergeCellsOutput struct {
	ObjectID string `json:"object_id"`
	Action   string `json:"action"`
	Range    string `json:"range"` // Description of the affected range
}

// validMergeActions maps action names to their normalized form.
var validMergeActions = map[string]string{
	"merge":   "merge",
	"MERGE":   "merge",
	"unmerge": "unmerge",
	"UNMERGE": "unmerge",
}

// MergeCells merges or unmerges cells in a table.
func (t *Tools) MergeCells(ctx context.Context, tokenSource oauth2.TokenSource, input MergeCellsInput) (*MergeCellsOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Normalize and validate action
	actionLower := strings.ToLower(input.Action)
	normalizedAction, ok := validMergeActions[actionLower]
	if !ok {
		return nil, fmt.Errorf("%w: action must be 'merge' or 'unmerge'", ErrInvalidMergeAction)
	}

	t.config.Logger.Info("merging/unmerging table cells",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", normalizedAction),
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

	// Build the requests based on action
	var requests []*slides.Request
	var rangeDescription string

	switch normalizedAction {
	case "merge":
		// Validate merge range
		if err := validateMergeRange(input.StartRow, input.StartColumn, input.EndRow, input.EndColumn, tableRows, tableCols); err != nil {
			return nil, err
		}

		rowSpan := input.EndRow - input.StartRow
		colSpan := input.EndColumn - input.StartColumn
		rangeDescription = fmt.Sprintf("rows %d-%d, columns %d-%d", input.StartRow, input.EndRow-1, input.StartColumn, input.EndColumn-1)

		requests = []*slides.Request{
			{
				MergeTableCells: &slides.MergeTableCellsRequest{
					ObjectId: input.ObjectID,
					TableRange: &slides.TableRange{
						Location: &slides.TableCellLocation{
							RowIndex:    int64(input.StartRow),
							ColumnIndex: int64(input.StartColumn),
						},
						RowSpan:    int64(rowSpan),
						ColumnSpan: int64(colSpan),
					},
				},
			},
		}

	case "unmerge":
		// Validate unmerge position
		if err := validateUnmergePosition(input.Row, input.Column, tableRows, tableCols); err != nil {
			return nil, err
		}

		rangeDescription = fmt.Sprintf("cell at row %d, column %d", input.Row, input.Column)

		// For unmerge, we use TableRange covering just the cell (span of 1x1 will unmerge any merged cell that contains this position)
		requests = []*slides.Request{
			{
				UnmergeTableCells: &slides.UnmergeTableCellsRequest{
					ObjectId: input.ObjectID,
					TableRange: &slides.TableRange{
						Location: &slides.TableCellLocation{
							RowIndex:    int64(input.Row),
							ColumnIndex: int64(input.Column),
						},
						RowSpan:    1,
						ColumnSpan: 1,
					},
				},
			},
		}
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
		if normalizedAction == "merge" {
			return nil, fmt.Errorf("%w: %v", ErrMergeCellsFailed, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrUnmergeCellsFailed, err)
	}

	output := &MergeCellsOutput{
		ObjectID: input.ObjectID,
		Action:   normalizedAction,
		Range:    rangeDescription,
	}

	t.config.Logger.Info("table cells merge/unmerge completed",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.String("action", output.Action),
		slog.String("range", output.Range),
	)

	return output, nil
}

// validateMergeRange validates the merge range is within table bounds and forms a valid rectangle.
func validateMergeRange(startRow, startCol, endRow, endCol, tableRows, tableCols int) error {
	// Validate non-negative indices
	if startRow < 0 || startCol < 0 {
		return fmt.Errorf("%w: start indices must be non-negative", ErrInvalidMergeRange)
	}

	// Validate end > start
	if endRow <= startRow {
		return fmt.Errorf("%w: end_row must be greater than start_row", ErrInvalidMergeRange)
	}
	if endCol <= startCol {
		return fmt.Errorf("%w: end_column must be greater than start_column", ErrInvalidMergeRange)
	}

	// Validate within table bounds
	if endRow > tableRows {
		return fmt.Errorf("%w: end_row %d exceeds table row count %d", ErrInvalidMergeRange, endRow, tableRows)
	}
	if endCol > tableCols {
		return fmt.Errorf("%w: end_column %d exceeds table column count %d", ErrInvalidMergeRange, endCol, tableCols)
	}

	// Validate that range spans at least 2 cells
	rowSpan := endRow - startRow
	colSpan := endCol - startCol
	if rowSpan == 1 && colSpan == 1 {
		return fmt.Errorf("%w: merge range must span at least 2 cells", ErrInvalidMergeRange)
	}

	return nil
}

// validateUnmergePosition validates the cell position is within table bounds.
func validateUnmergePosition(row, col, tableRows, tableCols int) error {
	// Validate non-negative indices
	if row < 0 || col < 0 {
		return fmt.Errorf("%w: row and column indices must be non-negative", ErrInvalidMergeRange)
	}

	// Validate within table bounds
	if row >= tableRows {
		return fmt.Errorf("%w: row %d is out of range (table has %d rows)", ErrInvalidMergeRange, row, tableRows)
	}
	if col >= tableCols {
		return fmt.Errorf("%w: column %d is out of range (table has %d columns)", ErrInvalidMergeRange, col, tableCols)
	}

	return nil
}
