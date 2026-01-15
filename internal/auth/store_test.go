package auth

import (
	"context"
	"testing"
	"time"
)

func TestMockAPIKeyStore_Store(t *testing.T) {
	store := NewMockAPIKeyStore()
	ctx := context.Background()

	record := &APIKeyRecord{
		APIKey:       "test-api-key-12345678",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}

	err := store.Store(ctx, record)
	if err != nil {
		t.Fatalf("failed to store record: %v", err)
	}

	if store.StoreCalls != 1 {
		t.Errorf("expected 1 store call, got %d", store.StoreCalls)
	}
}

func TestMockAPIKeyStore_Get(t *testing.T) {
	store := NewMockAPIKeyStore()
	ctx := context.Background()

	// Store a record
	original := &APIKeyRecord{
		APIKey:       "test-api-key-12345678",
		RefreshToken: "test-refresh-token",
		UserEmail:    "test@example.com",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}

	err := store.Store(ctx, original)
	if err != nil {
		t.Fatalf("failed to store record: %v", err)
	}

	// Get the record
	retrieved, err := store.Get(ctx, original.APIKey)
	if err != nil {
		t.Fatalf("failed to get record: %v", err)
	}

	if retrieved.APIKey != original.APIKey {
		t.Errorf("expected API key %s, got %s", original.APIKey, retrieved.APIKey)
	}

	if retrieved.RefreshToken != original.RefreshToken {
		t.Errorf("expected refresh token %s, got %s", original.RefreshToken, retrieved.RefreshToken)
	}

	if retrieved.UserEmail != original.UserEmail {
		t.Errorf("expected email %s, got %s", original.UserEmail, retrieved.UserEmail)
	}
}

func TestMockAPIKeyStore_Get_NotFound(t *testing.T) {
	store := NewMockAPIKeyStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent-key")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestMockAPIKeyStore_Exists(t *testing.T) {
	store := NewMockAPIKeyStore()
	ctx := context.Background()

	// Check nonexistent key
	exists, err := store.Exists(ctx, "nonexistent-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected key to not exist")
	}

	// Store a record
	record := &APIKeyRecord{
		APIKey:       "test-api-key-12345678",
		RefreshToken: "test-refresh-token",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	err = store.Store(ctx, record)
	if err != nil {
		t.Fatalf("failed to store record: %v", err)
	}

	// Check existing key
	exists, err = store.Exists(ctx, record.APIKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}
}

func TestMockAPIKeyStore_Delete(t *testing.T) {
	store := NewMockAPIKeyStore()
	ctx := context.Background()

	// Store a record
	record := &APIKeyRecord{
		APIKey:       "test-api-key-12345678",
		RefreshToken: "test-refresh-token",
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
	}
	err := store.Store(ctx, record)
	if err != nil {
		t.Fatalf("failed to store record: %v", err)
	}

	// Delete the record
	err = store.Delete(ctx, record.APIKey)
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}

	// Verify it's deleted
	exists, err := store.Exists(ctx, record.APIKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected key to be deleted")
	}
}

func TestMockAPIKeyStore_ErrorInjection(t *testing.T) {
	store := NewMockAPIKeyStore()
	ctx := context.Background()

	// Test store error
	store.StoreError = context.DeadlineExceeded
	record := &APIKeyRecord{
		APIKey: "test-key",
	}
	err := store.Store(ctx, record)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got %v", err)
	}

	// Reset and test get error
	store.Reset()
	store.GetError = context.Canceled
	_, err = store.Get(ctx, "test-key")
	if err != context.Canceled {
		t.Errorf("expected Canceled error, got %v", err)
	}
}

func TestAPIKeyRecord_Fields(t *testing.T) {
	now := time.Now()
	record := &APIKeyRecord{
		APIKey:       "test-api-key",
		RefreshToken: "test-refresh-token",
		UserEmail:    "user@example.com",
		CreatedAt:    now,
		LastUsed:     now,
	}

	if record.APIKey != "test-api-key" {
		t.Errorf("expected API key 'test-api-key', got %s", record.APIKey)
	}

	if record.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh token 'test-refresh-token', got %s", record.RefreshToken)
	}

	if record.UserEmail != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got %s", record.UserEmail)
	}

	if !record.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, record.CreatedAt)
	}

	if !record.LastUsed.Equal(now) {
		t.Errorf("expected LastUsed %v, got %v", now, record.LastUsed)
	}
}
