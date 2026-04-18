package handler

import (
	"net/http"

	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
)

type ListHandler struct {
	LocationsProvider pkgtui.LocationsProvider
}

func (handler *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
