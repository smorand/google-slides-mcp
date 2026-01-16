package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

// Sentinel errors for configure_footer tool.
var (
	ErrConfigureFooterFailed = errors.New("failed to configure footer")
	ErrInvalidApplyTo        = errors.New("invalid apply_to value")
	ErrNoFooterChanges       = errors.New("no footer changes specified")
	ErrNoFooterPlaceholders  = errors.New("no footer placeholders found in presentation")
)

// ConfigureFooterInput represents the input for the configure_footer tool.
type ConfigureFooterInput struct {
	PresentationID  string  `json:"presentation_id"`
	ShowSlideNumber *bool   `json:"show_slide_number,omitempty"` // Enable/disable slide numbers
	ShowDate        *bool   `json:"show_date,omitempty"`         // Enable/disable date display
	DateFormat      string  `json:"date_format,omitempty"`       // Date format (e.g., "2006-01-02", "January 2, 2006")
	FooterText      *string `json:"footer_text,omitempty"`       // Custom footer text (nil = don't change, "" = clear)
	ApplyTo         string  `json:"apply_to,omitempty"`          // "all" | "title_slides_only" | "exclude_title_slides"
}

// ConfigureFooterOutput represents the output of the configure_footer tool.
type ConfigureFooterOutput struct {
	Success              bool     `json:"success"`
	Message              string   `json:"message"`
	UpdatedSlideNumbers  int      `json:"updated_slide_numbers,omitempty"`
	UpdatedDates         int      `json:"updated_dates,omitempty"`
	UpdatedFooters       int      `json:"updated_footers,omitempty"`
	AffectedSlideIDs     []string `json:"affected_slide_ids,omitempty"`
	AppliedTo            string   `json:"applied_to"`
}

// footerPlaceholderInfo holds information about a footer placeholder.
type footerPlaceholderInfo struct {
	ObjectID        string
	PlaceholderType string
	PageObjectID    string   // The slide/master/layout this placeholder is on
	PageType        string   // "slide", "master", "layout"
	IsTitleSlide    bool     // Whether the parent slide uses a title layout
}

// ConfigureFooter configures the footer elements in a presentation.
// This includes slide numbers, date, and custom footer text.
//
// Note: The Google Slides API has limitations regarding footer configuration:
// - Footer placeholders must exist in the master/layout (they cannot be created via API)
// - "Showing" or "hiding" works by modifying text content of placeholders
// - For slides that don't have footer placeholders, this tool cannot add them
func (t *Tools) ConfigureFooter(ctx context.Context, tokenSource oauth2.TokenSource, input ConfigureFooterInput) (*ConfigureFooterOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}

	// Validate apply_to
	applyTo := strings.ToLower(strings.TrimSpace(input.ApplyTo))
	if applyTo == "" {
		applyTo = "all" // Default to all slides
	}
	validApplyTo := map[string]bool{
		"all":                  true,
		"title_slides_only":    true,
		"exclude_title_slides": true,
	}
	if !validApplyTo[applyTo] {
		return nil, fmt.Errorf("%w: must be 'all', 'title_slides_only', or 'exclude_title_slides'", ErrInvalidApplyTo)
	}

	// Check if any changes are requested
	if input.ShowSlideNumber == nil && input.ShowDate == nil && input.FooterText == nil {
		return nil, fmt.Errorf("%w: provide at least one of show_slide_number, show_date, or footer_text", ErrNoFooterChanges)
	}

	t.config.Logger.Info("configuring footer",
		slog.String("presentation_id", input.PresentationID),
		slog.String("apply_to", applyTo),
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

	// Find all footer placeholders in the presentation
	placeholders := t.findFooterPlaceholders(presentation, applyTo)

	if len(placeholders) == 0 {
		return nil, fmt.Errorf("%w: the presentation may not have footer placeholders in its master/layout, or no slides match the apply_to criteria", ErrNoFooterPlaceholders)
	}

	// Build requests to update the placeholders
	requests, stats := t.buildFooterUpdateRequests(placeholders, input)

	if len(requests) == 0 {
		return &ConfigureFooterOutput{
			Success:   true,
			Message:   "No placeholders needed updating based on the provided options",
			AppliedTo: applyTo,
		}, nil
	}

	// Execute batch update
	_, err = slidesService.BatchUpdate(ctx, input.PresentationID, requests)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: %v", ErrConfigureFooterFailed, err)
	}

	// Build success message
	var messageParts []string
	if stats.slideNumbers > 0 {
		messageParts = append(messageParts, fmt.Sprintf("slide numbers: %d", stats.slideNumbers))
	}
	if stats.dates > 0 {
		messageParts = append(messageParts, fmt.Sprintf("dates: %d", stats.dates))
	}
	if stats.footers > 0 {
		messageParts = append(messageParts, fmt.Sprintf("footers: %d", stats.footers))
	}
	message := fmt.Sprintf("Footer configuration updated successfully (%s)", strings.Join(messageParts, ", "))

	output := &ConfigureFooterOutput{
		Success:             true,
		Message:             message,
		UpdatedSlideNumbers: stats.slideNumbers,
		UpdatedDates:        stats.dates,
		UpdatedFooters:      stats.footers,
		AffectedSlideIDs:    stats.affectedSlideIDs,
		AppliedTo:           applyTo,
	}

	t.config.Logger.Info("footer configured successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("slide_numbers", stats.slideNumbers),
		slog.Int("dates", stats.dates),
		slog.Int("footers", stats.footers),
	)

	return output, nil
}

