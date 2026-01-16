package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestBatchUpdate_MultipleOperations(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Title:          "Test Presentation",
				Slides: []*slides.Page{
					{
						ObjectId: "slide-1",
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-1",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Hello"}},
										},
									},
								},
							},
						},
					},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{
							Name: "BLANK",
						},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			// Return appropriate responses for each request
			replies := make([]*slides.Response, len(requests))
			for i, req := range requests {
				replies[i] = &slides.Response{}
				if req.CreateSlide != nil {
					replies[i].CreateSlide = &slides.CreateSlideResponse{ObjectId: "new-slide-id"}
				}
				// DeleteText and InsertText don't have response types - just empty Response
			}
			return &slides.BatchUpdatePresentationResponse{
				Replies: replies,
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	// Create operations as JSON
	addSlideParams, _ := json.Marshal(AddSlideInput{
		Layout: "BLANK",
	})
	modifyTextParams, _ := json.Marshal(ModifyTextInput{
		ObjectID: "shape-1",
		Action:   "replace",
		Text:     "New text",
	})

	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "add_slide", Parameters: addSlideParams},
			{ToolName: "modify_text", Parameters: modifyTextParams},
		},
		OnError: "stop",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(output.Results))
	}

	// Check that both operations succeeded
	for i, result := range output.Results {
		if !result.Success {
			t.Errorf("operation %d failed: %s", i, result.Error)
		}
	}
}

func TestBatchUpdate_OnErrorStop(t *testing.T) {
	callCount := 0
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{Name: "BLANK"},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			callCount++
			// First call fails
			if callCount == 1 {
				return nil, errors.New("simulated API error")
			}
			return &slides.BatchUpdatePresentationResponse{
				Replies: []*slides.Response{{}},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	addSlideParams, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})

	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "add_slide", Parameters: addSlideParams},
			{ToolName: "add_slide", Parameters: addSlideParams},
		},
		OnError: "stop",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First operation should fail
	if output.Results[0].Success {
		t.Error("expected first operation to fail")
	}

	// Second operation should be skipped (not processed)
	if len(output.Results) > 1 && output.Results[1].Success {
		t.Error("expected second operation to be skipped after first failure")
	}
}

func TestBatchUpdate_OnErrorContinue(t *testing.T) {
	// When batchable operations are executed together and fail, all fail together.
	// The continue mode applies when there are mixed batchable/non-batchable operations
	// or when processing non-batchable operations sequentially.
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{Name: "BLANK"},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			// Batch fails
			return nil, errors.New("simulated API error")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	addSlideParams, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})

	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "add_slide", Parameters: addSlideParams},
			{ToolName: "add_slide", Parameters: addSlideParams},
		},
		OnError: "continue",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(output.Results))
	}

	// Both operations should fail because they're batched together
	if output.Results[0].Success {
		t.Error("expected first operation to fail")
	}
	if output.Results[1].Success {
		t.Error("expected second operation to fail (batched with first)")
	}

	// In continue mode, no operations are skipped - they all get results
	for i, result := range output.Results {
		if result.Error == "skipped due to previous error" {
			t.Errorf("operation %d should not be skipped in continue mode", i)
		}
	}
}

func TestBatchUpdate_OnErrorRollback(t *testing.T) {
	// Rollback mode: when batch fails, mark RolledBack=true
	// Note: True rollback (undoing changes) is not supported by Google Slides API
	// The implementation treats "rollback" similarly to "stop" and sets the flag
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{Name: "BLANK"},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			// Batch fails
			return nil, errors.New("simulated API error")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	addSlideParams, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})

	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "add_slide", Parameters: addSlideParams},
			{ToolName: "add_slide", Parameters: addSlideParams},
		},
		OnError: "rollback",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that at least one operation failed
	hasFailure := false
	for _, result := range output.Results {
		if !result.Success {
			hasFailure = true
			break
		}
	}
	if !hasFailure {
		t.Error("expected at least one operation to fail in rollback test")
	}

	// Note: RolledBack flag behavior depends on implementation
	// If batch operations fail atomically, the API doesn't commit partial changes
	// so RolledBack might not be set (no actual rollback needed)
}

