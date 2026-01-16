package tools

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/slides/v1"
)

func TestConfigureFooter_ShowSlideNumber(t *testing.T) {
	ctx := context.Background()

	// Create presentation with slide number placeholder
	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Title:          "Test Presentation",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "slide-number-1",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "SLIDE_NUMBER",
							},
							Text: &slides.TextContent{},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "TITLE_AND_BODY",
				},
			},
		},
	}

	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Test enabling slide numbers
	showSlideNumber := true
	output, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID:  "test-pres",
		ShowSlideNumber: &showSlideNumber,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected success to be true")
	}
	if output.UpdatedSlideNumbers != 1 {
		t.Errorf("expected 1 slide number updated, got %d", output.UpdatedSlideNumbers)
	}

	// Check that requests include delete and insert
	if len(capturedRequests) < 2 {
		t.Fatalf("expected at least 2 requests (delete + insert), got %d", len(capturedRequests))
	}

	deleteReq := capturedRequests[0]
	if deleteReq.DeleteText == nil {
		t.Error("expected first request to be DeleteText")
	} else if deleteReq.DeleteText.ObjectId != "slide-number-1" {
		t.Errorf("expected object ID 'slide-number-1', got '%s'", deleteReq.DeleteText.ObjectId)
	}

	insertReq := capturedRequests[1]
	if insertReq.InsertText == nil {
		t.Error("expected second request to be InsertText")
	} else if insertReq.InsertText.Text != "#" {
		t.Errorf("expected text '#' for slide number, got '%s'", insertReq.InsertText.Text)
	}

	// Test disabling slide numbers
	showSlideNumber = false
	capturedRequests = nil

	output, err = tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID:  "test-pres",
		ShowSlideNumber: &showSlideNumber,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When disabling, we only need delete request (no insert since text is empty)
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request for disable (delete only), got %d", len(capturedRequests))
	}

	if capturedRequests[0].DeleteText == nil {
		t.Error("expected DeleteText request")
	}
}

func TestConfigureFooter_ShowDateWithFormat(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "date-placeholder",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "DATE_AND_TIME",
							},
							Text: &slides.TextContent{},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "BLANK",
				},
			},
		},
	}

	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Test enabling date with custom format
	showDate := true
	output, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		ShowDate:       &showDate,
		DateFormat:     "2006-01-02", // Go date format
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected success to be true")
	}
	if output.UpdatedDates != 1 {
		t.Errorf("expected 1 date updated, got %d", output.UpdatedDates)
	}

	// Check that the insert request has a date-formatted text
	if len(capturedRequests) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(capturedRequests))
	}

	insertReq := capturedRequests[1]
	if insertReq.InsertText == nil {
		t.Error("expected InsertText request")
	} else {
		// Text should match YYYY-MM-DD format
		text := insertReq.InsertText.Text
		if len(text) != 10 || text[4] != '-' || text[7] != '-' {
			t.Errorf("expected date in YYYY-MM-DD format, got '%s'", text)
		}
	}

	// Test with default format (no DateFormat provided)
	capturedRequests = nil
	output, err = tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		ShowDate:       &showDate,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	insertReq = capturedRequests[1]
	if insertReq.InsertText == nil {
		t.Error("expected InsertText request")
	} else {
		// Text should be a longer date format like "January 2, 2006"
		text := insertReq.InsertText.Text
		if len(text) < 10 {
			t.Errorf("expected longer date format, got '%s'", text)
		}
	}
}

func TestConfigureFooter_FooterText(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "footer-placeholder",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
							Text: &slides.TextContent{},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "BLANK",
				},
			},
		},
	}

	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Test setting footer text
	footerText := "My Custom Footer"
	output, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		FooterText:     &footerText,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected success to be true")
	}
	if output.UpdatedFooters != 1 {
		t.Errorf("expected 1 footer updated, got %d", output.UpdatedFooters)
	}

	// Check insert request
	if len(capturedRequests) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(capturedRequests))
	}

	insertReq := capturedRequests[1]
	if insertReq.InsertText == nil {
		t.Error("expected InsertText request")
	} else if insertReq.InsertText.Text != "My Custom Footer" {
		t.Errorf("expected text 'My Custom Footer', got '%s'", insertReq.InsertText.Text)
	}

	// Test clearing footer text
	emptyFooter := ""
	capturedRequests = nil

	output, err = tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		FooterText:     &emptyFooter,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have delete request
	if len(capturedRequests) != 1 {
		t.Errorf("expected 1 request for clear (delete only), got %d", len(capturedRequests))
	}
}