// findFooterPlaceholders finds all footer-related placeholders in the presentation.
func (t *Tools) findFooterPlaceholders(presentation *slides.Presentation, applyTo string) []footerPlaceholderInfo {
	var placeholders []footerPlaceholderInfo

	// Build a map of layout IDs to whether they are title layouts
	titleLayoutIDs := make(map[string]bool)
	for _, layout := range presentation.Layouts {
		if layout.LayoutProperties != nil {
			layoutName := strings.ToUpper(layout.LayoutProperties.Name)
			displayName := strings.ToUpper(layout.LayoutProperties.DisplayName)
			// Check if this is a title layout (full-slide title layouts only, not content layouts)
			// Title layouts are: TITLE, TITLE_SLIDE, SECTION_HEADER
			// NOT title layouts: TITLE_AND_BODY, TITLE_AND_TWO_COLUMNS, TITLE_ONLY, etc.
			isTitleLayout := layoutName == "TITLE" ||
				layoutName == "TITLE_SLIDE" ||
				layoutName == "SECTION_HEADER" ||
				displayName == "TITLE" ||
				displayName == "TITLE_SLIDE" ||
				displayName == "SECTION_HEADER"
			titleLayoutIDs[layout.ObjectId] = isTitleLayout
		}
	}

	// Helper to check if a slide is a title slide
	isTitleSlide := func(slide *slides.Page) bool {
		if slide.SlideProperties == nil || slide.SlideProperties.LayoutObjectId == "" {
			return false
		}
		return titleLayoutIDs[slide.SlideProperties.LayoutObjectId]
	}

	// Helper to check if we should process this slide based on applyTo
	shouldProcessSlide := func(slide *slides.Page) bool {
		isTitle := isTitleSlide(slide)
		switch applyTo {
		case "title_slides_only":
			return isTitle
		case "exclude_title_slides":
			return !isTitle
		default: // "all"
			return true
		}
	}

	// Find placeholders on slides
	for _, slide := range presentation.Slides {
		if !shouldProcessSlide(slide) {
			continue
		}

		for _, element := range slide.PageElements {
			if element.Shape != nil && element.Shape.Placeholder != nil {
				placeholderType := element.Shape.Placeholder.Type
				if isFooterPlaceholderType(placeholderType) {
					placeholders = append(placeholders, footerPlaceholderInfo{
						ObjectID:        element.ObjectId,
						PlaceholderType: placeholderType,
						PageObjectID:    slide.ObjectId,
						PageType:        "slide",
						IsTitleSlide:    isTitleSlide(slide),
					})
				}
			}
		}
	}

	// If no placeholders found on slides, also check masters and layouts
	// This is useful because some presentations may have footer placeholders only on masters
	if len(placeholders) == 0 {
		// Check masters
		for _, master := range presentation.Masters {
			for _, element := range master.PageElements {
				if element.Shape != nil && element.Shape.Placeholder != nil {
					placeholderType := element.Shape.Placeholder.Type
					if isFooterPlaceholderType(placeholderType) {
						placeholders = append(placeholders, footerPlaceholderInfo{
							ObjectID:        element.ObjectId,
							PlaceholderType: placeholderType,
							PageObjectID:    master.ObjectId,
							PageType:        "master",
							IsTitleSlide:    false,
						})
					}
				}
			}
		}

		// Check layouts
		for _, layout := range presentation.Layouts {
			for _, element := range layout.PageElements {
				if element.Shape != nil && element.Shape.Placeholder != nil {
					placeholderType := element.Shape.Placeholder.Type
					if isFooterPlaceholderType(placeholderType) {
						placeholders = append(placeholders, footerPlaceholderInfo{
							ObjectID:        element.ObjectId,
							PlaceholderType: placeholderType,
							PageObjectID:    layout.ObjectId,
							PageType:        "layout",
							IsTitleSlide:    titleLayoutIDs[layout.ObjectId],
						})
					}
				}
			}
		}
	}

	return placeholders
}

