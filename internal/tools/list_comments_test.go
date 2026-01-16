package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

func TestListComments(t *testing.T) {
	ctx := context.Background()

	t.Run("lists all unresolved comments by default", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				if fileID != "pres-123" {
					t.Errorf("expected fileID 'pres-123', got '%s'", fileID)
				}
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "This is an unresolved comment",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author: &drive.User{
								DisplayName:  "John Doe",
								EmailAddress: "john@example.com",
							},
						},
						{
							Id:          "comment-2",
							Content:     "This is a resolved comment",
							Resolved:    true,
							CreatedTime: "2024-01-14T09:00:00Z",
							Author: &drive.User{
								DisplayName:  "Jane Doe",
								EmailAddress: "jane@example.com",
							},
						},
						{
							Id:          "comment-3",
							Content:     "Another unresolved comment",
							Resolved:    false,
							CreatedTime: "2024-01-13T08:00:00Z",
							Author: &drive.User{
								DisplayName: "Bob Smith",
							},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID:  "pres-123",
			IncludeResolved: false, // Default behavior
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should only include unresolved comments
		if output.TotalCount != 2 {
			t.Errorf("expected TotalCount 2 (unresolved only), got %d", output.TotalCount)
		}
		if output.UnresolvedCount != 2 {
			t.Errorf("expected UnresolvedCount 2, got %d", output.UnresolvedCount)
		}
		if output.ResolvedCount != 0 {
			t.Errorf("expected ResolvedCount 0, got %d", output.ResolvedCount)
		}

		// Verify only unresolved comments are returned
		for _, c := range output.Comments {
			if c.Resolved {
				t.Errorf("expected only unresolved comments, got resolved comment: %s", c.CommentID)
			}
		}
	})

	t.Run("include_resolved shows resolved comments", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Unresolved comment",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author:      &drive.User{DisplayName: "John Doe"},
						},
						{
							Id:          "comment-2",
							Content:     "Resolved comment",
							Resolved:    true,
							CreatedTime: "2024-01-14T09:00:00Z",
							Author:      &drive.User{DisplayName: "Jane Doe"},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID:  "pres-123",
			IncludeResolved: true,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should include both resolved and unresolved
		if output.TotalCount != 2 {
			t.Errorf("expected TotalCount 2, got %d", output.TotalCount)
		}
		if output.UnresolvedCount != 1 {
			t.Errorf("expected UnresolvedCount 1, got %d", output.UnresolvedCount)
		}
		if output.ResolvedCount != 1 {
			t.Errorf("expected ResolvedCount 1, got %d", output.ResolvedCount)
		}

		// Verify both types are present
		hasResolved := false
		hasUnresolved := false
		for _, c := range output.Comments {
			if c.Resolved {
				hasResolved = true
			} else {
				hasUnresolved = true
			}
		}
		if !hasResolved {
			t.Error("expected resolved comment to be included")
		}
		if !hasUnresolved {
			t.Error("expected unresolved comment to be included")
		}
	})

	t.Run("replies are included", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Original comment",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author: &drive.User{
								DisplayName:  "John Doe",
								EmailAddress: "john@example.com",
							},
							Replies: []*drive.Reply{
								{
									Id:          "reply-1",
									Content:     "First reply",
									CreatedTime: "2024-01-15T11:00:00Z",
									Author: &drive.User{
										DisplayName:  "Jane Doe",
										EmailAddress: "jane@example.com",
									},
								},
								{
									Id:           "reply-2",
									Content:      "Second reply",
									CreatedTime:  "2024-01-15T12:00:00Z",
									ModifiedTime: "2024-01-15T12:30:00Z",
									Author: &drive.User{
										DisplayName: "Bob Smith",
									},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(output.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(output.Comments))
		}

		comment := output.Comments[0]
		if len(comment.Replies) != 2 {
			t.Fatalf("expected 2 replies, got %d", len(comment.Replies))
		}

		// Verify first reply
		reply1 := comment.Replies[0]
		if reply1.ReplyID != "reply-1" {
			t.Errorf("expected reply ID 'reply-1', got '%s'", reply1.ReplyID)
		}
		if reply1.Content != "First reply" {
			t.Errorf("expected content 'First reply', got '%s'", reply1.Content)
		}
		if reply1.Author.DisplayName != "Jane Doe" {
			t.Errorf("expected author 'Jane Doe', got '%s'", reply1.Author.DisplayName)
		}
		if reply1.Author.EmailAddress != "jane@example.com" {
			t.Errorf("expected email 'jane@example.com', got '%s'", reply1.Author.EmailAddress)
		}

		// Verify second reply
		reply2 := comment.Replies[1]
		if reply2.ReplyID != "reply-2" {
			t.Errorf("expected reply ID 'reply-2', got '%s'", reply2.ReplyID)
		}
		if reply2.ModifiedTime != "2024-01-15T12:30:00Z" {
			t.Errorf("expected modified time '2024-01-15T12:30:00Z', got '%s'", reply2.ModifiedTime)
		}
	})

	t.Run("anchor information is provided", func(t *testing.T) {
		anchorJSON := `{"r":"headings","a":[{"startIndex":5,"endIndex":15}]}`
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Comment with anchor",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Anchor:      anchorJSON,
							Author:      &drive.User{DisplayName: "John Doe"},
						},
						{
							Id:          "comment-2",
							Content:     "Comment without anchor",
							Resolved:    false,
							CreatedTime: "2024-01-15T11:00:00Z",
							Author:      &drive.User{DisplayName: "Jane Doe"},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(output.Comments) != 2 {
			t.Fatalf("expected 2 comments, got %d", len(output.Comments))
		}

		// Verify comment with anchor
		comment1 := output.Comments[0]
		if comment1.AnchorInfo != anchorJSON {
			t.Errorf("expected anchor info '%s', got '%s'", anchorJSON, comment1.AnchorInfo)
		}

		// Verify comment without anchor
		comment2 := output.Comments[1]
		if comment2.AnchorInfo != "" {
			t.Errorf("expected empty anchor info, got '%s'", comment2.AnchorInfo)
		}
	})

	t.Run("handles pagination", func(t *testing.T) {
		callCount := 0
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				callCount++
				if callCount == 1 {
					return &drive.CommentList{
						Comments: []*drive.Comment{
							{
								Id:          "comment-1",
								Content:     "First page comment",
								Resolved:    false,
								CreatedTime: "2024-01-15T10:00:00Z",
								Author:      &drive.User{DisplayName: "John Doe"},
							},
						},
						NextPageToken: "page2token",
					}, nil
				}
				// Second page
				if pageToken != "page2token" {
					t.Errorf("expected page token 'page2token', got '%s'", pageToken)
				}
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-2",
							Content:     "Second page comment",
							Resolved:    false,
							CreatedTime: "2024-01-14T10:00:00Z",
							Author:      &drive.User{DisplayName: "Jane Doe"},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if callCount != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", callCount)
		}

		if output.TotalCount != 2 {
			t.Errorf("expected TotalCount 2, got %d", output.TotalCount)
		}
	})

	t.Run("returns empty list when no comments", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.TotalCount != 0 {
			t.Errorf("expected TotalCount 0, got %d", output.TotalCount)
		}
		if len(output.Comments) != 0 {
			t.Errorf("expected empty comments slice, got %d comments", len(output.Comments))
		}
	})

	t.Run("returns error for empty presentation ID", func(t *testing.T) {
		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)

		_, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "",
		})

		if err == nil {
			t.Fatal("expected error for empty presentation ID")
		}
		if !errors.Is(err, ErrInvalidPresentationID) {
			t.Errorf("expected ErrInvalidPresentationID, got %v", err)
		}
	})

	t.Run("returns error when presentation not found", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return nil, &notFoundError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "nonexistent-pres",
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
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return nil, &forbiddenError{}
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "private-pres",
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
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return nil, errors.New("internal drive error")
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		_, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err == nil {
			t.Fatal("expected error for drive service failure")
		}
		if !errors.Is(err, ErrDriveAPIError) {
			t.Errorf("expected ErrDriveAPIError, got %v", err)
		}
	})

	t.Run("handles nil author gracefully", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Comment with nil author",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author:      nil, // nil author
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(output.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(output.Comments))
		}

		// Author should be empty but not panic
		if output.Comments[0].Author.DisplayName != "" {
			t.Errorf("expected empty display name for nil author, got '%s'", output.Comments[0].Author.DisplayName)
		}
	})

	t.Run("handles nil comment in list gracefully", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						nil, // nil comment
						{
							Id:          "comment-1",
							Content:     "Valid comment",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author:      &drive.User{DisplayName: "John Doe"},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should skip nil comment
		if len(output.Comments) != 1 {
			t.Errorf("expected 1 comment (nil should be skipped), got %d", len(output.Comments))
		}
	})

	t.Run("handles nil reply in list gracefully", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Comment with replies",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author:      &drive.User{DisplayName: "John Doe"},
							Replies: []*drive.Reply{
								nil, // nil reply
								{
									Id:          "reply-1",
									Content:     "Valid reply",
									CreatedTime: "2024-01-15T11:00:00Z",
									Author:      &drive.User{DisplayName: "Jane Doe"},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(output.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(output.Comments))
		}

		// Should skip nil reply
		if len(output.Comments[0].Replies) != 1 {
			t.Errorf("expected 1 reply (nil should be skipped), got %d", len(output.Comments[0].Replies))
		}
	})

	t.Run("includes HTML content when available", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Plain text content",
							HtmlContent: "<p>HTML <b>content</b></p>",
							Resolved:    false,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author:      &drive.User{DisplayName: "John Doe"},
							Replies: []*drive.Reply{
								{
									Id:          "reply-1",
									Content:     "Reply content",
									HtmlContent: "<p>Reply <i>HTML</i></p>",
									CreatedTime: "2024-01-15T11:00:00Z",
									Author:      &drive.User{DisplayName: "Jane Doe"},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		comment := output.Comments[0]
		if comment.HTMLContent != "<p>HTML <b>content</b></p>" {
			t.Errorf("expected HTML content '<p>HTML <b>content</b></p>', got '%s'", comment.HTMLContent)
		}

		reply := comment.Replies[0]
		if reply.HTMLContent != "<p>Reply <i>HTML</i></p>" {
			t.Errorf("expected reply HTML content '<p>Reply <i>HTML</i></p>', got '%s'", reply.HTMLContent)
		}
	})

	t.Run("includes deleted flag when set", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{
					Comments: []*drive.Comment{
						{
							Id:          "comment-1",
							Content:     "Deleted comment",
							Resolved:    false,
							Deleted:     true,
							CreatedTime: "2024-01-15T10:00:00Z",
							Author:      &drive.User{DisplayName: "John Doe"},
							Replies: []*drive.Reply{
								{
									Id:          "reply-1",
									Content:     "Deleted reply",
									Deleted:     true,
									CreatedTime: "2024-01-15T11:00:00Z",
									Author:      &drive.User{DisplayName: "Jane Doe"},
								},
							},
						},
					},
				}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "pres-123",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		comment := output.Comments[0]
		if !comment.Deleted {
			t.Error("expected Deleted flag to be true for comment")
		}

		reply := comment.Replies[0]
		if !reply.Deleted {
			t.Error("expected Deleted flag to be true for reply")
		}
	})

	t.Run("returns presentation ID in output", func(t *testing.T) {
		mockDrive := &mockDriveService{
			ListCommentsFunc: func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
				return &drive.CommentList{Comments: []*drive.Comment{}}, nil
			},
		}

		tools := NewToolsWithDrive(DefaultToolsConfig(), nil, func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
			return mockDrive, nil
		})

		output, err := tools.ListComments(ctx, nil, ListCommentsInput{
			PresentationID: "my-pres-id",
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output.PresentationID != "my-pres-id" {
			t.Errorf("expected PresentationID 'my-pres-id', got '%s'", output.PresentationID)
		}
	})
}

// Helper error types for testing
type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

type forbiddenError struct{}

func (e *forbiddenError) Error() string { return "forbidden" }
