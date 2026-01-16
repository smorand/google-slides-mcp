package integration

import (
	"testing"

	"github.com/smorand/google-slides-mcp/internal/tools"
)

// TestListObjects_ListsSlideObjects verifies object listing.
func TestListObjects_ListsSlideObjects(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - List Objects")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// First add a text box so we have something to list
	_, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Test text for list objects",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 200, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	output, err := toolsInstance.ListObjects(ctx, fixtures.TokenSource(), tools.ListObjectsInput{
		PresentationID: pres.PresentationId,
	})

	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}

	if len(output.Objects) == 0 {
		t.Error("Expected at least one object")
	}

	t.Logf("Listed %d objects", len(output.Objects))
}

// TestAddTextBox_AddsTextBox verifies text box creation.
func TestAddTextBox_AddsTextBox(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Add Text Box")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Hello, Integration Test!",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 300, Height: 50},
	})

	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	if output.ObjectID == "" {
		t.Error("Expected non-empty object ID")
	}

	t.Logf("Added text box: %s", output.ObjectID)
}

// TestAddTextBox_WithStyling verifies styled text box creation.
func TestAddTextBox_WithStyling(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Styled Text Box")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Styled Text",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 300, Height: 50},
		Style: &tools.TextStyleInput{
			FontFamily: "Arial",
			FontSize:   24,
			Bold:       true,
			Color:      "#FF0000",
		},
	})

	if err != nil {
		t.Fatalf("Failed to add styled text box: %v", err)
	}

	if output.ObjectID == "" {
		t.Error("Expected non-empty object ID")
	}

	t.Logf("Added styled text box: %s", output.ObjectID)
}

// TestModifyText_ModifiesTextContent verifies text modification.
func TestModifyText_ModifiesTextContent(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Modify Text")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add a text box
	addOutput, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Original text",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 300, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	// Test replace action
	modifyOutput, err := toolsInstance.ModifyText(ctx, fixtures.TokenSource(), tools.ModifyTextInput{
		PresentationID: pres.PresentationId,
		ObjectID:       addOutput.ObjectID,
		Action:         "replace",
		Text:           "Replaced text",
	})

	if err != nil {
		t.Fatalf("Failed to modify text: %v", err)
	}

	if modifyOutput.UpdatedText != "Replaced text" {
		t.Errorf("Expected 'Replaced text', got '%s'", modifyOutput.UpdatedText)
	}

	t.Logf("Modified text to: %s", modifyOutput.UpdatedText)
}

// TestSearchText_FindsText verifies text search.
func TestSearchText_FindsText(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Search Text")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add a text box with searchable content
	_, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "This is a unique searchable phrase XYZ123",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 400, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	// Search for the text
	searchOutput, err := toolsInstance.SearchText(ctx, fixtures.TokenSource(), tools.SearchTextInput{
		PresentationID: pres.PresentationId,
		Query:          "XYZ123",
	})

	if err != nil {
		t.Fatalf("Failed to search text: %v", err)
	}

	if searchOutput.TotalMatches == 0 {
		t.Error("Expected to find at least one match")
	}

	t.Logf("Found %d matches for 'XYZ123'", searchOutput.TotalMatches)
}

// TestReplaceText_ReplacesAllOccurrences verifies text replacement.
func TestReplaceText_ReplacesAllOccurrences(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Replace Text")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add text boxes with replaceable content
	_, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Replace PLACEHOLDER with value",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 400, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	// Replace text
	replaceOutput, err := toolsInstance.ReplaceText(ctx, fixtures.TokenSource(), tools.ReplaceTextInput{
		PresentationID: pres.PresentationId,
		Find:           "PLACEHOLDER",
		ReplaceWith:    "ACTUAL_VALUE",
		Scope:          "all",
	})

	if err != nil {
		t.Fatalf("Failed to replace text: %v", err)
	}

	t.Logf("Replaced %d occurrences", replaceOutput.ReplacementCount)
}

// TestCreateTable_CreatesTable verifies table creation.
func TestCreateTable_CreatesTable(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Create Table")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	output, err := toolsInstance.CreateTable(ctx, fixtures.TokenSource(), tools.CreateTableInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Rows:           3,
		Columns:        4,
		Position:       &tools.PositionInput{X: 50, Y: 100},
		Size:           &tools.SizeInput{Width: 620, Height: 200},
	})

	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	if output.ObjectID == "" {
		t.Error("Expected non-empty object ID")
	}

	t.Logf("Created table: %s (rows: %d, columns: %d)", output.ObjectID, output.Rows, output.Columns)
}

// TestTransformObject_MovesObject verifies object transformation.
func TestTransformObject_MovesObject(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Transform Object")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add a text box to transform
	addOutput, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Move me!",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 200, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	// Transform (move) the object
	transformOutput, err := toolsInstance.TransformObject(ctx, fixtures.TokenSource(), tools.TransformObjectInput{
		PresentationID: pres.PresentationId,
		ObjectID:       addOutput.ObjectID,
		Position:       &tools.PositionInput{X: 200, Y: 200},
	})

	if err != nil {
		t.Fatalf("Failed to transform object: %v", err)
	}

	t.Logf("Transformed object to position (%v, %v)",
		transformOutput.Position.X, transformOutput.Position.Y)
}

