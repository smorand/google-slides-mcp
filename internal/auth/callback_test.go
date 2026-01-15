package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestNewAPIKeyCallback_Success(t *testing.T) {
	store := NewMockAPIKeyStore()
	callback := NewAPIKeyCallback(TokenCallbackConfig{
		Store: store,
	})

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	result, err := callback(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.APIKey == "" {
		t.Error("expected API key in result")
	}

	// Verify UUID format
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(result.APIKey) {
		t.Errorf("API key does not match UUID v4 format: %s", result.APIKey)
	}

	// Verify it was stored
	if store.StoreCalls != 1 {
		t.Errorf("expected 1 store call, got %d", store.StoreCalls)
	}
}

func TestNewAPIKeyCallback_NoRefreshToken(t *testing.T) {
	store := NewMockAPIKeyStore()
	callback := NewAPIKeyCallback(TokenCallbackConfig{
		Store: store,
	})

	token := &oauth2.Token{
		AccessToken: "test-access-token",
		// No refresh token
		Expiry: time.Now().Add(time.Hour),
	}

	result, err := callback(context.Background(), token)
	if err == nil {
		t.Error("expected error when no refresh token")
	}

	if result != nil {
		t.Error("expected nil result on error")
	}

	// Verify nothing was stored
	if store.StoreCalls != 0 {
		t.Errorf("expected 0 store calls, got %d", store.StoreCalls)
	}
}

func TestNewAPIKeyCallback_StoreError(t *testing.T) {
	store := NewMockAPIKeyStore()
	store.StoreError = errors.New("firestore unavailable")

	callback := NewAPIKeyCallback(TokenCallbackConfig{
		Store: store,
	})

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	result, err := callback(context.Background(), token)
	if err == nil {
		t.Error("expected error when store fails")
	}

	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestNewAPIKeyCallback_RecordContainsRequiredFields(t *testing.T) {
	store := NewMockAPIKeyStore()
	callback := NewAPIKeyCallback(TokenCallbackConfig{
		Store: store,
	})

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token-xyz",
		Expiry:       time.Now().Add(time.Hour),
	}

	result, err := callback(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the stored record
	record, err := store.Get(context.Background(), result.APIKey)
	if err != nil {
		t.Fatalf("failed to get stored record: %v", err)
	}

	// Verify required fields
	if record.APIKey != result.APIKey {
		t.Errorf("expected API key %s, got %s", result.APIKey, record.APIKey)
	}

	if record.RefreshToken != "test-refresh-token-xyz" {
		t.Errorf("expected refresh token 'test-refresh-token-xyz', got %s", record.RefreshToken)
	}

	if record.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	if record.LastUsed.IsZero() {
		t.Error("expected non-zero LastUsed")
	}

	// CreatedAt and LastUsed should be the same initially
	if !record.CreatedAt.Equal(record.LastUsed) {
		t.Error("expected CreatedAt and LastUsed to be equal initially")
	}
}

func TestNewAPIKeyCallback_NilStore(t *testing.T) {
	// This should panic or fail gracefully
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior - panic on nil store
		}
	}()

	callback := NewAPIKeyCallback(TokenCallbackConfig{
		Store: nil,
	})

	token := &oauth2.Token{
		RefreshToken: "test",
	}

	// This should fail
	callback(context.Background(), token)
}
