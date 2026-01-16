package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

func TestAddComment(t *testing.T) {
	ctx := context.Background()

	t.Run("adds comment to presentation successfully", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				if fileID != "pres-123" {
					t.Errorf("expected fileID 'pres-123', got '%s'", fileID)
				}
				if comment.Content != "This is a test comment" {
					t.Errorf("expected content 'This is a test comment', got '%s'", comment.Content)
				}
				return &drive.Comment{
					Id:          "comment-456",
					Content:     comment.Content,
					CreatedTime: "2024-01-15T10:00:00Z",
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "This is a test comment",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.CommentID != "comment-456" {
			t.Errorf("expected CommentID 'comment-456', got '%s'", output.CommentID)
		}
		if output.PresentationID != "pres-123" {
			t.Errorf("expected PresentationID 'pres-123', got '%s'", output.PresentationID)
		}
		if output.Content != "This is a test comment" {
			t.Errorf("expected Content 'This is a test comment', got '%s'", output.Content)
		}
		if output.CreatedTime != "2024-01-15T10:00:00Z" {
			t.Errorf("expected CreatedTime '2024-01-15T10:00:00Z', got '%s'", output.CreatedTime)
		}
	})

	t.Run("comment can be anchored to object", func(t *testing.T) {
		var capturedComment *drive.Comment
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:      "comment-789",
					Content: comment.Content,
					Anchor:  comment.Anchor,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "Comment on object",
			AnchorObjectID: "shape-xyz",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify anchor was set correctly
		expectedAnchor := `{"r":"content","a":[{"n":"objectId","v":"shape-xyz"}]}`
		if capturedComment.Anchor != expectedAnchor {
			t.Errorf("expected anchor '%s', got '%s'", expectedAnchor, capturedComment.Anchor)
		}

		// Verify output has anchor info
		if output.AnchorInfo != expectedAnchor {
			t.Errorf("expected AnchorInfo '%s', got '%s'", expectedAnchor, output.AnchorInfo)
		}
	})

	t.Run("comment can be anchored to slide", func(t *testing.T) {
		var capturedComment *drive.Comment
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:      "comment-101",
					Content: comment.Content,
					Anchor:  comment.Anchor,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		pageIndex := 2 // 0-based, so slide 3
		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID:  "pres-123",
			Content:         "Comment on slide 3",
			AnchorPageIndex: &pageIndex,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify anchor was set correctly (page numbers are 1-based in anchor)
		expectedAnchor := `{"r":"content","a":[{"n":"pageNumber","v":"3"}]}`
		if capturedComment.Anchor != expectedAnchor {
			t.Errorf("expected anchor '%s', got '%s'", expectedAnchor, capturedComment.Anchor)
		}

		if output.AnchorInfo != expectedAnchor {
			t.Errorf("expected AnchorInfo '%s', got '%s'", expectedAnchor, output.AnchorInfo)
		}
	})

	t.Run("comment can be anchored to first slide (page 0)", func(t *testing.T) {
		var capturedComment *drive.Comment
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:      "comment-102",
					Content: comment.Content,
					Anchor:  comment.Anchor,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		pageIndex := 0 // First slide
		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID:  "pres-123",
			Content:         "Comment on first slide",
			AnchorPageIndex: &pageIndex,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify anchor was set correctly (page 0 becomes page 1 in anchor)
		expectedAnchor := `{"r":"content","a":[{"n":"pageNumber","v":"1"}]}`
		if capturedComment.Anchor != expectedAnchor {
			t.Errorf("expected anchor '%s', got '%s'", expectedAnchor, capturedComment.Anchor)
		}

		if output.AnchorInfo != expectedAnchor {
			t.Errorf("expected AnchorInfo '%s', got '%s'", expectedAnchor, output.AnchorInfo)
		}
	})

	t.Run("object anchor takes precedence over page anchor", func(t *testing.T) {
		var capturedComment *drive.Comment
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:      "comment-103",
					Content: comment.Content,
					Anchor:  comment.Anchor,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		pageIndex := 5
		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID:  "pres-123",
			Content:         "Comment with both anchors",
			AnchorObjectID:  "shape-abc",
			AnchorPageIndex: &pageIndex,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Object anchor should be used, not page anchor
		expectedAnchor := `{"r":"content","a":[{"n":"objectId","v":"shape-abc"}]}`
		if capturedComment.Anchor != expectedAnchor {
			t.Errorf("expected object anchor '%s', got '%s'", expectedAnchor, capturedComment.Anchor)
		}
	})

	t.Run("returns comment_id", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				return &drive.Comment{
					Id:      "new-comment-id-12345",
					Content: comment.Content,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "Test comment",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.CommentID != "new-comment-id-12345" {
			t.Errorf("expected CommentID 'new-comment-id-12345', got '%s'", output.CommentID)
		}
	})

	t.Run("returns error for empty presentation ID", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "",
			Content:        "Test comment",
		})

		if err == nil {
			t.Fatal("expected error for empty presentation ID")
		}
		if !errors.Is(err, ErrInvalidPresentationID) {
			t.Errorf("expected ErrInvalidPresentationID, got %v", err)
		}
	})

	t.Run("returns error for empty content", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "",
		})

		if err == nil {
			t.Fatal("expected error for empty content")
		}
		if !errors.Is(err, ErrInvalidCommentText) {
			t.Errorf("expected ErrInvalidCommentText, got %v", err)
		}
	})

	t.Run("returns error when presentation not found", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				return nil, &notFoundError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "nonexistent-pres",
			Content:        "Test comment",
		})

		if err == nil {
			t.Fatal("expected error for not found presentation")
		}
		if !errors.Is(err, ErrPresentationNotFound) {
			t.Errorf("expected ErrPresentationNotFound, got %v", err)
		}
	})

	t.Run("returns error when access denied", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				return nil, &forbiddenError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "private-pres",
			Content:        "Test comment",
		})

		if err == nil {
			t.Fatal("expected error for access denied")
		}
		if !errors.Is(err, ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("returns error when drive service fails", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				return nil, errors.New("internal drive error")
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "Test comment",
		})

		if err == nil {
			t.Fatal("expected error for drive service failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("returns error when drive service factory fails", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return nil, errors.New("failed to create drive service")
		})

		_, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "Test comment",
		})

		if err == nil {
			t.Fatal("expected error for drive service factory failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("no anchor when neither object nor page specified", func(t *testing.T) {
		var capturedComment *drive.Comment
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				capturedComment = comment
				return &drive.Comment{
					Id:      "comment-no-anchor",
					Content: comment.Content,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "pres-123",
			Content:        "Comment without anchor",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify no anchor was set
		if capturedComment.Anchor != "" {
			t.Errorf("expected empty anchor, got '%s'", capturedComment.Anchor)
		}

		if output.AnchorInfo != "" {
			t.Errorf("expected empty AnchorInfo, got '%s'", output.AnchorInfo)
		}
	})

	t.Run("returns presentation ID in output", func(t *testing.T) {
		mockDrive := &mockDriveService{
			CreateCommentFunc: func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
				return &drive.Comment{
					Id:      "comment-123",
					Content: comment.Content,
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.AddComment(ctx, nil, AddCommentInput{
			PresentationID: "my-pres-id",
			Content:        "Test",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.PresentationID != "my-pres-id" {
			t.Errorf("expected PresentationID 'my-pres-id', got '%s'", output.PresentationID)
		}
	})
}
