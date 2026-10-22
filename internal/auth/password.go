package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidPasswordHash = errors.New("invalid or unparseable password hash")
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
)

// Argon2Params encapsulates configuration for Argon2id hashing.
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Params adheres to OWASP guidelines for Argon2id password hashing.
var DefaultArgon2Params = Argon2Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  1,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword hashes a plaintext password using Argon2id with cryptographically random salt.
func HashPassword(password string) (string, error) {
	return HashPasswordWithParams(password, DefaultArgon2Params)
}

// HashPasswordWithParams hashes a plaintext password using specific Argon2 parameters.
func HashPasswordWithParams(password string, params Argon2Params) (string, error) {
	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$<b64salt>$<b64hash>
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// VerifyPassword securely verifies a plaintext password against an Argon2id PHC-encoded hash string.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, ErrInvalidPasswordHash
	}

	if parts[1] != "argon2id" {
		return false, errors.New("unsupported hash algorithm")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidPasswordHash
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, ErrInvalidPasswordHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidPasswordHash
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidPasswordHash
	}

	keyLen := uint32(len(expectedHash))
	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLen,
	)

	if subtle.ConstantTimeCompare(expectedHash, computedHash) == 1 {
		return true, nil
	}

	return false, nil
}
