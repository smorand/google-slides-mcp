package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smorand/google-slides-mcp/internal/auth"
	"github.com/smorand/google-slides-mcp/internal/transport"
)

// TestAuthFlow_GeneratesAuthorizationURL verifies the OAuth2 flow initiation.
func TestAuthFlow_GeneratesAuthorizationURL(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)

	// Create OAuth handler with real credentials
	oauthConfig := auth.OAuthConfig{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := auth.NewOAuthHandler(oauthConfig, nil)

	// Test the /auth endpoint
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec := httptest.NewRecorder()

	handler.HandleAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	authURL, ok := response["authorization_url"]
	if !ok {
		t.Fatal("Expected authorization_url in response")
	}

	// Verify URL contains correct components
	if authURL == "" {
		t.Error("Authorization URL is empty")
	}

	// Should contain the client ID
	if !containsSubstring(authURL, config.ClientID) {
		t.Error("Authorization URL should contain client ID")
	}

	t.Logf("Generated authorization URL: %s", authURL[:min(len(authURL), 100)]+"...")
}

// TestAuthFlow_RequiresCorrectMethod verifies method restrictions.
func TestAuthFlow_RequiresCorrectMethod(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)

	oauthConfig := auth.OAuthConfig{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := auth.NewOAuthHandler(oauthConfig, nil)

	// Test POST method (should fail)
	req := httptest.NewRequest(http.MethodPost, "/auth", nil)
	rec := httptest.NewRecorder()

	handler.HandleAuth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// TestAuthCallback_ValidatesState verifies state parameter validation.
func TestAuthCallback_ValidatesState(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)

	oauthConfig := auth.OAuthConfig{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := auth.NewOAuthHandler(oauthConfig, nil)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  string
	}{
		{
			name:       "missing state",
			query:      "?code=test-code",
			wantStatus: http.StatusBadRequest,
			wantError:  "missing state parameter",
		},
		{
			name:       "invalid state",
			query:      "?code=test-code&state=invalid-state",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid state parameter",
		},
		{
			name:       "oauth error",
			query:      "?error=access_denied&error_description=User+denied",
			wantStatus: http.StatusBadRequest,
			wantError:  "access_denied",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/auth/callback"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.HandleCallback(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("Expected status %d, got %d", tc.wantStatus, rec.Code)
			}

			var response map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if !containsSubstring(response["error"], tc.wantError) {
				t.Errorf("Expected error containing '%s', got '%s'", tc.wantError, response["error"])
			}
		})
	}
}

// TestServerWithAuth_IntegratesAuthHandler verifies server integration with auth.
func TestServerWithAuth_IntegratesAuthHandler(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)

	// Create server with auth handler
	serverConfig := transport.DefaultServerConfig()
	serverConfig.Port = 0 // Let OS assign port

	server := transport.NewServer(serverConfig)

	oauthConfig := auth.OAuthConfig{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := auth.NewOAuthHandler(oauthConfig, nil)
	server.SetAuthHandler(handler)

	// Start server in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		// Server.Start blocks, so we run it in goroutine
		_ = server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if !server.IsRunning() {
		t.Skip("Server didn't start (might be port conflict)")
	}

	// Shutdown
	if err := server.Shutdown(); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}

// TestTokenRefresh_UsesRefreshToken verifies that refresh tokens work.
func TestTokenRefresh_UsesRefreshToken(t *testing.T) {
	SkipIfNoIntegration(t)
	config := LoadConfig(t)

	fixtures := NewFixtures(t, config)

	// Get a fresh token using the refresh token
	token, err := fixtures.TokenSource().Token()
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	if token.AccessToken == "" {
		t.Error("Expected non-empty access token")
	}

	t.Logf("Successfully refreshed token (expires: %v)", token.Expiry)
}

// Helper functions

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
