package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTIssuer interface {
	IssueToken(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, error)

	RefreshToken(ctx context.Context, w http.ResponseWriter, r *http.Request, currentToken string) (renewed bool, token string, err error)
}

type JWTValidator interface {
	ValidateToken(ctx context.Context, token string) (valid bool, reason string, err error)

	ParseToken(ctx context.Context, token string) (*jwt.RegisteredClaims, error)
}

type StaticKeyJWTValidator struct {
	secretProvider SecretProvider
}

func NewStaticKeyJWTValidator(secretProvider SecretProvider) *StaticKeyJWTValidator {
	return &StaticKeyJWTValidator{
		secretProvider: secretProvider,
	}
}

func (s *StaticKeyJWTValidator) ParseToken(ctx context.Context, tokenString string) (*jwt.RegisteredClaims, error) {
	_, claims, _, err := s.doValidateToken(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("internal error, can not parse token: %w", err)
	}
	if claims == nil {
		return nil, fmt.Errorf("token is nil or invalid")
	}
	return claims, nil
}

// returns: (valid, claims, reason, error)
func (s *StaticKeyJWTValidator) doValidateToken(_ context.Context, tokenString string) (*jwt.Token, *jwt.RegisteredClaims, string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, s.secretProvider.GetSecret, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, nil, fmt.Sprintf("Invalid token: %s", err.Error()), nil
	}
	if token == nil || !token.Valid {
		return nil, nil, "Invalid token: token is nil or invalid", nil
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.ID == "" {
		return nil, nil, "Invalid token, can not parse a *jwt.RegisteredClaims", nil
	}

	return token, claims, "", nil
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

const defaultValidity time.Duration = 7 * 24 * 60 * 60 * time.Second

type StaticKeyJWTIssuer struct {
	issuer         string
	cookieModifier func(*http.Cookie) *http.Cookie
	validator      JWTValidator
	secretProvider SecretProvider
	validity       time.Duration
}

// pass a validity of 0 to use default validity
func NewStaticKeyJWTIssuer(secretProvider SecretProvider, validity time.Duration, issuer string, cookieModifier func(*http.Cookie) *http.Cookie) *StaticKeyJWTIssuer {
	if validity == 0 {
		validity = defaultValidity
	}
	return &StaticKeyJWTIssuer{
		issuer:         issuer,
		cookieModifier: cookieModifier,
		validator:      NewStaticKeyJWTValidator(secretProvider),
		secretProvider: secretProvider,
		validity:       validity,
	}
}

const AudSession string = "session"

func (s *StaticKeyJWTIssuer) signShortLiveToken(notBefore time.Time, notAfter time.Time) (string, error) {
	if s.issuer == "" {
		return "", errors.New("issuer is not specified, can not sign token")
	}

	secret, err := s.secretProvider.GetSecret(nil)
	if err != nil {
		return "", fmt.Errorf("failed to get secret to sign the token: %w", err)
	}

	randomId := uuid.NewString()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    s.issuer,
		IssuedAt:  jwt.NewNumericDate(notBefore),
		ExpiresAt: jwt.NewNumericDate(notAfter),
		ID:        randomId,
		Audience:  jwt.ClaimStrings{AudSession},
	})
	return token.SignedString(secret)
}

const defaultJWTCookieKey = "jwt"

func (s *StaticKeyJWTIssuer) setJWTCookie(w http.ResponseWriter, tokenString string) {
	cookie := &http.Cookie{
		Name:     defaultJWTCookieKey,
		Value:    tokenString,
		HttpOnly: true,
		Path:     "/",
	}
	if s.cookieModifier != nil {
		cookie = s.cookieModifier(cookie)
	}
	http.SetCookie(w, cookie)
}

func (s *StaticKeyJWTIssuer) IssueToken(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, error) {
	now := time.Now()

	if s.validity == 0 {
		return "", fmt.Errorf("validity is 0, cannot issue token")
	}

	tokenString, err := s.signShortLiveToken(now, now.Add(s.validity))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	s.setJWTCookie(w, tokenString)
	return tokenString, nil
}

// the token will be refreshed only when it's deemed to be invalid
func (s *StaticKeyJWTIssuer) RefreshToken(ctx context.Context, w http.ResponseWriter, r *http.Request, currentToken string) (bool, string, error) {
	valid, _, err := s.validator.ValidateToken(ctx, currentToken)
	if err != nil {
		return false, "", fmt.Errorf("failed to refresh token, invalid error: %w", err)
	}

	if !valid {
		token, err := s.IssueToken(ctx, w, r)
		return true, token, err
	}

	return false, currentToken, nil
}
