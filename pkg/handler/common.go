package handler

import (
	"net/http"
	"strings"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const DefaultJWTCookieKey = "jwt"
const DefaultNonceCookieKey = "nonce"

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

	if cookie, err := r.Cookie(DefaultJWTCookieKey); err == nil {
		return cookie.Value
	}

	return ""
}

func WithBearerToken(next http.Handler, bearerToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+bearerToken)
		next.ServeHTTP(w, r)
	})
}

func WithCFServiceToken(next http.Handler, cfCLIId string, cfSec string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("CF-Access-Client-Id", cfCLIId)
		r.Header.Set("CF-Access-Client-Secret", cfSec)
		next.ServeHTTP(w, r)
	})
}
