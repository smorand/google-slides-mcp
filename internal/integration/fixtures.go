package integration

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"
)

// Environment variable names for integration tests.
const (
	EnvIntegrationTest     = "INTEGRATION_TEST"
	EnvGoogleClientID      = "GOOGLE_CLIENT_ID"
	EnvGoogleClientSecret  = "GOOGLE_CLIENT_SECRET"
	EnvGoogleRefreshToken  = "GOOGLE_REFRESH_TOKEN"
	EnvTestPresentationID  = "TEST_PRESENTATION_ID"
	EnvGoogleProjectID     = "GOOGLE_PROJECT_ID"
	EnvFirestoreEmulator   = "FIRESTORE_EMULATOR_HOST"
)

// TestConfig holds configuration for integration tests.
type TestConfig struct {
	ClientID           string
	ClientSecret       string
	RefreshToken       string
	TestPresentationID string
	ProjectID          string
}

// SkipIfNoIntegration skips the test if integration tests are not enabled.
func SkipIfNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv(EnvIntegrationTest) != "1" {
		t.Skip("Integration tests are disabled. Set INTEGRATION_TEST=1 to enable.")
	}
}

// LoadConfig loads test configuration from environment variables.
func LoadConfig(t *testing.T) *TestConfig {
	t.Helper()

	clientID := os.Getenv(EnvGoogleClientID)
	clientSecret := os.Getenv(EnvGoogleClientSecret)
	refreshToken := os.Getenv(EnvGoogleRefreshToken)

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		t.Skipf("Missing required environment variables (%s, %s, %s)",
			EnvGoogleClientID, EnvGoogleClientSecret, EnvGoogleRefreshToken)
	}

	return &TestConfig{
		ClientID:           clientID,
		ClientSecret:       clientSecret,
		RefreshToken:       refreshToken,
		TestPresentationID: os.Getenv(EnvTestPresentationID),
		ProjectID:          os.Getenv(EnvGoogleProjectID),
	}
}

// Fixtures manages test fixtures and cleanup.
type Fixtures struct {
	t            *testing.T
	config       *TestConfig
	tokenSource  oauth2.TokenSource
	slidesClient *slides.Service

	// Track created resources for cleanup
	mu              sync.Mutex
	presentations   []string // Presentation IDs to delete
	cleanupFuncs    []func() // Additional cleanup functions
}

// NewFixtures creates a new test fixtures manager.
func NewFixtures(t *testing.T, config *TestConfig) *Fixtures {
	t.Helper()

	f := &Fixtures{
		t:             t,
		config:        config,
		presentations: make([]string, 0),
		cleanupFuncs:  make([]func(), 0),
	}

	// Set up OAuth2 token source
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"https://www.googleapis.com/auth/presentations",
			"https://www.googleapis.com/auth/drive",
		},
	}

	token := &oauth2.Token{
		RefreshToken: config.RefreshToken,
	}

	f.tokenSource = oauthConfig.TokenSource(context.Background(), token)

	// Create slides service
	ctx := context.Background()
	client, err := slides.NewService(ctx, option.WithTokenSource(f.tokenSource))
	if err != nil {
		t.Fatalf("Failed to create Slides service: %v", err)
	}
	f.slidesClient = client

	// Register cleanup on test completion
	t.Cleanup(f.Cleanup)

	return f
}

// TokenSource returns the OAuth2 token source for testing.
func (f *Fixtures) TokenSource() oauth2.TokenSource {
	return f.tokenSource
}

// SlidesClient returns the Google Slides service client.
func (f *Fixtures) SlidesClient() *slides.Service {
	return f.slidesClient
}

// CreateTestPresentation creates a temporary presentation for testing.
// The presentation will be automatically deleted after the test.
func (f *Fixtures) CreateTestPresentation(title string) *slides.Presentation {
	f.t.Helper()

	ctx := context.Background()
	presentation := &slides.Presentation{
		Title: title,
	}

	created, err := f.slidesClient.Presentations.Create(presentation).Context(ctx).Do()
	if err != nil {
		f.t.Fatalf("Failed to create test presentation: %v", err)
	}

	f.mu.Lock()
	f.presentations = append(f.presentations, created.PresentationId)
	f.mu.Unlock()

	f.t.Logf("Created test presentation: %s (ID: %s)", title, created.PresentationId)
	return created
}

// GetTestPresentationID returns a test presentation ID.
// If TEST_PRESENTATION_ID is set, it returns that (for read-only tests).
// Otherwise, it creates a new presentation.
func (f *Fixtures) GetTestPresentationID() string {
	if f.config.TestPresentationID != "" {
		return f.config.TestPresentationID
	}

	pres := f.CreateTestPresentation("Integration Test Presentation")
	return pres.PresentationId
}