// isFooterPlaceholderType checks if the placeholder type is a footer-related type.
func isFooterPlaceholderType(placeholderType string) bool {
	return placeholderType == "FOOTER" ||
		placeholderType == "SLIDE_NUMBER" ||
		placeholderType == "DATE_AND_TIME"
}

// footerUpdateStats tracks what was updated.
type footerUpdateStats struct {
	slideNumbers     int
	dates            int
	footers          int
	affectedSlideIDs []string
}

// buildFooterUpdateRequests creates batch update requests for footer placeholders.
func (t *Tools) buildFooterUpdateRequests(placeholders []footerPlaceholderInfo, input ConfigureFooterInput) ([]*slides.Request, footerUpdateStats) {
	var requests []*slides.Request
	var stats footerUpdateStats
	affectedSlides := make(map[string]bool)

	for _, placeholder := range placeholders {
		var textToSet string
		var shouldUpdate bool

		switch placeholder.PlaceholderType {
		case "SLIDE_NUMBER":
			if input.ShowSlideNumber != nil {
				shouldUpdate = true
				if *input.ShowSlideNumber {
					// To show slide numbers, we use a special placeholder marker
					// The actual slide number is rendered by Google Slides
					// We just need to ensure the placeholder has content
					textToSet = "#"  // Special marker that Slides interprets as slide number
				} else {
					// To hide, we clear the text
					textToSet = ""
				}
				stats.slideNumbers++
			}

		case "DATE_AND_TIME":
			if input.ShowDate != nil {
				shouldUpdate = true
				if *input.ShowDate {
					// Format the date
					dateFormat := input.DateFormat
					if dateFormat == "" {
						dateFormat = "January 2, 2006" // Default format
					}
					textToSet = time.Now().Format(dateFormat)
				} else {
					// To hide, we clear the text
					textToSet = ""
				}
				stats.dates++
			}

		case "FOOTER":
			if input.FooterText != nil {
				shouldUpdate = true
				textToSet = *input.FooterText
				stats.footers++
			}
		}

		if shouldUpdate {
			// Build requests to update the placeholder text
			// First delete all existing text, then insert new text
			requests = append(requests, &slides.Request{
				DeleteText: &slides.DeleteTextRequest{
					ObjectId: placeholder.ObjectID,
					TextRange: &slides.Range{
						Type: "ALL",
					},
				},
			})

			if textToSet != "" {
				requests = append(requests, &slides.Request{
					InsertText: &slides.InsertTextRequest{
						ObjectId:       placeholder.ObjectID,
						InsertionIndex: 0,
						Text:           textToSet,
					},
				})
			}

			if placeholder.PageType == "slide" {
				affectedSlides[placeholder.PageObjectID] = true
			}
		}
	}

	// Convert affected slides map to slice
	for slideID := range affectedSlides {
		stats.affectedSlideIDs = append(stats.affectedSlideIDs, slideID)
	}

	return requests, stats
}
