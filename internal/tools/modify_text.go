package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for modify_text tool.
var (
	ErrModifyTextFailed  = errors.New("failed to modify text")
	ErrInvalidAction     = errors.New("invalid action")
	ErrInvalidObjectID   = errors.New("invalid object_id")
	ErrTextRequired      = errors.New("text is required for this action")
	ErrInvalidTextRange  = errors.New("invalid text range")
	ErrNotTextObject     = errors.New("object does not contain editable text")
)

// ModifyTextInput represents the input for the modify_text tool.
type ModifyTextInput struct {
	PresentationID string `json:"presentation_id"`
	ObjectID       string `json:"object_id"`
	Action         string `json:"action"` // "replace" | "append" | "prepend" | "delete"
	Text           string `json:"text,omitempty"`
	StartIndex     *int   `json:"start_index,omitempty"` // Optional, for partial replacement
	EndIndex       *int   `json:"end_index,omitempty"`   // Optional, for partial replacement
}

// ModifyTextOutput represents the output of the modify_text tool.
type ModifyTextOutput struct {
	ObjectID    string `json:"object_id"`
	UpdatedText string `json:"updated_text"`
	Action      string `json:"action"`
}

// ModifyText modifies text content in an existing shape.
func (t *Tools) ModifyText(ctx context.Context, tokenSource oauth2.TokenSource, input ModifyTextInput) (*ModifyTextOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required", ErrInvalidObjectID)
	}

	// Validate action
	validActions := map[string]bool{
		"replace": true,
		"append":  true,
		"prepend": true,
		"delete":  true,
	}
	if !validActions[input.Action] {
		return nil, fmt.Errorf("%w: action must be 'replace', 'append', 'prepend', or 'delete'", ErrInvalidAction)
	}

	// Text is required for replace, append, prepend (but not for delete)
	if input.Action != "delete" && input.Text == "" {
		return nil, fmt.Errorf("%w: text is required for '%s' action", ErrTextRequired, input.Action)
	}

	// Validate indices if provided
	if input.StartIndex != nil && *input.StartIndex < 0 {
		return nil, fmt.Errorf("%w: start_index cannot be negative", ErrInvalidTextRange)
	}
	if input.EndIndex != nil && *input.EndIndex < 0 {
		return nil, fmt.Errorf("%w: end_index cannot be negative", ErrInvalidTextRange)
	}
	if input.StartIndex != nil && input.EndIndex != nil && *input.StartIndex > *input.EndIndex {
		return nil, fmt.Errorf("%w: start_index cannot be greater than end_index", ErrInvalidTextRange)
	}

	t.config.Logger.Info("modifying text",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", input.Action),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to find the object and its current text
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

	// Find the target element
	var targetElement *slides.PageElement
	for _, slide := range presentation.Slides {
		element := findElementByID(slide.PageElements, input.ObjectID)
		if element != nil {
			targetElement = element
			break
		}
	}

	if targetElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Get current text content
	currentText := ""
	if targetElement.Shape != nil && targetElement.Shape.Text != nil {
		currentText = extractTextFromTextContent(targetElement.Shape.Text)
	} else if targetElement.Table != nil {
		// Tables have text but not as a single editable field
		return nil, fmt.Errorf("%w: tables must be modified cell by cell", ErrNotTextObject)
	} else {
		return nil, fmt.Errorf("%w: object '%s' does not support text modification", ErrNotTextObject, input.ObjectID)
	}

	// Build requests based on action
	requests, expectedText := buildModifyTextRequests(input, currentText)

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrModifyTextFailed, err)
	}

	output := &ModifyTextOutput{
		ObjectID:    input.ObjectID,
		UpdatedText: expectedText,
		Action:      input.Action,
	}

	t.config.Logger.Info("text modified successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("object_id", input.ObjectID),
		slog.String("action", input.Action),
	)

	return output, nil
}

// buildModifyTextRequests creates the batch update requests for text modification.
func buildModifyTextRequests(input ModifyTextInput, currentText string) ([]*slides.Request, string) {
	var requests []*slides.Request
	var expectedText string

	switch input.Action {
	case "replace":
		if input.StartIndex != nil && input.EndIndex != nil {
			// Partial replacement
			startIdx := *input.StartIndex
			endIdx := *input.EndIndex

			// Clamp indices to current text length
			textLen := len(currentText)
			if startIdx > textLen {
				startIdx = textLen
			}
			if endIdx > textLen {
				endIdx = textLen
			}

			// Delete the range first
			if endIdx > startIdx {
				startIdx64 := int64(startIdx)
				endIdx64 := int64(endIdx)
				requests = append(requests, &slides.Request{
					DeleteText: &slides.DeleteTextRequest{
						ObjectId: input.ObjectID,
						TextRange: &slides.Range{
							Type:       "FIXED_RANGE",
							StartIndex: &startIdx64,
							EndIndex:   &endIdx64,
						},
					},
				})
			}

			// Insert new text at start position
			requests = append(requests, &slides.Request{
				InsertText: &slides.InsertTextRequest{
					ObjectId:       input.ObjectID,
					InsertionIndex: int64(startIdx),
					Text:           input.Text,
				},
			})

			// Calculate expected text
			expectedText = currentText[:startIdx] + input.Text + currentText[endIdx:]
		} else {
			// Full replacement - delete all text first, then insert new text
			if len(currentText) > 0 {
				requests = append(requests, &slides.Request{
					DeleteText: &slides.DeleteTextRequest{
						ObjectId: input.ObjectID,
						TextRange: &slides.Range{
							Type: "ALL",
						},
					},
				})
			}

			requests = append(requests, &slides.Request{
				InsertText: &slides.InsertTextRequest{
					ObjectId:       input.ObjectID,
					InsertionIndex: 0,
					Text:           input.Text,
				},
			})

			expectedText = input.Text
		}

	case "append":
		// Insert text at the end
		// Note: Google Slides adds a trailing newline character automatically
		// We insert at position len(currentText) which handles this
		insertionIdx := len(currentText)
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       input.ObjectID,
				InsertionIndex: int64(insertionIdx),
				Text:           input.Text,
			},
		})

		expectedText = currentText + input.Text

	case "prepend":
		// Insert text at the beginning
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       input.ObjectID,
				InsertionIndex: 0,
				Text:           input.Text,
			},
		})

		expectedText = input.Text + currentText

	case "delete":
		// Delete all text
		if len(currentText) > 0 {
			requests = append(requests, &slides.Request{
				DeleteText: &slides.DeleteTextRequest{
					ObjectId: input.ObjectID,
					TextRange: &slides.Range{
						Type: "ALL",
					},
				},
			})
		}

		expectedText = ""
	}

	return requests, expectedText
}
