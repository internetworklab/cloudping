package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type IPRegistryRequester interface {
	GetFullURL(ip string, apiKey string) (*url.URL, error)
}

type IPRegistryProxyHandler struct {
	Requestor IPRegistryRequester
	APIKey    string
}

func (h *IPRegistryProxyHandler) getQueryingIPFromRequestURL(uObj *url.URL) string {
	ip := uObj.Path
	if x, ok := strings.CutPrefix(ip, "/"); ok {
		ip = x
	}
	ipSegs := strings.FieldsFunc(ip, func(r rune) bool { return r == '/' })
	if len(ipSegs) > 0 {
		ip = ipSegs[len(ipSegs)-1]
	}
	if ipObj := net.ParseIP(ip); ipObj != nil {
		// this step is necessary, if the provided ip is empty, the last segment could be non-ip string.
		if ip4 := ipObj.To4(); ip4 != nil {
			return ip4.String()
		}
		return ipObj.String()
	}
	return ""
}

func (h *IPRegistryProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	subjAny := r.Context().Value(pkgutils.CtxKeySubjectId)
	if subjAny == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("unauthorized, can not get subject context")})
		return
	}

	subj := subjAny.(string)

	queryingIp := h.getQueryingIPFromRequestURL(r.URL)

	log.Printf("IPRegistry proxy handle request for %s, remote: %s, query ip: %s", subj, pkgutils.GetRemoteAddr(r), queryingIp)

	keyUsed := h.APIKey
	if keyProvided := r.URL.Query().Get("key"); keyProvided != "" {
		keyUsed = keyProvided
	}

	urlObj, err := h.Requestor.GetFullURL(queryingIp, keyUsed)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := r.Context()
	req, err := http.NewRequestWithContext(ctx, "GET", urlObj.String(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to create request to upstream ipregistry api: %s", err.Error())})
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to do request to upstream ipregistry api: %s", err.Error())})
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("http request failed with status code: %d", resp.StatusCode)})
		return
	}

	respObj := new(pkgipinfo.IPRegistryCOResponse)
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to decode ip2location http response: %v", err)})
		return
	}

	json.NewEncoder(w).Encode(respObj)
}
