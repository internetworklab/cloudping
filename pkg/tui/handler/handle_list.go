package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuirenderer "github.com/internetworklab/cloudping/pkg/tui/renderer"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type ListHandler struct {
	LocationsProvider pkgtui.LocationsProvider
}

func (handler *ListHandler) getLocsProvider() (pkgtui.LocationsProvider, error) {
	if handler.LocationsProvider == nil {
		return nil, errors.New("LocationsProvider is not provided")
	}
	return handler.LocationsProvider, nil
}

func (handler *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	provider, err := handler.getLocsProvider()
	if err != nil {
		writeErrorResponse(w, fmt.Sprintf("Can't get location provider: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	allLocs, err := provider.GetAllLocations(r.Context())
	if err != nil {
		writeErrorResponse(w, fmt.Sprintf("Can't get locations: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if len(allLocs) == 0 {
		writeErrorResponse(w, "No node is available.", http.StatusServiceUnavailable)
		return
	}

	renderer := &pkgtuirenderer.LocationsTableRenderer{}
	var nodesTable pkgtable.Table = renderer.Render(allLocs)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nodesTable.GetReadableHTMLTable()))
}

func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: message})
}
