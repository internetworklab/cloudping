package handler

import (
	"errors"
	"fmt"
	"net/http"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuirenderer "github.com/internetworklab/cloudping/pkg/tui/renderer"
)

type ListCMD struct{}

func (cmd *ListCMD) Run(globalCtx *CLICtx) error {
	handler := &ListHandler{
		LocationsProvider: globalCtx.LocationsProvider,
	}
	handler.ServeHTTP(globalCtx.ResponseWriter, globalCtx.Request)
	return nil
}

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

	renderer := &pkgtuirenderer.LocationsTableRenderer{}
	var nodesTable pkgtable.TableLike = renderer.Render(allLocs)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nodesTable.GetReadableHTMLTable()))
}
