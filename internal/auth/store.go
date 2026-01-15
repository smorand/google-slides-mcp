package auth

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

// APIKeyRecord represents an API key stored in Firestore.
type APIKeyRecord struct {
	APIKey       string    `firestore:"api_key"`
	RefreshToken string    `firestore:"refresh_token"`
	UserEmail    string    `firestore:"user_email,omitempty"`
	CreatedAt    time.Time `firestore:"created_at"`
	LastUsed     time.Time `firestore:"last_used"`
}

// APIKeyStore handles storage of API keys in Firestore.
type APIKeyStore struct {
	client     *firestore.Client
	collection string
}

// NewAPIKeyStore creates a new APIKeyStore.
func NewAPIKeyStore(ctx context.Context, projectID, collection string) (*APIKeyStore, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &APIKeyStore{
		client:     client,
		collection: collection,
	}, nil
}

// NewAPIKeyStoreWithClient creates a new APIKeyStore with an existing Firestore client.
// This is useful for testing and dependency injection.
func NewAPIKeyStoreWithClient(client *firestore.Client, collection string) *APIKeyStore {
	return &APIKeyStore{
		client:     client,
		collection: collection,
	}
}

// Close closes the Firestore client.
func (s *APIKeyStore) Close() error {
	return s.client.Close()
}

// Store stores a new API key record in Firestore.
// The document ID is the API key itself for fast lookups.
func (s *APIKeyStore) Store(ctx context.Context, record *APIKeyRecord) error {
	_, err := s.client.Collection(s.collection).Doc(record.APIKey).Set(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to store API key: %w", err)
	}
	return nil
}

// Get retrieves an API key record from Firestore.
func (s *APIKeyStore) Get(ctx context.Context, apiKey string) (*APIKeyRecord, error) {
	doc, err := s.client.Collection(s.collection).Doc(apiKey).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	var record APIKeyRecord
	if err := doc.DataTo(&record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API key record: %w", err)
	}

	return &record, nil
}

// UpdateLastUsed updates the last_used timestamp for an API key.
func (s *APIKeyStore) UpdateLastUsed(ctx context.Context, apiKey string) error {
	_, err := s.client.Collection(s.collection).Doc(apiKey).Update(ctx, []firestore.Update{
		{Path: "last_used", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to update last_used: %w", err)
	}
	return nil
}

// Delete deletes an API key record from Firestore.
func (s *APIKeyStore) Delete(ctx context.Context, apiKey string) error {
	_, err := s.client.Collection(s.collection).Doc(apiKey).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

// Exists checks if an API key exists in Firestore.
func (s *APIKeyStore) Exists(ctx context.Context, apiKey string) (bool, error) {
	doc, err := s.client.Collection(s.collection).Doc(apiKey).Get(ctx)
	if err != nil {
		// Check if error is "not found"
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check API key existence: %w", err)
	}
	return doc.Exists(), nil
}

// isNotFoundError checks if the error is a Firestore "not found" error.
func isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "rpc error: code = NotFound desc = Document not found" ||
		err.Error() == "rpc error: code = NotFound desc = document not found" ||
		err.Error() == "document not found")
}

// APIKeyStoreInterface defines the interface for API key storage.
// This allows for easy mocking in tests.
type APIKeyStoreInterface interface {
	Store(ctx context.Context, record *APIKeyRecord) error
	Get(ctx context.Context, apiKey string) (*APIKeyRecord, error)
	UpdateLastUsed(ctx context.Context, apiKey string) error
	Delete(ctx context.Context, apiKey string) error
	Exists(ctx context.Context, apiKey string) (bool, error)
	Close() error
}

// Ensure APIKeyStore implements APIKeyStoreInterface.
var _ APIKeyStoreInterface = (*APIKeyStore)(nil)