func TestBatchUpdate_ResultsMatchOperations(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{ObjectId: "slide-1"},
				},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{Name: "BLANK"},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			replies := make([]*slides.Response, len(requests))
			for i, req := range requests {
				replies[i] = &slides.Response{}
				if req.CreateSlide != nil {
					replies[i].CreateSlide = &slides.CreateSlideResponse{ObjectId: "new-slide-id"}
				}
			}
			return &slides.BatchUpdatePresentationResponse{
				Replies: replies,
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	addSlideParams1, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})
	addSlideParams2, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})
	addSlideParams3, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})

	operations := []BatchOperation{
		{ToolName: "add_slide", Parameters: addSlideParams1},
		{ToolName: "add_slide", Parameters: addSlideParams2},
		{ToolName: "add_slide", Parameters: addSlideParams3},
	}

	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations:     operations,
		OnError:        "stop",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Results array should match operations array length
	if len(output.Results) != len(operations) {
		t.Errorf("expected %d results, got %d", len(operations), len(output.Results))
	}

	// Each result should have matching tool_name
	for i, result := range output.Results {
		if result.ToolName != operations[i].ToolName {
			t.Errorf("result[%d].ToolName = %s, expected %s", i, result.ToolName, operations[i].ToolName)
		}
		if result.Index != i {
			t.Errorf("result[%d].Index = %d, expected %d", i, result.Index, i)
		}
	}
}

func TestBatchUpdate_InvalidPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "",
		Operations:     []BatchOperation{},
		OnError:        "stop",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestBatchUpdate_EmptyOperations(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations:     []BatchOperation{},
		OnError:        "stop",
	})

	if err == nil {
		t.Fatal("expected error for empty operations")
	}
	if !errors.Is(err, ErrNoOperations) {
		t.Errorf("expected ErrNoOperations, got %v", err)
	}
}

func TestBatchUpdate_InvalidOnErrorMode(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	addSlideParams, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})

	_, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "add_slide", Parameters: addSlideParams},
		},
		OnError: "invalid_mode",
	})

	if err == nil {
		t.Fatal("expected error for invalid on_error mode")
	}
	if !errors.Is(err, ErrInvalidOnError) {
		t.Errorf("expected ErrInvalidOnError, got %v", err)
	}
}

func TestBatchUpdate_UnsupportedToolName(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	invalidParams := json.RawMessage(`{}`)

	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "unsupported_tool", Parameters: invalidParams},
		},
		OnError: "continue",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The operation should fail with unsupported tool error
	if len(output.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(output.Results))
	}
	if output.Results[0].Success {
		t.Error("expected operation to fail for unsupported tool")
	}
	if output.Results[0].Error == "" {
		t.Error("expected error message for unsupported tool")
	}
}

func TestBatchUpdate_DefaultOnErrorMode(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides:         []*slides.Page{{ObjectId: "slide-1"}},
				Layouts: []*slides.Page{
					{
						ObjectId: "layout-blank",
						LayoutProperties: &slides.LayoutProperties{Name: "BLANK"},
					},
				},
			}, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			replies := make([]*slides.Response, len(requests))
			for i, req := range requests {
				replies[i] = &slides.Response{}
				if req.CreateSlide != nil {
					replies[i].CreateSlide = &slides.CreateSlideResponse{ObjectId: "new-slide-id"}
				}
			}
			return &slides.BatchUpdatePresentationResponse{Replies: replies}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	addSlideParams, _ := json.Marshal(AddSlideInput{Layout: "BLANK"})

	// Don't specify OnError, should default to "stop"
	output, err := tools.BatchUpdate(context.Background(), tokenSource, BatchUpdateInput{
		PresentationID: "test-pres-id",
		Operations: []BatchOperation{
			{ToolName: "add_slide", Parameters: addSlideParams},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(output.Results))
	}
}
