package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/internetworklab/cloudping/pkg/session"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const defaultJWTCookieKey = "jwt"

func extractJWTFromRequest(r *http.Request) string {
	tokenFromCtx := r.Context().Value(pkgutils.CtxKeyJustIssuedJWTToken)
	if tokenFromCtx != nil {
		return tokenFromCtx.(string)
	}

	tokenString := r.Header.Get("Authorization")
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimPrefix(tokenString, "bearer ")

	if tokenString != "" {
		return tokenString
	}

	if cookie, err := r.Cookie(defaultJWTCookieKey); err == nil {
		return cookie.Value
	}

	return ""
}

func QuicValidateJWT(tokenString *string, secret []byte) (bool, *jwt.Token, error) {
	if tokenString == nil {
		return false, nil, fmt.Errorf("token string is nil")
	}
	token, err := jwt.ParseWithClaims(*tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		// in future, one should determine which key to use base on the 'kid' (key ID) claim of the token
		// for now, return a fixed key is enough, becuase the people who use our service can be counted on one hand.
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		return false, nil, fmt.Errorf("failed to parse JWT: %v", err)
	}

	if token == nil {
		return false, nil, fmt.Errorf("couldn't get JWT token")
	}

	if !token.Valid {
		return false, nil, fmt.Errorf("invalid JWT")
	}

	return true, token, nil
}

func WithJWTAuth(handler http.Handler, secret []byte, rejectInvalid bool) http.Handler {
	if secret == nil {
		panic("WithJWTAuth: JWT secret must not be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractJWTFromRequest(r)

		rejectWithErr := func(nextHandler http.Handler, rejectInvalid bool) {
			if rejectInvalid {
				unAuthErr := pkgutils.ErrorResponse{Error: "Unauthorized"}
				remote := pkgutils.GetRemoteAddr(r)
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(unAuthErr)
				log.Printf("Remote peer %s is rejected by JWT middleware", remote)
			} else {
				nextHandler.ServeHTTP(w, r)
			}
		}

		if tokenString == "" {
			rejectWithErr(handler, rejectInvalid)
			return
		}

		if len(secret) < 4 {
			log.Printf("WARN: JWT middleware is applied but JWT secret is too short, is that reliable ? (")
		}

		valid, token, err := QuicValidateJWT(&tokenString, secret)
		if err != nil || !valid || token == nil {
			rejectWithErr(handler, rejectInvalid)
			return
		}

		ctx := r.Context()
		if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
			if claims.ID != "" {
				ctx = context.WithValue(ctx, pkgutils.CtxKeySessionId, claims.ID)
			}
			if claims.Subject != "" {
				ctx = context.WithValue(ctx, pkgutils.CtxKeySubjectId, claims.Subject)
			}
		} else {
			// if the user is authenticated, and the type of claims map isn't like what is expected,
			// it's definitely something wrong with the code, so it's a unrecoverable error.
			log.Panicf("the token claims map wasn't parsed as *jwt.RegisteredClaims! it's %T", token.Claims)
		}

		r = r.WithContext(ctx)

		handler.ServeHTTP(w, r)
	})
}

type JWTIssuer interface {
	IssueToken(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, error)

	RefreshToken(ctx context.Context, w http.ResponseWriter, r *http.Request, currentToken string) (renewed bool, token string, err error)

	ValidateToken(ctx context.Context, token string) (valid bool, reason string, err error)
}

func WithJWTCookieIssue(handler http.Handler, issuer JWTIssuer, allowAnonymous bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractJWTFromRequest(r)

		ctx := r.Context()

		var err error
		if tokenString == "" {
			if !allowAnonymous {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("No valid token is present and allowAnonymous is set to %v", allowAnonymous)})
				return
			}
			tokenString, err = issuer.IssueToken(ctx, w, r)
			if err != nil {
				log.Printf("WithJWTCookieIssue: remote %s failed to issue token: %v", pkgutils.GetRemoteAddr(r), err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "failed to issue token"})
				return
			}
			ctx = context.WithValue(ctx, pkgutils.CtxKeyJustIssuedJWTToken, tokenString)
			log.Printf("WithJWTCookieIssue: remote %s issued token", pkgutils.GetRemoteAddr(r))
		} else {
			valid, reason, err := issuer.ValidateToken(ctx, tokenString)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("Can not validate token, internal error: %v", err)})
				return
			}
			if !valid && !allowAnonymous {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("No valid token is present and allowAnonymous is set to %v, details: %s", allowAnonymous, reason)})
				return
			}
			var renewed bool
			renewed, tokenString, err = issuer.RefreshToken(ctx, w, r, tokenString)
			if err != nil {
				log.Printf("WithJWTCookieIssue: remote %s failed to refresh token: %v", pkgutils.GetRemoteAddr(r), err)
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "failed to renew token"})
				return
			}
			if renewed {
				ctx = context.WithValue(ctx, pkgutils.CtxKeyJustIssuedJWTToken, tokenString)
				log.Printf("WithJWTCookieIssue: remote %s renewed token", pkgutils.GetRemoteAddr(r))
			}
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

type SessionBasedJWTIssuer struct {
	sessionManager session.SessionManager
	secret         []byte
	issuer         string
	cookieModifier func(*http.Cookie) *http.Cookie
}

func NewSessionBasedJWTIssuer(sessionManager session.SessionManager, secret []byte, issuer string, cookieModifier func(*http.Cookie) *http.Cookie) *SessionBasedJWTIssuer {
	return &SessionBasedJWTIssuer{
		sessionManager: sessionManager,
		secret:         secret,
		issuer:         issuer,
		cookieModifier: cookieModifier,
	}
}

const AudSession string = "session"

func (s *SessionBasedJWTIssuer) signTokenFromDescriptor(descriptor *session.SessionDescriptor) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    s.issuer,
		IssuedAt:  jwt.NewNumericDate(descriptor.StartedAt),
		ExpiresAt: jwt.NewNumericDate(descriptor.ExpiredAt),
		ID:        descriptor.Id,
		Audience:  jwt.ClaimStrings{AudSession},
	})
	return token.SignedString(s.secret)
}

func (s *SessionBasedJWTIssuer) setJWTCookie(w http.ResponseWriter, tokenString string) {
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

func (s *SessionBasedJWTIssuer) IssueToken(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, error) {
	descriptor, err := s.sessionManager.CreateSession(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}

	tokenString, err := s.signTokenFromDescriptor(descriptor)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	s.setJWTCookie(w, tokenString)
	return tokenString, nil
}

// returns: (valid, reason, error)
func (s *SessionBasedJWTIssuer) ValidateToken(ctx context.Context, tokenString string) (bool, string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return false, "Invalid token", nil
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.ID == "" {
		return false, "Invalid token, not a *jwt.RegisteredClaims", nil
	}

	if !s.sessionManager.ValidateSession(ctx, claims.ID) {
		return false, "Session is expired", nil
	}

	return true, "", nil
}

func (s *SessionBasedJWTIssuer) RefreshToken(ctx context.Context, w http.ResponseWriter, r *http.Request, currentToken string) (bool, string, error) {
	token, err := jwt.ParseWithClaims(currentToken, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		token, err := s.IssueToken(ctx, w, r)
		return true, token, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.ID == "" {
		token, err := s.IssueToken(ctx, w, r)
		return true, token, err
	}

	if slices.Index([]string(claims.Audience), AudSession) == -1 {
		return false, currentToken, nil
	}

	if !s.sessionManager.ValidateSession(ctx, claims.ID) {
		token, err := s.IssueToken(ctx, w, r)
		return true, token, err
	}

	return false, currentToken, nil
}