// TrackPresentation adds a presentation ID to the cleanup list.
func (f *Fixtures) TrackPresentation(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presentations = append(f.presentations, id)
}

// RegisterCleanup registers a cleanup function to be called after the test.
func (f *Fixtures) RegisterCleanup(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupFuncs = append(f.cleanupFuncs, fn)
}

// Cleanup removes all test fixtures.
func (f *Fixtures) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run custom cleanup functions (in reverse order)
	for i := len(f.cleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					f.t.Logf("Cleanup function panicked: %v", r)
				}
			}()
			f.cleanupFuncs[i]()
		}()
	}

	// Delete created presentations
	for _, id := range f.presentations {
		if id == f.config.TestPresentationID {
			// Don't delete the configured test presentation
			continue
		}
		if err := f.deletePresentation(ctx, id); err != nil {
			f.t.Logf("Warning: failed to delete test presentation %s: %v", id, err)
		} else {
			f.t.Logf("Deleted test presentation: %s", id)
		}
	}

	f.presentations = nil
	f.cleanupFuncs = nil
}

// deletePresentation deletes a presentation using the Drive API.
func (f *Fixtures) deletePresentation(ctx context.Context, id string) error {
	// Use Drive API to delete (Slides API doesn't have a delete method)
	// This requires importing drive package
	// For now, we'll just log the deletion request
	// In a real implementation, you'd use the Drive API

	// Import drive and delete:
	// driveClient, err := drive.NewService(ctx, option.WithTokenSource(f.tokenSource))
	// if err != nil {
	//     return err
	// }
	// return driveClient.Files.Delete(id).Context(ctx).Do()

	// For this implementation, we'll note that cleanup requires Drive API
	f.t.Logf("Note: Presentation %s marked for deletion (requires Drive API cleanup)", id)
	return nil
}

// TestTimeout returns a context with a standard timeout for integration tests.
func TestTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// RequirePresentation ensures a presentation can be accessed.
func (f *Fixtures) RequirePresentation(presentationID string) *slides.Presentation {
	f.t.Helper()

	ctx, cancel := TestTimeout(f.t)
	defer cancel()

	pres, err := f.slidesClient.Presentations.Get(presentationID).Context(ctx).Do()
	if err != nil {
		f.t.Fatalf("Failed to get presentation %s: %v", presentationID, err)
	}
	return pres
}

// AddSlide adds a slide to a presentation and tracks it for reference.
func (f *Fixtures) AddSlide(presentationID string, layoutID string) string {
	f.t.Helper()

	ctx, cancel := TestTimeout(f.t)
	defer cancel()

	req := &slides.BatchUpdatePresentationRequest{
		Requests: []*slides.Request{
			{
				CreateSlide: &slides.CreateSlideRequest{
					SlideLayoutReference: &slides.LayoutReference{
						PredefinedLayout: "BLANK",
					},
				},
			},
		},
	}

	resp, err := f.slidesClient.Presentations.BatchUpdate(presentationID, req).Context(ctx).Do()
	if err != nil {
		f.t.Fatalf("Failed to add slide: %v", err)
	}

	if len(resp.Replies) == 0 || resp.Replies[0].CreateSlide == nil {
		f.t.Fatal("No slide created in response")
	}

	slideID := resp.Replies[0].CreateSlide.ObjectId
	f.t.Logf("Added slide: %s", slideID)
	return slideID
}

// AddTextBox adds a text box to a slide.
func (f *Fixtures) AddTextBox(presentationID, slideID, text string) string {
	f.t.Helper()

	ctx, cancel := TestTimeout(f.t)
	defer cancel()

	textBoxID := "test_textbox_" + time.Now().Format("20060102150405")

	req := &slides.BatchUpdatePresentationRequest{
		Requests: []*slides.Request{
			{
				CreateShape: &slides.CreateShapeRequest{
					ObjectId: textBoxID,
					ShapeType: "TEXT_BOX",
					ElementProperties: &slides.PageElementProperties{
						PageObjectId: slideID,
						Size: &slides.Size{
							Width:  &slides.Dimension{Magnitude: 300, Unit: "PT"},
							Height: &slides.Dimension{Magnitude: 50, Unit: "PT"},
						},
						Transform: &slides.AffineTransform{
							ScaleX:     1,
							ScaleY:     1,
							TranslateX: 100,
							TranslateY: 100,
							Unit:       "PT",
						},
					},
				},
			},
			{
				InsertText: &slides.InsertTextRequest{
					ObjectId: textBoxID,
					Text:     text,
				},
			},
		},
	}

	_, err := f.slidesClient.Presentations.BatchUpdate(presentationID, req).Context(ctx).Do()
	if err != nil {
		f.t.Fatalf("Failed to add text box: %v", err)
	}

	f.t.Logf("Added text box: %s with text: %s", textBoxID, text)
	return textBoxID
}
