package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Note: Uses ErrInvalidPresentationID and other sentinel errors from export_pdf.go

// ListSlidesInput represents the input for the list_slides tool.
type ListSlidesInput struct {
	PresentationID    string `json:"presentation_id"`
	IncludeThumbnails bool   `json:"include_thumbnails,omitempty"`
}

// ListSlidesOutput represents the output of the list_slides tool.
type ListSlidesOutput struct {
	PresentationID string           `json:"presentation_id"`
	Title          string           `json:"title"`
	Slides         []SlideListItem  `json:"slides"`
	Statistics     SlidesStatistics `json:"statistics"`
}

// SlideListItem represents metadata about a single slide.
type SlideListItem struct {
	Index           int    `json:"index"`
	SlideID         string `json:"slide_id"`
	Title           string `json:"title,omitempty"`
	LayoutType      string `json:"layout_type,omitempty"`
	ObjectCount     int    `json:"object_count"`
	ThumbnailBase64 string `json:"thumbnail_base64,omitempty"`
}

// SlidesStatistics represents summary statistics about the presentation.
type SlidesStatistics struct {
	TotalSlides      int `json:"total_slides"`
	SlidesWithNotes  int `json:"slides_with_notes"`
	SlidesWithVideos int `json:"slides_with_videos"`
}

// ListSlides lists all slides in a presentation with metadata.
func (t *Tools) ListSlides(ctx context.Context, tokenSource oauth2.TokenSource, input ListSlidesInput) (*ListSlidesOutput, error) {
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	t.config.Logger.Info("listing slides",
		slog.String("presentation_id", input.PresentationID),
		slog.Bool("include_thumbnails", input.IncludeThumbnails),
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

	// Build output
	output := &ListSlidesOutput{
		PresentationID: presentation.PresentationId,
		Title:          presentation.Title,
		Slides:         make([]SlideListItem, len(presentation.Slides)),
		Statistics: SlidesStatistics{
			TotalSlides: len(presentation.Slides),
		},
	}

	// Process each slide
	for i, slide := range presentation.Slides {
		slideItem := SlideListItem{
			Index:       i + 1, // 1-based index
			SlideID:     slide.ObjectId,
			ObjectCount: len(slide.PageElements),
		}

		// Get layout type
		slideItem.LayoutType = getLayoutType(slide, presentation.Layouts)

		// Extract slide title (first title placeholder text)
		slideItem.Title = extractSlideTitle(slide)

		// Check for speaker notes
		if hasSpeakerNotes(slide) {
			output.Statistics.SlidesWithNotes++
		}

		// Check for videos
		if hasVideos(slide.PageElements) {
			output.Statistics.SlidesWithVideos++
		}

		// Get thumbnail if requested
		if input.IncludeThumbnails {
			thumbnail, err := slidesService.GetThumbnail(ctx, input.PresentationID, slide.ObjectId)
			if err == nil && thumbnail != nil {
				thumbnailData, err := fetchThumbnailImage(ctx, thumbnail.ContentUrl)
				if err != nil {
					t.config.Logger.Warn("failed to fetch thumbnail",
						slog.Int("slide", i+1),
						slog.Any("error", err),
					)
				} else {
					slideItem.ThumbnailBase64 = base64.StdEncoding.EncodeToString(thumbnailData)
				}
			} else if err != nil {
				t.config.Logger.Warn("failed to get thumbnail",
					slog.Int("slide", i+1),
					slog.Any("error", err),
				)
			}
		}

		output.Slides[i] = slideItem
	}

	t.config.Logger.Info("slides listed successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("total_slides", output.Statistics.TotalSlides),
	)

	return output, nil
}

// getLayoutType determines the layout type for a slide.
func getLayoutType(slide *slides.Page, layouts []*slides.Page) string {
	if slide.SlideProperties == nil || slide.SlideProperties.LayoutObjectId == "" {
		return ""
	}

	// Find the layout and return its type
	for _, layout := range layouts {
		if layout.ObjectId == slide.SlideProperties.LayoutObjectId {
			if layout.LayoutProperties != nil {
				// Return the layout name (e.g., TITLE, TITLE_AND_BODY, BLANK)
				if layout.LayoutProperties.Name != "" {
					return layout.LayoutProperties.Name
				}
				// Fallback to display name
				if layout.LayoutProperties.DisplayName != "" {
					return layout.LayoutProperties.DisplayName
				}
			}
			break
		}
	}

	return ""
}

// extractSlideTitle extracts the title text from a slide.
func extractSlideTitle(slide *slides.Page) string {
	if slide == nil || len(slide.PageElements) == 0 {
		return ""
	}

	// Look for title placeholder
	for _, element := range slide.PageElements {
		if element.Shape != nil && element.Shape.Placeholder != nil {
			// Check if this is a title placeholder
			placeholderType := element.Shape.Placeholder.Type
			if placeholderType == "TITLE" || placeholderType == "CENTERED_TITLE" {
				if element.Shape.Text != nil {
					return extractTextFromTextContent(element.Shape.Text)
				}
			}
		}
	}

	return ""
}

// hasSpeakerNotes checks if a slide has speaker notes.
func hasSpeakerNotes(slide *slides.Page) bool {
	if slide == nil || slide.SlideProperties == nil {
		return false
	}

	notesPage := slide.SlideProperties.NotesPage
	if notesPage == nil || len(notesPage.PageElements) == 0 {
		return false
	}

	// Check for any non-empty text in the notes page
	for _, element := range notesPage.PageElements {
		if element.Shape != nil && element.Shape.Text != nil {
			text := extractTextFromTextContent(element.Shape.Text)
			if text != "" {
				return true
			}
		}
	}

	return false
}

// hasVideos checks if any page elements are videos.
func hasVideos(elements []*slides.PageElement) bool {
	for _, element := range elements {
		if element == nil {
			continue
		}
		if element.Video != nil {
			return true
		}
		// Check groups recursively
		if element.ElementGroup != nil {
			if hasVideos(element.ElementGroup.Children) {
				return true
			}
		}
	}
	return false
}
