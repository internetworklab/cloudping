package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const (
	QueryKeyIP       = "ip"
	QueryKeyProvider = "provider"
)

func ptr(s string) *string { return &s }

// DataResponse is a generic JSON envelope for successful responses.
type DataResponse[T any] struct {
	Data T `json:"data"`
}

// QueryResultEntry represents a single IP query result from a specific provider.
type QueryResultEntry struct {
	Err    *string                `json:"err,omitempty"`
	From   string                 `json:"from"`
	IP     string                 `json:"ip"`
	Result *pkgipinfo.BasicIPInfo `json:"result,omitempty"`
}

// IPQueryDirectoryHandler is an http.Handler that serves as a directory
// for IP information query endpoints.
type IPQueryDirectoryHandler struct {
	IPInfoProvidersRegistry *pkgipinfo.IPInfoProviderRegistry
}

func (h *IPQueryDirectoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")

	if strings.HasSuffix(path, "/providers") {
		h.serveProviders(w, r)
	} else if strings.HasSuffix(path, "/query") {
		h.serveQuery(w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "unknown endpoint"})
	}
}

func (h *IPQueryDirectoryHandler) serveProviders(w http.ResponseWriter, _ *http.Request) {
	providers := h.IPInfoProvidersRegistry.GetRegisteredAdapterNames()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(DataResponse[[]string]{Data: providers}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "failed to encode provider list"})
	}
}

func (h *IPQueryDirectoryHandler) serveQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	providers := h.getProvidersOrAll(r)

	ips := h.parseIPs(r)
	if len(ips) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResultEntry{
			Err:  ptr("missing or empty 'ip' query parameter"),
			From: "query",
		})
		return
	}

	ipMap := make(map[string]struct{}, len(ips))
	for _, ipStr := range ips {
		if _, err := netip.ParseAddr(ipStr); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(QueryResultEntry{
				Err:  ptr(fmt.Sprintf("invalid ip address %q: %v", ipStr, err)),
				From: "query",
			})
			return
		}
		ipMap[ipStr] = struct{}{}
	}

	providerMap := make(map[string]pkgipinfo.GeneralIPInfoAdapter, len(providers))
	for _, name := range providers {
		adapter, err := h.IPInfoProvidersRegistry.GetAdapter(name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(QueryResultEntry{
				Err:  ptr(fmt.Sprintf("failed to get provider %q: %v", name, err)),
				From: name,
			})
			return
		}
		if adapter == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(QueryResultEntry{
				Err:  ptr(fmt.Sprintf("provider %q not found", name)),
				From: name,
			})
			return
		}
		providerMap[name] = adapter
	}

	flusher, canFlush := w.(http.Flusher)

	for ip := range ipMap {
		for name, adapter := range providerMap {
			info, err := adapter.GetIPInfo(r.Context(), ip)
			if err != nil {
				json.NewEncoder(w).Encode(QueryResultEntry{
					Err:  ptr(fmt.Sprintf("provider %q query for %s failed: %v", name, ip, err)),
					From: name,
					IP:   ip,
				})
			} else {
				json.NewEncoder(w).Encode(QueryResultEntry{
					From:   name,
					IP:     ip,
					Result: info,
				})
			}
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

func parseQueryVals(r *http.Request, key string) []string {
	result := make([]string, 0)
	for _, val := range r.URL.Query()[key] {
		for p := range strings.SplitSeq(val, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
	}
	return result
}

func (h *IPQueryDirectoryHandler) parseProviders(r *http.Request) []string {
	return parseQueryVals(r, QueryKeyProvider)
}

func (h *IPQueryDirectoryHandler) parseIPs(r *http.Request) []string {
	return parseQueryVals(r, QueryKeyIP)
}

func (h *IPQueryDirectoryHandler) getProvidersOrAll(r *http.Request) []string {
	providers := h.parseProviders(r)
	if len(providers) == 0 {
		providers = h.IPInfoProvidersRegistry.GetRegisteredAdapterNames()
	}
	return providers
}
