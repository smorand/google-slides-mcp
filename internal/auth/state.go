package auth

import (
	"crypto/rand"
	"encoding/base64"
)

const stateLength = 32

// generateState generates a cryptographically secure random state string.
func generateState() (string, error) {
	b := make([]byte, stateLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
