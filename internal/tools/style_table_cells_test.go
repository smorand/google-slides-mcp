package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestStyleTableCells(t *testing.T) {
	ctx := context.Background()

	createPresentationWithTableForStyle := func(tableID string, rows, cols int) *slides.Presentation {
		tableRows := make([]*slides.TableRow, rows)
		for r := 0; r < rows; r++ {
			cells := make([]*slides.TableCell, cols)
			for c := 0; c < cols; c++ {
				cells[c] = &slides.TableCell{}
			}
			tableRows[r] = &slides.TableRow{
				TableCells: cells,
			}
		}
		return &slides.Presentation{
			PresentationId: "test-pres",
			Slides: []*slides.Page{
				{
					ObjectId: "slide-1",
					PageElements: []*slides.PageElement{
						{
							ObjectId: tableID,
							Table: &slides.Table{
								TableRows: tableRows,
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name         string
		input        StyleTableCellsInput
		mockPres     *slides.Presentation
		mockError    error
		wantError    bool
		wantErrType  error
		checkRequest func(t *testing.T, requests []*slides.Request)
	}{
		{
			name: "apply background color to all cells",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style: &TableCellsStyleInput{
					BackgroundColor: "#FF0000",
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 2, 3),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 6 requests (2 rows x 3 cols)
				if len(requests) != 6 {
					t.Errorf("expected 6 requests, got %d", len(requests))
					return
				}
				// Verify first request
				req := requests[0]
				if req.UpdateTableCellProperties == nil {
					t.Error("expected UpdateTableCellProperties request")
					return
				}
				if req.UpdateTableCellProperties.TableCellProperties.TableCellBackgroundFill == nil {
					t.Error("expected TableCellBackgroundFill to be set")
				}
			},
		},
		{
			name: "apply background color to specific row",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{Row: intPtr(1)},
				Style: &TableCellsStyleInput{
					BackgroundColor: "#00FF00",
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 3, 4),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 4 requests (row 1, 4 columns)
				if len(requests) != 4 {
					t.Errorf("expected 4 requests, got %d", len(requests))
					return
				}
				// All should target row 1
				for _, req := range requests {
					if req.UpdateTableCellProperties.TableRange.Location.RowIndex != 1 {
						t.Errorf("expected row 1, got %d", req.UpdateTableCellProperties.TableRange.Location.RowIndex)
					}
				}
			},
		},
		{
			name: "apply background color to specific column",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{Column: intPtr(2)},
				Style: &TableCellsStyleInput{
					BackgroundColor: "#0000FF",
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 3, 4),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 3 requests (column 2, 3 rows)
				if len(requests) != 3 {
					t.Errorf("expected 3 requests, got %d", len(requests))
					return
				}
				// All should target column 2
				for _, req := range requests {
					if req.UpdateTableCellProperties.TableRange.Location.ColumnIndex != 2 {
						t.Errorf("expected column 2, got %d", req.UpdateTableCellProperties.TableRange.Location.ColumnIndex)
					}
				}
			},
		},
		{
			name: "apply background color to specific cells",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells: CellSelector{
					Positions: []CellPosition{
						{Row: 0, Column: 0},
						{Row: 1, Column: 1},
					},
				},
				Style: &TableCellsStyleInput{
					BackgroundColor: "#FFFF00",
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 3, 3),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 2 requests (2 specific cells)
				if len(requests) != 2 {
					t.Errorf("expected 2 requests, got %d", len(requests))
				}
			},
		},
		{
			name: "apply border to cells",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style: &TableCellsStyleInput{
					BorderTop: &TableBorderInput{
						Color: "#000000",
						Width: 2.0,
					},
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 2, 2),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 4 requests (4 cells with top border)
				if len(requests) != 4 {
					t.Errorf("expected 4 requests, got %d", len(requests))
					return
				}
				// Check first request
				req := requests[0]
				if req.UpdateTableBorderProperties == nil {
					t.Error("expected UpdateTableBorderProperties request")
					return
				}
				if req.UpdateTableBorderProperties.BorderPosition != "TOP" {
					t.Errorf("expected TOP border, got %s", req.UpdateTableBorderProperties.BorderPosition)
				}
			},
		},
		{
			name: "apply multiple borders to cells",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{Positions: []CellPosition{{Row: 0, Column: 0}}},
				Style: &TableCellsStyleInput{
					BorderTop: &TableBorderInput{
						Color: "#FF0000",
						Width: 1.0,
					},
					BorderBottom: &TableBorderInput{
						Color: "#00FF00",
						Width: 1.0,
					},
					BorderLeft: &TableBorderInput{
						Color: "#0000FF",
						Width: 1.0,
					},
					BorderRight: &TableBorderInput{
						Color: "#FFFF00",
						Width: 1.0,
					},
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 2, 2),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 4 requests (4 borders for 1 cell)
				if len(requests) != 4 {
					t.Errorf("expected 4 requests, got %d", len(requests))
				}
			},
		},
		{
			name: "apply border with dash style",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style: &TableCellsStyleInput{
					BorderBottom: &TableBorderInput{
						Color:     "#000000",
						Width:     1.5,
						DashStyle: "DASH",
					},
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 1, 1),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.UpdateTableBorderProperties.TableBorderProperties.DashStyle != "DASH" {
					t.Errorf("expected DASH, got %s", req.UpdateTableBorderProperties.TableBorderProperties.DashStyle)
				}
			},
		},
		{
			name: "apply background and borders together",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style: &TableCellsStyleInput{
					BackgroundColor: "#FFFFFF",
					BorderTop: &TableBorderInput{
						Color: "#000000",
						Width: 1.0,
					},
				},
			},
			mockPres: createPresentationWithTableForStyle("table-1", 1, 1),
			checkRequest: func(t *testing.T, requests []*slides.Request) {
				// Should have 2 requests (1 background + 1 border)
				if len(requests) != 2 {
					t.Errorf("expected 2 requests, got %d", len(requests))
				}
			},
		},
		{
			name: "error: empty presentation_id",
			input: StyleTableCellsInput{
				PresentationID: "",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			wantError:   true,
			wantErrType: ErrInvalidPresentationID,
		},
		{
			name: "error: empty object_id",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			wantError:   true,
			wantErrType: ErrInvalidObjectID,
		},
		{
			name: "error: no style provided",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style:          nil,
			},
			wantError:   true,
			wantErrType: ErrNoCellStyle,
		},
		{
			name: "error: empty style",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{},
			},
			wantError:   true,
			wantErrType: ErrNoCellStyle,
		},
		{
			name: "error: invalid dash style",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style: &TableCellsStyleInput{
					BorderTop: &TableBorderInput{
						DashStyle: "INVALID",
					},
				},
			},
			wantError:   true,
			wantErrType: ErrStyleTableCellsFailed,
		},
		{
			name: "error: object not found",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "nonexistent-table",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockPres:    createPresentationWithTableForStyle("table-1", 2, 2),
			wantError:   true,
			wantErrType: ErrObjectNotFound,
		},
		{
			name: "error: object is not a table",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "shape-1",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockPres: &slides.Presentation{
				PresentationId: "test-pres",
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
			},
			wantError:   true,
			wantErrType: ErrNotATable,
		},
		{
			name: "error: row out of range",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{Row: intPtr(10)},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockPres:    createPresentationWithTableForStyle("table-1", 3, 3),
			wantError:   true,
			wantErrType: ErrInvalidCellSelector,
		},
		{
			name: "error: column out of range",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{Column: intPtr(10)},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockPres:    createPresentationWithTableForStyle("table-1", 3, 3),
			wantError:   true,
			wantErrType: ErrInvalidCellSelector,
		},
		{
			name: "error: cell position out of range",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells: CellSelector{
					Positions: []CellPosition{{Row: 10, Column: 0}},
				},
				Style: &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockPres:    createPresentationWithTableForStyle("table-1", 3, 3),
			wantError:   true,
			wantErrType: ErrInvalidCellSelector,
		},
		{
			name: "error: presentation not found",
			input: StyleTableCellsInput{
				PresentationID: "nonexistent",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockError:   errors.New("404 not found"),
			wantError:   true,
			wantErrType: ErrPresentationNotFound,
		},
		{
			name: "error: access denied",
			input: StyleTableCellsInput{
				PresentationID: "test-pres",
				ObjectID:       "table-1",
				Cells:          CellSelector{All: true},
				Style:          &TableCellsStyleInput{BackgroundColor: "#FF0000"},
			},
			mockError:   errors.New("403 forbidden"),
			wantError:   true,
			wantErrType: ErrAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequests []*slides.Request

			mockSlidesService := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockPres, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					capturedRequests = requests
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			tools := NewTools(DefaultToolsConfig(), func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlidesService, nil
			})

			output, err := tools.StyleTableCells(ctx, nil, tt.input)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.wantErrType != nil && !errors.Is(err, tt.wantErrType) {
					t.Errorf("expected error type %v, got %v", tt.wantErrType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output == nil {
				t.Error("expected output, got nil")
				return
			}

			if output.ObjectID != tt.input.ObjectID {
				t.Errorf("expected object_id %s, got %s", tt.input.ObjectID, output.ObjectID)
			}

			if tt.checkRequest != nil {
				tt.checkRequest(t, capturedRequests)
			}
		})
	}
}

