package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"github.com/oschwald/maxminddb-golang/v2"
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
	asnReader  *maxminddb.Reader
	cityReader *maxminddb.Reader
}

// NewMaxMindProxyHandler creates a new MaxMindProxyHandler.
// Either asnReader or cityReader may be nil, but at least one must be non-nil.
func NewMaxMindProxyHandler(asnReader, cityReader *maxminddb.Reader) (*MaxMindProxyHandler, error) {
	if asnReader == nil && cityReader == nil {
		return nil, fmt.Errorf("maxmind proxy: at least one of asnReader or cityReader must be provided")
	}
	return &MaxMindProxyHandler{
		asnReader:  asnReader,
		cityReader: cityReader,
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
	if h.asnReader != nil {
		var asnRec pkgipinfo.ASNRecord
		if err := h.asnReader.Lookup(addr).Decode(&asnRec); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("ASN lookup failed: %v", err)})
			return
		}
		resp.ASN = &asnRec
	}

	// --- City lookup ---
	if h.cityReader != nil {
		var cityRec pkgipinfo.CityRecord
		if err := h.cityReader.Lookup(addr).Decode(&cityRec); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("City lookup failed: %v", err)})
			return
		}
		resp.City = &cityRec
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
