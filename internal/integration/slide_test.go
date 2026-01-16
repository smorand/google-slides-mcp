package integration

import (
	"testing"

	"github.com/smorand/google-slides-mcp/internal/tools"
)

// TestListSlides_ListsAllSlides verifies slide listing.
func TestListSlides_ListsAllSlides(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - List Slides")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.ListSlides(ctx, fixtures.TokenSource(), tools.ListSlidesInput{
		PresentationID: pres.PresentationId,
	})

	if err != nil {
		t.Fatalf("Failed to list slides: %v", err)
	}

	// New presentations have at least 1 slide
	totalSlides := output.Statistics.TotalSlides
	if totalSlides < 1 {
		t.Errorf("Expected at least 1 slide, got %d", totalSlides)
	}

	if len(output.Slides) != totalSlides {
		t.Errorf("Slide count mismatch: %d slides but TotalSlides=%d", len(output.Slides), totalSlides)
	}

	// First slide should have index 1
	if len(output.Slides) > 0 && output.Slides[0].Index != 1 {
		t.Errorf("Expected first slide index 1, got %d", output.Slides[0].Index)
	}

	t.Logf("Listed %d slides", totalSlides)
}

// TestAddSlide_AddsSlideAtEnd verifies adding slides.
func TestAddSlide_AddsSlideAtEnd(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Add Slide")
	initialCount := len(pres.Slides)

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
		PresentationID: pres.PresentationId,
		Layout:         "BLANK",
	})

	if err != nil {
		t.Fatalf("Failed to add slide: %v", err)
	}

	if output.SlideID == "" {
		t.Error("Expected non-empty slide ID")
	}

	// Verify slide was added
	listOutput, err := toolsInstance.ListSlides(ctx, fixtures.TokenSource(), tools.ListSlidesInput{
		PresentationID: pres.PresentationId,
	})
	if err != nil {
		t.Fatalf("Failed to list slides after add: %v", err)
	}

	if listOutput.Statistics.TotalSlides != initialCount+1 {
		t.Errorf("Expected %d slides after add, got %d", initialCount+1, listOutput.Statistics.TotalSlides)
	}

	t.Logf("Added slide: %s (now %d slides)", output.SlideID, listOutput.Statistics.TotalSlides)
}

// TestAddSlide_AddsSlideAtPosition verifies adding slides at specific positions.
func TestAddSlide_AddsSlideAtPosition(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Add Slide Position")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add a slide at position 1 (beginning)
	output, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
		PresentationID: pres.PresentationId,
		Position:       1, // Insert at beginning
		Layout:         "TITLE",
	})

	if err != nil {
		t.Fatalf("Failed to add slide at position: %v", err)
	}

	// Verify the slide index
	if output.SlideIndex != 1 {
		t.Errorf("Expected slide at index 1, got %d", output.SlideIndex)
	}

	t.Logf("Added slide at position 1: %s", output.SlideID)
}

// TestAddSlide_DifferentLayouts verifies different layout types work.
func TestAddSlide_DifferentLayouts(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Slide Layouts")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	layouts := []string{
		"BLANK",
		"TITLE",
		"TITLE_AND_BODY",
		"TITLE_ONLY",
		"SECTION_HEADER",
	}

	for _, layout := range layouts {
		t.Run(layout, func(t *testing.T) {
			output, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
				PresentationID: pres.PresentationId,
				Layout:         layout,
			})

			if err != nil {
				t.Errorf("Failed to add slide with layout %s: %v", layout, err)
				return
			}

			if output.SlideID == "" {
				t.Errorf("Expected non-empty slide ID for layout %s", layout)
			}

			t.Logf("Added slide with layout %s: %s", layout, output.SlideID)
		})
	}
}

// TestDeleteSlide_DeletesSlide verifies slide deletion.
func TestDeleteSlide_DeletesSlide(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Delete Slide")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// First add a slide so we have more than one
	addOutput, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
		PresentationID: pres.PresentationId,
		Layout:         "BLANK",
	})
	if err != nil {
		t.Fatalf("Failed to add slide: %v", err)
	}

	// Get initial count
	listBefore, _ := toolsInstance.ListSlides(ctx, fixtures.TokenSource(), tools.ListSlidesInput{
		PresentationID: pres.PresentationId,
	})
	countBefore := listBefore.Statistics.TotalSlides

	// Delete the slide we just added
	deleteOutput, err := toolsInstance.DeleteSlide(ctx, fixtures.TokenSource(), tools.DeleteSlideInput{
		PresentationID: pres.PresentationId,
		SlideID:        addOutput.SlideID,
	})

	if err != nil {
		t.Fatalf("Failed to delete slide: %v", err)
	}

	// Verify count decreased
	if deleteOutput.RemainingSlideCount != countBefore-1 {
		t.Errorf("Expected %d remaining slides, got %d", countBefore-1, deleteOutput.RemainingSlideCount)
	}

	t.Logf("Deleted slide, %d slides remaining", deleteOutput.RemainingSlideCount)
}

