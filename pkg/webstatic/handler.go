// Package webstatic serves the statically exported Next.js frontend from an
// fs.FS (typically the assets embedded into the binary via the web package).
//
// The resolution rules intentionally mirror the nginx configuration that used
// to serve these assets:
//
//	try_files $uri $uri.html $uri/ =404;
package webstatic

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

const (
	indexPage      = "index.html"
	notFoundPage   = "404.html"
	versionTxtURL  = "/version.txt"
	versionTxtFile = "version.txt"
	// versionTxtDotFile is where build pipelines stash the version metadata
	// inside the export (dotfile so it is never served directly).
	versionTxtDotFile = ".version.txt"
)

// Handler serves static web assets from an fs.FS.
type Handler struct {
	fsys                fs.FS
	notFound            []byte
	versionTextFallback []byte
}

type Option func(*Handler)

// WithVersionTextFallback sets content to be served at /version.txt when
// neither version.txt nor .version.txt exists in the asset tree.
func WithVersionTextFallback(content []byte) Option {
	return func(h *Handler) {
		h.versionTextFallback = content
	}
}

func NewHandler(fsys fs.FS, opts ...Option) (*Handler, error) {
	if fsys == nil {
		return nil, fmt.Errorf("nil fs")
	}
	h := &Handler{fsys: fsys}
	for _, opt := range opts {
		opt(h)
	}
	// 404.html is part of the Next.js static export; cache it up front.
	if data, err := fs.ReadFile(fsys, notFoundPage); err == nil {
		h.notFound = data
	}
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.serveVersionTxt(w, r) {
		return
	}

	file, name, err := h.resolve(r.URL.Path)
	if err != nil {
		h.serveNotFound(w, r)
		return
	}
	defer file.Close()

	if strings.HasPrefix(name, "_next/") {
		// Next.js emits content-hashed assets under _next/.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		seeker = bytes.NewReader(data)
	}
	http.ServeContent(w, r, name, time.Time{}, seeker)
}

// resolve maps a request path to a file inside the asset tree, mirroring
// nginx's `try_files $uri $uri.html $uri/ =404;`.
func (h *Handler) resolve(urlPath string) (fs.File, string, error) {
	name := strings.TrimPrefix(urlPath, "/")
	trailingSlash := strings.HasSuffix(urlPath, "/")
	if name == "" {
		name = indexPage
	} else if trailingSlash {
		name += indexPage
	}
	if !fs.ValidPath(name) || hasHiddenSegment(name) {
		return nil, "", fs.ErrNotExist
	}

	candidates := []string{name}
	if !trailingSlash {
		candidates = append(candidates, name+".html", name+"/"+indexPage)
	}
	for _, candidate := range candidates {
		file, err := h.openFile(candidate)
		if err == nil {
			return file, candidate, nil
		}
	}
	return nil, "", fs.ErrNotExist
}

func (h *Handler) openFile(name string) (fs.File, error) {
	file, err := h.fsys.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if info.IsDir() {
		file.Close()
		return nil, fmt.Errorf("%q is a directory", name)
	}
	return file, nil
}

// serveVersionTxt handles /version.txt specially: the file may be embedded as
// a dotfile (build pipelines stash it as .version.txt), or synthesized from
// the binary's own build metadata as a fallback. Returns true if the request
// was handled.
func (h *Handler) serveVersionTxt(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != versionTxtURL {
		return false
	}
	if info, err := fs.Stat(h.fsys, versionTxtFile); err == nil && !info.IsDir() {
		// A real version.txt exists; let the normal resolution serve it.
		return false
	}
	if data, err := fs.ReadFile(h.fsys, versionTxtDotFile); err == nil {
		serveData(w, r, versionTxtFile, data)
		return true
	}
	if h.versionTextFallback != nil {
		serveData(w, r, versionTxtFile, h.versionTextFallback)
		return true
	}
	return false
}

func (h *Handler) serveNotFound(w http.ResponseWriter, r *http.Request) {
	if h.notFound == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if r.Method != http.MethodHead {
		w.Write(h.notFound)
	}
}

func serveData(w http.ResponseWriter, r *http.Request, name string, data []byte) {
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

// hasHiddenSegment reports whether any path segment starts with a dot, which
// keeps dotfiles like .version.txt and .gitkeep from being served.
func hasHiddenSegment(name string) bool {
	for _, seg := range strings.Split(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}
