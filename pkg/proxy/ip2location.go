package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	pkgipinfo "example.com/rbmq-demo/pkg/ipinfo"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
)

type IP2LocationProxyHandler struct {
	BackendEndpoint string
	APIKey          string
}

func (h *IP2LocationProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(pkgutils.CtxKeyJWTToken).(*jwt.Token)
	if token == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "unauthorized"})
		return
	}

	subj, err := token.Claims.GetSubject()
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("unauthorized, can not get subject from token claims: %v", err)})
		return
	}

	log.Printf("IP2Location proxy handle request for %s, remote: %s, query ip: %s", subj, pkgutils.GetRemoteAddr(r), r.URL.Query().Get("ip"))

	urlObj, err := url.Parse(h.BackendEndpoint)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: err.Error()})
		return
	}
	values := url.Values{}
	values.Add("ip", r.URL.Query().Get("ip"))
	values.Add("key", h.APIKey)
	urlObj.RawQuery = values.Encode()

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
