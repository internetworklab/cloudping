package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type IP2LocationRequester interface {
	GetFullURL(ip string, apiKey string) (*url.URL, error)
}

type IP2LocationProxyHandler struct {
	Requestor IP2LocationRequester
	APIKey    string
}

func (h *IP2LocationProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	subjAny := r.Context().Value(pkgutils.CtxKeySubjectId)
	if subjAny == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("unauthorized, can not get subject context")})
		return
	}

	subj := subjAny.(string)

	log.Printf("IP2Location proxy handle request for %s, remote: %s, query ip: %s", subj, pkgutils.GetRemoteAddr(r), r.URL.Query().Get("ip"))

	urlObj, err := h.Requestor.GetFullURL(r.URL.Query().Get("ip"), h.APIKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("faild to get url of upstream ip2location api: %v", err)})
		return
	}

	ctx := r.Context()

	req, err := http.NewRequestWithContext(ctx, "GET", urlObj.String(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: err.Error()})
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("http request failed with status code: %d", resp.StatusCode)})
		return
	}

	respObj := new(pkgipinfo.IP2LocationIPInfoResponse)
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to decode ip2location http response: %v", err)})
		return
	}

	json.NewEncoder(w).Encode(respObj)
}