func TestParseCellSelector(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantAll   bool
		wantRow   *int
		wantCol   *int
		wantPos   []CellPosition
		wantError bool
	}{
		{
			name:    "all string",
			input:   "all",
			wantAll: true,
		},
		{
			name:    "ALL uppercase",
			input:   "ALL",
			wantAll: true,
		},
		{
			name:    "row:0",
			input:   "row:0",
			wantRow: intPtr(0),
		},
		{
			name:    "row:5",
			input:   "row:5",
			wantRow: intPtr(5),
		},
		{
			name:    "column:0",
			input:   "column:0",
			wantCol: intPtr(0),
		},
		{
			name:    "column:3",
			input:   "column:3",
			wantCol: intPtr(3),
		},
		{
			name: "array of positions",
			input: []interface{}{
				map[string]interface{}{"row": float64(0), "column": float64(1)},
				map[string]interface{}{"row": float64(2), "column": float64(3)},
			},
			wantPos: []CellPosition{
				{Row: 0, Column: 1},
				{Row: 2, Column: 3},
			},
		},
		{
			name:      "invalid string",
			input:     "invalid",
			wantError: true,
		},
		{
			name:      "row with negative",
			input:     "row:-1",
			wantError: true,
		},
		{
			name:      "column with negative",
			input:     "column:-1",
			wantError: true,
		},
		{
			name:      "empty array",
			input:     []interface{}{},
			wantError: true,
		},
		{
			name: "array missing row",
			input: []interface{}{
				map[string]interface{}{"column": float64(1)},
			},
			wantError: true,
		},
		{
			name: "array missing column",
			input: []interface{}{
				map[string]interface{}{"row": float64(0)},
			},
			wantError: true,
		},
		{
			name:      "invalid type",
			input:     12345,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCellSelector(tt.input)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.All != tt.wantAll {
				t.Errorf("All: expected %v, got %v", tt.wantAll, result.All)
			}

			if (result.Row == nil) != (tt.wantRow == nil) {
				t.Errorf("Row: expected %v, got %v", tt.wantRow, result.Row)
			} else if tt.wantRow != nil && *result.Row != *tt.wantRow {
				t.Errorf("Row: expected %d, got %d", *tt.wantRow, *result.Row)
			}

			if (result.Column == nil) != (tt.wantCol == nil) {
				t.Errorf("Column: expected %v, got %v", tt.wantCol, result.Column)
			} else if tt.wantCol != nil && *result.Column != *tt.wantCol {
				t.Errorf("Column: expected %d, got %d", *tt.wantCol, *result.Column)
			}

			if len(result.Positions) != len(tt.wantPos) {
				t.Errorf("Positions: expected %d, got %d", len(tt.wantPos), len(result.Positions))
			} else {
				for i, pos := range result.Positions {
					if pos.Row != tt.wantPos[i].Row || pos.Column != tt.wantPos[i].Column {
						t.Errorf("Position[%d]: expected (%d,%d), got (%d,%d)",
							i, tt.wantPos[i].Row, tt.wantPos[i].Column, pos.Row, pos.Column)
					}
				}
			}
		})
	}
}

