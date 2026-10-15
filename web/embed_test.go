package web

import (
	"io/fs"
	"testing"
)

func TestDistFS(t *testing.T) {
	fsys, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS: %v", err)
	}
	if fsys == nil {
		t.Fatal("DistFS returned nil")
	}
	// The tree must always be openable; its contents depend on whether the
	// frontend was built before go build/test (only .gitkeep otherwise).
	if _, err := fs.Stat(fsys, "."); err != nil {
		t.Fatalf("stat root: %v", err)
	}
}

func TestHasDistAssets(t *testing.T) {
	// Just exercise the function; the result depends on whether web/out was
	// populated at build time.
	_ = HasDistAssets()
}
