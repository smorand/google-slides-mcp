package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

// Sentinel errors for search_presentations tool.
var (
	ErrDriveAPIError = errors.New("drive API error")
	ErrInvalidQuery  = errors.New("invalid search query")
)

// Google Slides MIME type constant.
const presentationMimeType = "application/vnd.google-apps.presentation"

// SearchPresentationsInput represents the input for the search_presentations tool.
type SearchPresentationsInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// SearchPresentationsOutput represents the output of the search_presentations tool.
type SearchPresentationsOutput struct {
	Presentations []PresentationResult `json:"presentations"`
	TotalResults  int                  `json:"total_results"`
	Query         string               `json:"query"`
}

// PresentationResult represents a single presentation in search results.
type PresentationResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Owner        string `json:"owner,omitempty"`
	ModifiedDate string `json:"modified_date,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// SearchPresentations searches for Google Slides presentations in Drive.
func (t *Tools) SearchPresentations(ctx context.Context, tokenSource oauth2.TokenSource, input SearchPresentationsInput) (*SearchPresentationsOutput, error) {
	// Validate input
	if input.Query == "" {
		return nil, fmt.Errorf("%w: query is required", ErrInvalidQuery)
	}

	// Set default max results
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}
	// Cap at 100 to prevent excessive results
	if maxResults > 100 {
		maxResults = 100
	}

	t.config.Logger.Info("searching presentations",
		slog.String("query", input.Query),
		slog.Int("max_results", maxResults),
	)

	// Create Drive service
	driveService, err := t.driveServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create drive service: %v", ErrDriveAPIError, err)
	}

	// Build query with mime type filter
	// Support for advanced Drive search operators is preserved by appending to user query
	driveQuery := buildDriveQuery(input.Query)

	// Fields to request from Drive API
	fields := googleapi.Field("files(id,name,owners,modifiedTime,thumbnailLink)")

	// Execute search
	fileList, err := driveService.ListFiles(ctx, driveQuery, int64(maxResults), fields)
	if err != nil {
		if isNotFoundError(err) {
			// No results is not an error
			return &SearchPresentationsOutput{
				Presentations: []PresentationResult{},
				TotalResults:  0,
				Query:         input.Query,
			}, nil
		}
		if isForbiddenError(err) {
			return nil, fmt.Errorf("%w: access denied", ErrAccessDenied)
		}
		return nil, fmt.Errorf("%w: %v", ErrDriveAPIError, err)
	}

	// Transform results
	presentations := make([]PresentationResult, 0, len(fileList.Files))
	for _, file := range fileList.Files {
		result := PresentationResult{
			ID:           file.Id,
			Title:        file.Name,
			ModifiedDate: file.ModifiedTime,
			ThumbnailURL: file.ThumbnailLink,
		}

		// Extract owner email if available
		if len(file.Owners) > 0 && file.Owners[0] != nil {
			result.Owner = file.Owners[0].EmailAddress
		}

		presentations = append(presentations, result)
	}

	output := &SearchPresentationsOutput{
		Presentations: presentations,
		TotalResults:  len(presentations),
		Query:         input.Query,
	}

	t.config.Logger.Info("search completed",
		slog.String("query", input.Query),
		slog.Int("results_count", output.TotalResults),
	)

	return output, nil
}

// buildDriveQuery constructs a Drive API query string from user input.
// It ensures only Google Slides presentations are returned while
// supporting advanced Drive search operators in the user's query.
func buildDriveQuery(userQuery string) string {
	// Always filter by mime type
	mimeFilter := fmt.Sprintf("mimeType='%s'", presentationMimeType)

	// Check if user query already contains a fullText search or other operators
	// If it's a simple query (no operators), wrap in fullText
	if isSimpleQuery(userQuery) {
		// Simple search term - wrap in fullText contains
		return fmt.Sprintf("%s and fullText contains '%s'", mimeFilter, escapeQueryString(userQuery))
	}

	// Advanced query - combine with AND
	// User might be using operators like: name contains, fullText contains, etc.
	return fmt.Sprintf("%s and (%s)", mimeFilter, userQuery)
}

// isSimpleQuery checks if the query is a simple search term (no Drive operators).
func isSimpleQuery(query string) bool {
	// Drive query operators
	operators := []string{
		"fullText contains",
		"name contains",
		"name =",
		"mimeType",
		"modifiedTime",
		"createdTime",
		"trashed",
		"starred",
		"parents",
		"owners",
		"writers",
		"readers",
		"sharedWithMe",
		"properties",
		"appProperties",
		"visibility",
	}

	queryLower := strings.ToLower(query)
	for _, op := range operators {
		if strings.Contains(queryLower, strings.ToLower(op)) {
			return false
		}
	}
	return true
}

// escapeQueryString escapes special characters in the query string.
func escapeQueryString(s string) string {
	// Escape single quotes by doubling them
	return strings.ReplaceAll(s, "'", "\\'")
}
