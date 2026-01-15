package auth

import (
	"regexp"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	apiKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("failed to generate API key: %v", err)
	}

	if apiKey == "" {
		t.Error("expected non-empty API key")
	}

	// Check UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(apiKey) {
		t.Errorf("API key does not match UUID v4 format: %s", apiKey)
	}
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	numKeys := 100

	for i := 0; i < numKeys; i++ {
		apiKey, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("failed to generate API key %d: %v", i, err)
		}

		if keys[apiKey] {
			t.Errorf("duplicate API key generated: %s", apiKey)
		}
		keys[apiKey] = true
	}
}

func TestGenerateAPIKey_UUIDv4Version(t *testing.T) {
	apiKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("failed to generate API key: %v", err)
	}

	// UUID v4 has version '4' at position 14 (0-indexed)
	if apiKey[14] != '4' {
		t.Errorf("expected UUID version 4 at position 14, got %c", apiKey[14])
	}
}

func TestGenerateAPIKey_UUIDv4Variant(t *testing.T) {
	apiKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("failed to generate API key: %v", err)
	}

	// UUID v4 has variant bits (8, 9, a, or b) at position 19 (0-indexed)
	variantChar := apiKey[19]
	if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
		t.Errorf("expected UUID variant char (8, 9, a, or b) at position 19, got %c", variantChar)
	}
}
