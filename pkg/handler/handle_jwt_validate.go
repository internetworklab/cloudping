package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func WithJWTAuth(nextHandler http.Handler, jwtValidator pkgauth.JWTValidator, onRejectHandler http.Handler) http.Handler {
	if jwtValidator == nil {
		panic("WithJWTAuth: JWTValidator secret must not be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractJWTFromRequest(r)

		rejectWithErr := func(additionalMsg string) {
			if onRejectHandler != nil {
				onRejectHandler.ServeHTTP(w, r)
				return
			}

			unAuthErr := pkgutils.ErrorResponse{Error: fmt.Sprintf("Unauthorized: %s", additionalMsg)}
			remote := pkgutils.GetRemoteAddr(r)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(unAuthErr)
			log.Printf("Remote peer %s is rejected by JWT middleware", remote)
		}

		ctx := r.Context()
		claims, customClaimsAny, err := jwtValidator.ParseToken(ctx, tokenString)
		if err != nil {
			rejectWithErr(err.Error())
			return
		} else if claims == nil {
			rejectWithErr("Got nil token")
			return
		}

		if customClaimsAny != nil {
			if customClaims, ok := customClaimsAny.(*pkgauth.CustomClaimType); ok && customClaims != nil {
				ctx = context.WithValue(ctx, pkgutils.CtxKeyUsername, customClaims.Username)
			}
		}

		if claims.ID != "" {
			ctx = context.WithValue(ctx, pkgutils.CtxKeySessionId, claims.ID)
		}

		if claims.Subject != "" {
			ctx = context.WithValue(ctx, pkgutils.CtxKeySubjectId, claims.Subject)
		}

		r = r.WithContext(ctx)

		nextHandler.ServeHTTP(w, r)
	})
}
