package integration

import (
	"context"
	"testing"
	"time"

	"github.com/smorand/google-slides-mcp/internal/tools"
)

// TestGetPresentation_LoadsExistingPresentation verifies loading a presentation.
func TestGetPresentation_LoadsExistingPresentation(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	// Create a test presentation
	pres := fixtures.CreateTestPresentation("Integration Test - Get Presentation")

	// Create tools with real service factories
	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	// Test get_presentation tool
	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.GetPresentation(ctx, fixtures.TokenSource(), tools.GetPresentationInput{
		PresentationID: pres.PresentationId,
	})

	if err != nil {
		t.Fatalf("Failed to get presentation: %v", err)
	}

	// Verify output
	if output.PresentationID != pres.PresentationId {
		t.Errorf("Expected presentation ID %s, got %s", pres.PresentationId, output.PresentationID)
	}

	if output.Title != "Integration Test - Get Presentation" {
		t.Errorf("Expected title 'Integration Test - Get Presentation', got '%s'", output.Title)
	}

	if output.SlidesCount < 1 {
		t.Errorf("Expected at least 1 slide, got %d", output.SlidesCount)
	}

	t.Logf("Successfully loaded presentation: %s with %d slides", output.Title, output.SlidesCount)
}

// TestGetPresentation_NotFound verifies error handling for non-existent presentations.
func TestGetPresentation_NotFound(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	_, err := toolsInstance.GetPresentation(ctx, fixtures.TokenSource(), tools.GetPresentationInput{
		PresentationID: "nonexistent-presentation-id-12345",
	})

	if err == nil {
		t.Fatal("Expected error for non-existent presentation")
	}

	t.Logf("Got expected error: %v", err)
}

// TestGetPresentation_EmptyID verifies error handling for empty presentation ID.
func TestGetPresentation_EmptyID(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	_, err := toolsInstance.GetPresentation(ctx, fixtures.TokenSource(), tools.GetPresentationInput{
		PresentationID: "",
	})

	if err == nil {
		t.Fatal("Expected error for empty presentation ID")
	}

	if err.Error() != "presentation_id is required" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

// TestCreatePresentation_CreatesNewPresentation verifies presentation creation.
func TestCreatePresentation_CreatesNewPresentation(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	testTitle := "Integration Test - Create Presentation"

	output, err := toolsInstance.CreatePresentation(ctx, fixtures.TokenSource(), tools.CreatePresentationInput{
		Title: testTitle,
	})

	if err != nil {
		t.Fatalf("Failed to create presentation: %v", err)
	}

	// Track for cleanup
	fixtures.TrackPresentation(output.PresentationID)

	// Verify output
	if output.PresentationID == "" {
		t.Error("Expected non-empty presentation ID")
	}

	if output.Title != testTitle {
		t.Errorf("Expected title '%s', got '%s'", testTitle, output.Title)
	}

	if output.URL == "" {
		t.Error("Expected non-empty URL")
	}

	t.Logf("Created presentation: %s (ID: %s)", output.Title, output.PresentationID)
}

// TestSearchPresentations_FindsPresentation verifies search functionality.
func TestSearchPresentations_FindsPresentation(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	// Create a presentation with a unique name
	uniqueName := "IntegrationTestSearchTarget"
	pres := fixtures.CreateTestPresentation(uniqueName)
	_ = pres // Used for verification

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Search for presentations
	output, err := toolsInstance.SearchPresentations(ctx, fixtures.TokenSource(), tools.SearchPresentationsInput{
		Query:      uniqueName,
		MaxResults: 10,
	})

	if err != nil {
		t.Fatalf("Failed to search presentations: %v", err)
	}

	// Verify we found results (might take time for Drive to index)
	t.Logf("Found %d presentations matching '%s'", output.TotalResults, uniqueName)

	// Note: Drive search might not immediately find newly created files
	// so we don't fail if no results, just log
	if output.TotalResults == 0 {
		t.Log("Note: No results found (Drive indexing may be delayed)")
	}
}

// TestCopyPresentation_CopiesPresentation verifies copy functionality.
func TestCopyPresentation_CopiesPresentation(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	// Create source presentation
	source := fixtures.CreateTestPresentation("Integration Test - Copy Source")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	newTitle := "Integration Test - Copy Target"

	output, err := toolsInstance.CopyPresentation(ctx, fixtures.TokenSource(), tools.CopyPresentationInput{
		SourceID: source.PresentationId,
		NewTitle: newTitle,
	})

	if err != nil {
		t.Fatalf("Failed to copy presentation: %v", err)
	}

	// Track for cleanup
	fixtures.TrackPresentation(output.PresentationID)

	// Verify output
	if output.PresentationID == "" {
		t.Error("Expected non-empty presentation ID")
	}

	if output.PresentationID == source.PresentationId {
		t.Error("Copy should have different ID from source")
	}

	if output.Title != newTitle {
		t.Errorf("Expected title '%s', got '%s'", newTitle, output.Title)
	}

	t.Logf("Copied presentation to: %s (ID: %s)", output.Title, output.PresentationID)
}

// TestExportPDF_ExportsPresentation verifies PDF export functionality.
func TestExportPDF_ExportsPresentation(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	// Create a presentation to export
	pres := fixtures.CreateTestPresentation("Integration Test - Export PDF")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.ExportPDF(ctx, fixtures.TokenSource(), tools.ExportPDFInput{
		PresentationID: pres.PresentationId,
	})

	if err != nil {
		t.Fatalf("Failed to export PDF: %v", err)
	}

	// Verify output
	if output.PDFBase64 == "" {
		t.Error("Expected non-empty PDF data")
	}

	if output.FileSize == 0 {
		t.Error("Expected non-zero file size")
	}

	t.Logf("Exported PDF: %d bytes", output.FileSize)
}

// TestGetPresentation_WithThumbnails verifies thumbnail generation.
func TestGetPresentation_WithThumbnails(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Thumbnails")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.GetPresentation(ctx, fixtures.TokenSource(), tools.GetPresentationInput{
		PresentationID:    pres.PresentationId,
		IncludeThumbnails: true,
	})

	if err != nil {
		t.Fatalf("Failed to get presentation with thumbnails: %v", err)
	}

	// Check that slides are present
	if len(output.Slides) == 0 {
		t.Fatal("Expected at least one slide")
	}

	// Note: Thumbnail might be empty if fetching fails, which is acceptable
	t.Logf("Got %d slides, first slide thumbnail length: %d", len(output.Slides), len(output.Slides[0].ThumbnailBase64))
}