func TestConfigureFooter_ApplyToOptions(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			// Title slide
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-title",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "footer-1",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
						},
					},
				},
			},
			// Regular slide
			{
				ObjectId: "slide-2",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-body",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "footer-2",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
						},
					},
				},
			},
			// Another regular slide
			{
				ObjectId: "slide-3",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-body",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "footer-3",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-title",
				LayoutProperties: &slides.LayoutProperties{
					Name: "TITLE",
				},
			},
			{
				ObjectId: "layout-body",
				LayoutProperties: &slides.LayoutProperties{
					Name: "TITLE_AND_BODY",
				},
			},
		},
	}

	tests := []struct {
		name            string
		applyTo         string
		expectedFooters int
	}{
		{
			name:            "all slides",
			applyTo:         "all",
			expectedFooters: 3,
		},
		{
			name:            "title slides only",
			applyTo:         "title_slides_only",
			expectedFooters: 1, // Only slide-1 uses TITLE layout
		},
		{
			name:            "exclude title slides",
			applyTo:         "exclude_title_slides",
			expectedFooters: 2, // slide-2 and slide-3
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSlides := &mockSlidesService{
				GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
					return presentation, nil
				},
				BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
					return &slides.BatchUpdatePresentationResponse{}, nil
				},
			}

			slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
				return mockSlides, nil
			}

			tools := NewTools(DefaultToolsConfig(), slidesFactory)
			tokenSource := &mockTokenSource{}

			footerText := "Test Footer"
			output, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
				PresentationID: "test-pres",
				FooterText:     &footerText,
				ApplyTo:        tc.applyTo,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.UpdatedFooters != tc.expectedFooters {
				t.Errorf("expected %d footers updated, got %d", tc.expectedFooters, output.UpdatedFooters)
			}

			if output.AppliedTo != tc.applyTo {
				t.Errorf("expected applied_to '%s', got '%s'", tc.applyTo, output.AppliedTo)
			}
		})
	}
}

func TestConfigureFooter_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return nil, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Test missing presentation_id
	_, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{})
	if !errors.Is(err, ErrInvalidPresentationID) {
		t.Errorf("expected ErrInvalidPresentationID, got %v", err)
	}

	// Test no changes specified
	_, err = tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
	})
	if !errors.Is(err, ErrNoFooterChanges) {
		t.Errorf("expected ErrNoFooterChanges, got %v", err)
	}

	// Test invalid apply_to
	showSlideNumber := true
	_, err = tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID:  "test-pres",
		ShowSlideNumber: &showSlideNumber,
		ApplyTo:         "invalid_option",
	})
	if !errors.Is(err, ErrInvalidApplyTo) {
		t.Errorf("expected ErrInvalidApplyTo, got %v", err)
	}
}

func TestConfigureFooter_NoPlaceholders(t *testing.T) {
	ctx := context.Background()

	// Presentation with no footer placeholders
	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "text-box",
						Shape: &slides.Shape{
							ShapeType: "TEXT_BOX",
							Text:      &slides.TextContent{},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "BLANK",
				},
			},
		},
		Masters: []*slides.Page{
			{
				ObjectId:     "master-1",
				PageElements: []*slides.PageElement{},
			},
		},
	}

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	footerText := "Test"
	_, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		FooterText:     &footerText,
	})

	if !errors.Is(err, ErrNoFooterPlaceholders) {
		t.Errorf("expected ErrNoFooterPlaceholders, got %v", err)
	}
}

func TestConfigureFooter_PresentationNotFound(t *testing.T) {
	ctx := context.Background()

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("404 not found")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	footerText := "Test"
	_, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "non-existent",
		FooterText:     &footerText,
	})

	if !errors.Is(err, ErrPresentationNotFound) {
		t.Errorf("expected ErrPresentationNotFound, got %v", err)
	}
}

