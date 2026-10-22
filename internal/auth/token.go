package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateSecureToken generates a cryptographically random, URL-safe base64 string.
func GenerateSecureToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken computes the SHA-256 hash of an opaque token string.
// The resulting 32-byte slice matches the BYTEA column in the refresh_sessions table.
func HashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
