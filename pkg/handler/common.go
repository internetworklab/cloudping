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
