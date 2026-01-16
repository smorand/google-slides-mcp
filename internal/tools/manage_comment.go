package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

// Sentinel errors for manage_comment tool.
var (
	ErrManageCommentFailed    = errors.New("failed to manage comment")
	ErrInvalidCommentAction   = errors.New("invalid action: must be 'reply', 'resolve', 'unresolve', or 'delete'")
	ErrInvalidCommentID       = errors.New("comment_id is required")
	ErrReplyContentRequired   = errors.New("content is required for reply action")
	ErrCommentNotFound        = errors.New("comment not found")
)

// ManageCommentInput represents the input for the manage_comment tool.
type ManageCommentInput struct {
	PresentationID string `json:"presentation_id"`
	CommentID      string `json:"comment_id"`
	Action         string `json:"action"`   // "reply", "resolve", "unresolve", "delete"
	Content        string `json:"content,omitempty"` // Required for "reply" action
}

// ManageCommentOutput represents the output of the manage_comment tool.
type ManageCommentOutput struct {
	PresentationID string `json:"presentation_id"`
	CommentID      string `json:"comment_id"`
	Action         string `json:"action"`
	ReplyID        string `json:"reply_id,omitempty"`   // Only for "reply" action
	Success        bool   `json:"success"`
	Message        string `json:"message"`
}

// ManageComment handles reply, resolve, unresolve, and delete actions for comments.
func (t *Tools) ManageComment(ctx context.Context, tokenSource oauth2.TokenSource, input ManageCommentInput) (*ManageCommentOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.CommentID == "" {
		return nil, ErrInvalidCommentID
	}

	// Normalize action to lowercase
	action := strings.ToLower(strings.TrimSpace(input.Action))

	// Validate action
	validActions := map[string]bool{
		"reply":     true,
		"resolve":   true,
		"unresolve": true,
		"delete":    true,
	}
	if !validActions[action] {
		return nil, ErrInvalidCommentAction
	}

	// Validate content for reply action
	if action == "reply" && strings.TrimSpace(input.Content) == "" {
		return nil, ErrReplyContentRequired
	}

	t.config.Logger.Info("managing comment",
		slog.String("presentation_id", input.PresentationID),
		slog.String("comment_id", input.CommentID),
		slog.String("action", action),
	)

	// Create Drive service (comments are managed via Drive API)
	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
	}

	var output *ManageCommentOutput

	switch action {
	case "reply":
		output, err = t.handleReply(ctx, driveService, input)
	case "resolve":
		output, err = t.handleResolve(ctx, driveService, input, true)
	case "unresolve":
		output, err = t.handleResolve(ctx, driveService, input, false)
	case "delete":
		output, err = t.handleDelete(ctx, driveService, input)
	}

	if err != nil {
		return nil, err
	}

	t.config.Logger.Info("comment managed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("comment_id", input.CommentID),
		slog.String("action", action),
	)

	return output, nil
}

// handleReply creates a reply to a comment.
func (t *Tools) handleReply(ctx context.Context, driveService DriveService, input ManageCommentInput) (*ManageCommentOutput, error) {
	reply := &drive.Reply{
		Content: strings.TrimSpace(input.Content),
	}

	createdReply, err := driveService.CreateReply(ctx, input.PresentationID, input.CommentID, reply)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrCommentNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDriveAPIError, err)
	}

	return &ManageCommentOutput{
		PresentationID: input.PresentationID,
		CommentID:      input.CommentID,
		Action:         "reply",
		ReplyID:        createdReply.Id,
		Success:        true,
		Message:        "Reply added successfully",
	}, nil
}

// handleResolve resolves or unresolves a comment.
func (t *Tools) handleResolve(ctx context.Context, driveService DriveService, input ManageCommentInput, resolve bool) (*ManageCommentOutput, error) {
	comment := &drive.Comment{
		Resolved: resolve,
	}

	_, err := driveService.UpdateComment(ctx, input.PresentationID, input.CommentID, comment)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrCommentNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDriveAPIError, err)
	}

	action := "resolve"
	message := "Comment resolved successfully"
	if !resolve {
		action = "unresolve"
		message = "Comment reopened successfully"
	}

	return &ManageCommentOutput{
		PresentationID: input.PresentationID,
		CommentID:      input.CommentID,
		Action:         action,
		Success:        true,
		Message:        message,
	}, nil
}

// handleDelete deletes a comment.
func (t *Tools) handleDelete(ctx context.Context, driveService DriveService, input ManageCommentInput) (*ManageCommentOutput, error) {
	err := driveService.DeleteComment(ctx, input.PresentationID, input.CommentID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrCommentNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrDriveAPIError, err)
	}

	return &ManageCommentOutput{
		PresentationID: input.PresentationID,
		CommentID:      input.CommentID,
		Action:         "delete",
		Success:        true,
		Message:        "Comment deleted successfully",
	}, nil
}