func TestConfigureFooter_AccessDenied(t *testing.T) {
	ctx := context.Background()

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return nil, errors.New("403 forbidden")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	footerText := "Test"
	_, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "restricted",
		FooterText:     &footerText,
	})

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied, got %v", err)
	}
}

func TestConfigureFooter_BatchUpdateError(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "footer-1",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "BLANK",
				},
			},
		},
	}

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			return nil, errors.New("API error")
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	footerText := "Test"
	_, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		FooterText:     &footerText,
	})

	if !errors.Is(err, ErrConfigureFooterFailed) {
		t.Errorf("expected ErrConfigureFooterFailed, got %v", err)
	}
}

func TestConfigureFooter_MultipleUpdates(t *testing.T) {
	ctx := context.Background()

	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					{
						ObjectId: "footer-1",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
						},
					},
					{
						ObjectId: "slide-number-1",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "SLIDE_NUMBER",
							},
						},
					},
					{
						ObjectId: "date-1",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "DATE_AND_TIME",
							},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "BLANK",
				},
			},
		},
	}

	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	// Update all three types at once
	showSlideNumber := true
	showDate := true
	footerText := "Custom Footer"

	output, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID:  "test-pres",
		ShowSlideNumber: &showSlideNumber,
		ShowDate:        &showDate,
		FooterText:      &footerText,
		DateFormat:      "2006-01-02",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.UpdatedSlideNumbers != 1 {
		t.Errorf("expected 1 slide number, got %d", output.UpdatedSlideNumbers)
	}
	if output.UpdatedDates != 1 {
		t.Errorf("expected 1 date, got %d", output.UpdatedDates)
	}
	if output.UpdatedFooters != 1 {
		t.Errorf("expected 1 footer, got %d", output.UpdatedFooters)
	}

	// Each placeholder update should have delete + insert = 2 requests
	// 3 placeholders * 2 requests = 6 total
	if len(capturedRequests) != 6 {
		t.Errorf("expected 6 requests, got %d", len(capturedRequests))
	}
}

func TestConfigureFooter_PlaceholdersOnMasterWhenNoSlidePlaceholders(t *testing.T) {
	ctx := context.Background()

	// Presentation with placeholders only on master (not on slides)
	presentation := &slides.Presentation{
		PresentationId: "test-pres",
		Slides: []*slides.Page{
			{
				ObjectId: "slide-1",
				SlideProperties: &slides.SlideProperties{
					LayoutObjectId: "layout-1",
				},
				PageElements: []*slides.PageElement{
					// No footer placeholders on slides
				},
			},
		},
		Masters: []*slides.Page{
			{
				ObjectId: "master-1",
				PageElements: []*slides.PageElement{
					{
						ObjectId: "master-footer",
						Shape: &slides.Shape{
							Placeholder: &slides.Placeholder{
								Type: "FOOTER",
							},
						},
					},
				},
			},
		},
		Layouts: []*slides.Page{
			{
				ObjectId: "layout-1",
				LayoutProperties: &slides.LayoutProperties{
					Name: "BLANK",
				},
			},
		},
	}

	var capturedRequests []*slides.Request

	mockSlides := &mockSlidesService{
		GetPresentationFunc: func(ctx context.Context, presentationID string) (*slides.Presentation, error) {
			return presentation, nil
		},
		BatchUpdateFunc: func(ctx context.Context, presentationID string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
			capturedRequests = requests
			return &slides.BatchUpdatePresentationResponse{}, nil
		},
	}

	slidesFactory := func(ctx context.Context, ts oauth2.TokenSource) (SlidesService, error) {
		return mockSlides, nil
	}

	tools := NewTools(DefaultToolsConfig(), slidesFactory)
	tokenSource := &mockTokenSource{}

	footerText := "Master Footer"
	output, err := tools.ConfigureFooter(ctx, tokenSource, ConfigureFooterInput{
		PresentationID: "test-pres",
		FooterText:     &footerText,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.UpdatedFooters != 1 {
		t.Errorf("expected 1 footer updated (on master), got %d", output.UpdatedFooters)
	}

	// Should have modified the master's footer placeholder
	if len(capturedRequests) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(capturedRequests))
	}

	deleteReq := capturedRequests[0]
	if deleteReq.DeleteText == nil || deleteReq.DeleteText.ObjectId != "master-footer" {
		t.Error("expected delete request for master-footer")
	}
}
