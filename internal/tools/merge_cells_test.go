package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestMergeCells(t *testing.T) {
	tests := []struct {
		name         string
		input        MergeCellsInput
		mockService  func() *mockSlidesService
		wantErr      error
		wantAction   string
		wantRange    string
		validateReqs func(t *testing.T, requests []*slides.Request)
	}{
		{
			name: "merges cells in a 2x2 range",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
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
			wantAction: "merge",
			wantRange:  "rows 0-1, columns 0-1",
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.MergeTableCells == nil {
					t.Error("expected MergeTableCells request")
					return
				}
				if req.MergeTableCells.ObjectId != "table-1" {
					t.Errorf("expected table-1, got %s", req.MergeTableCells.ObjectId)
				}
				if req.MergeTableCells.TableRange.Location.RowIndex != 0 {
					t.Errorf("expected row index 0, got %d", req.MergeTableCells.TableRange.Location.RowIndex)
				}
				if req.MergeTableCells.TableRange.Location.ColumnIndex != 0 {
					t.Errorf("expected column index 0, got %d", req.MergeTableCells.TableRange.Location.ColumnIndex)
				}
				if req.MergeTableCells.TableRange.RowSpan != 2 {
					t.Errorf("expected row span 2, got %d", req.MergeTableCells.TableRange.RowSpan)
				}
				if req.MergeTableCells.TableRange.ColumnSpan != 2 {
					t.Errorf("expected column span 2, got %d", req.MergeTableCells.TableRange.ColumnSpan)
				}
			},
		},
		{
			name: "merges entire row",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       1,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      4,
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
			wantAction: "merge",
			wantRange:  "rows 1-1, columns 0-3",
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.MergeTableCells.TableRange.RowSpan != 1 {
					t.Errorf("expected row span 1, got %d", req.MergeTableCells.TableRange.RowSpan)
				}
				if req.MergeTableCells.TableRange.ColumnSpan != 4 {
					t.Errorf("expected column span 4, got %d", req.MergeTableCells.TableRange.ColumnSpan)
				}
			},
		},
		{
			name: "merges entire column",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    2,
				EndRow:         3,
				EndColumn:      3,
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
			wantAction: "merge",
			wantRange:  "rows 0-2, columns 2-2",
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.MergeTableCells.TableRange.RowSpan != 3 {
					t.Errorf("expected row span 3, got %d", req.MergeTableCells.TableRange.RowSpan)
				}
				if req.MergeTableCells.TableRange.ColumnSpan != 1 {
					t.Errorf("expected column span 1, got %d", req.MergeTableCells.TableRange.ColumnSpan)
				}
			},
		},
		{
			name: "unmerges cells at position",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "unmerge",
				Row:            1,
				Column:         2,
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
			wantAction: "unmerge",
			wantRange:  "cell at row 1, column 2",
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.UnmergeTableCells == nil {
					t.Error("expected UnmergeTableCells request")
					return
				}
				if req.UnmergeTableCells.ObjectId != "table-1" {
					t.Errorf("expected table-1, got %s", req.UnmergeTableCells.ObjectId)
				}
				if req.UnmergeTableCells.TableRange.Location.RowIndex != 1 {
					t.Errorf("expected row index 1, got %d", req.UnmergeTableCells.TableRange.Location.RowIndex)
				}
				if req.UnmergeTableCells.TableRange.Location.ColumnIndex != 2 {
					t.Errorf("expected column index 2, got %d", req.UnmergeTableCells.TableRange.Location.ColumnIndex)
				}
				if req.UnmergeTableCells.TableRange.RowSpan != 1 {
					t.Errorf("expected row span 1, got %d", req.UnmergeTableCells.TableRange.RowSpan)
				}
				if req.UnmergeTableCells.TableRange.ColumnSpan != 1 {
					t.Errorf("expected column span 1, got %d", req.UnmergeTableCells.TableRange.ColumnSpan)
				}
			},
		},
		{
			name: "action is case-insensitive (MERGE)",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "MERGE",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
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
			wantAction: "merge",
			wantRange:  "rows 0-1, columns 0-1",
		},
		{
			name: "action is case-insensitive (UNMERGE)",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "UNMERGE",
				Row:            0,
				Column:         0,
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
			wantAction: "unmerge",
			wantRange:  "cell at row 0, column 0",
		},
		{
			name: "returns error for empty presentation_id",
			input: MergeCellsInput{
				PresentationID: "",
				ObjectID:       "table-1",
				Action:         "merge",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "returns error for empty object_id",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "",
				Action:         "merge",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "returns error for invalid action",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "invalid",
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidMergeAction,
		},
		{
			name: "returns error when presentation not found",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("404 Not Found")
					},
				}
			},
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "returns error when access denied",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return nil, errors.New("403 Forbidden")
					},
				}
			},
			wantErr: ErrAccessDenied,
		},
		{
			name: "returns error when table not found",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "nonexistent-table",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
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
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
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
			name: "returns error for merge range with negative start row",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       -1,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for merge range exceeding table rows",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         5, // Table only has 3 rows
				EndColumn:      2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for merge range exceeding table columns",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      6, // Table only has 4 columns
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for merge range with end_row <= start_row",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       2,
				StartColumn:    0,
				EndRow:         2, // Same as start
				EndColumn:      2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for merge range with end_column <= start_column",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    2,
				EndRow:         2,
				EndColumn:      1, // Less than start
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for merge range of single cell",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       1,
				StartColumn:    1,
				EndRow:         2,
				EndColumn:      2, // Only 1x1 range
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for unmerge with negative row",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "unmerge",
				Row:            -1,
				Column:         0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for unmerge with row out of range",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "unmerge",
				Row:            5, // Table only has 3 rows
				Column:         0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error for unmerge with column out of range",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "unmerge",
				Row:            0,
				Column:         10, // Table only has 4 columns
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidMergeRange,
		},
		{
			name: "returns error when batch update fails (merge)",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "merge",
				StartRow:       0,
				StartColumn:    0,
				EndRow:         2,
				EndColumn:      2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("merge failed: non-rectangular range")
					},
				}
			},
			wantErr: ErrMergeCellsFailed,
		},
		{
			name: "returns error when batch update fails (unmerge)",
			input: MergeCellsInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Action:         "unmerge",
				Row:            0,
				Column:         0,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("unmerge failed")
					},
				}
			},
			wantErr: ErrUnmergeCellsFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture requests for validation
			var capturedRequests []*slides.Request
			mockSvc := tt.mockService()
			if originalBatchUpdate := mockSvc.BatchUpdateFunc; originalBatchUpdate != nil {
				mockSvc.BatchUpdateFunc = func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedRequests = requests
					return originalBatchUpdate(ctx, presentationID, requests)
				}
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSvc, nil
			})

			output, err := tools.MergeCells(context.Background(), nil, tt.input)

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

			if output.ObjectID != tt.input.ObjectID {
				t.Errorf("expected object_id %s, got %s", tt.input.ObjectID, output.ObjectID)
			}

			if output.Action != tt.wantAction {
				t.Errorf("expected action %s, got %s", tt.wantAction, output.Action)
			}

			if output.Range != tt.wantRange {
				t.Errorf("expected range %s, got %s", tt.wantRange, output.Range)
			}

			if tt.validateReqs != nil {
				tt.validateReqs(t, capturedRequests)
			}
		})
	}
}

