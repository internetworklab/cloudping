package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
)

const defaultJWTCookieKey = "jwt"

func extractJWTFromRequest(r *http.Request) string {
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
	token, err := jwt.Parse(*tokenString, func(token *jwt.Token) (any, error) {
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
		ctx = context.WithValue(ctx, pkgutils.CtxKeyJWTToken, token)
		r = r.WithContext(ctx)

		handler.ServeHTTP(w, r)
	})
}
