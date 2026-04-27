package handler

import (
	"log"
	"net/http"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func WithLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realIp := pkgutils.GetRemoteAddr(r)
		uri := r.RequestURI

		ctx := r.Context()

		subj := ""
		if subjAny := ctx.Value(pkgutils.CtxKeySubjectId); subjAny != nil {
			subj = subjAny.(string)
		}

		sessId := ""
		if sessIdAny := ctx.Value(pkgutils.CtxKeySessionId); sessIdAny != nil {
			sessId = sessIdAny.(string)
		}

		log.Printf("request realIp: %s, uri: %q, subj: %q, sessId: %q", realIp, uri, subj, sessId)

		next.ServeHTTP(w, r)
	})
}
