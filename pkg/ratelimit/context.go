package ratelimit

import (
	"context"
	"net/http"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func WithRatelimiters(originalHandler http.Handler, enforcer RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, pkgutils.CtxKeySharedRateLimitEnforcer, enforcer)
		r = r.WithContext(ctx)
		originalHandler.ServeHTTP(w, r)
	})
}
