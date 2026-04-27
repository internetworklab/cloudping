package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type VersionCMD struct{}

func (v *VersionCMD) Run(globalCtx *CLICtx) error {
	w := globalCtx.ResponseWriter
	r := globalCtx.Request
	gctx := globalCtx.GlobalSharedContext

	if gctx == nil || gctx.BuildVersion == nil {
		w.Header().Set("Content-Type", "application/json")
		err := errors.New("Internal error, GlobalSharedContext is not provided")
		json.NewEncoder(globalCtx.ResponseWriter).Encode(pkgutils.ErrorResponse{Error: err.Error()})
		return err
	}

	accept := "text/plain"
	if a := r.Header.Get("Accept"); a != "" {
		accept = a
	}

	// Serialize BuildVersion to JSON, then re-parse as a dictionary
	jsonBytes, _ := json.Marshal(gctx.BuildVersion)
	var versionMap map[string]any
	json.Unmarshal(jsonBytes, &versionMap)

	// Build table with one (key, value) pair per row
	tbl := &pkgtable.Table{}
	for k, v := range versionMap {
		tbl.Rows = append(tbl.Rows, pkgtable.Row{Cells: []string{k, fmt.Sprintf("%v", v)}})
	}

	if strings.HasPrefix(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, tbl.GetReadableHTMLTable())
	} else if strings.HasPrefix(accept, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gctx.BuildVersion)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		const defaultColGap int = 2
		fmt.Fprint(w, tbl.GetHumanReadableText(defaultColGap, 0, 0))
	}

	return nil
}
