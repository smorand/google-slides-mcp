package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestGetObject_Success_Shape(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "shape-1",
								Transform: &slides.AffineTransform{
									TranslateX: 127000,   // 10 points
									TranslateY: 254000,   // 20 points
								},
								Size: &slides.Size{
									Width:  &slides.Dimension{Magnitude: 300, Unit: "PT"},
									Height: &slides.Dimension{Magnitude: 100, Unit: "PT"},
								},
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{
												TextRun: &slides.TextRun{
													Content: "Hello World",
													Style: &slides.TextStyle{
														FontFamily: "Arial",
														FontSize:   &slides.Dimension{Magnitude: 24, Unit: "PT"},
														Bold:       true,
														ForegroundColor: &slides.OptionalColor{
															OpaqueColor: &slides.OpaqueColor{
																RgbColor: &slides.RgbColor{
																	Red:   1.0,
																	Green: 0.0,
																	Blue:  0.0,
																},
															},
														},
														Link: &slides.Link{
															Url: "https://example.com",
														},
													},
												},
											},
										},
									},
									ShapeProperties: &slides.ShapeProperties{
										ShapeBackgroundFill: &slides.ShapeBackgroundFill{
											SolidFill: &slides.SolidFill{
												Color: &slides.OpaqueColor{
													RgbColor: &slides.RgbColor{
														Red:   0.0,
														Green: 0.5,
														Blue:  1.0,
													},
												},
											},
										},
										Outline: &slides.Outline{
											OutlineFill: &slides.OutlineFill{
												SolidFill: &slides.SolidFill{
													Color: &slides.OpaqueColor{
														RgbColor: &slides.RgbColor{
															Red:   0.0,
															Green: 0.0,
															Blue:  0.0,
														},
													},
												},
											},
											Weight:    &slides.Dimension{Magnitude: 2, Unit: "PT"},
											DashStyle: "SOLID",
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "shape-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify basic info
	if output.PresentationID != "test-presentation-id" {
		t.Errorf("expected presentation ID 'test-presentation-id', got '%s'", output.PresentationID)
	}
	if output.ObjectID != "shape-1" {
		t.Errorf("expected object ID 'shape-1', got '%s'", output.ObjectID)
	}
	if output.ObjectType != "TEXT_BOX" {
		t.Errorf("expected object type 'TEXT_BOX', got '%s'", output.ObjectType)
	}
	if output.SlideIndex != 1 {
		t.Errorf("expected slide index 1, got %d", output.SlideIndex)
	}

	// Verify position
	if output.Position == nil {
		t.Fatal("expected position to be set")
	}
	expectedX := 127000.0 / 12700.0 // ~10 points
	expectedY := 254000.0 / 12700.0 // ~20 points
	if output.Position.X != expectedX {
		t.Errorf("expected X position %f, got %f", expectedX, output.Position.X)
	}
	if output.Position.Y != expectedY {
		t.Errorf("expected Y position %f, got %f", expectedY, output.Position.Y)
	}

	// Verify size
	if output.Size == nil {
		t.Fatal("expected size to be set")
	}
	if output.Size.Width != 300 {
		t.Errorf("expected width 300, got %f", output.Size.Width)
	}
	if output.Size.Height != 100 {
		t.Errorf("expected height 100, got %f", output.Size.Height)
	}

	// Verify shape details
	if output.Shape == nil {
		t.Fatal("expected shape details to be set")
	}
	if output.Shape.ShapeType != "TEXT_BOX" {
		t.Errorf("expected shape type 'TEXT_BOX', got '%s'", output.Shape.ShapeType)
	}
	if output.Shape.Text != "Hello World" {
		t.Errorf("expected text 'Hello World', got '%s'", output.Shape.Text)
	}

	// Verify text style
	if output.Shape.TextStyle == nil {
		t.Fatal("expected text style to be set")
	}
	if output.Shape.TextStyle.FontFamily != "Arial" {
		t.Errorf("expected font family 'Arial', got '%s'", output.Shape.TextStyle.FontFamily)
	}
	if output.Shape.TextStyle.FontSize == nil || *output.Shape.TextStyle.FontSize != 24 {
		t.Errorf("expected font size 24, got %v", output.Shape.TextStyle.FontSize)
	}
	if output.Shape.TextStyle.Bold == nil || !*output.Shape.TextStyle.Bold {
		t.Errorf("expected bold to be true")
	}
	if output.Shape.TextStyle.Color != "#FF0000" {
		t.Errorf("expected color '#FF0000', got '%s'", output.Shape.TextStyle.Color)
	}
	if output.Shape.TextStyle.LinkURL != "https://example.com" {
		t.Errorf("expected link URL 'https://example.com', got '%s'", output.Shape.TextStyle.LinkURL)
	}

	// Verify fill
	if output.Shape.Fill == nil {
		t.Fatal("expected fill to be set")
	}
	if output.Shape.Fill.Type != "SOLID" {
		t.Errorf("expected fill type 'SOLID', got '%s'", output.Shape.Fill.Type)
	}
	if output.Shape.Fill.SolidColor != "#007FFF" {
		t.Errorf("expected solid color '#007FFF', got '%s'", output.Shape.Fill.SolidColor)
	}

	// Verify outline
	if output.Shape.Outline == nil {
		t.Fatal("expected outline to be set")
	}
	if output.Shape.Outline.Color != "#000000" {
		t.Errorf("expected outline color '#000000', got '%s'", output.Shape.Outline.Color)
	}
	if output.Shape.Outline.Weight != 2 {
		t.Errorf("expected outline weight 2, got %f", output.Shape.Outline.Weight)
	}
	if output.Shape.Outline.DashStyle != "SOLID" {
		t.Errorf("expected dash style 'SOLID', got '%s'", output.Shape.Outline.DashStyle)
	}
}

func TestGetObject_Success_Image(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "image-1",
								Image: &slides.Image{
									ContentUrl: "https://example.com/image.png",
									SourceUrl:  "https://source.com/original.png",
									ImageProperties: &slides.ImageProperties{
										Brightness:   0.5,
										Contrast:     0.3,
										Transparency: 0.1,
										CropProperties: &slides.CropProperties{
											TopOffset:    0.1,
											BottomOffset: 0.2,
											LeftOffset:   0.05,
											RightOffset:  0.15,
										},
										Recolor: &slides.Recolor{
											Name: "GRAYSCALE",
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "image-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "IMAGE" {
		t.Errorf("expected object type 'IMAGE', got '%s'", output.ObjectType)
	}

	// Verify image details
	if output.Image == nil {
		t.Fatal("expected image details to be set")
	}
	if output.Image.ContentURL != "https://example.com/image.png" {
		t.Errorf("expected content URL 'https://example.com/image.png', got '%s'", output.Image.ContentURL)
	}
	if output.Image.SourceURL != "https://source.com/original.png" {
		t.Errorf("expected source URL 'https://source.com/original.png', got '%s'", output.Image.SourceURL)
	}
	if output.Image.Brightness != 0.5 {
		t.Errorf("expected brightness 0.5, got %f", output.Image.Brightness)
	}
	if output.Image.Contrast != 0.3 {
		t.Errorf("expected contrast 0.3, got %f", output.Image.Contrast)
	}
	if output.Image.Transparency != 0.1 {
		t.Errorf("expected transparency 0.1, got %f", output.Image.Transparency)
	}
	if output.Image.Recolor != "GRAYSCALE" {
		t.Errorf("expected recolor 'GRAYSCALE', got '%s'", output.Image.Recolor)
	}

	// Verify crop
	if output.Image.Crop == nil {
		t.Fatal("expected crop to be set")
	}
	if output.Image.Crop.Top != 0.1 {
		t.Errorf("expected crop top 0.1, got %f", output.Image.Crop.Top)
	}
	if output.Image.Crop.Bottom != 0.2 {
		t.Errorf("expected crop bottom 0.2, got %f", output.Image.Crop.Bottom)
	}
	if output.Image.Crop.Left != 0.05 {
		t.Errorf("expected crop left 0.05, got %f", output.Image.Crop.Left)
	}
	if output.Image.Crop.Right != 0.15 {
		t.Errorf("expected crop right 0.15, got %f", output.Image.Crop.Right)
	}
}

func TestGetObject_Success_Table(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "table-1",
								Table: &slides.Table{
									TableRows: []*slides.TableRow{
										{
											TableCells: []*slides.TableCell{
												{
													Text: &slides.TextContent{
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "Header 1"}},
														},
													},
													RowSpan:    1,
													ColumnSpan: 1,
													TableCellProperties: &slides.TableCellProperties{
														TableCellBackgroundFill: &slides.TableCellBackgroundFill{
															SolidFill: &slides.SolidFill{
																Color: &slides.OpaqueColor{
																	RgbColor: &slides.RgbColor{
																		Red:   0.9,
																		Green: 0.9,
																		Blue:  0.9,
																	},
																},
															},
														},
													},
												},
												{
													Text: &slides.TextContent{
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "Header 2"}},
														},
													},
												},
											},
										},
										{
											TableCells: []*slides.TableCell{
												{
													Text: &slides.TextContent{
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "Cell A1"}},
														},
													},
												},
												{
													Text: &slides.TextContent{
														TextElements: []*slides.TextElement{
															{TextRun: &slides.TextRun{Content: "Cell B1"}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "table-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "TABLE" {
		t.Errorf("expected object type 'TABLE', got '%s'", output.ObjectType)
	}

	// Verify table details
	if output.Table == nil {
		t.Fatal("expected table details to be set")
	}
	if output.Table.Rows != 2 {
		t.Errorf("expected 2 rows, got %d", output.Table.Rows)
	}
	if output.Table.Columns != 2 {
		t.Errorf("expected 2 columns, got %d", output.Table.Columns)
	}

	// Verify cells
	if len(output.Table.Cells) != 2 {
		t.Fatalf("expected 2 cell rows, got %d", len(output.Table.Cells))
	}
	if len(output.Table.Cells[0]) != 2 {
		t.Fatalf("expected 2 cells in first row, got %d", len(output.Table.Cells[0]))
	}
	if output.Table.Cells[0][0].Text != "Header 1" {
		t.Errorf("expected cell text 'Header 1', got '%s'", output.Table.Cells[0][0].Text)
	}
	if output.Table.Cells[0][0].Background != "#E5E5E5" {
		t.Errorf("expected background '#E5E5E5', got '%s'", output.Table.Cells[0][0].Background)
	}
	if output.Table.Cells[1][1].Text != "Cell B1" {
		t.Errorf("expected cell text 'Cell B1', got '%s'", output.Table.Cells[1][1].Text)
	}
}

func TestGetObject_Success_Video(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "video-1",
								Video: &slides.Video{
									Id:     "dQw4w9WgXcQ",
									Source: "YOUTUBE",
									Url:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
									VideoProperties: &slides.VideoProperties{
										Start:    30000, // 30 seconds in ms
										End:      60000, // 60 seconds in ms
										AutoPlay: true,
										Mute:     false,
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "video-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "VIDEO" {
		t.Errorf("expected object type 'VIDEO', got '%s'", output.ObjectType)
	}

	// Verify video details
	if output.Video == nil {
		t.Fatal("expected video details to be set")
	}
	if output.Video.VideoID != "dQw4w9WgXcQ" {
		t.Errorf("expected video ID 'dQw4w9WgXcQ', got '%s'", output.Video.VideoID)
	}
	if output.Video.Source != "YOUTUBE" {
		t.Errorf("expected source 'YOUTUBE', got '%s'", output.Video.Source)
	}
	if output.Video.URL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("expected URL, got '%s'", output.Video.URL)
	}
	if output.Video.StartTime != 30 {
		t.Errorf("expected start time 30 seconds, got %f", output.Video.StartTime)
	}
	if output.Video.EndTime != 60 {
		t.Errorf("expected end time 60 seconds, got %f", output.Video.EndTime)
	}
	if !output.Video.Autoplay {
		t.Error("expected autoplay to be true")
	}
	if output.Video.Mute {
		t.Error("expected mute to be false")
	}
}

func TestGetObject_Success_Line(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "line-1",
								Line: &slides.Line{
									LineType:     "STRAIGHT_CONNECTOR_1",
									LineCategory: "STRAIGHT",
									LineProperties: &slides.LineProperties{
										StartArrow: "ARROW",
										EndArrow:   "NONE",
										DashStyle:  "DASH",
										LineFill: &slides.LineFill{
											SolidFill: &slides.SolidFill{
												Color: &slides.OpaqueColor{
													RgbColor: &slides.RgbColor{
														Red:   0.0,
														Green: 0.0,
														Blue:  1.0,
													},
												},
											},
										},
										Weight: &slides.Dimension{Magnitude: 3, Unit: "PT"},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "line-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "LINE" {
		t.Errorf("expected object type 'LINE', got '%s'", output.ObjectType)
	}

	// Verify line details
	if output.Line == nil {
		t.Fatal("expected line details to be set")
	}
	if output.Line.LineType != "STRAIGHT_CONNECTOR_1" {
		t.Errorf("expected line type 'STRAIGHT_CONNECTOR_1', got '%s'", output.Line.LineType)
	}
	if output.Line.StartArrow != "ARROW" {
		t.Errorf("expected start arrow 'ARROW', got '%s'", output.Line.StartArrow)
	}
	if output.Line.EndArrow != "NONE" {
		t.Errorf("expected end arrow 'NONE', got '%s'", output.Line.EndArrow)
	}
	if output.Line.DashStyle != "DASH" {
		t.Errorf("expected dash style 'DASH', got '%s'", output.Line.DashStyle)
	}
	if output.Line.Color != "#0000FF" {
		t.Errorf("expected color '#0000FF', got '%s'", output.Line.Color)
	}
	if output.Line.Weight != 3 {
		t.Errorf("expected weight 3, got %f", output.Line.Weight)
	}
}

func TestGetObject_Success_Group(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{ObjectId: "child-1", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
										{ObjectId: "child-2", Image: &slides.Image{}},
										{ObjectId: "child-3", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "group-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "GROUP" {
		t.Errorf("expected object type 'GROUP', got '%s'", output.ObjectType)
	}

	// Verify group details
	if output.Group == nil {
		t.Fatal("expected group details to be set")
	}
	if output.Group.ChildCount != 3 {
		t.Errorf("expected 3 children, got %d", output.Group.ChildCount)
	}
	expectedIDs := []string{"child-1", "child-2", "child-3"}
	for i, id := range expectedIDs {
		if output.Group.ChildIDs[i] != id {
			t.Errorf("expected child ID '%s', got '%s'", id, output.Group.ChildIDs[i])
		}
	}
}

func TestGetObject_Success_SheetsChart(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "chart-1",
								SheetsChart: &slides.SheetsChart{
									SpreadsheetId: "spreadsheet-abc123",
									ChartId:       12345,
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "chart-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "SHEETS_CHART" {
		t.Errorf("expected object type 'SHEETS_CHART', got '%s'", output.ObjectType)
	}

	// Verify chart details
	if output.Chart == nil {
		t.Fatal("expected chart details to be set")
	}
	if output.Chart.SpreadsheetID != "spreadsheet-abc123" {
		t.Errorf("expected spreadsheet ID 'spreadsheet-abc123', got '%s'", output.Chart.SpreadsheetID)
	}
	if output.Chart.ChartID != 12345 {
		t.Errorf("expected chart ID 12345, got %d", output.Chart.ChartID)
	}
}

func TestGetObject_Success_WordArt(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "wordart-1",
								WordArt: &slides.WordArt{
									RenderedText: "Cool Text",
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "wordart-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectType != "WORD_ART" {
		t.Errorf("expected object type 'WORD_ART', got '%s'", output.ObjectType)
	}

	// Verify word art details
	if output.WordArt == nil {
		t.Fatal("expected word art details to be set")
	}
	if output.WordArt.RenderedText != "Cool Text" {
		t.Errorf("expected rendered text 'Cool Text', got '%s'", output.WordArt.RenderedText)
	}
}

func TestGetObject_ObjectInGroup(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "group-1",
								ElementGroup: &slides.Group{
									Children: []*slides.PageElement{
										{
											ObjectId: "nested-shape",
											Shape: &slides.Shape{
												ShapeType: "RECTANGLE",
												Text: &slides.TextContent{
													TextElements: []*slides.TextElement{
														{TextRun: &slides.TextRun{Content: "Nested text"}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	// Should find object nested inside group
	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "nested-shape",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ObjectID != "nested-shape" {
		t.Errorf("expected object ID 'nested-shape', got '%s'", output.ObjectID)
	}
	if output.Shape == nil || output.Shape.Text != "Nested text" {
		t.Error("expected to find nested shape with text content")
	}
}

func TestGetObject_ObjectInSecondSlide(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
						},
					},
					{
						ObjectId:        "slide-2",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{ObjectId: "shape-2", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "shape-2",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.SlideIndex != 2 {
		t.Errorf("expected slide index 2, got %d", output.SlideIndex)
	}
}

func TestGetObject_EmptyPresentationID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "",
		ObjectID:       "object-1",
	})

	if err == nil {
		t.Fatal("expected error for empty presentation ID")
	}
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}
}

func TestGetObject_EmptyObjectID(t *testing.T) {
	tools := NewTools(DefaultToolsConfig(), nil)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "",
	})

	if err == nil {
		t.Fatal("expected error for empty object ID")
	}
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestGetObject_ObjectNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements:    []*slides.PageElement{},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "nonexistent-object",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent object")
	}
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestGetObject_PresentationNotFound(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 404: Requested entity was not found")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "nonexistent-presentation",
		ObjectID:       "object-1",
	})

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestGetObject_AccessDenied(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("googleapi: Error 403: The caller does not have permission")
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "forbidden-presentation",
		ObjectID:       "object-1",
	})

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestGetObject_ServiceFactoryError(t *testing.T) {
	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, errors.New("failed to create service")
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	_, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-id",
		ObjectID:       "object-1",
	})

	if err == nil {
		t.Fatal("expected error when service factory fails")
	}
	if !errors.Is(err, ErrSlidesAPIError) {
		t.Errorf("expected ErrSlidesAPIError, got %v", err)
	}
}

func TestExtractColor(t *testing.T) {
	testCases := []struct {
		name     string
		color    *slides.OpaqueColor
		expected string
	}{
		{
			name:     "nil color",
			color:    nil,
			expected: "",
		},
		{
			name: "RGB red",
			color: &slides.OpaqueColor{
				RgbColor: &slides.RgbColor{Red: 1.0, Green: 0.0, Blue: 0.0},
			},
			expected: "#FF0000",
		},
		{
			name: "RGB green",
			color: &slides.OpaqueColor{
				RgbColor: &slides.RgbColor{Red: 0.0, Green: 1.0, Blue: 0.0},
			},
			expected: "#00FF00",
		},
		{
			name: "RGB blue",
			color: &slides.OpaqueColor{
				RgbColor: &slides.RgbColor{Red: 0.0, Green: 0.0, Blue: 1.0},
			},
			expected: "#0000FF",
		},
		{
			name: "RGB gray",
			color: &slides.OpaqueColor{
				RgbColor: &slides.RgbColor{Red: 0.5, Green: 0.5, Blue: 0.5},
			},
			expected: "#7F7F7F",
		},
		{
			name: "theme color",
			color: &slides.OpaqueColor{
				ThemeColor: "ACCENT1",
			},
			expected: "theme:ACCENT1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractColor(tc.color)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestFindElementByID(t *testing.T) {
	elements := []*slides.PageElement{
		{ObjectId: "shape-1", Shape: &slides.Shape{ShapeType: "TEXT_BOX"}},
		{ObjectId: "image-1", Image: &slides.Image{}},
		{
			ObjectId: "group-1",
			ElementGroup: &slides.Group{
				Children: []*slides.PageElement{
					{ObjectId: "nested-shape", Shape: &slides.Shape{ShapeType: "RECTANGLE"}},
					{
						ObjectId: "nested-group",
						ElementGroup: &slides.Group{
							Children: []*slides.PageElement{
								{ObjectId: "deeply-nested", Shape: &slides.Shape{ShapeType: "ELLIPSE"}},
							},
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name     string
		objectID string
		found    bool
	}{
		{"top level shape", "shape-1", true},
		{"top level image", "image-1", true},
		{"top level group", "group-1", true},
		{"nested shape", "nested-shape", true},
		{"deeply nested", "deeply-nested", true},
		{"not found", "nonexistent", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := findElementByID(elements, tc.objectID)
			if tc.found && result == nil {
				t.Error("expected to find element, got nil")
			}
			if !tc.found && result != nil {
				t.Errorf("expected nil, got element with ID '%s'", result.ObjectId)
			}
			if tc.found && result != nil && result.ObjectId != tc.objectID {
				t.Errorf("expected ID '%s', got '%s'", tc.objectID, result.ObjectId)
			}
		})
	}
}

func TestGetObject_ShapeWithPlaceholder(t *testing.T) {
	mockService := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return &slides.Presentation{
				PresentationId: presentationID,
				Slides: []*slides.Page{
					{
						ObjectId:        "slide-1",
						SlideProperties: &slides.SlideProperties{},
						PageElements: []*slides.PageElement{
							{
								ObjectId: "placeholder-1",
								Shape: &slides.Shape{
									ShapeType: "TEXT_BOX",
									Placeholder: &slides.Placeholder{
										Type: "TITLE",
									},
									Text: &slides.TextContent{
										TextElements: []*slides.TextElement{
											{TextRun: &slides.TextRun{Content: "Slide Title"}},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	factory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockService, nil
	}

	tools := NewTools(DefaultToolsConfig(), factory)
	tokenSource := &mockTokenSource{}

	output, err := tools.GetObject(context.Background(), tokenSource, GetObjectInput{
		PresentationID: "test-presentation-id",
		ObjectID:       "placeholder-1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Shape == nil {
		t.Fatal("expected shape details")
	}
	if output.Shape.PlaceholderType != "TITLE" {
		t.Errorf("expected placeholder type 'TITLE', got '%s'", output.Shape.PlaceholderType)
	}
}
