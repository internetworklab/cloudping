package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

// MaxMindProxyResponse is the JSON envelope returned by the MaxMind proxy endpoint.
type MaxMindProxyResponse struct {
	City *pkgipinfo.CityRecord `json:"city,omitempty"`
	ASN  *pkgipinfo.ASNRecord  `json:"asn,omitempty"`
}

// MaxMindProxyHandler is an http.Handler that resolves IP geo/ASN information
// from local MaxMind GeoLite2 City and ASN MMDB files and returns the raw
// database records as JSON.
type MaxMindProxyHandler struct {
	maxmindAdapter *pkgipinfo.MaxMindMMDBAdapter
}

// NewMaxMindProxyHandler creates a new MaxMindProxyHandler.
// Either asnReader or cityReader may be nil, but at least one must be non-nil.
func NewMaxMindProxyHandler(maxmindAdapter *pkgipinfo.MaxMindMMDBAdapter) (*MaxMindProxyHandler, error) {
	if maxmindAdapter == nil {
		return nil, fmt.Errorf("maxmind proxy: maxmindAdapter must be provided")
	}
	return &MaxMindProxyHandler{
		maxmindAdapter: maxmindAdapter,
	}, nil
}

func (h *MaxMindProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subjAny := r.Context().Value(pkgutils.CtxKeySubjectId)
	if subjAny == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "unauthorized, can not get subject context"})
		return
	}
	subj := subjAny.(string)

	queryingIP := r.URL.Query().Get("ip")
	if queryingIP == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "missing or empty 'ip' query parameter, expected /proxy/maxmind?ip=<address>"})
		return
	}

	log.Printf("MaxMind proxy handle request for %s, remote: %s, query ip: %s", subj, pkgutils.GetRemoteAddr(r), queryingIP)

	addr, err := netip.ParseAddr(queryingIP)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("invalid IP address: %v", err)})
		return
	}

	resp := new(MaxMindProxyResponse)

	// --- ASN lookup ---
	asnRec, err := h.maxmindAdapter.GetASNRecord(addr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("ASN lookup failed: %v", err)})
		return
	}
	resp.ASN = asnRec

	// --- City lookup ---
	cityRec, err := h.maxmindAdapter.GetCityRecord(addr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("City lookup failed: %v", err)})
		return
	}
	resp.City = cityRec

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
