package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuth2 scopes required for Google Slides, Drive, and Translate APIs.
var DefaultScopes = []string{
	"https://www.googleapis.com/auth/presentations",
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/cloud-translation",
}

// OAuthConfig holds OAuth2 configuration.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
}

// OAuthHandler handles OAuth2 authentication flow.
type OAuthHandler struct {
	config      *oauth2.Config
	logger      *slog.Logger
	states      map[string]bool // Track valid state tokens
	mu          sync.RWMutex
	onTokenFunc func(ctx context.Context, token *oauth2.Token) error
}

// NewOAuthHandler creates a new OAuth handler.
func NewOAuthHandler(config OAuthConfig, logger *slog.Logger) *OAuthHandler {
	if logger == nil {
		logger = slog.Default()
	}

	scopes := config.Scopes
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}

	return &OAuthHandler{
		config: oauth2Config,
		logger: logger,
		states: make(map[string]bool),
	}
}

// SetOnTokenFunc sets the callback function called when a token is obtained.
func (h *OAuthHandler) SetOnTokenFunc(fn func(ctx context.Context, token *oauth2.Token) error) {
	h.onTokenFunc = fn
}

// HandleAuth handles GET /auth and initiates the OAuth2 flow.
func (h *OAuthHandler) HandleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	state, err := generateState()
	if err != nil {
		h.logger.Error("failed to generate state", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	h.mu.Lock()
	h.states[state] = true
	h.mu.Unlock()

	authURL := h.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	h.logger.Info("OAuth2 flow initiated",
		slog.String("redirect_uri", h.config.RedirectURL),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"authorization_url": authURL,
		"message":           "Please visit the authorization URL to complete authentication",
	})
}

// HandleCallback handles GET /auth/callback with the OAuth2 authorization code.
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check for error from OAuth provider
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		h.logger.Error("OAuth2 error from provider",
			slog.String("error", errParam),
			slog.String("description", errDesc),
		)
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("OAuth2 error: %s - %s", errParam, errDesc))
		return
	}

	// Validate state parameter
	state := r.URL.Query().Get("state")
	if state == "" {
		h.writeError(w, http.StatusBadRequest, "missing state parameter")
		return
	}

	h.mu.Lock()
	validState := h.states[state]
	if validState {
		delete(h.states, state)
	}
	h.mu.Unlock()

	if !validState {
		h.writeError(w, http.StatusBadRequest, "invalid state parameter")
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		h.writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	// Exchange code for token
	token, err := h.config.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("failed to exchange code for token", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "failed to exchange code for token")
		return
	}

	h.logger.Info("OAuth2 token obtained",
		slog.Bool("has_refresh_token", token.RefreshToken != ""),
		slog.Time("expiry", token.Expiry),
	)

	// Call the token callback if set
	if h.onTokenFunc != nil {
		if err := h.onTokenFunc(r.Context(), token); err != nil {
			h.logger.Error("token callback failed", slog.Any("error", err))
			h.writeError(w, http.StatusInternalServerError, "failed to process token")
			return
		}
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"message": "Authentication successful",
		"expiry":  token.Expiry,
	}

	// Include refresh token status (but not the actual token for security)
	if token.RefreshToken != "" {
		response["has_refresh_token"] = true
	}

	json.NewEncoder(w).Encode(response)
}

// GetAuthURL returns the OAuth2 authorization URL with the given state.
func (h *OAuthHandler) GetAuthURL(state string) string {
	return h.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges an authorization code for tokens.
func (h *OAuthHandler) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return h.config.Exchange(ctx, code)
}

// RefreshToken refreshes an access token using a refresh token.
func (h *OAuthHandler) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}
	tokenSource := h.config.TokenSource(ctx, token)
	return tokenSource.Token()
}

// GetConfig returns the OAuth2 config (for testing).
func (h *OAuthHandler) GetConfig() *oauth2.Config {
	return h.config
}

// writeError writes an error response.
func (h *OAuthHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
