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

// Sentinel errors for manage_speaker_notes tool.
var (
	ErrManageSpeakerNotesFailed = errors.New("failed to manage speaker notes")
	ErrInvalidSpeakerNotesAction = errors.New("invalid speaker notes action")
	ErrNotesTextRequired         = errors.New("notes_text is required for this action")
	ErrNotesShapeNotFound        = errors.New("speaker notes shape not found")
)

// ManageSpeakerNotesInput represents the input for the manage_speaker_notes tool.
type ManageSpeakerNotesInput struct {
	PresentationID string `json:"presentation_id"`
	SlideIndex     int    `json:"slide_index,omitempty"` // 1-based index
	SlideID        string `json:"slide_id,omitempty"`
	Action         string `json:"action"`              // "get" | "set" | "append" | "clear"
	NotesText      string `json:"notes_text,omitempty"` // Required for "set" and "append"
}

// ManageSpeakerNotesOutput represents the output of the manage_speaker_notes tool.
type ManageSpeakerNotesOutput struct {
	SlideID      string `json:"slide_id"`
	SlideIndex   int    `json:"slide_index"`
	Action       string `json:"action"`
	NotesContent string `json:"notes_content"`
}

// ManageSpeakerNotes gets, sets, appends, or clears speaker notes on a slide.
func (t *Tools) ManageSpeakerNotes(ctx context.Context, tokenSource oauth2.TokenSource, input ManageSpeakerNotesInput) (*ManageSpeakerNotesOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.SlideIndex == 0 && input.SlideID == "" {
		return nil, fmt.Errorf("%w: either slide_index or slide_id is required", ErrInvalidSlideReference)
	}

	// Validate action
	action := strings.ToLower(input.Action)
	validActions := map[string]bool{
		"get":    true,
		"set":    true,
		"append": true,
		"clear":  true,
	}
	if !validActions[action] {
		return nil, fmt.Errorf("%w: action must be 'get', 'set', 'append', or 'clear'", ErrInvalidSpeakerNotesAction)
	}

	// Notes text is required for set and append
	if (action == "set" || action == "append") && input.NotesText == "" {
		return nil, fmt.Errorf("%w: notes_text is required for '%s' action", ErrNotesTextRequired, action)
	}

	t.config.Logger.Info("managing speaker notes",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_index", input.SlideIndex),
		slog.String("slide_id", input.SlideID),
		slog.String("action", action),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation
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
	var targetSlide *slides.Page
	var slideIndex int

	if input.SlideID != "" {
		// Find by slide ID
		for i, slide := range presentation.Slides {
			if slide.ObjectId == input.SlideID {
				targetSlide = slide
				slideIndex = i + 1
				break
			}
		}
		if targetSlide == nil {
			return nil, fmt.Errorf("%w: slide with ID '%s' not found", ErrSlideNotFound, input.SlideID)
		}
	} else {
		// Find by slide index (1-based)
		if input.SlideIndex < 1 || input.SlideIndex > len(presentation.Slides) {
			return nil, fmt.Errorf("%w: slide index %d out of range (1-%d)", ErrSlideNotFound, input.SlideIndex, len(presentation.Slides))
		}
		targetSlide = presentation.Slides[input.SlideIndex-1]
		slideIndex = input.SlideIndex
	}

	// Find the speaker notes shape
	notesShapeID, currentNotes := findSpeakerNotesShape(targetSlide)

	// For 'get' action, just return the current notes
	if action == "get" {
		return &ManageSpeakerNotesOutput{
			SlideID:      targetSlide.ObjectId,
			SlideIndex:   slideIndex,
			Action:       action,
			NotesContent: currentNotes,
		}, nil
	}

	// For modification actions, we need the notes shape ID
	if notesShapeID == "" {
		return nil, fmt.Errorf("%w: no speaker notes placeholder found on slide %d", ErrNotesShapeNotFound, slideIndex)
	}

	// Build requests based on action
	requests, expectedNotes := buildSpeakerNotesRequests(notesShapeID, action, input.NotesText, currentNotes)

	// Execute batch update if there are requests
	if len(requests) > 0 {
		_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
		if err != nil {
			if isNotFoundError(err) {
				return nil, ErrPresentationNotFound
			}
			if isForbiddenError(err) {
				return nil, ErrAccessDenied
			}
			return nil, fmt.Errorf("%w: %v", ErrManageSpeakerNotesFailed, err)
		}
	}

	output := &ManageSpeakerNotesOutput{
		SlideID:      targetSlide.ObjectId,
		SlideIndex:   slideIndex,
		Action:       action,
		NotesContent: expectedNotes,
	}

	t.config.Logger.Info("speaker notes managed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("slide_id", targetSlide.ObjectId),
		slog.Int("slide_index", slideIndex),
		slog.String("action", action),
	)

	return output, nil
}

// findSpeakerNotesShape finds the speaker notes shape and returns its ID and current text.
func findSpeakerNotesShape(slide *slides.Page) (shapeID string, currentText string) {
	if slide == nil || slide.SlideProperties == nil {
		return "", ""
	}

	notesPage := slide.SlideProperties.NotesPage
	if notesPage == nil || len(notesPage.PageElements) == 0 {
		return "", ""
	}

	// First, look for the BODY placeholder (standard speaker notes location)
	for _, element := range notesPage.PageElements {
		if element.Shape != nil && element.Shape.Placeholder != nil {
			if element.Shape.Placeholder.Type == "BODY" {
				text := ""
				if element.Shape.Text != nil {
					text = extractTextFromTextContent(element.Shape.Text)
				}
				return element.ObjectId, text
			}
		}
	}

	// Fallback: look for any shape with text in the notes page
	for _, element := range notesPage.PageElements {
		if element.Shape != nil {
			// Skip non-body placeholders (like the slide image placeholder)
			if element.Shape.Placeholder != nil && element.Shape.Placeholder.Type != "BODY" {
				continue
			}
			text := ""
			if element.Shape.Text != nil {
				text = extractTextFromTextContent(element.Shape.Text)
			}
			return element.ObjectId, text
		}
	}

	return "", ""
}

// buildSpeakerNotesRequests creates the batch update requests for speaker notes modification.
func buildSpeakerNotesRequests(shapeID, action, notesText, currentNotes string) ([]*slides.Request, string) {
	var requests []*slides.Request
	var expectedNotes string

	switch action {
	case "set":
		// Delete existing text first (if any), then insert new text
		if len(currentNotes) > 0 {
			requests = append(requests, &slides.Request{
				DeleteText: &slides.DeleteTextRequest{
					ObjectId: shapeID,
					TextRange: &slides.Range{
						Type: "ALL",
					},
				},
			})
		}
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       shapeID,
				InsertionIndex: 0,
				Text:           notesText,
			},
		})
		expectedNotes = notesText

	case "append":
		// Insert text at the end
		insertionIdx := len(currentNotes)
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       shapeID,
				InsertionIndex: int64(insertionIdx),
				Text:           notesText,
			},
		})
		expectedNotes = currentNotes + notesText

	case "clear":
		// Delete all text
		if len(currentNotes) > 0 {
			requests = append(requests, &slides.Request{
				DeleteText: &slides.DeleteTextRequest{
					ObjectId: shapeID,
					TextRange: &slides.Range{
						Type: "ALL",
					},
				},
			})
		}
		expectedNotes = ""
	}

	return requests, expectedNotes
}
