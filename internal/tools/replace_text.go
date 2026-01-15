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

// Sentinel errors for replace_text tool.
var (
	ErrReplaceTextFailed = errors.New("failed to replace text")
	ErrInvalidFind       = errors.New("find text is required")
	ErrInvalidScope      = errors.New("invalid scope")
)

// ReplaceTextInput represents the input for the replace_text tool.
type ReplaceTextInput struct {
	PresentationID string `json:"presentation_id"`
	Find           string `json:"find"`
	ReplaceWith    string `json:"replace_with"`
	CaseSensitive  bool   `json:"case_sensitive,omitempty"` // Default: false
	Scope          string `json:"scope,omitempty"`          // "all" | "slide" | "object" - Default: "all"
	SlideID        string `json:"slide_id,omitempty"`       // Required when scope is "slide"
	ObjectID       string `json:"object_id,omitempty"`      // Required when scope is "object"
}

// ReplaceTextOutput represents the output of the replace_text tool.
type ReplaceTextOutput struct {
	PresentationID     string           `json:"presentation_id"`
	Find               string           `json:"find"`
	ReplaceWith        string           `json:"replace_with"`
	CaseSensitive      bool             `json:"case_sensitive"`
	Scope              string           `json:"scope"`
	ReplacementCount   int              `json:"replacement_count"`
	AffectedObjects    []AffectedObject `json:"affected_objects,omitempty"`
}

// AffectedObject represents an object that was affected by the replacement.
type AffectedObject struct {
	SlideIndex int    `json:"slide_index"` // 1-based
	SlideID    string `json:"slide_id"`
	ObjectID   string `json:"object_id"`
	ObjectType string `json:"object_type"`
}

// ReplaceText finds and replaces text across a presentation.
func (t *Tools) ReplaceText(ctx context.Context, tokenSource oauth2.TokenSource, input ReplaceTextInput) (*ReplaceTextOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.Find == "" {
		return nil, fmt.Errorf("%w: find text cannot be empty", ErrInvalidFind)
	}

	// Set default scope
	if input.Scope == "" {
		input.Scope = "all"
	}

	// Validate scope
	validScopes := map[string]bool{
		"all":    true,
		"slide":  true,
		"object": true,
	}
	if !validScopes[input.Scope] {
		return nil, fmt.Errorf("%w: scope must be 'all', 'slide', or 'object'", ErrInvalidScope)
	}

	// Validate scope-specific parameters
	if input.Scope == "slide" && input.SlideID == "" {
		return nil, fmt.Errorf("%w: slide_id is required when scope is 'slide'", ErrInvalidScope)
	}
	if input.Scope == "object" && input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required when scope is 'object'", ErrInvalidScope)
	}

	t.config.Logger.Info("replacing text in presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.String("find", input.Find),
		slog.String("replace_with", input.ReplaceWith),
		slog.Bool("case_sensitive", input.CaseSensitive),
		slog.String("scope", input.Scope),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation first to identify affected objects and validate scope
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

	// Build page object IDs based on scope
	var pageObjectIds []string
	switch input.Scope {
	case "slide":
		// Validate slide exists
		slideFound := false
		for _, slide := range presentation.Slides {
			if slide.ObjectId == input.SlideID {
				slideFound = true
				pageObjectIds = []string{input.SlideID}
				break
			}
		}
		if !slideFound {
			return nil, fmt.Errorf("%w: slide '%s' not found", ErrSlideNotFound, input.SlideID)
		}
	case "object":
		// Find the slide containing the object
		objectSlide := findSlideContainingObject(presentation.Slides, input.ObjectID)
		if objectSlide == nil {
			return nil, fmt.Errorf("%w: object '%s' not found", ErrObjectNotFound, input.ObjectID)
		}
		// Note: Google Slides API doesn't support scoping to a single object within a slide,
		// only to pages (slides). We'll limit to the slide containing the object and then
		// do our own counting to report accurately.
		pageObjectIds = []string{objectSlide.ObjectId}
	case "all":
		// No page object IDs - search entire presentation
		pageObjectIds = nil
	}

	// Find affected objects BEFORE replacement (for reporting)
	affectedObjects := findAffectedObjects(presentation.Slides, input.Find, input.CaseSensitive, input.Scope, input.SlideID, input.ObjectID)

	// Build the replace request
	replaceRequest := &slides.Request{
		ReplaceAllText: &slides.ReplaceAllTextRequest{
			ContainsText: &slides.SubstringMatchCriteria{
				Text:      input.Find,
				MatchCase: input.CaseSensitive,
			},
			ReplaceText:   input.ReplaceWith,
			PageObjectIds: pageObjectIds,
		},
	}

	// Execute batch update
	response, err := slidesService.BatchUpdate(ctx, input.PresentationID, []*slides.Request{replaceRequest})
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrReplaceTextFailed, err)
	}

	// Extract replacement count from response
	replacementCount := int64(0)
	if len(response.Replies) > 0 && response.Replies[0].ReplaceAllText != nil {
		replacementCount = response.Replies[0].ReplaceAllText.OccurrencesChanged
	}

	output := &ReplaceTextOutput{
		PresentationID:   input.PresentationID,
		Find:             input.Find,
		ReplaceWith:      input.ReplaceWith,
		CaseSensitive:    input.CaseSensitive,
		Scope:            input.Scope,
		ReplacementCount: int(replacementCount),
		AffectedObjects:  affectedObjects,
	}

	t.config.Logger.Info("text replacement completed",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("replacement_count", int(replacementCount)),
		slog.Int("affected_objects", len(affectedObjects)),
	)

	return output, nil
}

