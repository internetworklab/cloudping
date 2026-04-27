package handler

import (
	"context"
	"net/http"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func WithRealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realIp := pkgutils.GetRemoteAddr(r)
		ctx := r.Context()
		ctx = context.WithValue(ctx, pkgutils.CtxKeyRealIP, realIp)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