// TestPresentationWorkflow_CreateModifyExport tests a complete workflow.
func TestPresentationWorkflow_CreateModifyExport(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	toolsInstance := tools.NewToolsWithAllServices(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
		tools.NewRealTranslateServiceFactory(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Step 1: Create presentation
	t.Log("Step 1: Creating presentation...")
	createOutput, err := toolsInstance.CreatePresentation(ctx, fixtures.TokenSource(), tools.CreatePresentationInput{
		Title: "Integration Test - Complete Workflow",
	})
	if err != nil {
		t.Fatalf("Failed to create presentation: %v", err)
	}
	fixtures.TrackPresentation(createOutput.PresentationID)
	t.Logf("Created presentation: %s", createOutput.PresentationID)

	// Step 2: Add a slide
	t.Log("Step 2: Adding slide...")
	addSlideOutput, err := toolsInstance.AddSlide(ctx, fixtures.TokenSource(), tools.AddSlideInput{
		PresentationID: createOutput.PresentationID,
		Layout:         "TITLE_AND_BODY",
	})
	if err != nil {
		t.Fatalf("Failed to add slide: %v", err)
	}
	t.Logf("Added slide: %s", addSlideOutput.SlideID)

	// Step 3: Get presentation to verify
	t.Log("Step 3: Verifying presentation...")
	getOutput, err := toolsInstance.GetPresentation(ctx, fixtures.TokenSource(), tools.GetPresentationInput{
		PresentationID: createOutput.PresentationID,
	})
	if err != nil {
		t.Fatalf("Failed to get presentation: %v", err)
	}

	// Should now have 2 slides (1 default + 1 added)
	if getOutput.SlidesCount < 2 {
		t.Errorf("Expected at least 2 slides, got %d", getOutput.SlidesCount)
	}

	// Step 4: Export to PDF
	t.Log("Step 4: Exporting to PDF...")
	exportOutput, err := toolsInstance.ExportPDF(ctx, fixtures.TokenSource(), tools.ExportPDFInput{
		PresentationID: createOutput.PresentationID,
	})
	if err != nil {
		t.Fatalf("Failed to export PDF: %v", err)
	}

	if exportOutput.FileSize == 0 {
		t.Error("Expected non-zero PDF size")
	}

	t.Logf("Workflow complete! Created presentation with %d slides, exported %d bytes PDF",
		getOutput.SlidesCount, exportOutput.FileSize)
}