// TestDeleteObject_DeletesObject verifies object deletion.
func TestDeleteObject_DeletesObject(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Delete Object")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add a text box to delete
	addOutput, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Delete me!",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 200, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	// Delete the object
	deleteOutput, err := toolsInstance.DeleteObject(ctx, fixtures.TokenSource(), tools.DeleteObjectInput{
		PresentationID: pres.PresentationId,
		ObjectID:       addOutput.ObjectID,
	})

	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	if deleteOutput.DeletedCount != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleteOutput.DeletedCount)
	}

	t.Logf("Deleted %d objects", deleteOutput.DeletedCount)
}

// TestGroupObjects_GroupsAndUngroups verifies object grouping.
func TestGroupObjects_GroupsAndUngroups(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Group Objects")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Add two text boxes to group
	tb1, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Text 1",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 100, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box 1: %v", err)
	}

	tb2, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Text 2",
		Position:       &tools.PositionInput{X: 250, Y: 100},
		Size:           &tools.SizeInput{Width: 100, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box 2: %v", err)
	}

	// Group the objects
	groupOutput, err := toolsInstance.GroupObjects(ctx, fixtures.TokenSource(), tools.GroupObjectsInput{
		PresentationID: pres.PresentationId,
		Action:         "group",
		ObjectIDs:      []string{tb1.ObjectID, tb2.ObjectID},
	})

	if err != nil {
		t.Fatalf("Failed to group objects: %v", err)
	}

	if groupOutput.GroupID == "" {
		t.Error("Expected non-empty group ID")
	}

	t.Logf("Grouped objects into: %s", groupOutput.GroupID)

	// Ungroup
	ungroupOutput, err := toolsInstance.GroupObjects(ctx, fixtures.TokenSource(), tools.GroupObjectsInput{
		PresentationID: pres.PresentationId,
		Action:         "ungroup",
		ObjectID:       groupOutput.GroupID,
	})

	if err != nil {
		t.Fatalf("Failed to ungroup objects: %v", err)
	}

	if len(ungroupOutput.ObjectIDs) != 2 {
		t.Errorf("Expected 2 ungrouped objects, got %d", len(ungroupOutput.ObjectIDs))
	}

	t.Logf("Ungrouped into %d objects", len(ungroupOutput.ObjectIDs))
}

// TestObjectWorkflow_CompleteObjectManipulation tests a complete object workflow.
func TestObjectWorkflow_CompleteObjectManipulation(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)
	fixtures := NewFixtures(t, config)

	pres := fixtures.CreateTestPresentation("Integration Test - Object Workflow")

	toolsInstance := tools.NewToolsWithDrive(
		tools.DefaultToolsConfig(),
		tools.NewRealSlidesServiceFactory(),
		tools.NewRealDriveServiceFactory(),
	)

	ctx, cancel := TestTimeout(t)
	defer cancel()

	// Step 1: Add text box
	t.Log("Step 1: Adding text box...")
	addOutput, err := toolsInstance.AddTextBox(ctx, fixtures.TokenSource(), tools.AddTextBoxInput{
		PresentationID: pres.PresentationId,
		SlideIndex:     1,
		Text:           "Workflow Test",
		Position:       &tools.PositionInput{X: 100, Y: 100},
		Size:           &tools.SizeInput{Width: 200, Height: 50},
	})
	if err != nil {
		t.Fatalf("Failed to add text box: %v", err)
	}

	// Step 2: Style the text
	t.Log("Step 2: Styling text...")
	bold := true
	_, err = toolsInstance.StyleText(ctx, fixtures.TokenSource(), tools.StyleTextInput{
		PresentationID: pres.PresentationId,
		ObjectID:       addOutput.ObjectID,
		Style: &tools.StyleTextStyleSpec{
			Bold:            &bold,
			FontSize:        18,
			ForegroundColor: "#0000FF",
		},
	})
	if err != nil {
		t.Fatalf("Failed to style text: %v", err)
	}

	// Step 3: Transform (resize)
	t.Log("Step 3: Resizing object...")
	_, err = toolsInstance.TransformObject(ctx, fixtures.TokenSource(), tools.TransformObjectInput{
		PresentationID: pres.PresentationId,
		ObjectID:       addOutput.ObjectID,
		Size:           &tools.SizeInput{Width: 300, Height: 80},
	})
	if err != nil {
		t.Fatalf("Failed to transform object: %v", err)
	}

	// Step 4: Get object details
	t.Log("Step 4: Getting object details...")
	getOutput, err := toolsInstance.GetObject(ctx, fixtures.TokenSource(), tools.GetObjectInput{
		PresentationID: pres.PresentationId,
		ObjectID:       addOutput.ObjectID,
	})
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}

	if getOutput.ObjectID != addOutput.ObjectID {
		t.Errorf("Object ID mismatch: expected %s, got %s", addOutput.ObjectID, getOutput.ObjectID)
	}

	t.Logf("Workflow complete! Object %s: type=%s", getOutput.ObjectID, getOutput.ObjectType)
}
