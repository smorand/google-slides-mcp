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

// Sentinel errors for modify_table_structure tool.
var (
	ErrModifyTableStructureFailed = errors.New("failed to modify table structure")
	ErrInvalidTableAction         = errors.New("invalid table action")
	ErrInvalidTableIndex          = errors.New("invalid table index")
	ErrNotATable                  = errors.New("object is not a table")
	ErrInvalidCount               = errors.New("count must be at least 1")
)

// ModifyTableStructureInput represents the input for the modify_table_structure tool.
type ModifyTableStructureInput struct {
	PresentationID string `json:"presentation_id"`
	ObjectID       string `json:"object_id"`                // Table object ID
	Action         string `json:"action"`                   // 'add_row' | 'delete_row' | 'add_column' | 'delete_column'
	Index          int    `json:"index"`                    // 0-based index where to add/which to delete
	Count          int    `json:"count,omitempty"`          // How many to add/delete (default 1)
	InsertAfter    *bool  `json:"insert_after,omitempty"`   // For add actions: insert after index (default true)
}

// ModifyTableStructureOutput represents the output of the modify_table_structure tool.
type ModifyTableStructureOutput struct {
	ObjectID    string `json:"object_id"`
	Action      string `json:"action"`
	Index       int    `json:"index"`
	Count       int    `json:"count"`
	NewRows     int    `json:"new_rows"`     // Updated row count
	NewColumns  int    `json:"new_columns"`  // Updated column count
}

// validTableActions maps action names to their normalized form.
var validTableActions = map[string]string{
	"add_row":       "add_row",
	"ADD_ROW":       "add_row",
	"delete_row":    "delete_row",
	"DELETE_ROW":    "delete_row",
	"add_column":    "add_column",
	"ADD_COLUMN":    "add_column",
	"delete_column": "delete_column",
	"DELETE_COLUMN": "delete_column",
}

// ModifyTableStructure adds or removes rows/columns from a table.
func (t *Tools) ModifyTableStructure(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyTableStructureInput) (*ModifyTableStructureOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Normalize and validate action
	actionLower := strings.ToLower(input.Action)
	normalizedAction, ok := validTableActions[actionLower]
	if !ok {
		return nil, fmt.Errorf("%w: action must be 'add_row', 'delete_row', 'add_column', or 'delete_column'", ErrInvalidTableAction)
	}

	// Default count to 1 if not provided
	if input.Count == 0 {
		input.Count = 1
	}
	if input.Count < 1 {
		return nil, fmt.Errorf("%w: count must be at least 1", ErrInvalidCount)
	}

	// Validate index is non-negative
	if input.Index < 0 {
		return nil, fmt.Errorf("%w: index must be non-negative", ErrInvalidTableIndex)
	}

	t.config.Logger.Info("modifying table structure",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", normalizedAction),
		slog.Int("index", input.Index),
		slog.Int("count", input.Count),
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
	currentRows := len(table.TableRows)
	currentCols := 0
	if currentRows > 0 && len(table.TableRows[0].TableCells) > 0 {
		currentCols = len(table.TableRows[0].TableCells)
	}

	// Validate index is within bounds for delete operations
	switch normalizedAction {
	case "delete_row":
		if input.Index >= currentRows {
			return nil, fmt.Errorf("%w: row index %d is out of range (table has %d rows)", ErrInvalidTableIndex, input.Index, currentRows)
		}
		// Check if we would delete too many rows
		if input.Index+input.Count > currentRows {
			return nil, fmt.Errorf("%w: cannot delete %d rows starting at index %d (table has %d rows)", ErrInvalidCount, input.Count, input.Index, currentRows)
		}
		// Table must have at least 1 row after deletion
		if currentRows-input.Count < 1 {
			return nil, fmt.Errorf("%w: cannot delete all rows (table must have at least 1 row)", ErrInvalidCount)
		}
	case "delete_column":
		if input.Index >= currentCols {
			return nil, fmt.Errorf("%w: column index %d is out of range (table has %d columns)", ErrInvalidTableIndex, input.Index, currentCols)
		}
		// Check if we would delete too many columns
		if input.Index+input.Count > currentCols {
			return nil, fmt.Errorf("%w: cannot delete %d columns starting at index %d (table has %d columns)", ErrInvalidCount, input.Count, input.Index, currentCols)
		}
		// Table must have at least 1 column after deletion
		if currentCols-input.Count < 1 {
			return nil, fmt.Errorf("%w: cannot delete all columns (table must have at least 1 column)", ErrInvalidCount)
		}
	case "add_row":
		// For add, allow index up to current row count (inserting at end)
		if input.Index > currentRows {
			return nil, fmt.Errorf("%w: row index %d is out of range (table has %d rows, max index is %d)", ErrInvalidTableIndex, input.Index, currentRows, currentRows)
		}
	case "add_column":
		// For add, allow index up to current column count (inserting at end)
		if input.Index > currentCols {
			return nil, fmt.Errorf("%w: column index %d is out of range (table has %d columns, max index is %d)", ErrInvalidTableIndex, input.Index, currentCols, currentCols)
		}
	}

	// Build the requests
	requests := buildModifyTableStructureRequests(input.ObjectID, normalizedAction, input.Index, input.Count, input.InsertAfter)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrModifyTableStructureFailed, err)
	}

	// Calculate new dimensions
	newRows := currentRows
	newCols := currentCols
	switch normalizedAction {
	case "add_row":
		newRows += input.Count
	case "delete_row":
		newRows -= input.Count
	case "add_column":
		newCols += input.Count
	case "delete_column":
		newCols -= input.Count
	}

	output := &ModifyTableStructureOutput{
		ObjectID:   input.ObjectID,
		Action:     normalizedAction,
		Index:      input.Index,
		Count:      input.Count,
		NewRows:    newRows,
		NewColumns: newCols,
	}

	t.config.Logger.Info("table structure modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.String("action", output.Action),
		slog.Int("new_rows", output.NewRows),
		slog.Int("new_columns", output.NewColumns),
	)

	return output, nil
}

