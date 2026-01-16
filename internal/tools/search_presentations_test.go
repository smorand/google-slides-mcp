package tools

import (
	"context"
	"errors"
	"io"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// mockDriveService implements DriveService for testing.
type mockDriveService struct {
	ListFilesFunc      func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error)
	CopyFileFunc       func(ctx context.Context, fileID string, file *drive.File) (*drive.File, error)
	ExportFileFunc     func(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error)
	MoveFileFunc       func(ctx context.Context, fileID string, folderID string) error
	UploadFileFunc     func(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error)
	MakeFilePublicFunc func(ctx context.Context, fileID string) error
	ListCommentsFunc   func(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error)
	CreateCommentFunc  func(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error)
	CreateReplyFunc    func(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error)
	UpdateCommentFunc  func(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error)
	DeleteCommentFunc  func(ctx context.Context, fileID, commentID string) error
}

func (m *mockDriveService) ListFiles(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
	if m.ListFilesFunc != nil {
		return m.ListFilesFunc(ctx, query, pageSize, fields)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) CopyFile(ctx context.Context, fileID string, file *drive.File) (*drive.File, error) {
	if m.CopyFileFunc != nil {
		return m.CopyFileFunc(ctx, fileID, file)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) ExportFile(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
	if m.ExportFileFunc != nil {
		return m.ExportFileFunc(ctx, fileID, mimeType)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) MoveFile(ctx context.Context, fileID string, folderID string) error {
	if m.MoveFileFunc != nil {
		return m.MoveFileFunc(ctx, fileID, folderID)
	}
	return errors.New("not implemented")
}

func (m *mockDriveService) UploadFile(ctx context.Context, name, mimeType string, content io.Reader) (*drive.File, error) {
	if m.UploadFileFunc != nil {
		return m.UploadFileFunc(ctx, name, mimeType, content)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) MakeFilePublic(ctx context.Context, fileID string) error {
	if m.MakeFilePublicFunc != nil {
		return m.MakeFilePublicFunc(ctx, fileID)
	}
	return nil // Default to success for tests that don't care about this
}

func (m *mockDriveService) ListComments(ctx context.Context, fileID string, includeDeleted bool, pageSize int64, pageToken string) (*drive.CommentList, error) {
	if m.ListCommentsFunc != nil {
		return m.ListCommentsFunc(ctx, fileID, includeDeleted, pageSize, pageToken)
	}
	return &drive.CommentList{Comments: []*drive.Comment{}}, nil // Default to empty list
}

func (m *mockDriveService) CreateComment(ctx context.Context, fileID string, comment *drive.Comment) (*drive.Comment, error) {
	if m.CreateCommentFunc != nil {
		return m.CreateCommentFunc(ctx, fileID, comment)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) CreateReply(ctx context.Context, fileID, commentID string, reply *drive.Reply) (*drive.Reply, error) {
	if m.CreateReplyFunc != nil {
		return m.CreateReplyFunc(ctx, fileID, commentID, reply)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) UpdateComment(ctx context.Context, fileID, commentID string, comment *drive.Comment) (*drive.Comment, error) {
	if m.UpdateCommentFunc != nil {
		return m.UpdateCommentFunc(ctx, fileID, commentID, comment)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDriveService) DeleteComment(ctx context.Context, fileID, commentID string) error {
	if m.DeleteCommentFunc != nil {
		return m.DeleteCommentFunc(ctx, fileID, commentID)
	}
	return errors.New("not implemented")
}

func TestSearchPresentations_Success(t *testing.T) {
	mockService := &mockDriveService{
		ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
			// Verify query contains mime type filter
			if query == "" || !contains(query, "mimeType='application/vnd.google-apps.presentation'") {
				t.Errorf("expected query to contain mime type filter, got: %s", query)
			}
			// Verify page size
			if pageSize != 10 {
				t.Errorf("expected page size 10, got: %d", pageSize)
			}

			return &drive.FileList{
				Files: []*drive.File{
					{
						Id:           "presentation-1",
						Name:         "Test Presentation 1",
						ModifiedTime: "2024-01-15T10:30:00Z",
						ThumbnailLink: "https://drive.google.com/thumbnail/1",
						Owners: []*drive.User{
							{EmailAddress: "user@example.com"},
						},
					},
					{
						Id:           "presentation-2",
						Name:         "Test Presentation 2",
						ModifiedTime: "2024-01-14T09:00:00Z",
						Owners: []*drive.User{
							{EmailAddress: "other@example.com"},
						},
					},
				},
			}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.TotalResults != 2 {
		t.Errorf("expected 2 results, got %d", output.TotalResults)
	}

	if output.Query != "test" {
		t.Errorf("expected query 'test', got '%s'", output.Query)
	}

	// Verify first result
	if len(output.Presentations) < 1 {
		t.Fatal("expected at least 1 presentation")
	}
	p1 := output.Presentations[0]
	if p1.ID != "presentation-1" {
		t.Errorf("expected ID 'presentation-1', got '%s'", p1.ID)
	}
	if p1.Title != "Test Presentation 1" {
		t.Errorf("expected title 'Test Presentation 1', got '%s'", p1.Title)
	}
	if p1.Owner != "user@example.com" {
		t.Errorf("expected owner 'user@example.com', got '%s'", p1.Owner)
	}
	if p1.ModifiedDate != "2024-01-15T10:30:00Z" {
		t.Errorf("expected modified date '2024-01-15T10:30:00Z', got '%s'", p1.ModifiedDate)
	}
	if p1.ThumbnailURL != "https://drive.google.com/thumbnail/1" {
		t.Errorf("expected thumbnail URL, got '%s'", p1.ThumbnailURL)
	}
}

func TestSearchPresentations_EmptyQuery(t *testing.T) {
	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "",
	})

	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !errors.Is(err, ErrInvalidQuery) {
		t.Errorf("expected ErrInvalidQuery, got: %v", err)
	}
}

func TestSearchPresentations_NoResults(t *testing.T) {
	mockService := &mockDriveService{
		ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
			return &drive.FileList{
				Files: []*drive.File{},
			}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "nonexistent presentation xyz123",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.TotalResults != 0 {
		t.Errorf("expected 0 results, got %d", output.TotalResults)
	}
	if len(output.Presentations) != 0 {
		t.Errorf("expected empty presentations array, got %d", len(output.Presentations))
	}
}

func TestSearchPresentations_MaxResults(t *testing.T) {
	testCases := []struct {
		name           string
		inputMaxResults int
		expectedPageSize int64
	}{
		{
			name:           "default max results",
			inputMaxResults: 0,
			expectedPageSize: 10,
		},
		{
			name:           "custom max results",
			inputMaxResults: 25,
			expectedPageSize: 25,
		},
		{
			name:           "negative max results defaults to 10",
			inputMaxResults: -5,
			expectedPageSize: 10,
		},
		{
			name:           "large max results capped at 100",
			inputMaxResults: 500,
			expectedPageSize: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService := &mockDriveService{
				ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
					if pageSize != tc.expectedPageSize {
						t.Errorf("expected page size %d, got %d", tc.expectedPageSize, pageSize)
					}
					return &drive.FileList{Files: []*drive.File{}}, nil
				},
			}

			driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
				return mockService, nil
			}

			tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
			tokenSource := &mockTokenSource{}

			_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
				Query:      "test",
				MaxResults: tc.inputMaxResults,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSearchPresentations_OnlyReturnsSlides(t *testing.T) {
	var capturedQuery string

	mockService := &mockDriveService{
		ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
			capturedQuery = query
			return &drive.FileList{Files: []*drive.File{}}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "quarterly report",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the query filters by presentation mime type
	if !contains(capturedQuery, "mimeType='application/vnd.google-apps.presentation'") {
		t.Errorf("query should filter by presentation mime type, got: %s", capturedQuery)
	}
}

func TestSearchPresentations_AdvancedQuery(t *testing.T) {
	testCases := []struct {
		name          string
		query         string
		expectedContains string
	}{
		{
			name:          "simple query gets wrapped in fullText",
			query:         "quarterly report",
			expectedContains: "fullText contains",
		},
		{
			name:          "name contains query preserved",
			query:         "name contains 'Q4'",
			expectedContains: "name contains 'Q4'",
		},
		{
			name:          "fullText query preserved",
			query:         "fullText contains 'budget'",
			expectedContains: "fullText contains 'budget'",
		},
		{
			name:          "modifiedTime query preserved",
			query:         "modifiedTime > '2024-01-01'",
			expectedContains: "modifiedTime > '2024-01-01'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedQuery string

			mockService := &mockDriveService{
				ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
					capturedQuery = query
					return &drive.FileList{Files: []*drive.File{}}, nil
				},
			}

			driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
				return mockService, nil
			}

			tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
			tokenSource := &mockTokenSource{}

			_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
				Query: tc.query,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !contains(capturedQuery, tc.expectedContains) {
				t.Errorf("expected query to contain '%s', got: %s", tc.expectedContains, capturedQuery)
			}
		})
	}
}

func TestSearchPresentations_SharedPresentations(t *testing.T) {
	mockService := &mockDriveService{
		ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
			// Return a mix of owned and shared presentations
			return &drive.FileList{
				Files: []*drive.File{
					{
						Id:   "owned-1",
						Name: "My Presentation",
						Owners: []*drive.User{
							{EmailAddress: "me@example.com"},
						},
					},
					{
						Id:   "shared-1",
						Name: "Shared With Me",
						Owners: []*drive.User{
							{EmailAddress: "colleague@example.com"},
						},
					},
					{
						Id:   "team-drive-1",
						Name: "Team Presentation",
						// Team drive files may not have owners
						Owners: []*drive.User{},
					},
				},
			}, nil
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	output, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "presentation",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.TotalResults != 3 {
		t.Errorf("expected 3 results (owned + shared + team drive), got %d", output.TotalResults)
	}

	// Verify team drive presentation has empty owner
	teamPresentation := output.Presentations[2]
	if teamPresentation.Owner != "" {
		t.Errorf("expected empty owner for team drive presentation, got '%s'", teamPresentation.Owner)
	}
}

func TestSearchPresentations_DriveAPIError(t *testing.T) {
	mockService := &mockDriveService{
		ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
			return nil, errors.New("googleapi: Error 500: internal server error")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "test",
	})

	if err == nil {
		t.Fatal("expected error for Drive API error")
	}
	if !errors.Is(err, ErrDriveAPIError) {
		t.Errorf("expected ErrDriveAPIError, got: %v", err)
	}
}

func TestSearchPresentations_AccessDenied(t *testing.T) {
	mockService := &mockDriveService{
		ListFilesFunc: func(ctx context.Context, query string, pageSize int64, fields googleapi.Field) (*drive.FileList, error) {
			return nil, errors.New("googleapi: Error 403: forbidden")
		},
	}

	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return mockService, nil
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "test",
	})

	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestSearchPresentations_ServiceFactoryError(t *testing.T) {
	driveFactory := func(ctx context.Context, ts oauth2.TokenSource) (DriveService, error) {
		return nil, errors.New("failed to create drive service")
	}

	tools := NewToolsWithDrive(DefaultToolsConfig(), nil, driveFactory)
	tokenSource := &mockTokenSource{}

	_, err := tools.SearchPresentations(context.Background(), tokenSource, SearchPresentationsInput{
		Query: "test",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrDriveAPIError) {
		t.Errorf("expected ErrDriveAPIError, got: %v", err)
	}
}

func TestBuildDriveQuery(t *testing.T) {
	testCases := []struct {
		name           string
		userQuery      string
		wantContains   []string
		wantNotContains []string
	}{
		{
			name:      "simple query",
			userQuery: "quarterly report",
			wantContains: []string{
				"mimeType='application/vnd.google-apps.presentation'",
				"fullText contains 'quarterly report'",
			},
		},
		{
			name:      "query with single quote escaped",
			userQuery: "John's presentation",
			wantContains: []string{
				"mimeType='application/vnd.google-apps.presentation'",
				"fullText contains 'John\\'s presentation'",
			},
		},
		{
			name:      "advanced query with name contains",
			userQuery: "name contains 'Budget'",
			wantContains: []string{
				"mimeType='application/vnd.google-apps.presentation'",
				"name contains 'Budget'",
			},
			wantNotContains: []string{
				"fullText contains 'name contains",
			},
		},
		{
			name:      "advanced query with modifiedTime",
			userQuery: "modifiedTime > '2024-01-01'",
			wantContains: []string{
				"mimeType='application/vnd.google-apps.presentation'",
				"modifiedTime > '2024-01-01'",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildDriveQuery(tc.userQuery)

			for _, want := range tc.wantContains {
				if !contains(result, want) {
					t.Errorf("expected query to contain '%s', got: %s", want, result)
				}
			}

			for _, notWant := range tc.wantNotContains {
				if contains(result, notWant) {
					t.Errorf("expected query NOT to contain '%s', got: %s", notWant, result)
				}
			}
		})
	}
}

func TestIsSimpleQuery(t *testing.T) {
	testCases := []struct {
		query    string
		expected bool
	}{
		{"quarterly report", true},
		{"hello world", true},
		{"single", true},
		{"name contains 'test'", false},
		{"fullText contains 'budget'", false},
		{"modifiedTime > '2024-01-01'", false},
		{"trashed = false", false},
		{"starred = true", false},
		{"mimeType = 'something'", false},
		{"sharedWithMe = true", false},
		{"owners in 'user@example.com'", false},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			result := isSimpleQuery(tc.query)
			if result != tc.expected {
				t.Errorf("isSimpleQuery(%q) = %v, expected %v", tc.query, result, tc.expected)
			}
		})
	}
}

func TestEscapeQueryString(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"John's", "John\\'s"},
		{"it's a test", "it\\'s a test"},
		{"multiple 'quotes' here", "multiple \\'quotes\\' here"},
		{"no special chars", "no special chars"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := escapeQueryString(tc.input)
			if result != tc.expected {
				t.Errorf("escapeQueryString(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
