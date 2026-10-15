package webstatic

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                  {Data: []byte("<html>home</html>")},
		"login.html":                  {Data: []byte("<html>login</html>")},
		"404.html":                    {Data: []byte("<html>not found</html>")},
		"demos/demo4.html":            {Data: []byte("<html>demo4</html>")},
		"demos/demo4/index.txt":       {Data: []byte("rsc-payload")},
		"_next/static/chunks/app.js":  {Data: []byte("console.log(1)")},
		"favicon.ico":                 {Data: []byte{0x00, 0x00, 0x01, 0x00}},
		"flags/de.svg":                {Data: []byte("<svg></svg>")},
		"networkdescriptor.json":      {Data: []byte("{}")},
		".version.txt":                {Data: []byte("HEAD: abc123\n")},
		".gitkeep":                    {Data: nil},
		"dir-only/nested/index.html":  {Data: []byte("<html>nested</html>")},
	}
}

func newTestHandler(t *testing.T, opts ...Option) *Handler {
	t.Helper()
	h, err := NewHandler(testFS(), opts...)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

func doRequest(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServeIndexAtRoot(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "<html>home</html>" {
		t.Fatalf("body = %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestServeCleanURL(t *testing.T) {
	h := newTestHandler(t)

	for _, target := range []string{"/login", "/demos/demo4"} {
		rec := doRequest(t, h, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", target, rec.Code)
		}
	}

	if body := doRequest(t, h, http.MethodGet, "/login").Body.String(); body != "<html>login</html>" {
		t.Fatalf("/login body = %q", body)
	}
	if body := doRequest(t, h, http.MethodGet, "/demos/demo4").Body.String(); body != "<html>demo4</html>" {
		t.Fatalf("/demos/demo4 body = %q", body)
	}
}

func TestServeNestedIndex(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/dir-only/nested/")
	if rec.Code != http.StatusOK || rec.Body.String() != "<html>nested</html>" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestServeStaticAssets(t *testing.T) {
	h := newTestHandler(t)

	rec := doRequest(t, h, http.MethodGet, "/_next/static/chunks/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	rec = doRequest(t, h, http.MethodGet, "/flags/de.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("svg: status = %d, want 200", rec.Code)
	}

	rec = doRequest(t, h, http.MethodGet, "/networkdescriptor.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("json: status = %d, want 200", rec.Code)
	}
}

func TestNotFoundServes404Page(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/no/such/page")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); body != "<html>not found</html>" {
		t.Fatalf("body = %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestDotfilesAreNotServed(t *testing.T) {
	h := newTestHandler(t)
	for _, target := range []string{"/.gitkeep", "/.version.txt"} {
		rec := doRequest(t, h, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", target, rec.Code)
		}
	}
}

func TestVersionTxtFromDotFile(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/version.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "HEAD: abc123\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestVersionTxtFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("home")},
	}
	h, err := NewHandler(fsys, WithVersionTextFallback([]byte("HEAD: fallback\n")))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	rec := doRequest(t, h, http.MethodGet, "/version.txt")
	if rec.Code != http.StatusOK || rec.Body.String() != "HEAD: fallback\n" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestVersionTxtMissing(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("home")},
	}
	h, err := NewHandler(fsys)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	rec := doRequest(t, h, http.MethodGet, "/version.txt")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodPost, "/")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("Allow = %q", allow)
	}
}

func TestHeadRequest(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodHead, "/login")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD body must be empty, got %d bytes", rec.Body.Len())
	}
}

func TestPathTraversalRejected(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/../secret.txt")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	rec = doRequest(t, h, http.MethodGet, "/%2e%2e/secret.txt")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestNilFSRejected(t *testing.T) {
	if _, err := NewHandler(nil); err == nil {
		t.Fatal("expected error for nil fs")
	}
}

func TestServeContentRange(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("Range", "bytes=0-5")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != "<html>" {
		t.Fatalf("range body = %q", string(body))
	}
}