// buildModifyTableStructureRequests creates the batch update requests for table structure modification.
func buildModifyTableStructureRequests(tableObjectID, action string, index, count int, insertAfter *bool) []*slides.Request {
	requests := []*slides.Request{}

	switch action {
	case "add_row":
		// Default insertAfter to true (insert below)
		insertBelow := true
		if insertAfter != nil {
			insertBelow = *insertAfter
		}
		// For multiple rows, we create one request with the count
		requests = append(requests, &slides.Request{
			InsertTableRows: &slides.InsertTableRowsRequest{
				TableObjectId: tableObjectID,
				CellLocation: &slides.TableCellLocation{
					RowIndex: int64(index),
				},
				InsertBelow: insertBelow,
				Number:      int64(count),
			},
		})
	case "delete_row":
		// Delete rows one at a time starting from the highest index to avoid shifting issues
		for i := count - 1; i >= 0; i-- {
			requests = append(requests, &slides.Request{
				DeleteTableRow: &slides.DeleteTableRowRequest{
					TableObjectId: tableObjectID,
					CellLocation: &slides.TableCellLocation{
						RowIndex: int64(index + i),
					},
				},
			})
		}
	case "add_column":
		// Default insertAfter to true (insert right)
		insertRight := true
		if insertAfter != nil {
			insertRight = *insertAfter
		}
		// For multiple columns, we create one request with the count
		requests = append(requests, &slides.Request{
			InsertTableColumns: &slides.InsertTableColumnsRequest{
				TableObjectId: tableObjectID,
				CellLocation: &slides.TableCellLocation{
					ColumnIndex: int64(index),
				},
				InsertRight: insertRight,
				Number:      int64(count),
			},
		})
	case "delete_column":
		// Delete columns one at a time starting from the highest index to avoid shifting issues
		for i := count - 1; i >= 0; i-- {
			requests = append(requests, &slides.Request{
				DeleteTableColumn: &slides.DeleteTableColumnRequest{
					TableObjectId: tableObjectID,
					CellLocation: &slides.TableCellLocation{
						ColumnIndex: int64(index + i),
					},
				},
			})
		}
	}

	return requests
}

// findTableByID finds a page element by ID across all slides in a presentation.
func findTableByID(presentation *slides.Presentation, objectID string) *slides.PageElement {
	// Search through all slides
	for _, slide := range presentation.Slides {
		if element := findElementByID(slide.PageElements, objectID); element != nil {
			return element
		}
	}
	// Also search through layouts and masters if needed
	for _, layout := range presentation.Layouts {
		if element := findElementByID(layout.PageElements, objectID); element != nil {
			return element
		}
	}
	for _, master := range presentation.Masters {
		if element := findElementByID(master.PageElements, objectID); element != nil {
			return element
		}
	}
	return nil
}
