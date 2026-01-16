package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for create_table tool.
var (
	ErrCreateTableFailed = errors.New("failed to create table")
	ErrInvalidRowCount   = errors.New("rows must be at least 1")
	ErrInvalidColCount   = errors.New("columns must be at least 1")
)

// CreateTableInput represents the input for the create_table tool.
type CreateTableInput struct {
	PresentationID string         `json:"presentation_id"`
	SlideIndex     int            `json:"slide_index,omitempty"` // 1-based index
	SlideID        string         `json:"slide_id,omitempty"`    // Alternative to slide_index
	Rows           int            `json:"rows"`                  // Number of rows (min 1)
	Columns        int            `json:"columns"`               // Number of columns (min 1)
	Position       *PositionInput `json:"position,omitempty"`    // Position in points
	Size           *SizeInput     `json:"size,omitempty"`        // Size in points
}

// CreateTableOutput represents the output of the create_table tool.
type CreateTableOutput struct {
	ObjectID string `json:"object_id"`
	Rows     int    `json:"rows"`
	Columns  int    `json:"columns"`
}

// tableTimeNowFunc allows overriding the time function for tests.
var tableTimeNowFunc = time.Now

// generateTableObjectID generates a unique object ID for a new table element.
func generateTableObjectID() string {
	return fmt.Sprintf("table_%d", tableTimeNowFunc().UnixNano())
}

// CreateTable creates a new table on a slide.
func (t *Tools) CreateTable(ctx context.Context, tokenSource oauth2.TokenSource, input CreateTableInput) (*CreateTableOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, ErrInvalidSlideReference
	}

	if input.Rows < 1 {
		return nil, ErrInvalidRowCount
	}

	if input.Columns < 1 {
		return nil, ErrInvalidColCount
	}

	// Validate size if provided
	if input.Size != nil && (input.Size.Width <= 0 || input.Size.Height <= 0) {
		return nil, ErrInvalidSize
	}

	// Default position to (0, 0) if not provided
	if input.Position == nil {
		input.Position = &PositionInput{X: 0, Y: 0}
	}

	t.config.Logger.Info("creating table on slide",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.Int("rows", input.Rows),
		slog.Int("columns", input.Columns),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find the target slide
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

	// Find the target slide
	slideID, _, err := findSlide(presentation, input.SlideIndex, input.SlideID)
	if err != nil {
		return nil, err
	}

	// Generate a unique object ID for the table
	objectID := generateTableObjectID()

	// Build the requests for creating the table
	requests := buildCreateTableRequests(objectID, slideID, input)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrCreateTableFailed, err)
	}

	output := &CreateTableOutput{
		ObjectID: objectID,
		Rows:     input.Rows,
		Columns:  input.Columns,
	}

	t.config.Logger.Info("table created successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", output.ObjectID),
		slog.Int("rows", output.Rows),
		slog.Int("columns", output.Columns),
	)

	return output, nil
}

// buildCreateTableRequests creates the batch update requests to create a table.
func buildCreateTableRequests(objectID, slideID string, input CreateTableInput) []*slides.Request {
	requests := []*slides.Request{}

	// Create the table request
	createTableRequest := &slides.Request{
		CreateTable: &slides.CreateTableRequest{
			ObjectId: objectID,
			Rows:     int64(input.Rows),
			Columns:  int64(input.Columns),
			ElementProperties: &slides.PageElementProperties{
				PageObjectId: slideID,
			},
		},
	}

	// Add position transform if specified
	if input.Position != nil {
		createTableRequest.CreateTable.ElementProperties.Transform = &slides.AffineTransform{
			ScaleX:     1,
			ScaleY:     1,
			TranslateX: pointsToEMU(input.Position.X),
			TranslateY: pointsToEMU(input.Position.Y),
			Unit:       "EMU",
		}
	}

	// Add size if specified
	if input.Size != nil {
		createTableRequest.CreateTable.ElementProperties.Size = &slides.Size{
			Width: &slides.Dimension{
				Magnitude: pointsToEMU(input.Size.Width),
				Unit:      "EMU",
			},
			Height: &slides.Dimension{
				Magnitude: pointsToEMU(input.Size.Height),
				Unit:      "EMU",
			},
		}
	}

	requests = append(requests, createTableRequest)

	return requests
}
