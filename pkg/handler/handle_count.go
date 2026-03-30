package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type CountHandler struct {
	store        sync.Map
	initialValue int
}

func NewCountHandler(initialValue int) *CountHandler {
	return &CountHandler{
		initialValue: initialValue,
	}
}

type countResponse struct {
	Count int `json:"count"`
}

func (h *CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sid, ok := r.Context().Value(pkgutils.CtxKeySessionId).(string)
	if !ok || sid == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "no session id in context"})
		return
	}

	var preVal int
	for {
		val, _ := h.store.LoadOrStore(sid, h.initialValue)
		preVal = val.(int)

		if h.store.CompareAndSwap(sid, preVal, preVal+1) {
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(countResponse{Count: preVal})
}
