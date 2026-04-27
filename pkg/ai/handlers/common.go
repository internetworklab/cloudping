package handlers

import (
	"context"
	"encoding/json"
	"log"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func logRequest(ctx context.Context, args any, name string) {
	realIp := ""
	if realIpAny := ctx.Value(pkgutils.CtxKeyRealIP); realIpAny != nil {
		realIp = realIpAny.(string)
	}

	subj := ""
	if subjAny := ctx.Value(pkgutils.CtxKeySubjectId); subjAny != nil {
		subj = subjAny.(string)
	}

	sessId := ""
	if sessIdAny := ctx.Value(pkgutils.CtxKeySessionId); sessIdAny != nil {
		sessId = sessIdAny.(string)
	}

	j := ""
	if s, err := json.Marshal(args); err == nil {
		j = string(s)
	}
	log.Printf("mcp request name=%s, real_ip=%s, subj=%s, sessId=%s, args=%s", name, realIp, subj, sessId, j)
}
