package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var testFiles = fstest.MapFS{
	"index.html":         {Data: []byte("<!doctype html><title>map</title>")},
	"assets/app-1a2b.js": {Data: []byte("console.log(1)")},
}

func get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	h, err := newHandler(testFiles)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := get(t, "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>map</title>") {
		t.Errorf("GET / = %d %q, want index.html", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("index.html Cache-Control = %q, want none", cc)
	}
}

func TestAssetsAreCachedForever(t *testing.T) {
	rec := get(t, "/assets/app-1a2b.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
}

func TestMissingFileIsNotCached(t *testing.T) {
	rec := get(t, "/assets/nope.js")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("404 Cache-Control = %q, want none", cc)
	}
}

func TestUnbuiltFrontendIsAnError(t *testing.T) {
	if _, err := newHandler(fstest.MapFS{".gitkeep": {}}); err == nil {
		t.Error("newHandler without index.html succeeded, want error")
	}
}
