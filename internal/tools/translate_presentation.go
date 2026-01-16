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

// Sentinel errors for translate_presentation tool.
var (
	ErrTranslateFailed        = errors.New("failed to translate presentation")
	ErrInvalidTargetLanguage  = errors.New("target_language is required")
	ErrTranslateAPIError      = errors.New("translation API error")
	ErrNoTextToTranslate      = errors.New("no translatable text found")
)

// TranslateService abstracts the Google Cloud Translation API for testing.
type TranslateService interface {
	TranslateText(ctx context.Context, text, targetLanguage, sourceLanguage string) (string, error)
	TranslateTexts(ctx context.Context, texts []string, targetLanguage, sourceLanguage string) ([]string, error)
}

// TranslateServiceFactory creates a Translate service from a token source.
type TranslateServiceFactory func(ctx context.Context, tokenSource oauth2.TokenSource) (TranslateService, error)

// TranslatePresentationInput represents the input for the translate_presentation tool.
type TranslatePresentationInput struct {
	PresentationID string `json:"presentation_id"`
	TargetLanguage string `json:"target_language"` // ISO 639-1 code (e.g., "fr", "es", "de", "ja")
	SourceLanguage string `json:"source_language,omitempty"` // Optional, auto-detect if omitted
	Scope          string `json:"scope,omitempty"`  // "all" | "slide" | "object" - Default: "all"
	SlideIndex     int    `json:"slide_index,omitempty"` // 1-based, for scope="slide"
	SlideID        string `json:"slide_id,omitempty"`    // Alternative to slide_index for scope="slide"
	ObjectID       string `json:"object_id,omitempty"`   // For scope="object"
}

// TranslatePresentationOutput represents the output of the translate_presentation tool.
type TranslatePresentationOutput struct {
	PresentationID       string               `json:"presentation_id"`
	TargetLanguage       string               `json:"target_language"`
	SourceLanguage       string               `json:"source_language"`       // Detected or specified
	TranslatedCount      int                  `json:"translated_count"`      // Number of text elements translated
	AffectedSlides       []int                `json:"affected_slides"`       // 1-based slide indices
	TranslatedElements   []TranslatedElement  `json:"translated_elements,omitempty"`
}

// TranslatedElement represents a text element that was translated.
type TranslatedElement struct {
	SlideIndex   int    `json:"slide_index"` // 1-based
	ObjectID     string `json:"object_id"`
	ObjectType   string `json:"object_type"`
	OriginalText string `json:"original_text"`
	TranslatedText string `json:"translated_text"`
}

// TranslatePresentation translates all text in a presentation using Google Translate API.
func (t *Tools) TranslatePresentation(ctx context.Context, tokenSource oauth2.TokenSource, input TranslatePresentationInput) (*TranslatePresentationOutput, error) {
	// Validate input
	if input.PresentationID == "" {
		return nil, fmt.Errorf("%w: presentation_id is required", ErrInvalidPresentationID)
	}
	if input.TargetLanguage == "" {
		return nil, fmt.Errorf("%w: target_language is required (e.g., 'fr', 'es', 'de', 'ja')", ErrInvalidTargetLanguage)
	}

	// Set default scope
	if input.Scope == "" {
		input.Scope = "all"
	}

	// Validate scope
	validScopes := map[string]bool{
		"all":    true,
		"slide":  true,
		"object": true,
	}
	if !validScopes[input.Scope] {
		return nil, fmt.Errorf("%w: scope must be 'all', 'slide', or 'object'", ErrInvalidScope)
	}

	// Validate scope-specific parameters
	if input.Scope == "slide" && input.SlideIndex == 0 && input.SlideID == "" {
		return nil, fmt.Errorf("%w: slide_index or slide_id is required when scope is 'slide'", ErrInvalidScope)
	}
	if input.Scope == "object" && input.ObjectID == "" {
		return nil, fmt.Errorf("%w: object_id is required when scope is 'object'", ErrInvalidScope)
	}

	t.config.Logger.Info("translating presentation",
		slog.String("presentation_id", input.PresentationID),
		slog.String("target_language", input.TargetLanguage),
		slog.String("source_language", input.SourceLanguage),
		slog.String("scope", input.Scope),
	)

	// Create Slides service
	slidesService, err := t.slidesServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create slides service: %v", ErrSlidesAPIError, err)
	}

	// Create Translate service
	translateService, err := t.translateServiceFactory(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create translate service: %v", ErrTranslateAPIError, err)
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

	// Collect text elements to translate
	textElements, err := t.collectTextElements(presentation, input)
	if err != nil {
		return nil, err
	}

	if len(textElements) == 0 {
		return nil, fmt.Errorf("%w: no text found in the specified scope", ErrNoTextToTranslate)
	}

	// Extract all texts for batch translation
	texts := make([]string, len(textElements))
	for i, elem := range textElements {
		texts[i] = elem.OriginalText
	}

	// Translate all texts in a batch
	translatedTexts, err := translateService.TranslateTexts(ctx, texts, input.TargetLanguage, input.SourceLanguage)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTranslateAPIError, err)
	}

	if len(translatedTexts) != len(texts) {
		return nil, fmt.Errorf("%w: translation count mismatch", ErrTranslateFailed)
	}

	// Build batch update requests to replace text
	requests := make([]*slides.Request, 0, len(textElements)*2)
	translatedElements := make([]TranslatedElement, 0, len(textElements))
	affectedSlidesMap := make(map[int]bool)

	for i, elem := range textElements {
		translated := translatedTexts[i]
		if translated == "" || translated == elem.OriginalText {
			// Skip if translation is empty or unchanged
			continue
		}

		// Delete existing text and insert translated text
		if len(elem.OriginalText) > 0 {
			requests = append(requests, &slides.Request{
				DeleteText: &slides.DeleteTextRequest{
					ObjectId: elem.ObjectID,
					TextRange: &slides.Range{
						Type: "ALL",
					},
				},
			})
		}

		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       elem.ObjectID,
				InsertionIndex: 0,
				Text:           translated,
			},
		})

		translatedElements = append(translatedElements, TranslatedElement{
			SlideIndex:     elem.SlideIndex,
			ObjectID:       elem.ObjectID,
			ObjectType:     elem.ObjectType,
			OriginalText:   elem.OriginalText,
			TranslatedText: translated,
		})
		affectedSlidesMap[elem.SlideIndex] = true
	}

	if len(requests) == 0 {
		return nil, fmt.Errorf("%w: no text was translated (all texts unchanged or empty)", ErrNoTextToTranslate)
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
		return nil, fmt.Errorf("%w: %v", ErrTranslateFailed, err)
	}

	// Build affected slides list
	affectedSlides := make([]int, 0, len(affectedSlidesMap))
	for slideIdx := range affectedSlidesMap {
		affectedSlides = append(affectedSlides, slideIdx)
	}

	// Determine source language (detected or specified)
	sourceLanguage := input.SourceLanguage
	if sourceLanguage == "" {
		sourceLanguage = "auto-detected"
	}

	output := &TranslatePresentationOutput{
		PresentationID:     input.PresentationID,
		TargetLanguage:     input.TargetLanguage,
		SourceLanguage:     sourceLanguage,
		TranslatedCount:    len(translatedElements),
		AffectedSlides:     affectedSlides,
		TranslatedElements: translatedElements,
	}

	t.config.Logger.Info("translation completed",
		slog.String("presentation_id", input.PresentationID),
		slog.Int("translated_count", len(translatedElements)),
		slog.Int("affected_slides", len(affectedSlides)),
	)

	return output, nil
}

