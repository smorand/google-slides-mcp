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

// Sentinel errors for apply_theme tool.
var (
	ErrApplyThemeFailed     = errors.New("failed to apply theme")
	ErrInvalidThemeSource   = errors.New("invalid theme source")
	ErrGalleryNotSupported  = errors.New("gallery themes are not supported by the API")
	ErrNoMasterInSource     = errors.New("no master slides found in source presentation")
	ErrNoMasterInTarget     = errors.New("no master slides found in target presentation")
	ErrNoColorScheme        = errors.New("no color scheme found in source presentation")
	ErrInvalidSourcePresID  = errors.New("source presentation ID is required")
)

// ApplyThemeInput represents the input for the apply_theme tool.
type ApplyThemeInput struct {
	PresentationID         string `json:"presentation_id"`          // Target presentation
	ThemeSource            string `json:"theme_source"`             // "gallery" or "presentation"
	ThemeID                string `json:"theme_id,omitempty"`       // Gallery theme ID (not supported)
	SourcePresentationID   string `json:"source_presentation_id,omitempty"` // Source presentation for copying theme
}

// ApplyThemeOutput represents the output of the apply_theme tool.
type ApplyThemeOutput struct {
	Success           bool     `json:"success"`
	Message           string   `json:"message"`
	UpdatedProperties []string `json:"updated_properties,omitempty"`
	SourceMasterID    string   `json:"source_master_id,omitempty"`
	TargetMasterID    string   `json:"target_master_id,omitempty"`
}

// themeColorTypes are the first 12 ThemeColorTypes that can be edited.
// Only these colors can be updated via the API.
var themeColorTypes = []string{
	"DARK1",
	"LIGHT1",
	"DARK2",
	"LIGHT2",
	"ACCENT1",
	"ACCENT2",
	"ACCENT3",
	"ACCENT4",
	"ACCENT5",
	"ACCENT6",
	"HYPERLINK",
	"FOLLOWED_HYPERLINK",
}

