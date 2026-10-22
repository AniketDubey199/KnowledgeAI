package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken       = errors.New("invalid or malformed token")
	ErrExpiredToken       = errors.New("token has expired")
	ErrInvalidClaims      = errors.New("token claims validation failed")
	ErrUnexpectedSignAlg  = errors.New("unexpected token signing algorithm")
)

// Claims represents the standard and custom JWT claims used across KnowledgeBase AI.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager handles issuance and verification of JWT access tokens.
type JWTManager struct {
	signingKey []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
}

// NewJWTManager creates an initialized JWTManager instance.
func NewJWTManager(signingKey string, issuer, audience string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{
		signingKey: []byte(signingKey),
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
	}
}

// GenerateAccessToken mints a new HMAC-SHA256 signed JWT for the given user.
func (m *JWTManager) GenerateAccessToken(userID uuid.UUID, email string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)

	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign access token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateAccessToken parses and cryptographically verifies an access token string.
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Enforce HMAC-SHA256 only to prevent algorithm substitution attacks (e.g. 'none' or RSA public key confusion)
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSignAlg, t.Header["alg"])
		}
		return m.signingKey, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	if claims.UserID == uuid.Nil {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}