// textElementInfo holds information about a text element for translation.
type textElementInfo struct {
	SlideIndex   int
	ObjectID     string
	ObjectType   string
	OriginalText string
}

// collectTextElements collects all text elements from the presentation based on scope.
func (t *Tools) collectTextElements(presentation *slides.Presentation, input TranslatePresentationInput) ([]textElementInfo, error) {
	var elements []textElementInfo

	for slideIdx, slide := range presentation.Slides {
		if slide == nil {
			continue
		}

		slideIndex1Based := slideIdx + 1

		// Check if this slide matches the scope
		if input.Scope == "slide" {
			if input.SlideID != "" && slide.ObjectId != input.SlideID {
				continue
			}
			if input.SlideIndex != 0 && slideIndex1Based != input.SlideIndex {
				continue
			}
		}

		// Collect text elements from page elements
		slideElements := collectTextFromElements(slide.PageElements, slideIndex1Based, input.Scope, input.ObjectID)
		elements = append(elements, slideElements...)

		// Also collect from speaker notes
		if slide.SlideProperties != nil && slide.SlideProperties.NotesPage != nil {
			notesElements := collectTextFromElements(slide.SlideProperties.NotesPage.PageElements, slideIndex1Based, input.Scope, input.ObjectID)
			for i := range notesElements {
				notesElements[i].ObjectType = "SPEAKER_NOTES:" + notesElements[i].ObjectType
			}
			elements = append(elements, notesElements...)
		}
	}

	// Validate object exists if scope is "object"
	if input.Scope == "object" && len(elements) == 0 {
		return nil, fmt.Errorf("%w: object '%s' not found or has no text", ErrObjectNotFound, input.ObjectID)
	}

	// Validate slide exists if scope is "slide"
	if input.Scope == "slide" && len(elements) == 0 {
		if input.SlideID != "" {
			return nil, fmt.Errorf("%w: slide '%s' not found or has no text", ErrSlideNotFound, input.SlideID)
		}
		return nil, fmt.Errorf("%w: slide at index %d not found or has no text", ErrSlideNotFound, input.SlideIndex)
	}

	return elements, nil
}

// collectTextFromElements collects text elements from page elements.
func collectTextFromElements(pageElements []*slides.PageElement, slideIndex int, scope, targetObjectID string) []textElementInfo {
	var elements []textElementInfo

	for _, element := range pageElements {
		if element == nil {
			continue
		}

		// If scope is "object", only process that specific object
		if scope == "object" && element.ObjectId != targetObjectID {
			// Check in groups
			if element.ElementGroup != nil {
				childElements := collectTextFromElements(element.ElementGroup.Children, slideIndex, scope, targetObjectID)
				elements = append(elements, childElements...)
			}
			continue
		}

		// Extract text from shapes
		if element.Shape != nil && element.Shape.Text != nil {
			text := extractTextFromTextContent(element.Shape.Text)
			if strings.TrimSpace(text) != "" {
				elements = append(elements, textElementInfo{
					SlideIndex:   slideIndex,
					ObjectID:     element.ObjectId,
					ObjectType:   determineObjectType(element),
					OriginalText: text,
				})
			}
		}

		// Note: We don't translate table cells here because they need special handling
		// (each cell is a separate text element and would require cell-by-cell replacement)
		// This could be added in a future enhancement

		// Recursively process groups (only if not looking for specific object)
		if element.ElementGroup != nil && scope != "object" {
			childElements := collectTextFromElements(element.ElementGroup.Children, slideIndex, scope, targetObjectID)
			elements = append(elements, childElements...)
		}
	}

	return elements
}
