package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
)

// Sentinel errors for list_comments tool.
var (
	ErrListCommentsFailed = errors.New("failed to list comments")
)

// ListCommentsInput represents the input for the list_comments tool.
type ListCommentsInput struct {
	PresentationID  string `json:"presentation_id"`
	IncludeResolved bool   `json:"include_resolved,omitempty"` // Default: false (only unresolved)
}

// ListCommentsOutput represents the output of the list_comments tool.
type ListCommentsOutput struct {
	PresentationID string           `json:"presentation_id"`
	Comments       []CommentInfo    `json:"comments"`
	TotalCount     int              `json:"total_count"`
	UnresolvedCount int             `json:"unresolved_count"`
	ResolvedCount  int              `json:"resolved_count"`
}

// CommentInfo represents a comment with its details.
type CommentInfo struct {
	CommentID   string      `json:"comment_id"`
	Author      AuthorInfo  `json:"author"`
	Content     string      `json:"content"`
	HTMLContent string      `json:"html_content,omitempty"`
	AnchorInfo  string      `json:"anchor_info,omitempty"` // JSON string from anchor field
	Replies     []ReplyInfo `json:"replies,omitempty"`
	Resolved    bool        `json:"resolved"`
	Deleted     bool        `json:"deleted,omitempty"`
	CreatedTime string      `json:"created_time"`
	ModifiedTime string     `json:"modified_time,omitempty"`
}

// AuthorInfo represents the author of a comment or reply.
type AuthorInfo struct {
	DisplayName string `json:"display_name"`
	EmailAddress string `json:"email_address,omitempty"`
	PhotoLink   string `json:"photo_link,omitempty"`
}

// ReplyInfo represents a reply to a comment.
type ReplyInfo struct {
	ReplyID     string     `json:"reply_id"`
	Author      AuthorInfo `json:"author"`
	Content     string     `json:"content"`
	HTMLContent string     `json:"html_content,omitempty"`
	CreatedTime string     `json:"created_time"`
	ModifiedTime string    `json:"modified_time,omitempty"`
	Deleted     bool       `json:"deleted,omitempty"`
}

// ListComments lists all comments in a presentation.
func (t *Tools) ListComments(ctx context.Context, tokenSource oauth2.TokenSource, input ListCommentsInput) (*ListCommentsOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	t.config.Logger.Info("listing comments in presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.Bool("include_resolved", input.IncludeResolved),
	)

	// Create Drive service (comments are accessed via Drive API)
	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
	}

	// List all comments with pagination
	var allComments []CommentInfo
	pageToken := ""

	for {
		// List comments from Drive API
		// Note: include_resolved parameter in our API maps to showing resolved comments
		// By default, Drive API returns all non-deleted comments. We filter resolved later if needed.
		commentList, err := driveService.ListComments(ctx, input.PresentationID, false, 100, pageToken)
		if err != nil {
			if isNotFoundError(err) {
				return nil, ErrPresentationNotFound
			}
			if isForbiddenError(err) {
				return nil, ErrAccessDenied
			}
			return nil, fmt.Errorf("%w: %v", ErrDriveAPIError, err)
		}

		// Process comments
		for _, comment := range commentList.Comments {
			if comment == nil {
				continue
			}

			// Skip resolved comments if not requested
			if comment.Resolved && !input.IncludeResolved {
				continue
			}

			// Convert to our format
			commentInfo := CommentInfo{
				CommentID:    comment.Id,
				Content:      comment.Content,
				HTMLContent:  comment.HtmlContent,
				AnchorInfo:   comment.Anchor,
				Resolved:     comment.Resolved,
				Deleted:      comment.Deleted,
				CreatedTime:  comment.CreatedTime,
				ModifiedTime: comment.ModifiedTime,
			}

			// Add author info
			if comment.Author != nil {
				commentInfo.Author = AuthorInfo{
					DisplayName:  comment.Author.DisplayName,
					EmailAddress: comment.Author.EmailAddress,
					PhotoLink:    comment.Author.PhotoLink,
				}
			}

			// Add replies
			for _, reply := range comment.Replies {
				if reply == nil {
					continue
				}
				replyInfo := ReplyInfo{
					ReplyID:      reply.Id,
					Content:      reply.Content,
					HTMLContent:  reply.HtmlContent,
					CreatedTime:  reply.CreatedTime,
					ModifiedTime: reply.ModifiedTime,
					Deleted:      reply.Deleted,
				}
				if reply.Author != nil {
					replyInfo.Author = AuthorInfo{
						DisplayName:  reply.Author.DisplayName,
						EmailAddress: reply.Author.EmailAddress,
						PhotoLink:    reply.Author.PhotoLink,
					}
				}
				commentInfo.Replies = append(commentInfo.Replies, replyInfo)
			}

			allComments = append(allComments, commentInfo)
		}

		// Check for more pages
		if commentList.NextPageToken == "" {
			break
		}
		pageToken = commentList.NextPageToken
	}

	// Calculate statistics
	unresolvedCount := 0
	resolvedCount := 0
	for _, c := range allComments {
		if c.Resolved {
			resolvedCount++
		} else {
			unresolvedCount++
		}
	}

	output := &ListCommentsOutput{
		PresentationID:  input.PresentationID,
		Comments:        allComments,
		TotalCount:      len(allComments),
		UnresolvedCount: unresolvedCount,
		ResolvedCount:   resolvedCount,
	}

	t.config.Logger.Info("comments listed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("total_count", output.TotalCount),
		slog.Int("unresolved_count", output.UnresolvedCount),
		slog.Int("resolved_count", output.ResolvedCount),
	)

	return output, nil
}
