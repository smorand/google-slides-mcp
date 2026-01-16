package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Helper to create a test table with given dimensions
func createTestTable(rows, cols int) *slides.Table {
	tableRows := make([]*slides.TableRow, rows)
	for i := range rows {
		cells := make([]*slides.TableCell, cols)
		for j := range cols {
			cells[j] = &slides.TableCell{}
		}
		tableRows[i] = &slides.TableRow{TableCells: cells}
	}
	return &slides.Table{TableRows: tableRows}
}

// Helper to create a test presentation with a table
func createPresentationWithTable(tableID string, rows, cols int) *slides.Presentation {
	return &slides.Presentation{
		PresentationId: "test-presentation",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				PageElements: []*slides.PageElement{
					{
						ObjectId: tableID,
						Table:    createTestTable(rows, cols),
					},
				},
			},
		},
	}
}

func TestModifyTableStructure(t *testing.T) {
	tests := []struct {
		name           string
		input          ModifyTableStructureInput
		mockService    func() *mockSlidesService
		wantErr        error
		wantRows       int
		wantCols       int
		validateReqs   func(t *testing.T, requests []*slides.Request)
	}{
		{
			name: "adds single row after index",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          1,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 4, // 3 + 1
			wantCols: 4,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.InsertTableRows == nil {
					t.Error("expected InsertTableRows request")
					return
				}
				if req.InsertTableRows.TableObjectId != "table-1" {
					t.Errorf("expected table-1, got %s", req.InsertTableRows.TableObjectId)
				}
				if req.InsertTableRows.CellLocation.RowIndex != 1 {
					t.Errorf("expected row index 1, got %d", req.InsertTableRows.CellLocation.RowIndex)
				}
				if req.InsertTableRows.Number != 1 {
					t.Errorf("expected 1 row, got %d", req.InsertTableRows.Number)
				}
				if !req.InsertTableRows.InsertBelow {
					t.Error("expected InsertBelow to be true by default")
				}
			},
		},
		{
			name: "adds multiple rows",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
				Count:          3,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 2, 3), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 5, // 2 + 3
			wantCols: 3,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.InsertTableRows.Number != 3 {
					t.Errorf("expected 3 rows, got %d", req.InsertTableRows.Number)
				}
			},
		},
		{
			name: "adds row above when insert_after is false",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
				InsertAfter:    boolPtr(false),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 3), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 4,
			wantCols: 3,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.InsertTableRows.InsertBelow {
					t.Error("expected InsertBelow to be false")
				}
			},
		},
		{
			name: "deletes single row",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_row",
				Index:          1,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 2, // 3 - 1
			wantCols: 4,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.DeleteTableRow == nil {
					t.Error("expected DeleteTableRow request")
					return
				}
				if req.DeleteTableRow.CellLocation.RowIndex != 1 {
					t.Errorf("expected row index 1, got %d", req.DeleteTableRow.CellLocation.RowIndex)
				}
			},
		},
		{
			name: "deletes multiple rows",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_row",
				Index:          1,
				Count:          2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 5, 3), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 3, // 5 - 2
			wantCols: 3,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				// Should create 2 delete requests (one per row, from highest to lowest)
				if len(requests) != 2 {
					t.Errorf("expected 2 requests, got %d", len(requests))
					return
				}
				// First request should delete index 2 (highest)
				if requests[0].DeleteTableRow.CellLocation.RowIndex != 2 {
					t.Errorf("expected first delete at index 2, got %d", requests[0].DeleteTableRow.CellLocation.RowIndex)
				}
				// Second request should delete index 1
				if requests[1].DeleteTableRow.CellLocation.RowIndex != 1 {
					t.Errorf("expected second delete at index 1, got %d", requests[1].DeleteTableRow.CellLocation.RowIndex)
				}
			},
		},
		{
			name: "adds single column",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_column",
				Index:          2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 3,
			wantCols: 5, // 4 + 1
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.InsertTableColumns == nil {
					t.Error("expected InsertTableColumns request")
					return
				}
				if req.InsertTableColumns.CellLocation.ColumnIndex != 2 {
					t.Errorf("expected column index 2, got %d", req.InsertTableColumns.CellLocation.ColumnIndex)
				}
				if req.InsertTableColumns.Number != 1 {
					t.Errorf("expected 1 column, got %d", req.InsertTableColumns.Number)
				}
				if !req.InsertTableColumns.InsertRight {
					t.Error("expected InsertRight to be true by default")
				}
			},
		},
		{
			name: "adds column to the left when insert_after is false",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_column",
				Index:          0,
				InsertAfter:    boolPtr(false),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 3), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 3,
			wantCols: 4,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.InsertTableColumns.InsertRight {
					t.Error("expected InsertRight to be false")
				}
			},
		},
		{
			name: "deletes single column",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_column",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 3,
			wantCols: 3, // 4 - 1
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.DeleteTableColumn == nil {
					t.Error("expected DeleteTableColumn request")
					return
				}
				if req.DeleteTableColumn.CellLocation.ColumnIndex != 0 {
					t.Errorf("expected column index 0, got %d", req.DeleteTableColumn.CellLocation.ColumnIndex)
				}
			},
		},
		{
			name: "deletes multiple columns",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_column",
				Index:          1,
				Count:          2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 5), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 3,
			wantCols: 3, // 5 - 2
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				// Should create 2 delete requests (one per column, from highest to lowest)
				if len(requests) != 2 {
					t.Errorf("expected 2 requests, got %d", len(requests))
					return
				}
				// First request should delete index 2 (highest)
				if requests[0].DeleteTableColumn.CellLocation.ColumnIndex != 2 {
					t.Errorf("expected first delete at index 2, got %d", requests[0].DeleteTableColumn.CellLocation.ColumnIndex)
				}
				// Second request should delete index 1
				if requests[1].DeleteTableColumn.CellLocation.ColumnIndex != 1 {
					t.Errorf("expected second delete at index 1, got %d", requests[1].DeleteTableColumn.CellLocation.ColumnIndex)
				}
			},
		},
		{
			name: "normalizes action to lowercase",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "ADD_ROW",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 2, 2), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantRows: 3,
			wantCols: 2,
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if requests[0].InsertTableRows == nil {
					t.Error("expected InsertTableRows request")
				}
			},
		},
		// Error cases
		{
			name: "returns error for empty presentation_id",
			input: ModifyTableStructureInput{
				PresentationID: "",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "returns error for empty object_id",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "",
				Action:         "add_row",
				Index:          0,
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "returns error for invalid action",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "invalid_action",
				Index:          0,
			},
			wantErr: ErrInvalidTableAction,
		},
		{
			name: "returns error for negative index",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          -1,
			},
			wantErr: ErrInvalidTableIndex,
		},
		{
			name: "returns error for negative count",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
				Count:          -5,
			},
			wantErr: ErrInvalidCount,
		},
		{
			name: "returns error when presentation not found",
			input: ModifyTableStructureInput{
				PresentationID: "nonexistent",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("googleapi: Error 404: File not found")
					},
				}
			},
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "returns error when access denied",
			input: ModifyTableStructureInput{
				PresentationID: "forbidden",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("googleapi: Error 403: Forbidden")
					},
				}
			},
			wantErr: ErrAccessDenied,
		},
		{
			name: "returns error when table not found",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "nonexistent-table",
				Action:         "add_row",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrObjectNotFound,
		},
		{
			name: "returns error when object is not a table",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "add_row",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: "test-presentation",
							Slides: []*slides.Page{
								{
									ObjectId: "slide-1",
									PageElements: []*slides.PageElement{
										{
											ObjectId: "shape-1",
											Shape:    &slides.Shape{ShapeType: "RECTANGLE"},
										},
									},
								},
							},
						}, nil
					},
				}
			},
			wantErr: ErrNotATable,
		},
		{
			name: "returns error when row index out of range for delete",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_row",
				Index:          5,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidTableIndex,
		},
		{
			name: "returns error when column index out of range for delete",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_column",
				Index:          10,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidTableIndex,
		},
		{
			name: "returns error when trying to delete all rows",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_row",
				Index:          0,
				Count:          3,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidCount,
		},
		{
			name: "returns error when trying to delete all columns",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_column",
				Index:          0,
				Count:          4,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidCount,
		},
		{
			name: "returns error when trying to delete more rows than available",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "delete_row",
				Index:          2,
				Count:          3,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 4, 4), nil
					},
				}
			},
			wantErr: ErrInvalidCount,
		},
		{
			name: "returns error when batch update fails",
			input: ModifyTableStructureInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "add_row",
				Index:          0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("batch update failed")
					},
				}
			},
			wantErr: ErrModifyTableStructureFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var slidesFactory SlidesServiceFactory
			var capturedRequests []*slides.Request

			if tt.mockService != nil {
				mock := tt.mockService()
				// Wrap batch update to capture requests
				originalBatchUpdate := mock.BatchUpdateFunc
				if originalBatchUpdate != nil {
					mock.BatchUpdateFunc = func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						capturedRequests = requests
						return originalBatchUpdate(ctx, presentationID, requests)
					}
				}
				slidesFactory = func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
					return mock, nil
				}
			}

			tools := NewTools(DefaultToolsConfig(), slidesFactory)
			output, err := tools.ModifyTableStructure(context.Background(), nil, tt.input)

			// Check error
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check output
			if output == nil {
				t.Error("expected output, got nil")
				return
			}

			if output.NewRows != tt.wantRows {
				t.Errorf("expected %d rows, got %d", tt.wantRows, output.NewRows)
			}
			if output.NewColumns != tt.wantCols {
				t.Errorf("expected %d columns, got %d", tt.wantCols, output.NewColumns)
			}

			// Validate requests if validator provided
			if tt.validateReqs != nil && capturedRequests != nil {
				tt.validateReqs(t, capturedRequests)
			}
		})
	}
}