func TestResolveCellPositions(t *testing.T) {
	tests := []struct {
		name      string
		selector  CellSelector
		rows      int
		cols      int
		wantCount int
		wantError bool
	}{
		{
			name:      "all cells in 3x4 table",
			selector:  CellSelector{All: true},
			rows:      3,
			cols:      4,
			wantCount: 12,
		},
		{
			name:      "row 1 in 3x4 table",
			selector:  CellSelector{Row: intPtr(1)},
			rows:      3,
			cols:      4,
			wantCount: 4,
		},
		{
			name:      "column 2 in 3x4 table",
			selector:  CellSelector{Column: intPtr(2)},
			rows:      3,
			cols:      4,
			wantCount: 3,
		},
		{
			name: "specific positions",
			selector: CellSelector{
				Positions: []CellPosition{
					{Row: 0, Column: 0},
					{Row: 1, Column: 1},
					{Row: 2, Column: 2},
				},
			},
			rows:      3,
			cols:      3,
			wantCount: 3,
		},
		{
			name:      "row out of range",
			selector:  CellSelector{Row: intPtr(10)},
			rows:      3,
			cols:      3,
			wantError: true,
		},
		{
			name:      "column out of range",
			selector:  CellSelector{Column: intPtr(10)},
			rows:      3,
			cols:      3,
			wantError: true,
		},
		{
			name: "position row out of range",
			selector: CellSelector{
				Positions: []CellPosition{{Row: 10, Column: 0}},
			},
			rows:      3,
			cols:      3,
			wantError: true,
		},
		{
			name: "position column out of range",
			selector: CellSelector{
				Positions: []CellPosition{{Row: 0, Column: 10}},
			},
			rows:      3,
			cols:      3,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positions, err := resolveCellPositions(tt.selector, tt.rows, tt.cols)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(positions) != tt.wantCount {
				t.Errorf("expected %d positions, got %d", tt.wantCount, len(positions))
			}
		})
	}
}