func TestValidateMergeRange(t *testing.T) {
	tests := []struct {
		name      string
		startRow  int
		startCol  int
		endRow    int
		endCol    int
		tableRows int
		tableCols int
		wantErr   bool
	}{
		{
			name:      "valid 2x2 range",
			startRow:  0,
			startCol:  0,
			endRow:    2,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   false,
		},
		{
			name:      "valid full row merge",
			startRow:  1,
			startCol:  0,
			endRow:    2,
			endCol:    4,
			tableRows: 3,
			tableCols: 4,
			wantErr:   false,
		},
		{
			name:      "valid full column merge",
			startRow:  0,
			startCol:  2,
			endRow:    3,
			endCol:    3,
			tableRows: 3,
			tableCols: 4,
			wantErr:   false,
		},
		{
			name:      "negative start row",
			startRow:  -1,
			startCol:  0,
			endRow:    2,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "negative start column",
			startRow:  0,
			startCol:  -1,
			endRow:    2,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "end_row not greater than start_row",
			startRow:  2,
			startCol:  0,
			endRow:    2,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "end_column not greater than start_column",
			startRow:  0,
			startCol:  2,
			endRow:    2,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "end_row exceeds table rows",
			startRow:  0,
			startCol:  0,
			endRow:    5,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "end_column exceeds table columns",
			startRow:  0,
			startCol:  0,
			endRow:    2,
			endCol:    6,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "single cell range (1x1)",
			startRow:  1,
			startCol:  1,
			endRow:    2,
			endCol:    2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMergeRange(tt.startRow, tt.startCol, tt.endRow, tt.endCol, tt.tableRows, tt.tableCols)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMergeRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUnmergePosition(t *testing.T) {
	tests := []struct {
		name      string
		row       int
		col       int
		tableRows int
		tableCols int
		wantErr   bool
	}{
		{
			name:      "valid position",
			row:       1,
			col:       2,
			tableRows: 3,
			tableCols: 4,
			wantErr:   false,
		},
		{
			name:      "valid position at origin",
			row:       0,
			col:       0,
			tableRows: 3,
			tableCols: 4,
			wantErr:   false,
		},
		{
			name:      "valid position at last cell",
			row:       2,
			col:       3,
			tableRows: 3,
			tableCols: 4,
			wantErr:   false,
		},
		{
			name:      "negative row",
			row:       -1,
			col:       0,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "negative column",
			row:       0,
			col:       -1,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "row out of range",
			row:       5,
			col:       0,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
		{
			name:      "column out of range",
			row:       0,
			col:       10,
			tableRows: 3,
			tableCols: 4,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnmergePosition(tt.row, tt.col, tt.tableRows, tt.tableCols)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUnmergePosition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
