package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

// Sentinel errors for add_comment tool.
var (
	ErrAddCommentFailed    = errors.New("failed to add comment")
	ErrInvalidCommentText  = errors.New("comment content is required")
)

// AddCommentInput represents the input for the add_comment tool.
type AddCommentInput struct {
	PresentationID   string `json:"presentation_id"`
	Content          string `json:"content"`
	AnchorObjectID   string `json:"anchor_object_id,omitempty"`   // Optional - attach to specific object
	AnchorPageIndex  *int   `json:"anchor_page_index,omitempty"`  // Optional - attach to specific slide (0-based)
}

// AddCommentOutput represents the output of the add_comment tool.
type AddCommentOutput struct {
	CommentID      string `json:"comment_id"`
	PresentationID string `json:"presentation_id"`
	Content        string `json:"content"`
	AnchorInfo     string `json:"anchor_info,omitempty"`
	CreatedTime    string `json:"created_time,omitempty"`
}

// AddComment adds a comment to a presentation.
func (t *Tools) AddComment(ctx context.Context, tokenSource oauth2.TokenSource, input AddCommentInput) (*AddCommentOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.Content == "" {
		return nil, fmt.Errorf("%w", ErrInvalidCommentText)
	}

	t.config.Logger.Info("adding comment to presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Bool("has_anchor_object", input.AnchorObjectID != ""),
		slog.Bool("has_anchor_page", input.AnchorPageIndex != nil),
	)

	// Create Drive service (comments are managed via Drive API)
	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
	}

	// Build the comment
	comment := &drive.Comment{
		Content: input.Content,
	}

	// Build anchor if object ID or page index is provided
	// Google Drive anchor format for Slides:
	// - For object: {"r":"content","a":[{"n":"objectId","v":"<object_id>"}]}
	// - For page: {"r":"content","a":[{"n":"pageNumber","v":"<page_number>"}]}
	if input.AnchorObjectID != "" {
		comment.Anchor = fmt.Sprintf(`{"r":"content","a":[{"n":"objectId","v":"%s"}]}`, input.AnchorObjectID)
	} else if input.AnchorPageIndex != nil {
		// Page numbers in anchor are 1-based, but our input is 0-based
		pageNumber := *input.AnchorPageIndex + 1
		comment.Anchor = fmt.Sprintf(`{"r":"content","a":[{"n":"pageNumber","v":"%d"}]}`, pageNumber)
	}

	// Create the comment via Drive API
	createdComment, err := driveService.CreateComment(ctx, input.PresentationID, comment)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDriveAPIError, err)
	}

	output := &AddCommentOutput{
		CommentID:      createdComment.Id,
		PresentationID: input.PresentationID,
		Content:        createdComment.Content,
		AnchorInfo:     createdComment.Anchor,
		CreatedTime:    createdComment.CreatedTime,
	}

	t.config.Logger.Info("comment added successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("comment_id", output.CommentID),
	)

	return output, nil
}
