package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for search_text tool.
var (
	ErrSearchTextFailed = errors.New("failed to search text")
	// ErrInvalidQuery is declared in search_presentations.go
)

// SearchTextInput represents the input for the search_text tool.
type SearchTextInput struct {
	PresentationID string `json:"presentation_id"`
	Query          string `json:"query"`
	CaseSensitive  bool   `json:"case_sensitive,omitempty"` // Default: false
}

// SearchTextOutput represents the output of the search_text tool.
type SearchTextOutput struct {
	PresentationID string             `json:"presentation_id"`
	Query          string             `json:"query"`
	CaseSensitive  bool               `json:"case_sensitive"`
	TotalMatches   int                `json:"total_matches"`
	Results        []SearchTextResult `json:"results"`
}

// SearchTextResult represents a search result grouped by slide.
type SearchTextResult struct {
	SlideIndex int           `json:"slide_index"` // 1-based
	SlideID    string        `json:"slide_id"`
	Matches    []TextMatch   `json:"matches"`
}

// TextMatch represents a single text match within an object.
type TextMatch struct {
	ObjectID    string `json:"object_id"`
	ObjectType  string `json:"object_type"`
	StartIndex  int    `json:"start_index"`
	TextContext string `json:"text_context"` // Surrounding text (50 chars before/after)
}

// SearchText searches for text across all slides in a presentation.
func (t *Tools) SearchText(ctx context.Context, tokenSource oauth2.TokenSource, input SearchTextInput) (*SearchTextOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.Query == "" {
		return nil, fmt.Errorf("%w: query is required", ErrInvalidQuery)
	}

	t.config.Logger.Info("searching text in presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.String("query", input.Query),
		slog.Bool("case_sensitive", input.CaseSensitive),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get the presentation
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

	// Search through all slides
	var results []SearchTextResult
	totalMatches := 0

	for slideIdx, slide := range presentation.Slides {
		slideMatches := searchInSlide(slide, input.Query, input.CaseSensitive)
		if len(slideMatches) > 0 {
			results = append(results, SearchTextResult{
				SlideIndex: slideIdx + 1, // 1-based
				SlideID:    slide.ObjectId,
				Matches:    slideMatches,
			})
			totalMatches += len(slideMatches)
		}
	}

	output := &SearchTextOutput{
		PresentationID: input.PresentationID,
		Query:          input.Query,
		CaseSensitive:  input.CaseSensitive,
		TotalMatches:   totalMatches,
		Results:        results,
	}

	t.config.Logger.Info("text search completed",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("total_matches", totalMatches),
		slog.Int("slides_with_matches", len(results)),
	)

	return output, nil
}

// searchInSlide searches for text in a slide and returns all matches.
func searchInSlide(slide *slides.Page, query string, caseSensitive bool) []TextMatch {
	var matches []TextMatch

	if slide == nil {
		return matches
	}

	// Search in all page elements
	matches = append(matches, searchInElements(slide.PageElements, query, caseSensitive)...)

	// Search in speaker notes
	if slide.SlideProperties != nil && slide.SlideProperties.NotesPage != nil {
		noteMatches := searchInElements(slide.SlideProperties.NotesPage.PageElements, query, caseSensitive)
		// Mark these as from speaker notes by prefixing object type
		for i := range noteMatches {
			noteMatches[i].ObjectType = "SPEAKER_NOTES:" + noteMatches[i].ObjectType
		}
		matches = append(matches, noteMatches...)
	}

	return matches
}

// searchInElements searches for text in a list of page elements.
func searchInElements(elements []*slides.PageElement, query string, caseSensitive bool) []TextMatch {
	var matches []TextMatch

	for _, element := range elements {
		if element == nil {
			continue
		}

		// Search in shape text
		if element.Shape != nil && element.Shape.Text != nil {
			text := extractTextFromTextContent(element.Shape.Text)
			elementMatches := findMatchesInText(text, query, caseSensitive)
			for _, match := range elementMatches {
				matches = append(matches, TextMatch{
					ObjectID:    element.ObjectId,
					ObjectType:  determineObjectType(element),
					StartIndex:  match.startIndex,
					TextContext: match.context,
				})
			}
		}

		// Search in table cells
		if element.Table != nil {
			tableMatches := searchInTable(element.Table, element.ObjectId, query, caseSensitive)
			matches = append(matches, tableMatches...)
		}

		// Search in groups recursively
		if element.ElementGroup != nil {
			groupMatches := searchInElements(element.ElementGroup.Children, query, caseSensitive)
			matches = append(matches, groupMatches...)
		}
	}

	return matches
}

// searchInTable searches for text in all cells of a table.
func searchInTable(table *slides.Table, tableID string, query string, caseSensitive bool) []TextMatch {
	var matches []TextMatch

	if table == nil {
		return matches
	}

	for rowIdx, row := range table.TableRows {
		if row == nil {
			continue
		}
		for colIdx, cell := range row.TableCells {
			if cell == nil || cell.Text == nil {
				continue
			}
			text := extractTextFromTextContent(cell.Text)
			cellMatches := findMatchesInText(text, query, caseSensitive)
			for _, match := range cellMatches {
				matches = append(matches, TextMatch{
					ObjectID:    fmt.Sprintf("%s[%d,%d]", tableID, rowIdx, colIdx),
					ObjectType:  "TABLE_CELL",
					StartIndex:  match.startIndex,
					TextContext: match.context,
				})
			}
		}
	}

	return matches
}

// matchInfo holds information about a text match.
type matchInfo struct {
	startIndex int
	context    string
}

// findMatchesInText finds all occurrences of query in text.
func findMatchesInText(text, query string, caseSensitive bool) []matchInfo {
	var matches []matchInfo

	if text == "" || query == "" {
		return matches
	}

	searchText := text
	searchQuery := query
	if !caseSensitive {
		searchText = strings.ToLower(text)
		searchQuery = strings.ToLower(query)
	}

	// Find all occurrences
	startPos := 0
	for {
		idx := strings.Index(searchText[startPos:], searchQuery)
		if idx == -1 {
			break
		}

		actualIdx := startPos + idx
		context := extractContext(text, actualIdx, len(query), 50)

		matches = append(matches, matchInfo{
			startIndex: actualIdx,
			context:    context,
		})

		startPos = actualIdx + 1 // Move past this match to find overlapping matches
	}

	return matches
}

// extractContext extracts surrounding text (contextChars before and after) around a match.
func extractContext(text string, matchStart, matchLen, contextChars int) string {
	textLen := len(text)

	// Calculate context boundaries
	contextStart := matchStart - contextChars
	if contextStart < 0 {
		contextStart = 0
	}

	contextEnd := matchStart + matchLen + contextChars
	if contextEnd > textLen {
		contextEnd = textLen
	}

	// Extract the context
	context := text[contextStart:contextEnd]

	// Add ellipsis indicators if context is truncated
	prefix := ""
	suffix := ""
	if contextStart > 0 {
		prefix = "..."
	}
	if contextEnd < textLen {
		suffix = "..."
	}

	return prefix + context + suffix
}
