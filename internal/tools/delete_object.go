package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for delete_object tool.
var (
	ErrDeleteObjectFailed = errors.New("failed to delete object")
	ErrNoObjectsToDelete  = errors.New("no objects specified for deletion")
)

// DeleteObjectInput represents the input for the delete_object tool.
type DeleteObjectInput struct {
	PresentationID string   `json:"presentation_id"`
	ObjectID       string   `json:"object_id,omitempty"` // Single object ID (use this OR Multiple)
	Multiple       []string `json:"multiple,omitempty"`  // Array of object IDs for batch delete
}

// DeleteObjectOutput represents the output of the delete_object tool.
type DeleteObjectOutput struct {
	DeletedCount int      `json:"deleted_count"`           // Number of objects deleted
	DeletedIDs   []string `json:"deleted_ids"`             // List of deleted object IDs
	NotFoundIDs  []string `json:"not_found_ids,omitempty"` // Object IDs that were not found (if any)
}

// DeleteObject deletes one or more objects from a presentation.
func (t *Tools) DeleteObject(ctx context.Context, tokenSource oauth2.TokenSource, input DeleteObjectInput) (*DeleteObjectOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	// Determine which objects to delete
	objectIDsToDelete := t.collectObjectIDsToDelete(input)
	if len(objectIDsToDelete) == 0 {
		return nil, fmt.Errorf("%w: provide object_id or multiple", ErrNoObjectsToDelete)
	}

	t.config.Logger.Info("deleting objects from presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("object_count", len(objectIDsToDelete)),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation to validate objects exist
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

	// Verify which objects exist and which don't
	existingObjectIDs, notFoundIDs := t.categorizeObjectIDs(presentation, objectIDsToDelete)

	// If no objects found, return error
	if len(existingObjectIDs) == 0 {
		return nil, fmt.Errorf("%w: none of the specified objects were found", ErrObjectNotFound)
	}

	// Build delete requests for existing objects
	requests := make([]*slides.Request, len(existingObjectIDs))
	for i, objectID := range existingObjectIDs {
		requests[i] = &slides.Request{
			DeleteObject: &slides.DeleteObjectRequest{
				ObjectId: objectID,
			},
		}
	}

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDeleteObjectFailed, err)
	}

	output := &DeleteObjectOutput{
		DeletedCount: len(existingObjectIDs),
		DeletedIDs:   existingObjectIDs,
	}

	// Include not found IDs if any
	if len(notFoundIDs) > 0 {
		output.NotFoundIDs = notFoundIDs
	}

	t.config.Logger.Info("objects deleted successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("deleted_count", output.DeletedCount),
		slog.Int("not_found_count", len(notFoundIDs)),
	)

	return output, nil
}

// collectObjectIDsToDelete collects object IDs from input, deduplicating them.
func (t *Tools) collectObjectIDsToDelete(input DeleteObjectInput) []string {
	seen := make(map[string]bool)
	var result []string

	// Add single object ID if provided
	if input.ObjectID != "" {
		seen[input.ObjectID] = true
		result = append(result, input.ObjectID)
	}

	// Add multiple object IDs if provided
	for _, id := range input.Multiple {
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	return result
}

// categorizeObjectIDs separates object IDs into existing and not found categories.
func (t *Tools) categorizeObjectIDs(presentation *slides.Presentation, objectIDs []string) (existing, notFound []string) {
	// Build a set of all object IDs in the presentation
	presentationObjects := make(map[string]bool)
	t.collectAllObjectIDs(presentation, presentationObjects)

	// Categorize the requested object IDs
	for _, id := range objectIDs {
		if presentationObjects[id] {
			existing = append(existing, id)
		} else {
			notFound = append(notFound, id)
		}
	}

	return existing, notFound
}

// collectAllObjectIDs recursively collects all object IDs from a presentation.
func (t *Tools) collectAllObjectIDs(presentation *slides.Presentation, objectIDs map[string]bool) {
	// Collect from slides
	for _, slide := range presentation.Slides {
		objectIDs[slide.ObjectId] = true
		t.collectPageElementIDs(slide.PageElements, objectIDs)

		// Also collect from notes page
		if slide.SlideProperties != nil && slide.SlideProperties.NotesPage != nil {
			t.collectPageElementIDs(slide.SlideProperties.NotesPage.PageElements, objectIDs)
		}
	}

	// Collect from masters
	for _, master := range presentation.Masters {
		objectIDs[master.ObjectId] = true
		t.collectPageElementIDs(master.PageElements, objectIDs)
	}

	// Collect from layouts
	for _, layout := range presentation.Layouts {
		objectIDs[layout.ObjectId] = true
		t.collectPageElementIDs(layout.PageElements, objectIDs)
	}
}

// collectPageElementIDs recursively collects object IDs from page elements.
func (t *Tools) collectPageElementIDs(elements []*slides.PageElement, objectIDs map[string]bool) {
	for _, elem := range elements {
		objectIDs[elem.ObjectId] = true

		// Recursively collect from groups
		if elem.ElementGroup != nil && elem.ElementGroup.Children != nil {
			t.collectPageElementIDs(elem.ElementGroup.Children, objectIDs)
		}

		// Collect from table cells (they don't have separate object IDs, but rows do)
		if elem.Table != nil {
			for _, row := range elem.Table.TableRows {
				for _, cell := range row.TableCells {
					// Cells don't have object IDs in the standard sense
					// But if there are any nested elements, collect them
					_ = cell // Cells are identified by row/column, not object ID
				}
			}
		}
	}
}
