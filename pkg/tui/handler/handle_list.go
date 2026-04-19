package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/jhillyerd/enmime"

	pkgtable "github.com/internetworklab/cloudping/pkg/table"
	pkgtui "github.com/internetworklab/cloudping/pkg/tui"
	pkgtuirenderer "github.com/internetworklab/cloudping/pkg/tui/renderer"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"golang.org/x/net/html"
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
	defer r.Body.Close()

	// Parse message body with enmime.
	mail, err := enmime.ReadEnvelope(r.Body)
	if err != nil {
		fmt.Print(err)
		return
	}

	// Headers can be retrieved via Envelope.GetHeader(name).
	log.Printf("[dbg] From: %v\n", mail.GetHeader("From"))

	// Address-type headers can be parsed into a list of decoded mail.Address structs.
	alist, _ := mail.AddressList("To")
	for _, addr := range alist {
		log.Printf("[dbg] To: %s <%s>\n", addr.Name, addr.Address)
	}

	// enmime can decode quoted-printable headers.
	log.Printf("[dbg] Subject: %v\n", mail.GetHeader("Subject"))

	if rawText := mail.Text; len(rawText) > 0 {
		// fields from raw text
		rawFields := strings.Fields(rawText)
		log.Printf("[dbg] fields from raw: %s\n", strings.Join(rawFields, ", "))
	}

	if htmlText := mail.HTML; len(htmlText) > 0 {
		// Parsing html doc tree
		htmlReader := strings.NewReader(mail.HTML)
		doc, err := html.Parse(htmlReader)
		if err != nil {
			log.Panic(err)
		}
		htmlFields := make([]string, 0)
		for elem := range doc.Descendants() {
			if elem.Type == html.TextNode {
				if trimed := strings.TrimSpace(elem.Data); len(trimed) > 0 {
					htmlFields = append(htmlFields, strings.Fields(trimed)...)
				}
			}
		}
		if len(htmlFields) > 0 {
			log.Printf("[dbg] fields from html: %s\n", strings.Join(htmlFields, ", "))
		}
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

func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: message})
}
