package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuirenderer "github.com/internetworklab/cloudping/pkg/tui/renderer"
)

type CLIHandler struct {
	LocationsProvider pkgtui.LocationsProvider
}

func (handler *CLIHandler) getLocsProvider() (pkgtui.LocationsProvider, error) {
	if handler.LocationsProvider == nil {
		return nil, errors.New("LocationsProvider is not provided")
	}
	return handler.LocationsProvider, nil
}

func (handler *CLIHandler) getCLIArgsFromRawBody(r *http.Request) ([]string, error) {
	defer r.Body.Close()
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(rawBody)), nil
}

func (handler *CLIHandler) getCLIArgsAnyway(r *http.Request) ([]string, error) {
	ctx := r.Context()
	parsedMailAny := ctx.Value(CtxKeyWithEmailParsedEmail)
	if parsedMailAny != nil {
		parsedMail := parsedMailAny.(*ParsedEmail)
		fmt.Printf("[dbg] parsed mail:\n")
		json.NewEncoder(os.Stdout).Encode(parsedMail)
		return parsedMail.Args, nil
	}

	parsedCLIArgs, err := handler.getCLIArgsFromRawBody(r)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[dbg] parsed raw cli args:\n")
	json.NewEncoder(os.Stdout).Encode(parsedCLIArgs)
	if len(parsedCLIArgs) > 0 {
		return parsedCLIArgs, nil
	}

	return nil, errors.New("no cli args were parsed")
}

func (handler *CLIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := handler.getCLIArgsAnyway(r)
	if err != nil {
		writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	var nodesTable pkgtable.Table = renderer.Render(allLocs)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nodesTable.GetReadableHTMLTable()))
}
