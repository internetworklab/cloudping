package auth

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// The primary purpose of NonceIssuer is to generate some verifiable signed string with sufficient randomness
type NonceIssuer interface {
	IssueNonce(ctx context.Context) (string, error)
	ValidateNonce(ctx context.Context, nonce string) (bool, error)
}

type StaticKeyNonceIssuer struct {
	NonceLifespan  time.Duration
	SecretProvider SecretProvider
}

const defaultRandomness = 16

type nonceClaimType struct {
	jwt.RegisteredClaims
	RandomValue string `json:"random_value"`
}

func (h *StaticKeyNonceIssuer) IssueNonce(ctx context.Context) (string, error) {
	claims := &nonceClaimType{}
	now := time.Now()
	notAfter := now.Add(h.NonceLifespan)
	claims.NotBefore = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(notAfter)
	randomBytes := make([]byte, defaultRandomness)
	cryptoRand.Read(randomBytes)
	claims.RandomValue = base64.StdEncoding.EncodeToString(randomBytes)

	secret, err := h.SecretProvider.GetSecret(nil)
	if err != nil {
		return "", fmt.Errorf("failed to get secret to sign the token: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func (h *StaticKeyNonceIssuer) ValidateNonce(ctx context.Context, nonce string) (bool, error) {
	token, err := jwt.ParseWithClaims(nonce, &nonceClaimType{}, h.SecretProvider.GetSecret, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return false, fmt.Errorf("Failed to parse token: %w", err)
	}
	if token == nil || !token.Valid {
		return false, fmt.Errorf("Invalid token: token is nil or invalid")
	}

	return true, nil
}