// findSlideContainingObject finds the slide that contains a specific object.
func findSlideContainingObject(slides []*slides.Page, objectID string) *slides.Page {
	for _, slide := range slides {
		if slide == nil {
			continue
		}
		if containsObject(slide.PageElements, objectID) {
			return slide
		}
	}
	return nil
}

// containsObject checks if an object exists within page elements (including groups).
func containsObject(elements []*slides.PageElement, objectID string) bool {
	for _, element := range elements {
		if element == nil {
			continue
		}
		if element.ObjectId == objectID {
			return true
		}
		// Check in groups
		if element.ElementGroup != nil {
			if containsObject(element.ElementGroup.Children, objectID) {
				return true
			}
		}
	}
	return false
}

// findAffectedObjects finds objects that contain the search text.
func findAffectedObjects(allSlides []*slides.Page, find string, caseSensitive bool, scope, slideID, objectID string) []AffectedObject {
	var affected []AffectedObject

	for slideIdx, slide := range allSlides {
		if slide == nil {
			continue
		}

		// If scope is "slide", only process that specific slide
		if scope == "slide" && slide.ObjectId != slideID {
			continue
		}

		// Find affected objects on this slide
		slideAffected := findAffectedObjectsInElements(slide.PageElements, find, caseSensitive, scope, objectID)
		for _, obj := range slideAffected {
			obj.SlideIndex = slideIdx + 1 // 1-based
			obj.SlideID = slide.ObjectId
			affected = append(affected, obj)
		}
	}

	return affected
}

// findAffectedObjectsInElements finds objects within elements that contain the search text.
func findAffectedObjectsInElements(elements []*slides.PageElement, find string, caseSensitive bool, scope, objectID string) []AffectedObject {
	var affected []AffectedObject

	for _, element := range elements {
		if element == nil {
			continue
		}

		// If scope is "object", only check that specific object
		if scope == "object" && element.ObjectId != objectID {
			// But check children of groups
			if element.ElementGroup != nil {
				childAffected := findAffectedObjectsInElements(element.ElementGroup.Children, find, caseSensitive, scope, objectID)
				affected = append(affected, childAffected...)
			}
			continue
		}

		// Check if this element contains the search text
		if containsSearchText(element, find, caseSensitive) {
			affected = append(affected, AffectedObject{
				ObjectID:   element.ObjectId,
				ObjectType: determineObjectType(element),
			})
		}

		// Recursively check groups (but only if scope is not "object" targeting a specific element)
		if element.ElementGroup != nil && scope != "object" {
			childAffected := findAffectedObjectsInElements(element.ElementGroup.Children, find, caseSensitive, scope, objectID)
			affected = append(affected, childAffected...)
		}
	}

	return affected
}

// containsSearchText checks if an element contains the search text.
func containsSearchText(element *slides.PageElement, find string, caseSensitive bool) bool {
	if element == nil {
		return false
	}

	// Check shape text
	if element.Shape != nil && element.Shape.Text != nil {
		text := extractTextFromTextContent(element.Shape.Text)
		if textContains(text, find, caseSensitive) {
			return true
		}
	}

	// Check table cells
	if element.Table != nil {
		for _, row := range element.Table.TableRows {
			if row == nil {
				continue
			}
			for _, cell := range row.TableCells {
				if cell == nil || cell.Text == nil {
					continue
				}
				text := extractTextFromTextContent(cell.Text)
				if textContains(text, find, caseSensitive) {
					return true
				}
			}
		}
	}

	return false
}

// textContains checks if text contains find string with case sensitivity option.
func textContains(text, find string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.Contains(text, find)
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(find))
}
