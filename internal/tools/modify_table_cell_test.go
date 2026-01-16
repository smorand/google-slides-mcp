package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestModifyTableCell(t *testing.T) {
	tests := []struct {
		name          string
		input         ModifyTableCellInput
		mockService   func() *mockSlidesService
		wantErr       error
		wantRow       int
		wantColumn    int
		wantProps     []string
		validateReqs  func(t *testing.T, requests []*slides.Request)
	}{
		{
			name: "sets cell text content",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         1,
				Text:           ptrString("Hello World"),
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
			wantRow:    0,
			wantColumn: 1,
			wantProps:  []string{"text"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 2 {
					t.Errorf("expected 2 requests (delete + insert), got %d", len(requests))
					return
				}
				// First request should be DeleteText
				if requests[0].DeleteText == nil {
					t.Error("expected first request to be DeleteText")
					return
				}
				if requests[0].DeleteText.ObjectId != "table-1" {
					t.Errorf("expected table-1, got %s", requests[0].DeleteText.ObjectId)
				}
				if requests[0].DeleteText.CellLocation.RowIndex != 0 {
					t.Errorf("expected row index 0, got %d", requests[0].DeleteText.CellLocation.RowIndex)
				}
				if requests[0].DeleteText.CellLocation.ColumnIndex != 1 {
					t.Errorf("expected column index 1, got %d", requests[0].DeleteText.CellLocation.ColumnIndex)
				}
				// Second request should be InsertText
				if requests[1].InsertText == nil {
					t.Error("expected second request to be InsertText")
					return
				}
				if requests[1].InsertText.Text != "Hello World" {
					t.Errorf("expected 'Hello World', got '%s'", requests[1].InsertText.Text)
				}
			},
		},
		{
			name: "clears cell text with empty string",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            1,
				Column:         2,
				Text:           ptrString(""),
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
			wantRow:    1,
			wantColumn: 2,
			wantProps:  []string{"text"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				// Only DeleteText, no InsertText for empty string
				if len(requests) != 1 {
					t.Errorf("expected 1 request (delete only), got %d", len(requests))
					return
				}
				if requests[0].DeleteText == nil {
					t.Error("expected DeleteText request")
				}
			},
		},
		{
			name: "applies text styling",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Style: &TableCellStyleInput{
					FontFamily:      "Arial",
					FontSize:        24,
					Bold:            ptrBoolMTC(true),
					Italic:          ptrBoolMTC(false),
					ForegroundColor: "#FF0000",
				},
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
			wantRow:    0,
			wantColumn: 0,
			wantProps:  []string{"font_family=Arial", "font_size=24", "bold=true", "italic=false", "foreground_color=#FF0000"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.UpdateTextStyle == nil {
					t.Error("expected UpdateTextStyle request")
					return
				}
				if req.UpdateTextStyle.ObjectId != "table-1" {
					t.Errorf("expected table-1, got %s", req.UpdateTextStyle.ObjectId)
				}
				if req.UpdateTextStyle.CellLocation == nil {
					t.Error("expected CellLocation to be set")
					return
				}
				if req.UpdateTextStyle.CellLocation.RowIndex != 0 {
					t.Errorf("expected row index 0, got %d", req.UpdateTextStyle.CellLocation.RowIndex)
				}
				if req.UpdateTextStyle.Style.FontFamily != "Arial" {
					t.Errorf("expected Arial, got %s", req.UpdateTextStyle.Style.FontFamily)
				}
				if req.UpdateTextStyle.Style.FontSize.Magnitude != 24 {
					t.Errorf("expected font size 24, got %f", req.UpdateTextStyle.Style.FontSize.Magnitude)
				}
				if !req.UpdateTextStyle.Style.Bold {
					t.Error("expected bold to be true")
				}
			},
		},
		{
			name: "applies horizontal alignment",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            1,
				Column:         1,
				Alignment: &TableCellAlignInput{
					Horizontal: "CENTER",
				},
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
			wantRow:    1,
			wantColumn: 1,
			wantProps:  []string{"horizontal_alignment=CENTER"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.UpdateParagraphStyle == nil {
					t.Error("expected UpdateParagraphStyle request")
					return
				}
				if req.UpdateParagraphStyle.Style.Alignment != "CENTER" {
					t.Errorf("expected CENTER, got %s", req.UpdateParagraphStyle.Style.Alignment)
				}
				if req.UpdateParagraphStyle.CellLocation == nil {
					t.Error("expected CellLocation to be set")
				}
			},
		},
		{
			name: "applies vertical alignment",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            2,
				Column:         0,
				Alignment: &TableCellAlignInput{
					Vertical: "MIDDLE",
				},
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
			wantRow:    2,
			wantColumn: 0,
			wantProps:  []string{"vertical_alignment=MIDDLE"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.UpdateTableCellProperties == nil {
					t.Error("expected UpdateTableCellProperties request")
					return
				}
				if req.UpdateTableCellProperties.TableCellProperties.ContentAlignment != "MIDDLE" {
					t.Errorf("expected MIDDLE, got %s", req.UpdateTableCellProperties.TableCellProperties.ContentAlignment)
				}
				if req.UpdateTableCellProperties.TableRange.Location.RowIndex != 2 {
					t.Errorf("expected row 2, got %d", req.UpdateTableCellProperties.TableRange.Location.RowIndex)
				}
			},
		},
		{
			name: "applies both horizontal and vertical alignment",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Alignment: &TableCellAlignInput{
					Horizontal: "END",
					Vertical:   "BOTTOM",
				},
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
			wantRow:    0,
			wantColumn: 0,
			wantProps:  []string{"horizontal_alignment=END", "vertical_alignment=BOTTOM"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 2 {
					t.Errorf("expected 2 requests, got %d", len(requests))
					return
				}
				// First should be UpdateParagraphStyle for horizontal
				if requests[0].UpdateParagraphStyle == nil {
					t.Error("expected first request to be UpdateParagraphStyle")
				}
				// Second should be UpdateTableCellProperties for vertical
				if requests[1].UpdateTableCellProperties == nil {
					t.Error("expected second request to be UpdateTableCellProperties")
				}
			},
		},
		{
			name: "combines text, styling, and alignment",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            1,
				Column:         2,
				Text:           ptrString("Styled Text"),
				Style: &TableCellStyleInput{
					Bold:     ptrBoolMTC(true),
					FontSize: 18,
				},
				Alignment: &TableCellAlignInput{
					Horizontal: "CENTER",
				},
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
			wantRow:    1,
			wantColumn: 2,
			wantProps:  []string{"text", "font_size=18", "bold=true", "horizontal_alignment=CENTER"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				// Expect: DeleteText, InsertText, UpdateTextStyle, UpdateParagraphStyle
				if len(requests) != 4 {
					t.Errorf("expected 4 requests, got %d", len(requests))
					return
				}
				if requests[0].DeleteText == nil {
					t.Error("expected first request to be DeleteText")
				}
				if requests[1].InsertText == nil {
					t.Error("expected second request to be InsertText")
				}
				if requests[2].UpdateTextStyle == nil {
					t.Error("expected third request to be UpdateTextStyle")
				}
				if requests[3].UpdateParagraphStyle == nil {
					t.Error("expected fourth request to be UpdateParagraphStyle")
				}
			},
		},
		{
			name: "horizontal alignment is case-insensitive",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Alignment: &TableCellAlignInput{
					Horizontal: "center",
				},
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
			wantRow:    0,
			wantColumn: 0,
			wantProps:  []string{"horizontal_alignment=CENTER"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if requests[0].UpdateParagraphStyle.Style.Alignment != "CENTER" {
					t.Errorf("expected CENTER, got %s", requests[0].UpdateParagraphStyle.Style.Alignment)
				}
			},
		},
		{
			name: "vertical alignment is case-insensitive",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Alignment: &TableCellAlignInput{
					Vertical: "top",
				},
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
			wantRow:    0,
			wantColumn: 0,
			wantProps:  []string{"vertical_alignment=TOP"},
			validateReqs: func(t *testing.T, requests []*slides.Request) {
				if requests[0].UpdateTableCellProperties.TableCellProperties.ContentAlignment != "TOP" {
					t.Errorf("expected TOP, got %s", requests[0].UpdateTableCellProperties.TableCellProperties.ContentAlignment)
				}
			},
		},
		{
			name: "returns error for empty presentation_id",
			input: ModifyTableCellInput{
				PresentationID: "",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "returns error for empty object_id",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidObjectID,
		},
		{
			name: "returns error for negative row",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            -1,
				Column:         0,
				Text:           ptrString("test"),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidCellIndex,
		},
		{
			name: "returns error for negative column",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         -1,
				Text:           ptrString("test"),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidCellIndex,
		},
		{
			name: "returns error when no modification provided",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				// No text, style, or alignment
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrNoCellModification,
		},
		{
			name: "returns error for invalid horizontal alignment",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Alignment: &TableCellAlignInput{
					Horizontal: "INVALID",
				},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidHorizontalAlign,
		},
		{
			name: "returns error for invalid vertical alignment",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Alignment: &TableCellAlignInput{
					Vertical: "WRONG",
				},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{}
			},
			wantErr: ErrInvalidVerticalAlign,
		},
		{
			name: "returns error when presentation not found",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
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
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
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
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "nonexistent-table",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
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
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "shape-1",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
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
			name: "returns error for row out of range",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            10, // Table only has 3 rows
				Column:         0,
				Text:           ptrString("test"),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidCellIndex,
		},
		{
			name: "returns error for column out of range",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         10, // Table only has 4 columns
				Text:           ptrString("test"),
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return createPresentationWithTable("table-1", 3, 4), nil
					},
				}
			},
			wantErr: ErrInvalidCellIndex,
		},
		{
			name: "returns error when batch update fails",
			input: ModifyTableCellInput{
				PresentationID: "test-presentation",
				ObjectID:       "table-1",
				Row:            0,
				Column:         0,
				Text:           ptrString("test"),
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
			wantErr: ErrModifyTableCellFailed,
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

			output, err := tools.ModifyTableCell(context.Background(), nil, tt.input)

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

			if output.Row != tt.wantRow {
				t.Errorf("expected row %d, got %d", tt.wantRow, output.Row)
			}

			if output.Column != tt.wantColumn {
				t.Errorf("expected column %d, got %d", tt.wantColumn, output.Column)
			}

			// Check that expected properties are present
			for _, wantProp := range tt.wantProps {
				found := false
				for _, gotProp := range output.ModifiedProperties {
					if strings.Contains(gotProp, strings.Split(wantProp, "=")[0]) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected property %s in modified properties, got %v", wantProp, output.ModifiedProperties)
				}
			}

			if tt.validateReqs != nil {
				tt.validateReqs(t, capturedRequests)
			}
		})
	}
}