// ApplyTheme applies a theme to a presentation.
// For "presentation" source: copies theme colors from another presentation.
// For "gallery" source: not supported by the API (returns error with guidance).
func (t *Tools) ApplyTheme(ctx context.Context, tokenSource oauth2.TokenSource, input ApplyThemeInput) (*ApplyThemeOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.ThemeSource == "" {
		return nil, fmt.Errorf("%w: theme_source is required (must be 'gallery' or 'presentation')", ErrInvalidThemeSource)
	}

	// Normalize theme source
	themeSource := strings.ToLower(strings.TrimSpace(input.ThemeSource))
	if themeSource != "gallery" && themeSource != "presentation" {
		return nil, fmt.Errorf("%w: must be 'gallery' or 'presentation', got '%s'", ErrInvalidThemeSource, input.ThemeSource)
	}

	t.config.Logger.Info("applying theme",
		slog.String("presentation_id", input.PresentationID),
		slog.String("theme_source", themeSource),
	)

	// Gallery themes are not supported by the API
	if themeSource == "gallery" {
		return nil, fmt.Errorf("%w: gallery theme application is not available via the Google Slides API. "+
			"To apply gallery themes, use the Google Slides UI (Slide > Change theme) or "+
			"use theme_source='presentation' to copy theme colors from another presentation", ErrGalleryNotSupported)
	}

	// For presentation source, validate source presentation ID
	if input.SourcePresentationID == "" {
		return nil, fmt.Errorf("%w: source_presentation_id is required when theme_source is 'presentation'", ErrInvalidSourcePresID)
	}

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Get source presentation to extract theme colors
	sourcePresentation, err := slidesService.GetPresentation(ctx, input.SourcePresentationID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("%w: source presentation not found", ErrSourceNotFound)
		}
		if isForbiddenError(err) {
			return nil, fmt.Errorf("%w: access denied to source presentation", ErrAccessDenied)
		}
		return nil, fmt.Errorf("%w: failed to get source presentation: %v", ErrSlidesAPIError, err)
	}

	// Find color scheme from source master
	if len(sourcePresentation.Masters) == 0 {
		return nil, ErrNoMasterInSource
	}

	sourceMaster := sourcePresentation.Masters[0]
	if sourceMaster.PageProperties == nil || sourceMaster.PageProperties.ColorScheme == nil {
		return nil, ErrNoColorScheme
	}

	sourceColorScheme := sourceMaster.PageProperties.ColorScheme

	// Get target presentation to find its master
	targetPresentation, err := slidesService.GetPresentation(ctx, input.PresentationID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrPresentationNotFound
		}
		if isForbiddenError(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: failed to get target presentation: %v", ErrSlidesAPIError, err)
	}

	if len(targetPresentation.Masters) == 0 {
		return nil, ErrNoMasterInTarget
	}

	targetMaster := targetPresentation.Masters[0]

	// Build the color scheme update request
	// Must provide all 12 ThemeColorTypes for a complete update
	newColorScheme := buildColorSchemeFromSource(sourceColorScheme)
	if newColorScheme == nil || len(newColorScheme.Colors) == 0 {
		return nil, ErrNoColorScheme
	}

	// Create update request
	requests := []*slides.Request{
		{
			UpdatePageProperties: &slides.UpdatePagePropertiesRequest{
				ObjectId: targetMaster.ObjectId,
				PageProperties: &slides.PageProperties{
					ColorScheme: newColorScheme,
				},
				Fields: "colorScheme",
			},
		},
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
		return nil, fmt.Errorf("%w: %v", ErrApplyThemeFailed, err)
	}

	// Collect updated properties
	var updatedProps []string
	for _, color := range newColorScheme.Colors {
		if color != nil && color.Type != "" {
			updatedProps = append(updatedProps, fmt.Sprintf("color_%s", strings.ToLower(color.Type)))
		}
	}

	output := &ApplyThemeOutput{
		Success:           true,
		Message:           "Theme colors applied successfully from source presentation",
		UpdatedProperties: updatedProps,
		SourceMasterID:    sourceMaster.ObjectId,
		TargetMasterID:    targetMaster.ObjectId,
	}

	t.config.Logger.Info("theme applied successfully",
		slog.String("presentation_id", input.PresentationID),
		slog.String("source_presentation_id", input.SourcePresentationID),
		slog.Int("colors_updated", len(updatedProps)),
	)

	return output, nil
}

// buildColorSchemeFromSource creates a ColorScheme from the source color scheme.
// It extracts the first 12 ThemeColorTypes that are editable via the API.
func buildColorSchemeFromSource(source *slides.ColorScheme) *slides.ColorScheme {
	if source == nil || len(source.Colors) == 0 {
		return nil
	}

	// Build a map of source colors for easy lookup
	sourceColors := make(map[string]*slides.ThemeColorPair)
	for _, pair := range source.Colors {
		if pair != nil && pair.Type != "" {
			sourceColors[pair.Type] = pair
		}
	}

	// Create new color scheme with all 12 required colors
	var newColors []*slides.ThemeColorPair
	for _, colorType := range themeColorTypes {
		if sourcePair, ok := sourceColors[colorType]; ok && sourcePair.Color != nil {
			newColors = append(newColors, &slides.ThemeColorPair{
				Type:  colorType,
				Color: cloneRgbColor(sourcePair.Color),
			})
		}
	}

	if len(newColors) == 0 {
		return nil
	}

	return &slides.ColorScheme{
		Colors: newColors,
	}
}

// cloneRgbColor creates a deep copy of an RgbColor.
func cloneRgbColor(color *slides.RgbColor) *slides.RgbColor {
	if color == nil {
		return nil
	}
	return &slides.RgbColor{
		Red:   color.Red,
		Green: color.Green,
		Blue:  color.Blue,
	}
}