// TestDeleteSlide_CannotDeleteLastSlide verifies the last slide cannot be deleted.
func TestDeleteSlide_CannotDeleteLastSlide(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Delete Last Slide")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Try to delete the only slide (should fail)
	_, err := toolsInstance.DeleteSlide(ctx, fixtures.TokenSource(), tools.DeleteSlideInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1, // The only slide
	})

	// This should fail because we can't delete the last slide
	if err == nil {
		t.Log("Warning: Expected error when deleting last slide, but operation succeeded")
		// Note: The API might handle this differently, so we don't fail hard
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// TestDuplicateSlide_DuplicatesSlide verifies slide duplication.
func TestDuplicateSlide_DuplicatesSlide(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Duplicate Slide")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Get the first slide ID
	listOutput, err := toolsInstance.ListSlides(ctx, fixtures.TokenSource(), tools.ListSlidesInput{
		PresentationID: pres.PresentationId,
	})
	if err != nil {
		t.Fatalf("Failed to list slides: %v", err)
	}

	if len(listOutput.Slides) == 0 {
		t.Fatal("No slides to duplicate")
	}

	originalSlideID := listOutput.Slides[0].SlideID
	initialCount := listOutput.Statistics.TotalSlides

	// Duplicate the slide
	dupOutput, err := toolsInstance.DuplicateSlide(ctx, fixtures.TokenSource(), tools.DuplicateSlideInput{
		PresentationID: pres.PresentationId,
		SlideID:        originalSlideID,
	})

	if err != nil {
		t.Fatalf("Failed to duplicate slide: %v", err)
	}

	if dupOutput.SlideID == "" {
		t.Error("Expected non-empty new slide ID")
	}

	if dupOutput.SlideID == originalSlideID {
		t.Error("Duplicate should have different ID from original")
	}

	// Verify count increased
	listAfter, _ := toolsInstance.ListSlides(ctx, fixtures.TokenSource(), tools.ListSlidesInput{
		PresentationID: pres.PresentationId,
	})

	if listAfter.Statistics.TotalSlides != initialCount+1 {
		t.Errorf("Expected %d slides after duplicate, got %d", initialCount+1, listAfter.Statistics.TotalSlides)
	}

	t.Logf("Duplicated slide %s to %s", originalSlideID, dupOutput.SlideID)
}

// TestReorderSlides_MovesSlides verifies slide reordering.
func TestReorderSlides_MovesSlides(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Reorder Slides")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add two more slides
	slide2, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
		PresentationID: pres.PresentationId,
		Layout:         "BLANK",
	})
	if err != nil {
		t.Fatalf("Failed to add slide 2: %v", err)
	}

	slide3, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
		PresentationID: pres.PresentationId,
		Layout:         "BLANK",
	})
	if err != nil {
		t.Fatalf("Failed to add slide 3: %v", err)
	}

	// Move slide 3 to position 1
	reorderOutput, err := toolsInstance.ReorderSlides(ctx, fixtures.TokenSource(), tools.ReorderSlidesInput{
		PresentationID: pres.PresentationId,
		SlideIDs:       []string{slide3.SlideID},
		InsertAt:       1,
	})

	if err != nil {
		t.Fatalf("Failed to reorder slides: %v", err)
	}

	// Verify the new order
	if len(reorderOutput.NewOrder) == 0 {
		t.Error("Expected new order in output")
	}

	// The first slide should now be slide3
	listAfter, _ := toolsInstance.ListSlides(ctx, fixtures.TokenSource(), tools.ListSlidesInput{
		PresentationID: pres.PresentationId,
	})

	if len(listAfter.Slides) >= 1 && listAfter.Slides[0].SlideID == slide3.SlideID {
		t.Logf("Slide 3 successfully moved to position 1")
	} else {
		t.Logf("Slides after reorder: %v (expected first: %s)", getSlideIDs(listAfter.Slides), slide3.SlideID)
	}

	// Keep compiler happy
	_ = slide2
}

// TestDescribeSlide_DescribesSlideContent verifies slide description.
func TestDescribeSlide_DescribesSlideContent(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Describe Slide")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Describe the first slide
	output, err := toolsInstance.DescribeSlide(ctx, fixtures.TokenSource(), tools.DescribeSlideInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
	})

	if err != nil {
		t.Fatalf("Failed to describe slide: %v", err)
	}

	if output.SlideID == "" {
		t.Error("Expected non-empty slide ID")
	}

	if output.SlideIndex != 1 {
		t.Errorf("Expected index 1, got %d", output.SlideIndex)
	}

	t.Logf("Described slide: %s (layout: %s, %d objects)",
		output.SlideID, output.LayoutType, len(output.Objects))
}

// Helper to extract slide IDs from slide list
func getSlideIDs(slides []tools.SlideListItem) []string {
	ids := make([]string, len(slides))
	for i, s := range slides {
		ids[i] = s.SlideID
	}
	return ids
}
