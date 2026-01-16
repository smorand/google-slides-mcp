package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for group_objects tool.
var (
	ErrGroupObjectsFailed   = errors.New("failed to group objects")
	ErrUngroupObjectsFailed = errors.New("failed to ungroup objects")
	ErrInvalidGroupAction   = errors.New("invalid group action")
	ErrNotEnoughObjects     = errors.New("at least two objects are required to group")
	ErrObjectsOnDifferentPages = errors.New("all objects must be on the same page")
	ErrNotAGroup            = errors.New("object is not a group")
	ErrCannotGroupObject    = errors.New("object cannot be grouped")
)

// GroupObjectsInput represents the input for the group_objects tool.
type GroupObjectsInput struct {
	PresentationID string   `json:"presentation_id"`
	Action         string   `json:"action"`     // "group" or "ungroup"
	ObjectIDs      []string `json:"object_ids"` // For "group" action: array of object IDs to group
	ObjectID       string   `json:"object_id"`  // For "ungroup" action: ID of group to ungroup
}

// GroupObjectsOutput represents the output of the group_objects tool.
type GroupObjectsOutput struct {
	Action    string   `json:"action"`               // The action performed
	GroupID   string   `json:"group_id,omitempty"`   // For "group": the created group's object ID
	ObjectIDs []string `json:"object_ids,omitempty"` // For "ungroup": the ungrouped object IDs
}

// groupTimeNowFunc allows overriding time.Now for tests.
var groupTimeNowFunc = time.Now

// GroupObjects groups or ungroups objects in a presentation.
func (t *Tools) GroupObjects(ctx context.Context, tokenSource oauth2.TokenSource, input GroupObjectsInput) (*GroupObjectsOutput, error) {
	// Validate common input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.Action == "" {
		return nil, fmt.Errorf("%w: action is required (group, ungroup)", ErrInvalidGroupAction)
	}

	// Normalize action
	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action != "group" && action != "ungroup" {
		return nil, fmt.Errorf("%w: '%s' is not a valid action (use 'group' or 'ungroup')", ErrInvalidGroupAction, input.Action)
	}

	t.config.Logger.Info("group objects operation",
		slog.String("presentation_id", input.PresentationID),
		slog.String("action", action),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get presentation
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

	if action == "group" {
		return t.groupObjects(ctx, slidesService, presentation, input)
	}
	return t.ungroupObjects(ctx, slidesService, presentation, input)
}

// groupObjects groups multiple objects together.
func (t *Tools) groupObjects(ctx context.Context, slidesService SlidesService, presentation *slides.Presentation, input GroupObjectsInput) (*GroupObjectsOutput, error) {
	// Validate group-specific input
	if len(input.ObjectIDs) < 2 {
		return nil, fmt.Errorf("%w: got %d object(s)", ErrNotEnoughObjects, len(input.ObjectIDs))
	}

	// Find all objects and verify they're on the same page and can be grouped
	var slidePage *slides.Page
	objectIDSet := make(map[string]bool)
	for _, objectID := range input.ObjectIDs {
		objectIDSet[objectID] = true
	}

	for _, slide := range presentation.Slides {
		foundCount := 0
		for _, objectID := range input.ObjectIDs {
			element, inGroup := findElementAndCheckGroup(slide.PageElements, objectID)
			if element != nil {
				foundCount++

				// Check if object is already in a group
				if inGroup {
					return nil, fmt.Errorf("%w: object '%s' is already inside a group", ErrCannotGroupObject, objectID)
				}

				// Check if object type can be grouped (tables, videos, placeholders cannot be grouped)
				if element.Table != nil {
					return nil, fmt.Errorf("%w: tables cannot be grouped (object '%s')", ErrCannotGroupObject, objectID)
				}
				if element.Video != nil {
					return nil, fmt.Errorf("%w: videos cannot be grouped (object '%s')", ErrCannotGroupObject, objectID)
				}
				// Check for placeholder shapes
				if element.Shape != nil && element.Shape.Placeholder != nil {
					return nil, fmt.Errorf("%w: placeholder shapes cannot be grouped (object '%s')", ErrCannotGroupObject, objectID)
				}
			}
		}

		if foundCount > 0 {
			if foundCount != len(input.ObjectIDs) {
				// Some objects are on this page, some are not
				return nil, fmt.Errorf("%w: found %d of %d objects on slide '%s'", ErrObjectsOnDifferentPages, foundCount, len(input.ObjectIDs), slide.ObjectId)
			}
			slidePage = slide
			break
		}
	}

	if slidePage == nil {
		return nil, fmt.Errorf("%w: none of the specified objects were found in the presentation", ErrObjectNotFound)
	}

	// Generate a unique group object ID
	groupObjectID := fmt.Sprintf("group_%d", groupTimeNowFunc().UnixNano())

	// Create the group request
	req := &slides.Request{
		GroupObjects: &slides.GroupObjectsRequest{
			GroupObjectId:    groupObjectID,
			ChildrenObjectIds: input.ObjectIDs,
		},
	}

	resp, err := slidesService.BatchUpdate(ctx, input.PresentationID, []*slides.Request{req})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGroupObjectsFailed, err)
	}

	// Extract the created group ID from response
	createdGroupID := groupObjectID
	if resp != nil && len(resp.Replies) > 0 && resp.Replies[0].GroupObjects != nil {
		if resp.Replies[0].GroupObjects.ObjectId != "" {
			createdGroupID = resp.Replies[0].GroupObjects.ObjectId
		}
	}

	t.config.Logger.Info("grouped objects successfully",
		slog.String("group_id", createdGroupID),
		slog.Int("object_count", len(input.ObjectIDs)),
	)

	return &GroupObjectsOutput{
		Action:  "group",
		GroupID: createdGroupID,
	}, nil
}

