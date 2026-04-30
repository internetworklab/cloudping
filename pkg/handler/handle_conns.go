package handler

import (
	"encoding/json"
	"log"
	"net/http"

	pkgnodereg "github.com/internetworklab/cloudping/pkg/nodereg"
)

type ConnsHandler struct {
	cr                   *pkgnodereg.ConnRegistry
	RequireLivenessCheck bool
}

func NewConnsHandler(cr *pkgnodereg.ConnRegistry, requireLivenessCheck bool) *ConnsHandler {
	return &ConnsHandler{cr: cr, RequireLivenessCheck: requireLivenessCheck}
}

func (h *ConnsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	nodes := make(map[string]*pkgnodereg.ConnRegistryData)
	if h.RequireLivenessCheck {
		nodes = h.cr.DumpLive()
	} else {
		nodes = h.cr.Dump()
	}
	if err := json.NewEncoder(w).Encode(nodes); err != nil {
		log.Printf("Failed to encode connections: %v", err)
	}
}
