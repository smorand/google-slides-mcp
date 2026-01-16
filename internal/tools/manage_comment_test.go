package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

func TestManageComment(t *testing.T) {
	ctx := context.Background()

	t.Run("reply adds a reply to comment", func(t *testing.T) {
		var capturedReply *drive.Reply
		var capturedFileID, capturedCommentID string

		mockDrive := &mockDriveService{
			CreateReplyFunc: func(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error) {
				capturedFileID = fileID
				capturedCommentID = commentID
				capturedReply = reply
				return &drive.Reply{
					Id:      "reply-123",
					Content: reply.Content,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "reply",
			Content:        "This is my reply",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedFileID != "pres-123" {
			t.Errorf("expected fileID 'pres-123', got '%s'", capturedFileID)
		}
		if capturedCommentID != "comment-456" {
			t.Errorf("expected commentID 'comment-456', got '%s'", capturedCommentID)
		}
		if capturedReply.Content != "This is my reply" {
			t.Errorf("expected reply content 'This is my reply', got '%s'", capturedReply.Content)
		}

		if output.ReplyID != "reply-123" {
			t.Errorf("expected ReplyID 'reply-123', got '%s'", output.ReplyID)
		}
		if output.Action != "reply" {
			t.Errorf("expected Action 'reply', got '%s'", output.Action)
		}
		if !output.Success {
			t.Error("expected Success to be true")
		}
	})

	t.Run("resolve marks comment as resolved", func(t *testing.T) {
		var capturedComment *drive.Comment

		mockDrive := &mockDriveService{
			UpdateCommentFunc: func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:       commentID,
					Resolved: comment.Resolved,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "resolve",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !capturedComment.Resolved {
			t.Error("expected Resolved to be true")
		}
		if output.Action != "resolve" {
			t.Errorf("expected Action 'resolve', got '%s'", output.Action)
		}
		if !output.Success {
			t.Error("expected Success to be true")
		}
		if output.Message != "Comment resolved successfully" {
			t.Errorf("expected message 'Comment resolved successfully', got '%s'", output.Message)
		}
	})

	t.Run("unresolve reopens resolved comment", func(t *testing.T) {
		var capturedComment *drive.Comment

		mockDrive := &mockDriveService{
			UpdateCommentFunc: func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:       commentID,
					Resolved: comment.Resolved,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "unresolve",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedComment.Resolved {
			t.Error("expected Resolved to be false")
		}
		if output.Action != "unresolve" {
			t.Errorf("expected Action 'unresolve', got '%s'", output.Action)
		}
		if !output.Success {
			t.Error("expected Success to be true")
		}
		if output.Message != "Comment reopened successfully" {
			t.Errorf("expected message 'Comment reopened successfully', got '%s'", output.Message)
		}
	})

	t.Run("delete removes comment", func(t *testing.T) {
		var deletedFileID, deletedCommentID string

		mockDrive := &mockDriveService{
			DeleteCommentFunc: func(ctx context.Context, fileID, commentID string) error {
				deletedFileID = fileID
				deletedCommentID = commentID
				return nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "delete",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if deletedFileID != "pres-123" {
			t.Errorf("expected fileID 'pres-123', got '%s'", deletedFileID)
		}
		if deletedCommentID != "comment-456" {
			t.Errorf("expected commentID 'comment-456', got '%s'", deletedCommentID)
		}
		if output.Action != "delete" {
			t.Errorf("expected Action 'delete', got '%s'", output.Action)
		}
		if !output.Success {
			t.Error("expected Success to be true")
		}
		if output.Message != "Comment deleted successfully" {
			t.Errorf("expected message 'Comment deleted successfully', got '%s'", output.Message)
		}
	})

	t.Run("action is case insensitive", func(t *testing.T) {
		mockDrive := &mockDriveService{
			UpdateCommentFunc: func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
				return &drive.Comment{Id: commentID, Resolved: comment.Resolved}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		// Test uppercase
		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "RESOLVE",
		})

		if err != nil {
			t.Fatalf("unexpected error for uppercase action: %v", err)
		}
		if output.Action != "resolve" {
			t.Errorf("expected action 'resolve', got '%s'", output.Action)
		}

		// Test mixed case
		output, err = tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "UnResolve",
		})

		if err != nil {
			t.Fatalf("unexpected error for mixed case action: %v", err)
		}
		if output.Action != "unresolve" {
			t.Errorf("expected action 'unresolve', got '%s'", output.Action)
		}
	})

	t.Run("returns error for empty presentation ID", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "",
			CommentID:      "comment-456",
			Action:         "resolve",
		})

		if err == nil {
			t.Fatal("expected error for empty presentation ID")
		}
		if !errors.Is(err, ErrInvalidPresentationID) {
			t.Errorf("expected ErrInvalidPresentationID, got %v", err)
		}
	})

	t.Run("returns error for empty comment ID", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "",
			Action:         "resolve",
		})

		if err == nil {
			t.Fatal("expected error for empty comment ID")
		}
		if !errors.Is(err, ErrInvalidCommentID) {
			t.Errorf("expected ErrInvalidCommentID, got %v", err)
		}
	})

	t.Run("returns error for invalid action", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "invalid_action",
		})

		if err == nil {
			t.Fatal("expected error for invalid action")
		}
		if !errors.Is(err, ErrInvalidCommentAction) {
			t.Errorf("expected ErrInvalidCommentAction, got %v", err)
		}
	})

	t.Run("returns error for reply without content", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "reply",
			Content:        "",
		})

		if err == nil {
			t.Fatal("expected error for reply without content")
		}
		if !errors.Is(err, ErrReplyContentRequired) {
			t.Errorf("expected ErrReplyContentRequired, got %v", err)
		}
	})

	t.Run("returns error for reply with whitespace-only content", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "reply",
			Content:        "   ",
		})

		if err == nil {
			t.Fatal("expected error for reply with whitespace-only content")
		}
		if !errors.Is(err, ErrReplyContentRequired) {
			t.Errorf("expected ErrReplyContentRequired, got %v", err)
		}
	})

	t.Run("returns error when comment not found for reply", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateReplyFunc: func(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error) {
				return nil, &notFoundError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "nonexistent-comment",
			Action:         "reply",
			Content:        "Test reply",
		})

		if err == nil {
			t.Fatal("expected error for not found comment")
		}
		if !errors.Is(err, ErrCommentNotFound) {
			t.Errorf("expected ErrCommentNotFound, got %v", err)
		}
	})

	t.Run("returns error when comment not found for resolve", func(t *testing.T) {
		mockDrive := &mockDriveService{
			UpdateCommentFunc: func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
				return nil, &notFoundError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "nonexistent-comment",
			Action:         "resolve",
		})

		if err == nil {
			t.Fatal("expected error for not found comment")
		}
		if !errors.Is(err, ErrCommentNotFound) {
			t.Errorf("expected ErrCommentNotFound, got %v", err)
		}
	})

	t.Run("returns error when comment not found for delete", func(t *testing.T) {
		mockDrive := &mockDriveService{
			DeleteCommentFunc: func(ctx context.Context, fileID, commentID string) error {
				return &notFoundError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "nonexistent-comment",
			Action:         "delete",
		})

		if err == nil {
			t.Fatal("expected error for not found comment")
		}
		if !errors.Is(err, ErrCommentNotFound) {
			t.Errorf("expected ErrCommentNotFound, got %v", err)
		}
	})

	t.Run("returns error when access denied for reply", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateReplyFunc: func(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error) {
				return nil, &forbiddenError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "reply",
			Content:        "Test reply",
		})

		if err == nil {
			t.Fatal("expected error for access denied")
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("returns error when access denied for resolve", func(t *testing.T) {
		mockDrive := &mockDriveService{
			UpdateCommentFunc: func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
				return nil, &forbiddenError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "resolve",
		})

		if err == nil {
			t.Fatal("expected error for access denied")
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("returns error when access denied for delete", func(t *testing.T) {
		mockDrive := &mockDriveService{
			DeleteCommentFunc: func(ctx context.Context, fileID, commentID string) error {
				return &forbiddenError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "delete",
		})

		if err == nil {
			t.Fatal("expected error for access denied")
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("returns error when drive service factory fails", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return nil, errors.New("failed to create drive service")
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "resolve",
		})

		if err == nil {
			t.Fatal("expected error for drive service factory failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("returns error when drive API fails for reply", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateReplyFunc: func(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error) {
				return nil, errors.New("internal drive error")
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "reply",
			Content:        "Test reply",
		})

		if err == nil {
			t.Fatal("expected error for drive API failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("returns error when drive API fails for resolve", func(t *testing.T) {
		mockDrive := &mockDriveService{
			UpdateCommentFunc: func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
				return nil, errors.New("internal drive error")
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "resolve",
		})

		if err == nil {
			t.Fatal("expected error for drive API failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("returns error when drive API fails for delete", func(t *testing.T) {
		mockDrive := &mockDriveService{
			DeleteCommentFunc: func(ctx context.Context, fileID, commentID string) error {
				return errors.New("internal drive error")
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "delete",
		})

		if err == nil {
			t.Fatal("expected error for drive API failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("returns presentation_id in output", func(t *testing.T) {
		mockDrive := &mockDriveService{
			DeleteCommentFunc: func(ctx context.Context, fileID, commentID string) error {
				return nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "my-pres-id",
			CommentID:      "comment-456",
			Action:         "delete",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.PresentationID != "my-pres-id" {
			t.Errorf("expected PresentationID 'my-pres-id', got '%s'", output.PresentationID)
		}
	})

	t.Run("returns comment_id in output", func(t *testing.T) {
		mockDrive := &mockDriveService{
			DeleteCommentFunc: func(ctx context.Context, fileID, commentID string) error {
				return nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "my-comment-id",
			Action:         "delete",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.CommentID != "my-comment-id" {
			t.Errorf("expected CommentID 'my-comment-id', got '%s'", output.CommentID)
		}
	})

	t.Run("reply trims whitespace from content", func(t *testing.T) {
		var capturedReply *drive.Reply

		mockDrive := &mockDriveService{
			CreateReplyFunc: func(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error) {
				capturedReply = reply
				return &drive.Reply{
					Id:      "reply-123",
					Content: reply.Content,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ManageComment(ctx, nil, ManageCommentInput{
			PresentationID: "pres-123",
			CommentID:      "comment-456",
			Action:         "reply",
			Content:        "  Trimmed reply content  ",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedReply.Content != "Trimmed reply content" {
			t.Errorf("expected trimmed content 'Trimmed reply content', got '%s'", capturedReply.Content)
		}
	})
}
