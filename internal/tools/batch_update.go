package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for batch_update tool.
var (
	ErrBatchUpdateFailed   = errors.New("batch update failed")
	ErrInvalidOnError      = errors.New("invalid on_error value")
	ErrNoOperations        = errors.New("no operations provided")
	ErrInvalidOperation    = errors.New("invalid operation")
	ErrUnsupportedToolName = errors.New("unsupported tool name")
)

// OnErrorMode defines the behavior when an error occurs during batch processing.
type OnErrorMode string

const (
	OnErrorStop     OnErrorMode = "stop"
	OnErrorContinue OnErrorMode = "continue"
	OnErrorRollback OnErrorMode = "rollback"
)

// BatchOperation represents a single operation in a batch.
type BatchOperation struct {
	ToolName   string          `json:"tool_name"`
	Parameters json.RawMessage `json:"parameters"`
}

// BatchUpdateInput represents the input for the batch_update tool.
type BatchUpdateInput struct {
	PresentationID string           `json:"presentation_id"`
	Operations     []BatchOperation `json:"operations"`
	OnError        OnErrorMode      `json:"on_error,omitempty"` // Default: "stop"
}

// OperationResult represents the result of a single operation.
type OperationResult struct {
	Index     int             `json:"index"`
	ToolName  string          `json:"tool_name"`
	Success   bool            `json:"success"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	ErrorCode string          `json:"error_code,omitempty"`
}

// BatchUpdateOutput represents the output of the batch_update tool.
type BatchUpdateOutput struct {
	PresentationID   string            `json:"presentation_id"`
	TotalOperations  int               `json:"total_operations"`
	SuccessCount     int               `json:"success_count"`
	FailureCount     int               `json:"failure_count"`
	Results          []OperationResult `json:"results"`
	RolledBack       bool              `json:"rolled_back,omitempty"`
	RollbackError    string            `json:"rollback_error,omitempty"`
	StoppedAtIndex   *int              `json:"stopped_at_index,omitempty"`
	BatchOptimized   bool              `json:"batch_optimized"`
	APICallCount     int               `json:"api_call_count"`
}

// BatchableOperation contains info about whether an operation can be batched.
type batchableOperation struct {
	index     int
	toolName  string
	requests  []*slides.Request
	postFunc  func(response *slides.BatchUpdatePresentationResponse, startReplyIdx int) (json.RawMessage, error)
}

// BatchUpdate executes multiple operations in a batch for efficiency.
func (t *Tools) BatchUpdate(ctx context.Context, tokenSource oauth2.TokenSource, input BatchUpdateInput) (*BatchUpdateOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	if len(input.Operations) == 0 {
		return nil, ErrNoOperations
	}

	// Default on_error mode
	if input.OnError == "" {
		input.OnError = OnErrorStop
	}

	// Validate on_error mode
	if !isValidOnErrorMode(input.OnError) {
		return nil, fmt.Errorf("%w: must be 'stop', 'continue', or 'rollback'", ErrInvalidOnError)
	}

	t.config.Logger.Info("executing batch update",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("operation_count", len(input.Operations)),
		slog.String("on_error", string(input.OnError)),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Verify presentation exists
	_, err = slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrSlidesAPIError, err)
	}

	output := &BatchUpdateOutput{
		PresentationID:  input.PresentationID,
		TotalOperations: len(input.Operations),
		Results:         make([]OperationResult, len(input.Operations)),
		BatchOptimized:  true,
	}

	// Try to batch all operations that support Slides API batch requests
	batchableOps, nonBatchableIndices, parseErrors := t.classifyOperations(input.Operations, input.PresentationID)

	// Handle parse errors based on on_error mode
	for idx, parseErr := range parseErrors {
		if parseErr != nil {
			output.Results[idx] = OperationResult{
				Index:     idx,
				ToolName:  input.Operations[idx].ToolName,
				Success:   false,
				Error:     parseErr.Error(),
				ErrorCode: "PARSE_ERROR",
			}
			output.FailureCount++

			if input.OnError == OnErrorStop {
				stoppedAt := idx
				output.StoppedAtIndex = &stoppedAt
				// Mark remaining operations as skipped
				for j := idx + 1; j < len(input.Operations); j++ {
					output.Results[j] = OperationResult{
						Index:     j,
						ToolName:  input.Operations[j].ToolName,
						Success:   false,
						Error:     "skipped due to previous error",
						ErrorCode: "SKIPPED",
					}
				}
				return output, nil
			}
		}
	}

	// If there are batchable operations, execute them in one API call
	if len(batchableOps) > 0 {
		err = t.executeBatchableOperations(ctx, slidesService, input.PresentationID, batchableOps, output)
		if err != nil {
			if input.OnError == OnErrorRollback {
				output.RolledBack = true
				// Note: True rollback isn't possible with Slides API, but we can mark it as failed
				output.RollbackError = "atomic batch failed, no changes were applied"
			}
			// All batchable operations failed
			for _, op := range batchableOps {
				if output.Results[op.index].Error == "" {
					output.Results[op.index] = OperationResult{
						Index:     op.index,
						ToolName:  op.toolName,
						Success:   false,
						Error:     err.Error(),
						ErrorCode: "BATCH_ERROR",
					}
					output.FailureCount++
				}
			}
			if input.OnError == OnErrorStop || input.OnError == OnErrorRollback {
				// Mark non-batchable operations as skipped
				for _, idx := range nonBatchableIndices {
					if output.Results[idx].Error == "" {
						output.Results[idx] = OperationResult{
							Index:     idx,
							ToolName:  input.Operations[idx].ToolName,
							Success:   false,
							Error:     "skipped due to batch error",
							ErrorCode: "SKIPPED",
						}
					}
				}
				return output, nil
			}
		}
		output.APICallCount++
	}

	// Execute non-batchable operations individually
	for _, idx := range nonBatchableIndices {
		if output.Results[idx].Error != "" {
			continue // Already has an error
		}

		result, err := t.executeNonBatchableOperation(ctx, tokenSource, input.PresentationID, input.Operations[idx])
		output.APICallCount++

		if err != nil {
			output.Results[idx] = OperationResult{
				Index:     idx,
				ToolName:  input.Operations[idx].ToolName,
				Success:   false,
				Error:     err.Error(),
				ErrorCode: getErrorCode(err),
			}
			output.FailureCount++

			if input.OnError == OnErrorStop {
				stoppedAt := idx
				output.StoppedAtIndex = &stoppedAt
				// Mark remaining non-batchable operations as skipped
				for j := idx + 1; j < len(nonBatchableIndices); j++ {
					nextIdx := nonBatchableIndices[j]
					if output.Results[nextIdx].Error == "" {
						output.Results[nextIdx] = OperationResult{
							Index:     nextIdx,
							ToolName:  input.Operations[nextIdx].ToolName,
							Success:   false,
							Error:     "skipped due to previous error",
							ErrorCode: "SKIPPED",
						}
					}
				}
				break
			}
		} else {
			output.Results[idx] = OperationResult{
				Index:    idx,
				ToolName: input.Operations[idx].ToolName,
				Success:  true,
				Result:   result,
			}
			output.SuccessCount++
		}
	}

	// Calculate if batch optimization was used
	output.BatchOptimized = len(batchableOps) > 1 && output.APICallCount < len(input.Operations)

	t.config.Logger.Info("batch update completed",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("total_operations", output.TotalOperations),
		slog.Int("success_count", output.SuccessCount),
		slog.Int("failure_count", output.FailureCount),
		slog.Int("api_call_count", output.APICallCount),
		slog.Bool("batch_optimized", output.BatchOptimized),
	)

	return output, nil
}

// classifyOperations separates batchable from non-batchable operations.
func (t *Tools) classifyOperations(operations []BatchOperation, presentationID string) ([]batchableOperation, []int, map[int]error) {
	var batchable []batchableOperation
	var nonBatchable []int
	parseErrors := make(map[int]error)

	for i, op := range operations {
		requests, postFunc, err := t.operationToRequests(op, presentationID)
		if err != nil {
			if errors.Is(err, ErrUnsupportedToolName) {
				// This operation needs to run individually
				nonBatchable = append(nonBatchable, i)
			} else {
				// Parse error
				parseErrors[i] = err
			}
			continue
		}

		if len(requests) > 0 {
			batchable = append(batchable, batchableOperation{
				index:    i,
				toolName: op.ToolName,
				requests: requests,
				postFunc: postFunc,
			})
		} else {
			// No requests generated but no error - treat as non-batchable
			nonBatchable = append(nonBatchable, i)
		}
	}

	return batchable, nonBatchable, parseErrors
}

// operationToRequests converts an operation to Slides API requests.
// Returns ErrUnsupportedToolName if the operation doesn't support batching.
func (t *Tools) operationToRequests(op BatchOperation, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	switch strings.ToLower(op.ToolName) {
	case "add_slide":
		return t.addSlideToRequests(op.Parameters, presentationID)
	case "delete_slide":
		return t.deleteSlideToRequests(op.Parameters, presentationID)
	case "add_text_box":
		return t.addTextBoxToRequests(op.Parameters, presentationID)
	case "modify_text":
		return t.modifyTextToRequests(op.Parameters, presentationID)
	case "delete_object":
		return t.deleteObjectToRequests(op.Parameters, presentationID)
	case "create_shape":
		return t.createShapeToRequests(op.Parameters, presentationID)
	case "transform_object":
		return t.transformObjectToRequests(op.Parameters, presentationID)
	case "style_text":
		return t.styleTextToRequests(op.Parameters, presentationID)
	case "create_bullet_list":
		return t.createBulletListToRequests(op.Parameters, presentationID)
	case "create_numbered_list":
		return t.createNumberedListToRequests(op.Parameters, presentationID)
	default:
		// Not all tools support batching
		return nil, nil, ErrUnsupportedToolName
	}
}

// executeBatchableOperations executes all batchable operations in a single API call.
func (t *Tools) executeBatchableOperations(ctx context.Context, slidesService SlidesService, presentationID string, ops []batchableOperation, output *BatchUpdateOutput) error {
	// Collect all requests
	var allRequests []*slides.Request
	for _, op := range ops {
		allRequests = append(allRequests, op.requests...)
	}

	if len(allRequests) == 0 {
		return nil
	}

	// Execute single batch update
	response, err := slidesService.BatchUpdate(ctx, presentationID, allRequests)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBatchUpdateFailed, err)
	}

	// Process responses for each operation
	replyIdx := 0
	for _, op := range ops {
		numRequests := len(op.requests)

		if op.postFunc != nil {
			result, err := op.postFunc(response, replyIdx)
			if err != nil {
				output.Results[op.index] = OperationResult{
					Index:     op.index,
					ToolName:  op.toolName,
					Success:   false,
					Error:     err.Error(),
					ErrorCode: "POST_PROCESS_ERROR",
				}
				output.FailureCount++
			} else {
				output.Results[op.index] = OperationResult{
					Index:    op.index,
					ToolName: op.toolName,
					Success:  true,
					Result:   result,
				}
				output.SuccessCount++
			}
		} else {
			// No post-processing needed
			output.Results[op.index] = OperationResult{
				Index:    op.index,
				ToolName: op.toolName,
				Success:  true,
			}
			output.SuccessCount++
		}

		replyIdx += numRequests
	}

	return nil
}

// executeNonBatchableOperation executes a single non-batchable operation.
func (t *Tools) executeNonBatchableOperation(ctx context.Context, tokenSource oauth2.TokenSource, presentationID string, op BatchOperation) (json.RawMessage, error) {
	switch strings.ToLower(op.ToolName) {
	case "add_image":
		var input AddImageInput
		if err := json.Unmarshal(op.Parameters, &input); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
		}
		input.PresentationID = presentationID
		result, err := t.AddImage(ctx, tokenSource, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "add_video":
		var input AddVideoInput
		if err := json.Unmarshal(op.Parameters, &input); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
		}
		input.PresentationID = presentationID
		result, err := t.AddVideo(ctx, tokenSource, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "replace_image":
		var input ReplaceImageInput
		if err := json.Unmarshal(op.Parameters, &input); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
		}
		input.PresentationID = presentationID
		result, err := t.ReplaceImage(ctx, tokenSource, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "set_background":
		var input SetBackgroundInput
		if err := json.Unmarshal(op.Parameters, &input); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
		}
		input.PresentationID = presentationID
		result, err := t.SetBackground(ctx, tokenSource, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "translate_presentation":
		var input TranslatePresentationInput
		if err := json.Unmarshal(op.Parameters, &input); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
		}
		input.PresentationID = presentationID
		result, err := t.TranslatePresentation(ctx, tokenSource, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	default:
		return nil, fmt.Errorf("%w: '%s' is not supported in batch operations", ErrUnsupportedToolName, op.ToolName)
	}
}

// Helper functions for converting operations to requests

func (t *Tools) addSlideToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input AddSlideInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.Layout == "" {
		return nil, nil, fmt.Errorf("%w: layout is required", ErrInvalidLayout)
	}

	if !validLayoutTypes[input.Layout] {
		return nil, nil, fmt.Errorf("%w: unsupported layout '%s'", ErrInvalidLayout, input.Layout)
	}

	createSlideRequest := &slides.CreateSlideRequest{}

	// Use predefined layout type
	createSlideRequest.SlideLayoutReference = &slides.LayoutReference{
		PredefinedLayout: input.Layout,
	}

	if input.Position > 0 {
		createSlideRequest.InsertionIndex = int64(input.Position - 1)
	}

	requests := []*slides.Request{
		{CreateSlide: createSlideRequest},
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		var slideID string
		if startIdx < len(response.Replies) && response.Replies[startIdx].CreateSlide != nil {
			slideID = response.Replies[startIdx].CreateSlide.ObjectId
		}
		result := AddSlideOutput{
			SlideIndex: input.Position,
			SlideID:    slideID,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) deleteSlideToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input DeleteSlideInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.SlideID == "" && input.SlideIndex <= 0 {
		return nil, nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	// For batch operations with slide_index, we need to know the slide ID
	// This requires a pre-fetch which defeats batching, so we only support slide_id
	if input.SlideID == "" {
		return nil, nil, ErrUnsupportedToolName
	}

	requests := []*slides.Request{
		{DeleteObject: &slides.DeleteObjectRequest{ObjectId: input.SlideID}},
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := DeleteSlideOutput{
			DeletedSlideID: input.SlideID,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) addTextBoxToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input AddTextBoxInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.Text == "" {
		return nil, nil, fmt.Errorf("%w: text is required", ErrInvalidText)
	}

	if input.Size == nil || input.Size.Width <= 0 || input.Size.Height <= 0 {
		return nil, nil, fmt.Errorf("%w: size with positive width and height is required", ErrInvalidSize)
	}

	if input.SlideID == "" && input.SlideIndex <= 0 {
		return nil, nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	// For batch, we need slide_id
	if input.SlideID == "" {
		return nil, nil, ErrUnsupportedToolName
	}

	// Generate object ID
	objectID := batchGenerateObjectID("textbox")

	// Create shape request
	var x, y float64
	if input.Position != nil {
		x = input.Position.X
		y = input.Position.Y
	}

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  objectID,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: input.SlideID,
					Transform: &slides.AffineTransform{
						ScaleX:     1,
						ScaleY:     1,
						TranslateX: x * pointsPerEMU,
						TranslateY: y * pointsPerEMU,
						Unit:       "EMU",
					},
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: input.Size.Width * pointsPerEMU, Unit: "EMU"},
						Height: &slides.Dimension{Magnitude: input.Size.Height * pointsPerEMU, Unit: "EMU"},
					},
				},
			},
		},
		{
			InsertText: &slides.InsertTextRequest{
				ObjectId: objectID,
				Text:     input.Text,
			},
		},
	}

	// Add styling if provided
	if input.Style != nil {
		styleRequest := batchBuildTextStyleRequest(objectID, input.Style, nil, nil)
		if styleRequest != nil {
			requests = append(requests, styleRequest)
		}
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := AddTextBoxOutput{ObjectID: objectID}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) modifyTextToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input ModifyTextInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.ObjectID == "" {
		return nil, nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	action := strings.ToLower(input.Action)
	if action != "replace" && action != "append" && action != "prepend" && action != "delete" {
		return nil, nil, fmt.Errorf("%w: action must be 'replace', 'append', 'prepend', or 'delete'", ErrInvalidAction)
	}

	if action != "delete" && input.Text == "" {
		return nil, nil, fmt.Errorf("%w: text is required for %s action", ErrTextRequired, action)
	}

	var requests []*slides.Request

	switch action {
	case "replace":
		// Delete all text first
		requests = append(requests, &slides.Request{
			DeleteText: &slides.DeleteTextRequest{
				ObjectId: input.ObjectID,
				TextRange: &slides.Range{
					Type: "ALL",
				},
			},
		})
		// Insert new text
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       input.ObjectID,
				Text:           input.Text,
				InsertionIndex: 0,
			},
		})
	case "append":
		// Insert at end
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId: input.ObjectID,
				Text:     input.Text,
			},
		})
	case "prepend":
		// Insert at beginning
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       input.ObjectID,
				Text:           input.Text,
				InsertionIndex: 0,
			},
		})
	case "delete":
		// Delete all text
		requests = append(requests, &slides.Request{
			DeleteText: &slides.DeleteTextRequest{
				ObjectId: input.ObjectID,
				TextRange: &slides.Range{
					Type: "ALL",
				},
			},
		})
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := ModifyTextOutput{
			ObjectID:    input.ObjectID,
			UpdatedText: input.Text,
			Action:      action,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) deleteObjectToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input DeleteObjectInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	// Collect all object IDs
	var objectIDs []string
	if input.ObjectID != "" {
		objectIDs = append(objectIDs, input.ObjectID)
	}
	objectIDs = append(objectIDs, input.Multiple...)

	// Deduplicate
	seen := make(map[string]bool)
	var uniqueIDs []string
	for _, id := range objectIDs {
		if !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	if len(uniqueIDs) == 0 {
		return nil, nil, ErrNoObjectsToDelete
	}

	var requests []*slides.Request
	for _, id := range uniqueIDs {
		requests = append(requests, &slides.Request{
			DeleteObject: &slides.DeleteObjectRequest{ObjectId: id},
		})
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := DeleteObjectOutput{
			DeletedCount: len(uniqueIDs),
			DeletedIDs:   uniqueIDs,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) createShapeToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input CreateShapeInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.ShapeType == "" {
		return nil, nil, fmt.Errorf("%w: shape_type is required", ErrInvalidShapeType)
	}

	if input.Size == nil || input.Size.Width <= 0 || input.Size.Height <= 0 {
		return nil, nil, fmt.Errorf("%w: size with positive width and height is required", ErrInvalidSize)
	}

	if input.SlideID == "" && input.SlideIndex <= 0 {
		return nil, nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	// For batch, we need slide_id
	if input.SlideID == "" {
		return nil, nil, ErrUnsupportedToolName
	}

	objectID := batchGenerateObjectID("shape")

	var x, y float64
	if input.Position != nil {
		x = input.Position.X
		y = input.Position.Y
	}

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  objectID,
				ShapeType: strings.ToUpper(input.ShapeType),
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: input.SlideID,
					Transform: &slides.AffineTransform{
						ScaleX:     1,
						ScaleY:     1,
						TranslateX: x * pointsPerEMU,
						TranslateY: y * pointsPerEMU,
						Unit:       "EMU",
					},
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: input.Size.Width * pointsPerEMU, Unit: "EMU"},
						Height: &slides.Dimension{Magnitude: input.Size.Height * pointsPerEMU, Unit: "EMU"},
					},
				},
			},
		},
	}

	// Add fill and outline styling if provided
	if input.FillColor != "" || input.OutlineColor != "" || input.OutlineWeight != nil {
		styleReq := batchBuildShapeStyleRequest(objectID, input.FillColor, input.OutlineColor, input.OutlineWeight)
		if styleReq != nil {
			requests = append(requests, styleReq)
		}
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := CreateShapeOutput{ObjectID: objectID}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) transformObjectToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input TransformObjectInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.ObjectID == "" {
		return nil, nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Transform requires getting current transform state, so we can't batch it
	return nil, nil, ErrUnsupportedToolName
}

func (t *Tools) styleTextToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input StyleTextInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.ObjectID == "" {
		return nil, nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	if input.Style == nil {
		return nil, nil, fmt.Errorf("%w: style is required", ErrNoStyleProvided)
	}

	// Build the request
	textStyle := &slides.TextStyle{}
	var fields []string

	if input.Style.FontFamily != "" {
		textStyle.FontFamily = input.Style.FontFamily
		fields = append(fields, "fontFamily")
	}
	if input.Style.FontSize > 0 {
		textStyle.FontSize = &slides.Dimension{Magnitude: float64(input.Style.FontSize), Unit: "PT"}
		fields = append(fields, "fontSize")
	}
	if input.Style.Bold != nil {
		textStyle.Bold = *input.Style.Bold
		fields = append(fields, "bold")
	}
	if input.Style.Italic != nil {
		textStyle.Italic = *input.Style.Italic
		fields = append(fields, "italic")
	}
	if input.Style.Underline != nil {
		textStyle.Underline = *input.Style.Underline
		fields = append(fields, "underline")
	}
	if input.Style.Strikethrough != nil {
		textStyle.Strikethrough = *input.Style.Strikethrough
		fields = append(fields, "strikethrough")
	}
	if input.Style.ForegroundColor != "" {
		color := parseHexColor(input.Style.ForegroundColor)
		if color != nil {
			textStyle.ForegroundColor = &slides.OptionalColor{OpaqueColor: &slides.OpaqueColor{RgbColor: color}}
			fields = append(fields, "foregroundColor")
		}
	}
	if input.Style.BackgroundColor != "" {
		color := parseHexColor(input.Style.BackgroundColor)
		if color != nil {
			textStyle.BackgroundColor = &slides.OptionalColor{OpaqueColor: &slides.OpaqueColor{RgbColor: color}}
			fields = append(fields, "backgroundColor")
		}
	}
	if input.Style.LinkURL != "" {
		textStyle.Link = &slides.Link{Url: input.Style.LinkURL}
		fields = append(fields, "link")
	}

	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("%w: no valid style properties provided", ErrNoStyleProvided)
	}

	textRange := &slides.Range{Type: "ALL"}
	if input.StartIndex != nil && input.EndIndex != nil {
		startIdx := int64(*input.StartIndex)
		endIdx := int64(*input.EndIndex)
		textRange = &slides.Range{
			Type:       "FIXED_RANGE",
			StartIndex: &startIdx,
			EndIndex:   &endIdx,
		}
	}

	requests := []*slides.Request{
		{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId:  input.ObjectID,
				Style:     textStyle,
				Fields:    strings.Join(fields, ","),
				TextRange: textRange,
			},
		},
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := StyleTextOutput{
			ObjectID:      input.ObjectID,
			AppliedStyles: fields,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) createBulletListToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input CreateBulletListInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.ObjectID == "" {
		return nil, nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	bulletStyle := strings.ToUpper(input.BulletStyle)
	if bulletStyle == "" {
		return nil, nil, fmt.Errorf("%w: bullet_style is required", ErrInvalidBulletStyle)
	}

	preset := lookupBulletPreset(bulletStyle)

	requests := []*slides.Request{
		{
			CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
				ObjectId:     input.ObjectID,
				BulletPreset: preset,
				TextRange:    &slides.Range{Type: "ALL"},
			},
		},
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := CreateBulletListOutput{
			ObjectID:     input.ObjectID,
			BulletPreset: preset,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

func (t *Tools) createNumberedListToRequests(params json.RawMessage, presentationID string) ([]*slides.Request, func(*slides.BatchUpdatePresentationResponse, int) (json.RawMessage, error), error) {
	var input CreateNumberedListInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}

	if input.ObjectID == "" {
		return nil, nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	numberStyle := strings.ToUpper(input.NumberStyle)
	if numberStyle == "" {
		return nil, nil, fmt.Errorf("%w: number_style is required", ErrInvalidNumberStyle)
	}

	preset := lookupNumberPreset(numberStyle)

	requests := []*slides.Request{
		{
			CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
				ObjectId:     input.ObjectID,
				BulletPreset: preset,
				TextRange:    &slides.Range{Type: "ALL"},
			},
		},
	}

	postFunc := func(response *slides.BatchUpdatePresentationResponse, startIdx int) (json.RawMessage, error) {
		result := CreateNumberedListOutput{
			ObjectID:     input.ObjectID,
			NumberPreset: preset,
		}
		return json.Marshal(result)
	}

	return requests, postFunc, nil
}

// Helper functions

func isValidOnErrorMode(mode OnErrorMode) bool {
	return mode == OnErrorStop || mode == OnErrorContinue || mode == OnErrorRollback
}

func getErrorCode(err error) string {
	if errors.Is(err, ErrPresentationNotFound) {
		return "PRESENTATION_NOT_FOUND"
	}
	if errors.Is(err, ErrAccessDenied) {
		return "ACCESS_DENIED"
	}
	if errors.Is(err, ErrSlidesAPIError) {
		return "SLIDES_API_ERROR"
	}
	if errors.Is(err, ErrInvalidOperation) {
		return "INVALID_OPERATION"
	}
	return "UNKNOWN_ERROR"
}

// batchGenerateObjectID generates a unique object ID for batch operations.
func batchGenerateObjectID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, timeNowFunc().UnixNano())
}

// batchBuildTextStyleRequest creates a request to update text style for batch operations.
func batchBuildTextStyleRequest(objectID string, style *TextStyleInput, startIndex, endIndex *int) *slides.Request {
	if style == nil {
		return nil
	}

	textStyle := &slides.TextStyle{}
	var fields []string

	if style.FontFamily != "" {
		textStyle.FontFamily = style.FontFamily
		fields = append(fields, "fontFamily")
	}
	if style.FontSize > 0 {
		textStyle.FontSize = &slides.Dimension{Magnitude: float64(style.FontSize), Unit: "PT"}
		fields = append(fields, "fontSize")
	}
	if style.Bold {
		textStyle.Bold = true
		fields = append(fields, "bold")
	}
	if style.Italic {
		textStyle.Italic = true
		fields = append(fields, "italic")
	}
	if style.Color != "" {
		color := parseHexColor(style.Color)
		if color != nil {
			textStyle.ForegroundColor = &slides.OptionalColor{OpaqueColor: &slides.OpaqueColor{RgbColor: color}}
			fields = append(fields, "foregroundColor")
		}
	}

	if len(fields) == 0 {
		return nil
	}

	textRange := &slides.Range{Type: "ALL"}
	if startIndex != nil && endIndex != nil {
		start := int64(*startIndex)
		end := int64(*endIndex)
		textRange = &slides.Range{
			Type:       "FIXED_RANGE",
			StartIndex: &start,
			EndIndex:   &end,
		}
	}

	return &slides.Request{
		UpdateTextStyle: &slides.UpdateTextStyleRequest{
			ObjectId:  objectID,
			Style:     textStyle,
			Fields:    strings.Join(fields, ","),
			TextRange: textRange,
		},
	}
}

// lookupBulletPreset converts a bullet style name to the API preset.
func lookupBulletPreset(style string) string {
	if preset, ok := validBulletStyles[style]; ok {
		return preset
	}
	// Return a default if not found
	return "BULLET_DISC_CIRCLE_SQUARE"
}

// lookupNumberPreset converts a number style name to the API preset.
func lookupNumberPreset(style string) string {
	if preset, ok := validNumberStyles[style]; ok {
		return preset
	}
	// Return a default if not found
	return "NUMBERED_DECIMAL_ALPHA_ROMAN"
}

// batchBuildShapeStyleRequest creates a request to update shape style for batch operations.
func batchBuildShapeStyleRequest(objectID, fillColor, outlineColor string, outlineWeight *float64) *slides.Request {
	shapeProps := &slides.ShapeProperties{}
	var fields []string

	if fillColor != "" {
		if strings.ToLower(fillColor) == "transparent" {
			shapeProps.ShapeBackgroundFill = &slides.ShapeBackgroundFill{
				PropertyState: "NOT_RENDERED",
			}
		} else {
			color := parseHexColor(fillColor)
			if color != nil {
				shapeProps.ShapeBackgroundFill = &slides.ShapeBackgroundFill{
					PropertyState: "RENDERED",
					SolidFill: &slides.SolidFill{
						Color: &slides.OpaqueColor{RgbColor: color},
					},
				}
			}
		}
		fields = append(fields, "shapeBackgroundFill")
	}

	if outlineColor != "" {
		if strings.ToLower(outlineColor) == "transparent" {
			shapeProps.Outline = &slides.Outline{
				PropertyState: "NOT_RENDERED",
			}
		} else {
			color := parseHexColor(outlineColor)
			if color != nil {
				shapeProps.Outline = &slides.Outline{
					PropertyState: "RENDERED",
					OutlineFill: &slides.OutlineFill{
						SolidFill: &slides.SolidFill{
							Color: &slides.OpaqueColor{RgbColor: color},
						},
					},
				}
				if outlineWeight != nil {
					shapeProps.Outline.Weight = &slides.Dimension{Magnitude: *outlineWeight, Unit: "PT"}
				}
			}
		}
		fields = append(fields, "outline")
	}

	if len(fields) == 0 {
		return nil
	}

	return &slides.Request{
		UpdateShapeProperties: &slides.UpdateShapePropertiesRequest{
			ObjectId:        objectID,
			ShapeProperties: shapeProps,
			Fields:          strings.Join(fields, ","),
		},
	}
}