func TestModifyTableStructure_AddRowAtEnd(t *testing.T) {
	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return createPresentationWithTable("table-1", 3, 4), nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)

	// Add row at max index (last position)
	input := ModifyTableStructureInput{
		PresentationID: "test-presentation",
		ObjectID:       "table-1",
		Action:         "add_row",
		Index:          3, // At end of 3-row table
	}

	output, err := tools.ModifyTableStructure(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.NewRows != 4 {
		t.Errorf("expected 4 rows, got %d", output.NewRows)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.InsertTableRows.CellLocation.RowIndex != 3 {
		t.Errorf("expected row index 3, got %d", req.InsertTableRows.CellLocation.RowIndex)
	}
}

func TestBuildModifyTableStructureRequests(t *testing.T) {
	tests := []struct {
		name        string
		tableID     string
		action      string
		index       int
		count       int
		insertAfter *bool
		validate    func(t *testing.T, requests []*slides.Request)
	}{
		{
			name:    "add_row creates InsertTableRows request",
			tableID: "table-1",
			action:  "add_row",
			index:   2,
			count:   1,
			validate: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.InsertTableRows == nil {
					t.Error("expected InsertTableRows")
					return
				}
				if req.InsertTableRows.TableObjectId != "table-1" {
					t.Errorf("expected table-1, got %s", req.InsertTableRows.TableObjectId)
				}
				if req.InsertTableRows.CellLocation.RowIndex != 2 {
					t.Errorf("expected row index 2, got %d", req.InsertTableRows.CellLocation.RowIndex)
				}
				if req.InsertTableRows.Number != 1 {
					t.Errorf("expected number 1, got %d", req.InsertTableRows.Number)
				}
				if !req.InsertTableRows.InsertBelow {
					t.Error("expected InsertBelow true")
				}
			},
		},
		{
			name:        "add_row with insertAfter false",
			tableID:     "table-1",
			action:      "add_row",
			index:       0,
			count:       2,
			insertAfter: boolPtr(false),
			validate: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.InsertTableRows.InsertBelow {
					t.Error("expected InsertBelow false")
				}
				if req.InsertTableRows.Number != 2 {
					t.Errorf("expected number 2, got %d", req.InsertTableRows.Number)
				}
			},
		},
		{
			name:    "delete_row creates DeleteTableRow requests in reverse order",
			tableID: "table-1",
			action:  "delete_row",
			index:   1,
			count:   3,
			validate: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 3 {
					t.Errorf("expected 3 requests, got %d", len(requests))
					return
				}
				// Should delete from highest to lowest: 3, 2, 1
				expectedIndices := []int64{3, 2, 1}
				for i, req := range requests {
					if req.DeleteTableRow == nil {
						t.Errorf("request %d: expected DeleteTableRow", i)
						continue
					}
					if req.DeleteTableRow.CellLocation.RowIndex != expectedIndices[i] {
						t.Errorf("request %d: expected index %d, got %d", i, expectedIndices[i], req.DeleteTableRow.CellLocation.RowIndex)
					}
				}
			},
		},
		{
			name:    "add_column creates InsertTableColumns request",
			tableID: "table-1",
			action:  "add_column",
			index:   1,
			count:   2,
			validate: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.InsertTableColumns == nil {
					t.Error("expected InsertTableColumns")
					return
				}
				if req.InsertTableColumns.CellLocation.ColumnIndex != 1 {
					t.Errorf("expected column index 1, got %d", req.InsertTableColumns.CellLocation.ColumnIndex)
				}
				if req.InsertTableColumns.Number != 2 {
					t.Errorf("expected number 2, got %d", req.InsertTableColumns.Number)
				}
				if !req.InsertTableColumns.InsertRight {
					t.Error("expected InsertRight true")
				}
			},
		},
		{
			name:        "add_column with insertAfter false",
			tableID:     "table-1",
			action:      "add_column",
			index:       0,
			count:       1,
			insertAfter: boolPtr(false),
			validate: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.InsertTableColumns.InsertRight {
					t.Error("expected InsertRight false")
				}
			},
		},
		{
			name:    "delete_column creates DeleteTableColumn requests in reverse order",
			tableID: "table-1",
			action:  "delete_column",
			index:   0,
			count:   2,
			validate: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 2 {
					t.Errorf("expected 2 requests, got %d", len(requests))
					return
				}
				// Should delete from highest to lowest: 1, 0
				expectedIndices := []int64{1, 0}
				for i, req := range requests {
					if req.DeleteTableColumn == nil {
						t.Errorf("request %d: expected DeleteTableColumn", i)
						continue
					}
					if req.DeleteTableColumn.CellLocation.ColumnIndex != expectedIndices[i] {
						t.Errorf("request %d: expected index %d, got %d", i, expectedIndices[i], req.DeleteTableColumn.CellLocation.ColumnIndex)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildModifyTableStructureRequests(tt.tableID, tt.action, tt.index, tt.count, tt.insertAfter)
			tt.validate(t, requests)
		})
	}
}