// ungroupObjects ungroups a group into individual objects.
func (t *Tools) ungroupObjects(ctx context.Context, slidesService SlidesService, presentation *slides.Presentation, input GroupObjectsInput) (*GroupObjectsOutput, error) {
	// Validate ungroup-specific input
	if input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required for ungroup action", ErrObjectNotFound)
	}

	// Find the group and verify it's actually a group
	var groupElement *slides.PageElement
	var groupSlide *slides.Page
	for _, slide := range presentation.Slides {
		for _, elem := range slide.PageElements {
			if elem.ObjectId == input.ObjectID {
				groupElement = elem
				groupSlide = slide
				break
			}
		}
		if groupElement != nil {
			break
		}
	}

	if groupElement == nil {
		return nil, fmt.Errorf("%w: object '%s' not found in presentation", ErrObjectNotFound, input.ObjectID)
	}

	// Verify it's a group
	if groupElement.ElementGroup == nil {
		return nil, fmt.Errorf("%w: object '%s' is not a group", ErrNotAGroup, input.ObjectID)
	}

	// Check if the group itself is inside another group (not allowed)
	for _, slide := range presentation.Slides {
		for _, elem := range slide.PageElements {
			if elem.ElementGroup != nil && elem.ObjectId != input.ObjectID {
				if containsObjectID(elem.ElementGroup.Children, input.ObjectID) {
					return nil, fmt.Errorf("%w: group '%s' is inside another group and cannot be ungrouped", ErrUngroupObjectsFailed, input.ObjectID)
				}
			}
		}
	}

	// Collect child IDs before ungrouping
	var childIDs []string
	if groupElement.ElementGroup.Children != nil {
		for _, child := range groupElement.ElementGroup.Children {
			childIDs = append(childIDs, child.ObjectId)
		}
	}

	// Create the ungroup request
	req := &slides.Request{
		UngroupObjects: &slides.UngroupObjectsRequest{
			ObjectIds: []string{input.ObjectID},
		},
	}

	_, err := slidesService.BatchUpdate(ctx, input.PresentationID, []*slides.Request{req})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUngroupObjectsFailed, err)
	}

	t.config.Logger.Info("ungrouped objects successfully",
		slog.String("group_id", input.ObjectID),
		slog.Int("child_count", len(childIDs)),
		slog.String("slide_id", groupSlide.ObjectId),
	)

	return &GroupObjectsOutput{
		Action:    "ungroup",
		ObjectIDs: childIDs,
	}, nil
}

// containsObjectID checks if a slice of page elements contains an element with the given object ID.
func containsObjectID(elements []*slides.PageElement, objectID string) bool {
	for _, elem := range elements {
		if elem.ObjectId == objectID {
			return true
		}
		// Check recursively in nested groups
		if elem.ElementGroup != nil && elem.ElementGroup.Children != nil {
			if containsObjectID(elem.ElementGroup.Children, objectID) {
				return true
			}
		}
	}
	return false
}
