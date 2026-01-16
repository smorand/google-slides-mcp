package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestCreateTable(t *testing.T) {
	// Override time function for deterministic object IDs
	originalTimeFunc := tableTimeNowFunc
	tableTimeNowFunc = func() time.Time {
		return time.Unix(1234567890, 123456789)
	}
	defer func() { tableTimeNowFunc = originalTimeFunc }()

	expectedObjectID := "table_1234567890123456789"

	tests := []struct {
		name           string
		input          CreateTableInput
		mockService    func() *mockSlidesService
		wantErr        error
		wantObjectID   bool
		wantRows       int
		wantColumns    int
	}{
		{
			name: "creates table with slide_index",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           3,
				Columns:        4,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
								{ObjectId: "slide-2"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						if len(requests) != 1 {
							t.Errorf("expected 1 request, got %d", len(requests))
						}
						req := requests[0]
						if req.CreateTable == nil {
							t.Error("expected CreateTable request")
						}
						if req.CreateTable.ObjectId != expectedObjectID {
							t.Errorf("expected object ID %s, got %s", expectedObjectID, req.CreateTable.ObjectId)
						}
						if req.CreateTable.Rows != 3 {
							t.Errorf("expected 3 rows, got %d", req.CreateTable.Rows)
						}
						if req.CreateTable.Columns != 4 {
							t.Errorf("expected 4 columns, got %d", req.CreateTable.Columns)
						}
						if req.CreateTable.ElementProperties.PageObjectId != "slide-1" {
							t.Errorf("expected slide-1, got %s", req.CreateTable.ElementProperties.PageObjectId)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
			wantRows:     3,
			wantColumns:  4,
		},
		{
			name: "creates table with slide_id",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideID:        "custom-slide-id",
				Rows:           2,
				Columns:        2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides: []*slides.Page{
								{ObjectId: "slide-1"},
								{ObjectId: "custom-slide-id"},
							},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						req := requests[0]
						if req.CreateTable.ElementProperties.PageObjectId != "custom-slide-id" {
							t.Errorf("expected custom-slide-id, got %s", req.CreateTable.ElementProperties.PageObjectId)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
			wantRows:     2,
			wantColumns:  2,
		},
		{
			name: "creates table with position and size",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        3,
				Position:       &PositionInput{X: 100, Y: 50},
				Size:           &SizeInput{Width: 400, Height: 200},
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						req := requests[0]
						// Check transform for position
						if req.CreateTable.ElementProperties.Transform == nil {
							t.Error("expected Transform to be set")
						} else {
							expectedX := pointsToEMU(100)
							expectedY := pointsToEMU(50)
							if req.CreateTable.ElementProperties.Transform.TranslateX != expectedX {
								t.Errorf("expected TranslateX %f, got %f", expectedX, req.CreateTable.ElementProperties.Transform.TranslateX)
							}
							if req.CreateTable.ElementProperties.Transform.TranslateY != expectedY {
								t.Errorf("expected TranslateY %f, got %f", expectedY, req.CreateTable.ElementProperties.Transform.TranslateY)
							}
						}
						// Check size
						if req.CreateTable.ElementProperties.Size == nil {
							t.Error("expected Size to be set")
						} else {
							expectedWidth := pointsToEMU(400)
							expectedHeight := pointsToEMU(200)
							if req.CreateTable.ElementProperties.Size.Width.Magnitude != expectedWidth {
								t.Errorf("expected width %f, got %f", expectedWidth, req.CreateTable.ElementProperties.Size.Width.Magnitude)
							}
							if req.CreateTable.ElementProperties.Size.Height.Magnitude != expectedHeight {
								t.Errorf("expected height %f, got %f", expectedHeight, req.CreateTable.ElementProperties.Size.Height.Magnitude)
							}
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
			wantRows:     2,
			wantColumns:  3,
		},
		{
			name: "creates table with single row and column",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           1,
				Columns:        1,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						req := requests[0]
						if req.CreateTable.Rows != 1 {
							t.Errorf("expected 1 row, got %d", req.CreateTable.Rows)
						}
						if req.CreateTable.Columns != 1 {
							t.Errorf("expected 1 column, got %d", req.CreateTable.Columns)
						}
						return &slides.BatchUpdatePresentationResponse{}, nil
					},
				}
			},
			wantObjectID: true,
			wantRows:     1,
			wantColumns:  1,
		},
		// Error cases
		{
			name: "returns error for empty presentation_id",
			input: CreateTableInput{
				PresentationID: "",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
			},
			wantErr: ErrInvalidPresentationID,
		},
		{
			name: "returns error when neither slide_index nor slide_id provided",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				Rows:           2,
				Columns:        2,
			},
			wantErr: ErrInvalidSlideReference,
		},
		{
			name: "returns error for zero rows",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           0,
				Columns:        2,
			},
			wantErr: ErrInvalidRowCount,
		},
		{
			name: "returns error for negative rows",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           -1,
				Columns:        2,
			},
			wantErr: ErrInvalidRowCount,
		},
		{
			name: "returns error for zero columns",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        0,
			},
			wantErr: ErrInvalidColCount,
		},
		{
			name: "returns error for negative columns",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        -5,
			},
			wantErr: ErrInvalidColCount,
		},
		{
			name: "returns error for invalid size (zero width)",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
				Size:           &SizeInput{Width: 0, Height: 100},
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error for invalid size (negative height)",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
				Size:           &SizeInput{Width: 100, Height: -50},
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "returns error when presentation not found",
			input: CreateTableInput{
				PresentationID: "nonexistent",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
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
			input: CreateTableInput{
				PresentationID: "forbidden",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
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
			name: "returns error when slide not found by index",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     5,
				Rows:           2,
				Columns:        2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
				}
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "returns error when slide not found by ID",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideID:        "nonexistent-slide",
				Rows:           2,
				Columns:        2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
				}
			},
			wantErr: ErrSlideNotFound,
		},
		{
			name: "returns error when batch update fails",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("batch update failed")
					},
				}
			},
			wantErr: ErrCreateTableFailed,
		},
		{
			name: "returns error when batch update returns not found",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("googleapi: Error 404: not found")
					},
				}
			},
			wantErr: ErrPresentationNotFound,
		},
		{
			name: "returns error when batch update returns forbidden",
			input: CreateTableInput{
				PresentationID: "test-presentation",
				SlideIndex:     1,
				Rows:           2,
				Columns:        2,
			},
			mockService: func() *mockSlidesService {
				return &mockSlidesService{
					GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
						return &slides.Presentation{
							PresentationId: presentationID,
							Slides:         []*slides.Page{{ObjectId: "slide-1"}},
						}, nil
					},
					BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
						return nil, errors.New("googleapi: Error 403: forbidden")
					},
				}
			},
			wantErr: ErrAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var slidesFactory SlidesServiceFactory
			if tt.mockService != nil {
				mock := tt.mockService()
				slidesFactory = func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
					return mock, nil
				}
			}

			tools := NewTools(DefaultToolsConfig(), slidesFactory)
			output, err := tools.CreateTable(context.Background(), nil, tt.input)

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

			if tt.wantObjectID && output.ObjectID == "" {
				t.Error("expected object_id to be set")
			}

			if tt.wantObjectID && output.ObjectID != expectedObjectID {
				t.Errorf("expected object ID %s, got %s", expectedObjectID, output.ObjectID)
			}

			if output.Rows != tt.wantRows {
				t.Errorf("expected rows %d, got %d", tt.wantRows, output.Rows)
			}
			if output.Columns != tt.wantColumns {
				t.Errorf("expected columns %d, got %d", tt.wantColumns, output.Columns)
			}
		})
	}
}

