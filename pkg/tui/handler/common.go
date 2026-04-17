package handler

import (
	"encoding/json"
	"net/http"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: message})
}
