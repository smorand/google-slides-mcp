package auth

import (
	"context"
	"fmt"
	"sync"
)

// MockAPIKeyStore is an in-memory implementation of APIKeyStoreInterface for testing.
type MockAPIKeyStore struct {
	records map[string]*APIKeyRecord
	mu      sync.RWMutex

	// Track method calls for assertions
	StoreCalls         int
	GetCalls           int
	UpdateLastUsedCall int
	DeleteCalls        int
	ExistsCalls        int

	// Optional error injection for testing error paths
	StoreError          error
	GetError            error
	UpdateLastUsedError error
	DeleteError         error
	ExistsError         error
}

// NewMockAPIKeyStore creates a new MockAPIKeyStore.
func NewMockAPIKeyStore() *MockAPIKeyStore {
	return &MockAPIKeyStore{
		records: make(map[string]*APIKeyRecord),
	}
}

// Store stores a new API key record.
func (m *MockAPIKeyStore) Store(ctx context.Context, record *APIKeyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StoreCalls++

	if m.StoreError != nil {
		return m.StoreError
	}

	// Make a copy to avoid mutation
	recordCopy := *record
	m.records[record.APIKey] = &recordCopy
	return nil
}

// Get retrieves an API key record.
func (m *MockAPIKeyStore) Get(ctx context.Context, apiKey string) (*APIKeyRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetCalls++

	if m.GetError != nil {
		return nil, m.GetError
	}

	record, ok := m.records[apiKey]
	if !ok {
		return nil, fmt.Errorf("API key not found")
	}

	// Return a copy
	recordCopy := *record
	return &recordCopy, nil
}

// UpdateLastUsed updates the last_used timestamp.
func (m *MockAPIKeyStore) UpdateLastUsed(ctx context.Context, apiKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateLastUsedCall++

	if m.UpdateLastUsedError != nil {
		return m.UpdateLastUsedError
	}

	record, ok := m.records[apiKey]
	if !ok {
		return fmt.Errorf("API key not found")
	}

	record.LastUsed = record.LastUsed.Add(1) // Just update it
	return nil
}

// Delete deletes an API key record.
func (m *MockAPIKeyStore) Delete(ctx context.Context, apiKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteCalls++

	if m.DeleteError != nil {
		return m.DeleteError
	}

	delete(m.records, apiKey)
	return nil
}

// Exists checks if an API key exists.
func (m *MockAPIKeyStore) Exists(ctx context.Context, apiKey string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.ExistsCalls++

	if m.ExistsError != nil {
		return false, m.ExistsError
	}

	_, ok := m.records[apiKey]
	return ok, nil
}

// Close is a no-op for the mock.
func (m *MockAPIKeyStore) Close() error {
	return nil
}

// Reset clears all records and counters.
func (m *MockAPIKeyStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records = make(map[string]*APIKeyRecord)
	m.StoreCalls = 0
	m.GetCalls = 0
	m.UpdateLastUsedCall = 0
	m.DeleteCalls = 0
	m.ExistsCalls = 0
	m.StoreError = nil
	m.GetError = nil
	m.UpdateLastUsedError = nil
	m.DeleteError = nil
	m.ExistsError = nil
}

// Ensure MockAPIKeyStore implements APIKeyStoreInterface.
var _ APIKeyStoreInterface = (*MockAPIKeyStore)(nil)