func TestBuildModifyTableCellRequests(t *testing.T) {
	tests := []struct {
		name         string
		input        ModifyTableCellInput
		wantReqCount int
		wantProps    []string
	}{
		{
			name: "text only",
			input: ModifyTableCellInput{
				ObjectID: "table-1",
				Row:      0,
				Column:   0,
				Text:     ptrString("test"),
			},
			wantReqCount: 2, // delete + insert
			wantProps:    []string{"text"},
		},
		{
			name: "empty text only (delete)",
			input: ModifyTableCellInput{
				ObjectID: "table-1",
				Row:      0,
				Column:   0,
				Text:     ptrString(""),
			},
			wantReqCount: 1, // delete only
			wantProps:    []string{"text"},
		},
		{
			name: "style only",
			input: ModifyTableCellInput{
				ObjectID: "table-1",
				Row:      0,
				Column:   0,
				Style: &TableCellStyleInput{
					FontFamily: "Arial",
					FontSize:   12,
				},
			},
			wantReqCount: 1,
			wantProps:    []string{"font_family=Arial", "font_size=12"},
		},
		{
			name: "horizontal alignment only",
			input: ModifyTableCellInput{
				ObjectID: "table-1",
				Row:      0,
				Column:   0,
				Alignment: &TableCellAlignInput{
					Horizontal: "CENTER",
				},
			},
			wantReqCount: 1,
			wantProps:    []string{"horizontal_alignment=CENTER"},
		},
		{
			name: "vertical alignment only",
			input: ModifyTableCellInput{
				ObjectID: "table-1",
				Row:      0,
				Column:   0,
				Alignment: &TableCellAlignInput{
					Vertical: "MIDDLE",
				},
			},
			wantReqCount: 1,
			wantProps:    []string{"vertical_alignment=MIDDLE"},
		},
		{
			name: "all properties",
			input: ModifyTableCellInput{
				ObjectID: "table-1",
				Row:      1,
				Column:   2,
				Text:     ptrString("styled text"),
				Style: &TableCellStyleInput{
					Bold: ptrBoolMTC(true),
				},
				Alignment: &TableCellAlignInput{
					Horizontal: "END",
					Vertical:   "BOTTOM",
				},
			},
			wantReqCount: 5, // delete + insert + style + horizontal + vertical
			wantProps:    []string{"text", "bold=true", "horizontal_alignment=END", "vertical_alignment=BOTTOM"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, props := buildModifyTableCellRequests(tt.input)

			if len(requests) != tt.wantReqCount {
				t.Errorf("expected %d requests, got %d", tt.wantReqCount, len(requests))
			}

			for _, wantProp := range tt.wantProps {
				found := false
				for _, prop := range props {
					if prop == wantProp || strings.HasPrefix(prop, strings.Split(wantProp, "=")[0]) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected property %s in props, got %v", wantProp, props)
				}
			}
		})
	}
}