func TestCreateTable_DefaultPosition(t *testing.T) {
	// Override time function for deterministic object IDs
	originalTimeFunc := tableTimeNowFunc
	tableTimeNowFunc = func() time.Time {
		return time.Unix(1234567890, 123456789)
	}
	defer func() { tableTimeNowFunc = originalTimeFunc }()

	var capturedRequests []*slides.Request

	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
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

	// Test without position - should default to (0, 0)
	input := CreateTableInput{
		PresentationID: "test-presentation",
		SlideIndex:     1,
		Rows:           2,
		Columns:        2,
	}

	_, err := tools.CreateTable(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	req := capturedRequests[0]
	if req.CreateTable.ElementProperties.Transform == nil {
		t.Error("expected Transform to be set for default position")
	} else {
		if req.CreateTable.ElementProperties.Transform.TranslateX != 0 {
			t.Errorf("expected default TranslateX 0, got %f", req.CreateTable.ElementProperties.Transform.TranslateX)
		}
		if req.CreateTable.ElementProperties.Transform.TranslateY != 0 {
			t.Errorf("expected default TranslateY 0, got %f", req.CreateTable.ElementProperties.Transform.TranslateY)
		}
	}
}

func TestGenerateTableObjectID(t *testing.T) {
	// Override time function
	originalTimeFunc := tableTimeNowFunc
	tableTimeNowFunc = func() time.Time {
		return time.Unix(1234567890, 123456789)
	}
	defer func() { tableTimeNowFunc = originalTimeFunc }()

	objectID := generateTableObjectID()
	expected := "table_1234567890123456789"

	if objectID != expected {
		t.Errorf("expected %s, got %s", expected, objectID)
	}
}

func TestBuildCreateTableRequests(t *testing.T) {
	tests := []struct {
		name     string
		objectID string
		slideID  string
		input    CreateTableInput
		validate func(t *testing.T, requests []*slides.Request)
	}{
		{
			name:     "basic table without position or size",
			objectID: "table-123",
			slideID:  "slide-1",
			input: CreateTableInput{
				Rows:    3,
				Columns: 4,
			},
			validate: func(t *testing.T, requests []*slides.Request) {
				if len(requests) != 1 {
					t.Errorf("expected 1 request, got %d", len(requests))
					return
				}
				req := requests[0]
				if req.CreateTable == nil {
					t.Error("expected CreateTable request")
					return
				}
				if req.CreateTable.ObjectId != "table-123" {
					t.Errorf("expected object ID table-123, got %s", req.CreateTable.ObjectId)
				}
				if req.CreateTable.Rows != 3 {
					t.Errorf("expected 3 rows, got %d", req.CreateTable.Rows)
				}
				if req.CreateTable.Columns != 4 {
					t.Errorf("expected 4 columns, got %d", req.CreateTable.Columns)
				}
				if req.CreateTable.ElementProperties.PageObjectId != "slide-1" {
					t.Errorf("expected slide-1, got %s", req.CreateTable.ElementProperties.PageObjectId)
				}
				// No transform or size when not specified
				if req.CreateTable.ElementProperties.Transform != nil {
					t.Error("expected no Transform when position not specified")
				}
				if req.CreateTable.ElementProperties.Size != nil {
					t.Error("expected no Size when not specified")
				}
			},
		},
		{
			name:     "table with position",
			objectID: "table-456",
			slideID:  "slide-2",
			input: CreateTableInput{
				Rows:     2,
				Columns:  3,
				Position: &PositionInput{X: 100, Y: 200},
			},
			validate: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.CreateTable.ElementProperties.Transform == nil {
					t.Error("expected Transform to be set")
					return
				}
				expectedX := pointsToEMU(100)
				expectedY := pointsToEMU(200)
				if req.CreateTable.ElementProperties.Transform.TranslateX != expectedX {
					t.Errorf("expected TranslateX %f, got %f", expectedX, req.CreateTable.ElementProperties.Transform.TranslateX)
				}
				if req.CreateTable.ElementProperties.Transform.TranslateY != expectedY {
					t.Errorf("expected TranslateY %f, got %f", expectedY, req.CreateTable.ElementProperties.Transform.TranslateY)
				}
				if req.CreateTable.ElementProperties.Transform.ScaleX != 1 {
					t.Errorf("expected ScaleX 1, got %f", req.CreateTable.ElementProperties.Transform.ScaleX)
				}
				if req.CreateTable.ElementProperties.Transform.ScaleY != 1 {
					t.Errorf("expected ScaleY 1, got %f", req.CreateTable.ElementProperties.Transform.ScaleY)
				}
				if req.CreateTable.ElementProperties.Transform.Unit != "EMU" {
					t.Errorf("expected Unit EMU, got %s", req.CreateTable.ElementProperties.Transform.Unit)
				}
			},
		},
		{
			name:     "table with size",
			objectID: "table-789",
			slideID:  "slide-3",
			input: CreateTableInput{
				Rows:    1,
				Columns: 5,
				Size:    &SizeInput{Width: 500, Height: 100},
			},
			validate: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.CreateTable.ElementProperties.Size == nil {
					t.Error("expected Size to be set")
					return
				}
				expectedWidth := pointsToEMU(500)
				expectedHeight := pointsToEMU(100)
				if req.CreateTable.ElementProperties.Size.Width.Magnitude != expectedWidth {
					t.Errorf("expected width %f, got %f", expectedWidth, req.CreateTable.ElementProperties.Size.Width.Magnitude)
				}
				if req.CreateTable.ElementProperties.Size.Height.Magnitude != expectedHeight {
					t.Errorf("expected height %f, got %f", expectedHeight, req.CreateTable.ElementProperties.Size.Height.Magnitude)
				}
				if req.CreateTable.ElementProperties.Size.Width.Unit != "EMU" {
					t.Errorf("expected Width Unit EMU, got %s", req.CreateTable.ElementProperties.Size.Width.Unit)
				}
				if req.CreateTable.ElementProperties.Size.Height.Unit != "EMU" {
					t.Errorf("expected Height Unit EMU, got %s", req.CreateTable.ElementProperties.Size.Height.Unit)
				}
			},
		},
		{
			name:     "table with position and size",
			objectID: "table-full",
			slideID:  "slide-4",
			input: CreateTableInput{
				Rows:     4,
				Columns:  6,
				Position: &PositionInput{X: 50, Y: 75},
				Size:     &SizeInput{Width: 600, Height: 300},
			},
			validate: func(t *testing.T, requests []*slides.Request) {
				req := requests[0]
				if req.CreateTable.ElementProperties.Transform == nil {
					t.Error("expected Transform to be set")
				}
				if req.CreateTable.ElementProperties.Size == nil {
					t.Error("expected Size to be set")
				}
				if req.CreateTable.Rows != 4 {
					t.Errorf("expected 4 rows, got %d", req.CreateTable.Rows)
				}
				if req.CreateTable.Columns != 6 {
					t.Errorf("expected 6 columns, got %d", req.CreateTable.Columns)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildCreateTableRequests(tt.objectID, tt.slideID, tt.input)
			tt.validate(t, requests)
		})
	}
}