func TestBuildStyleTableCellsRequests(t *testing.T) {
	positions := []CellPosition{{Row: 0, Column: 0}}

	tests := []struct {
		name          string
		style         *TableCellsStyleInput
		wantReqCount  int
		wantStyles    int
	}{
		{
			name: "background color only",
			style: &TableCellsStyleInput{
				BackgroundColor: "#FF0000",
			},
			wantReqCount: 1,
			wantStyles:   1,
		},
		{
			name: "single border",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{
					Color: "#000000",
					Width: 1.0,
				},
			},
			wantReqCount: 1,
			wantStyles:   1,
		},
		{
			name: "all four borders",
			style: &TableCellsStyleInput{
				BorderTop:    &TableBorderInput{Color: "#000000"},
				BorderBottom: &TableBorderInput{Color: "#000000"},
				BorderLeft:   &TableBorderInput{Color: "#000000"},
				BorderRight:  &TableBorderInput{Color: "#000000"},
			},
			wantReqCount: 4,
			wantStyles:   4,
		},
		{
			name: "background and borders",
			style: &TableCellsStyleInput{
				BackgroundColor: "#FFFFFF",
				BorderTop:       &TableBorderInput{Color: "#000000"},
				BorderBottom:    &TableBorderInput{Color: "#000000"},
			},
			wantReqCount: 3, // 1 background + 2 borders
			wantStyles:   3,
		},
		{
			name: "border with all properties",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{
					Color:     "#FF0000",
					Width:     2.5,
					DashStyle: "DASH",
				},
			},
			wantReqCount: 1,
			wantStyles:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, styles := buildStyleTableCellsRequests("table-1", positions, tt.style)

			if len(requests) != tt.wantReqCount {
				t.Errorf("expected %d requests, got %d", tt.wantReqCount, len(requests))
			}

			if len(styles) != tt.wantStyles {
				t.Errorf("expected %d styles, got %d", tt.wantStyles, len(styles))
			}
		})
	}
}

func TestHasAnyStyle(t *testing.T) {
	tests := []struct {
		name  string
		style *TableCellsStyleInput
		want  bool
	}{
		{
			name:  "nil style",
			style: nil,
			want:  false,
		},
		{
			name:  "empty style",
			style: &TableCellsStyleInput{},
			want:  false,
		},
		{
			name: "only background color",
			style: &TableCellsStyleInput{
				BackgroundColor: "#FF0000",
			},
			want: true,
		},
		{
			name: "only border top",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{Color: "#000000"},
			},
			want: true,
		},
		{
			name: "only border bottom",
			style: &TableCellsStyleInput{
				BorderBottom: &TableBorderInput{Width: 1.0},
			},
			want: true,
		},
		{
			name: "only border left",
			style: &TableCellsStyleInput{
				BorderLeft: &TableBorderInput{DashStyle: "SOLID"},
			},
			want: true,
		},
		{
			name: "only border right",
			style: &TableCellsStyleInput{
				BorderRight: &TableBorderInput{Color: "#000000"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAnyStyle(tt.style)
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestValidateBorderStyles(t *testing.T) {
	tests := []struct {
		name      string
		style     *TableCellsStyleInput
		wantError bool
	}{
		{
			name: "valid SOLID",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{DashStyle: "SOLID"},
			},
			wantError: false,
		},
		{
			name: "valid lowercase dash",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{DashStyle: "dash"},
			},
			wantError: false,
		},
		{
			name: "valid LONG_DASH_DOT",
			style: &TableCellsStyleInput{
				BorderBottom: &TableBorderInput{DashStyle: "LONG_DASH_DOT"},
			},
			wantError: false,
		},
		{
			name: "invalid dash style",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{DashStyle: "INVALID"},
			},
			wantError: true,
		},
		{
			name: "no dash style specified",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{Color: "#000000"},
			},
			wantError: false,
		},
		{
			name: "empty border",
			style: &TableCellsStyleInput{
				BorderTop: &TableBorderInput{},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBorderStyles(tt.style)
			if (err != nil) != tt.wantError {
				t.Errorf("wantError=%v, got error=%v", tt.wantError, err)
			}
		})
	}
}