func TestBuildTableCellStyleRequest(t *testing.T) {
	tests := []struct {
		name       string
		style      *TableCellStyleInput
		wantNil    bool
		wantFields int
	}{
		{
			name: "all style properties",
			style: &TableCellStyleInput{
				FontFamily:      "Arial",
				FontSize:        24,
				Bold:            ptrBoolMTC(true),
				Italic:          ptrBoolMTC(true),
				Underline:       ptrBoolMTC(true),
				Strikethrough:   ptrBoolMTC(true),
				ForegroundColor: "#FF0000",
				BackgroundColor: "#00FF00",
			},
			wantNil:    false,
			wantFields: 8,
		},
		{
			name: "partial style properties",
			style: &TableCellStyleInput{
				FontFamily: "Roboto",
				Bold:       ptrBoolMTC(true),
			},
			wantNil:    false,
			wantFields: 2,
		},
		{
			name:       "empty style",
			style:      &TableCellStyleInput{},
			wantNil:    true,
			wantFields: 0,
		},
		{
			name: "invalid color is ignored",
			style: &TableCellStyleInput{
				ForegroundColor: "not-a-color",
				Bold:            ptrBoolMTC(true),
			},
			wantNil:    false,
			wantFields: 1, // Only bold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cellLocation := &slides.TableCellLocation{
				RowIndex:    0,
				ColumnIndex: 0,
			}
			req, props := buildTableCellStyleRequest("table-1", cellLocation, tt.style)

			if tt.wantNil {
				if req != nil {
					t.Error("expected nil request")
				}
				return
			}

			if req == nil {
				t.Error("expected non-nil request")
				return
			}

			if len(props) != tt.wantFields {
				t.Errorf("expected %d properties, got %d", tt.wantFields, len(props))
			}
		})
	}
}

// Helper function for creating pointer values
// Note: ptrString is defined in modify_image_test.go

func ptrBoolMTC(b bool) *bool {
	return &b
}
