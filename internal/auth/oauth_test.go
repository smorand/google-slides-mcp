package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestNewOAuthHandler(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := NewOAuthHandler(config, nil)

	if handler == nil {
		t.Fatal("expected handler to be created")
	}

	if handler.config.ClientID != config.ClientID {
		t.Errorf("expected client ID %s, got %s", config.ClientID, handler.config.ClientID)
	}

	if handler.config.ClientSecret != config.ClientSecret {
		t.Errorf("expected client secret %s, got %s", config.ClientSecret, handler.config.ClientSecret)
	}

	if handler.config.RedirectURL != config.RedirectURI {
		t.Errorf("expected redirect URI %s, got %s", config.RedirectURI, handler.config.RedirectURL)
	}
}

func TestNewOAuthHandler_DefaultScopes(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := NewOAuthHandler(config, nil)

	if len(handler.config.Scopes) != len(DefaultScopes) {
		t.Errorf("expected %d scopes, got %d", len(DefaultScopes), len(handler.config.Scopes))
	}

	for i, scope := range DefaultScopes {
		if handler.config.Scopes[i] != scope {
			t.Errorf("expected scope %s, got %s", scope, handler.config.Scopes[i])
		}
	}
}

func TestNewOAuthHandler_CustomScopes(t *testing.T) {
	customScopes := []string{"scope1", "scope2"}
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
		Scopes:       customScopes,
	}

	handler := NewOAuthHandler(config, nil)

	if len(handler.config.Scopes) != len(customScopes) {
		t.Errorf("expected %d scopes, got %d", len(customScopes), len(handler.config.Scopes))
	}
}

func TestHandleAuth_ReturnsAuthorizationURL(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := NewOAuthHandler(config, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec := httptest.NewRecorder()

	handler.HandleAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	authURL, ok := response["authorization_url"]
	if !ok {
		t.Fatal("expected authorization_url in response")
	}

	parsedURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse authorization URL: %v", err)
	}

	// Check required parameters
	if parsedURL.Host != "accounts.google.com" {
		t.Errorf("expected host accounts.google.com, got %s", parsedURL.Host)
	}

	query := parsedURL.Query()

	if query.Get("client_id") != config.ClientID {
		t.Errorf("expected client_id %s, got %s", config.ClientID, query.Get("client_id"))
	}

	if query.Get("redirect_uri") != config.RedirectURI {
		t.Errorf("expected redirect_uri %s, got %s", config.RedirectURI, query.Get("redirect_uri"))
	}

	if query.Get("access_type") != "offline" {
		t.Errorf("expected access_type offline, got %s", query.Get("access_type"))
	}

	if query.Get("response_type") != "code" {
		t.Errorf("expected response_type code, got %s", query.Get("response_type"))
	}

	if query.Get("state") == "" {
		t.Error("expected state parameter in authorization URL")
	}

	// Check scopes
	scope := query.Get("scope")
	for _, s := range DefaultScopes {
		if !strings.Contains(scope, s) {
			t.Errorf("expected scope to contain %s", s)
		}
	}
}

func TestHandleAuth_MethodNotAllowed(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/auth", nil)
			rec := httptest.NewRecorder()

			handler.HandleAuth(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
			}
		})
	}
}

func TestHandleCallback_MissingState(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code", nil)
	rec := httptest.NewRecorder()

	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "missing state parameter" {
		t.Errorf("expected 'missing state parameter' error, got %s", response["error"])
	}
}

func TestHandleCallback_InvalidState(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=invalid-state", nil)
	rec := httptest.NewRecorder()

	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid state parameter" {
		t.Errorf("expected 'invalid state parameter' error, got %s", response["error"])
	}
}

func TestHandleCallback_MissingCode(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	// First generate a valid state
	handler.mu.Lock()
	handler.states["valid-state"] = true
	handler.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=valid-state", nil)
	rec := httptest.NewRecorder()

	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "missing authorization code" {
		t.Errorf("expected 'missing authorization code' error, got %s", response["error"])
	}
}

func TestHandleCallback_OAuthError(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?error=access_denied&error_description=User+denied+access", nil)
	rec := httptest.NewRecorder()

	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(response["error"], "access_denied") {
		t.Errorf("expected error to contain 'access_denied', got %s", response["error"])
	}
}

func TestHandleCallback_MethodNotAllowed(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/auth/callback", nil)
			rec := httptest.NewRecorder()

			handler.HandleCallback(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
			}
		})
	}
}

func TestHandleCallback_StateIsConsumed(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	// Add valid state
	handler.mu.Lock()
	handler.states["valid-state"] = true
	handler.mu.Unlock()

	// First request should consume the state
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=valid-state&code=test-code", nil)
	rec := httptest.NewRecorder()
	handler.HandleCallback(rec, req)

	// Second request with same state should fail
	req = httptest.NewRequest(http.MethodGet, "/auth/callback?state=valid-state&code=test-code", nil)
	rec = httptest.NewRecorder()
	handler.HandleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for reused state, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetAuthURL(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
	}

	handler := NewOAuthHandler(config, nil)

	authURL := handler.GetAuthURL("test-state")

	parsedURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse authorization URL: %v", err)
	}

	if parsedURL.Query().Get("state") != "test-state" {
		t.Errorf("expected state test-state, got %s", parsedURL.Query().Get("state"))
	}
}

func TestSetOnTokenFunc(t *testing.T) {
	handler := NewOAuthHandler(OAuthConfig{}, nil)

	handler.SetOnTokenFunc(func(ctx context.Context, token *oauth2.Token) error {
		return nil
	})

	if handler.onTokenFunc == nil {
		t.Error("expected onTokenFunc to be set")
	}
}

func TestDefaultScopes_ContainsRequiredAPIs(t *testing.T) {
	expectedScopes := []string{
		"presentations", // Slides API
		"drive",         // Drive API
		"translation",   // Translate API
	}

	for _, expected := range expectedScopes {
		found := false
		for _, scope := range DefaultScopes {
			if strings.Contains(scope, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected DefaultScopes to contain scope for %s", expected)
		}
	}
}

func TestGenerateState(t *testing.T) {
	state1, err := generateState()
	if err != nil {
		t.Fatalf("failed to generate state: %v", err)
	}

	state2, err := generateState()
	if err != nil {
		t.Fatalf("failed to generate state: %v", err)
	}

	if state1 == "" {
		t.Error("expected non-empty state")
	}

	if state1 == state2 {
		t.Error("expected unique states")
	}

	// State should be base64 URL encoded
	if len(state1) < 32 {
		t.Errorf("expected state length >= 32, got %d", len(state1))
	}
}