func TestFindTableByID(t *testing.T) {
	tests := []struct {
		name         string
		presentation *slides.Presentation
		objectID     string
		wantFound    bool
	}{
		{
			name: "finds table in slide",
			presentation: &slides.Presentation{
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "table-1", Table: &slides.Table{}},
						},
					},
				},
			},
			objectID:  "table-1",
			wantFound: true,
		},
		{
			name: "finds table in nested group",
			presentation: &slides.Presentation{
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "table-1", Table: &slides.Table{}},
									},
								},
							},
						},
					},
				},
			},
			objectID:  "table-1",
			wantFound: true,
		},
		{
			name: "finds table in layout",
			presentation: &slides.Presentation{
				Slides: []*slides.Page{
					{ObjectId: "slide-1", PageElements: []*slides.PageElement{}},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "table-in-layout", Table: &slides.Table{}},
						},
					},
				},
			},
			objectID:  "table-in-layout",
			wantFound: true,
		},
		{
			name: "finds table in master",
			presentation: &slides.Presentation{
				Slides: []*slides.Page{
					{ObjectId: "slide-1", PageElements: []*slides.PageElement{}},
				},
				Masters: []*slides.Page{
					{
						ObjectId: "master-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "table-in-master", Table: &slides.Table{}},
						},
					},
				},
			},
			objectID:  "table-in-master",
			wantFound: true,
		},
		{
			name: "returns nil when not found",
			presentation: &slides.Presentation{
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{ObjectId: "table-1", Table: &slides.Table{}},
						},
					},
				},
			},
			objectID:  "nonexistent",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findTableByID(tt.presentation, tt.objectID)
			if tt.wantFound && result == nil {
				t.Error("expected to find element, got nil")
			}
			if !tt.wantFound && result != nil {
				t.Errorf("expected nil, got %+v", result)
			}
		})
	}
}
