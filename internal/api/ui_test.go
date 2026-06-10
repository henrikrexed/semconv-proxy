package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUIServesIndex(t *testing.T) {
	h := uiHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SemConv Explorer") {
		t.Error("index content missing")
	}
}

func TestUIServesAssets(t *testing.T) {
	h := uiHandler()
	for _, path := range []string{"/app.js", "/styles.css"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
}

func TestUIDeepLinkFallback(t *testing.T) {
	h := uiHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/deep/link", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SemConv Explorer") {
		t.Error("expected SPA shell fallback")
	}
}

// An unmatched /api/ path must not fall through to the SPA shell: API clients
// expect a JSON 404, not a 200 index.html.
func TestUIUnknownAPIPathReturnsJSON404(t *testing.T) {
	h := uiHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	if strings.Contains(rec.Body.String(), "SemConv Explorer") {
		t.Error("unknown API path leaked the SPA shell")
	}
}
