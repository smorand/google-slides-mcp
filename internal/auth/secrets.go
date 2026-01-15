package auth

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// SecretLoader loads secrets from Google Secret Manager.
type SecretLoader struct {
	client    *secretmanager.Client
	projectID string
}

// NewSecretLoader creates a new SecretLoader.
func NewSecretLoader(ctx context.Context, projectID string) (*SecretLoader, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &SecretLoader{
		client:    client,
		projectID: projectID,
	}, nil
}

// Close closes the secret manager client.
func (l *SecretLoader) Close() error {
	return l.client.Close()
}

// GetSecret retrieves a secret value by its ID.
func (l *SecretLoader) GetSecret(ctx context.Context, secretID string) (string, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", l.projectID, secretID)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := l.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", secretID, err)
	}

	return string(result.Payload.Data), nil
}

// LoadOAuthConfig loads OAuth configuration from Secret Manager.
func (l *SecretLoader) LoadOAuthConfig(ctx context.Context, clientIDSecret, clientSecretSecret, redirectURISecret string) (*OAuthConfig, error) {
	clientID, err := l.GetSecret(ctx, clientIDSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to load client ID: %w", err)
	}

	clientSecret, err := l.GetSecret(ctx, clientSecretSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to load client secret: %w", err)
	}

	redirectURI, err := l.GetSecret(ctx, redirectURISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to load redirect URI: %w", err)
	}

	return &OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Scopes:       DefaultScopes,
	}, nil
}

// OAuthConfigFromEnv creates an OAuthConfig from environment values.
// This is useful for local development and testing.
func OAuthConfigFromEnv(clientID, clientSecret, redirectURI string) *OAuthConfig {
	return &OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Scopes:       DefaultScopes,
	}
}
