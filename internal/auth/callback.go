package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/oauth2"
)

// TokenCallbackConfig configures the token callback function.
type TokenCallbackConfig struct {
	Store  APIKeyStoreInterface
	Logger *slog.Logger
}

// NewAPIKeyCallback creates a token callback function that generates and stores API keys.
// This function is intended to be used with OAuthHandler.SetOnTokenFuncWithResult.
func NewAPIKeyCallback(config TokenCallbackConfig) func(ctx context.Context, token *oauth2.Token) (*TokenCallbackResult, error) {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(ctx context.Context, token *oauth2.Token) (*TokenCallbackResult, error) {
		// Check for refresh token - required for API key generation
		if token.RefreshToken == "" {
			return nil, fmt.Errorf("no refresh token received; cannot generate API key")
		}

		// Generate UUID-format API key
		apiKey, err := GenerateAPIKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API key: %w", err)
		}

		// Create record for Firestore
		now := time.Now()
		record := &APIKeyRecord{
			APIKey:       apiKey,
			RefreshToken: token.RefreshToken,
			CreatedAt:    now,
			LastUsed:     now,
		}

		// Store in Firestore
		if err := config.Store.Store(ctx, record); err != nil {
			return nil, fmt.Errorf("failed to store API key: %w", err)
		}

		logger.Info("API key generated and stored",
			slog.String("api_key_prefix", apiKey[:8]+"..."),
		)

		return &TokenCallbackResult{
			APIKey: apiKey,
		}, nil
	}
}
