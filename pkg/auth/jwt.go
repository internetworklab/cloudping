package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaimType struct {
	jwt.RegisteredClaims
	Username string `json:"username,omitempty"`
}

type JWTIssuer interface {
	IssueToken(ctx context.Context, mapClaims jwt.MapClaims) (string, error)
}

type JWTValidator interface {
	ValidateToken(ctx context.Context, token string) (valid bool, reason string, err error)

	// the second return value is claim of custom claim type
	ParseToken(ctx context.Context, token string) (*jwt.RegisteredClaims, any, error)
}

type StaticKeyJWTValidator struct {
	secretProvider SecretProvider
	blacklist      BlackListProvider
}

func NewStaticKeyJWTValidator(secretProvider SecretProvider, blacklist BlackListProvider) *StaticKeyJWTValidator {
	return &StaticKeyJWTValidator{
		secretProvider: secretProvider,
		blacklist:      blacklist,
	}
}

func (s *StaticKeyJWTValidator) ParseToken(ctx context.Context, tokenString string) (*jwt.RegisteredClaims, any, error) {
	_, claims, _, err := s.doValidateToken(ctx, tokenString)
	if err != nil {
		return nil, nil, fmt.Errorf("internal error, can not parse token: %w", err)
	}
	if claims == nil {
		return nil, nil, fmt.Errorf("token is nil or invalid")
	}
	return &claims.RegisteredClaims, claims, nil
}

func (s *StaticKeyJWTValidator) checkBL(ctx context.Context, subj string) bool {
	hit, err := s.blacklist.CheckBlackList(ctx, subj)
	if err != nil {
		log.Printf("blacklist provider returned an error: %v", err)
		return true
	}

	return hit
}

// returns: (valid, claims, reason, error)
func (s *StaticKeyJWTValidator) doValidateToken(ctx context.Context, tokenString string) (*jwt.Token, *CustomClaimType, string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaimType{}, s.secretProvider.GetSecret, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, nil, fmt.Sprintf("Invalid token: %s", err.Error()), nil
	}
	if token == nil || !token.Valid {
		return nil, nil, "Invalid token: token is nil or invalid", nil
	}

	if claims, ok := token.Claims.(*CustomClaimType); ok && claims != nil {
		if s.checkBL(ctx, claims.Subject) {
			return nil, nil, fmt.Sprintf("Token is blacklisted, subj=%s", claims.Subject), nil
		}

		return token, claims, "", nil
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && claims != nil {
		if s.checkBL(ctx, claims.Subject) {
			return nil, nil, fmt.Sprintf("Token is blacklisted, subj=%s", claims.Subject), nil
		}

		return token, &CustomClaimType{RegisteredClaims: *claims}, "", nil
	}

	return nil, nil, "Invalid token, can not parse a *jwt.RegisteredClaims", nil
}

// returns: (valid, reason, error)
func (s *StaticKeyJWTValidator) ValidateToken(ctx context.Context, tokenString string) (bool, string, error) {
	token, _, reason, err := s.doValidateToken(ctx, tokenString)
	if err != nil {
		return false, "", fmt.Errorf("internal error, can not validate token: %w", err)
	}
	if token == nil {
		return false, reason, nil
	}
	return true, "", nil
}

type SecretProvider interface {
	// `GetSecret` returns the signing key for the given token.
	// Implementations must handle `token` being `nil` (used during signing).
	GetSecret(token *jwt.Token) (any, error)
}

type StaticSecretProvider struct {
	secret []byte
}

func NewStaticSecretProvider(secret []byte) *StaticSecretProvider {
	return &StaticSecretProvider{secret: secret}
}

func (provider *StaticSecretProvider) GetSecret(_ *jwt.Token) (any, error) {
	return provider.secret, nil
}

type StaticKeyJWTIssuer struct {
	issuer         string
	secretProvider SecretProvider
}

// pass a validity of 0 to use default validity
func NewStaticKeyJWTIssuer(secretProvider SecretProvider, issuer string) *StaticKeyJWTIssuer {

	return &StaticKeyJWTIssuer{
		issuer:         issuer,
		secretProvider: secretProvider,
	}
}

const AudSession string = "session"

func (s *StaticKeyJWTIssuer) IssueToken(ctx context.Context, mapClaims jwt.MapClaims) (string, error) {
	if s.issuer == "" {
		return "", errors.New("issuer is not specified, can not sign token")
	}

	secret, err := s.secretProvider.GetSecret(nil)
	if err != nil {
		return "", fmt.Errorf("failed to get secret to sign the token: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	return token.SignedString(secret)
}

func NewMapClaims(customClaims *CustomClaimType) (jwt.MapClaims, error) {
	tmpBuf := &bytes.Buffer{}
	if err := json.NewEncoder(tmpBuf).Encode(customClaims); err != nil {
		return nil, fmt.Errorf("failed to encode registered claims: %w", err)
	}

	var mapClaims jwt.MapClaims
	if err := json.NewDecoder(tmpBuf).Decode(&mapClaims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal map claims: %w", err)
	}

	return mapClaims, nil
}
