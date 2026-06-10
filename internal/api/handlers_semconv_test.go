package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/henrikrexed/semconv-proxy/internal/semconv"
)

func newSemconvServer(t *testing.T) *Server {
	t.Helper()
	reg, err := semconv.Load()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return &Server{semconv: reg}
}

func TestCommunitySearch(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community?q=http.request.method&limit=10", nil)
	rec := httptest.NewRecorder()
	s.handleCommunitySearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var res semconv.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Total == 0 || len(res.Entries) == 0 {
		t.Fatal("expected results for http.request.method")
	}
	if len(res.Facets.Type) == 0 {
		t.Error("expected type facets")
	}
}

func TestCommunitySearchTypeFilter(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community?type=metric&limit=5000", nil)
	rec := httptest.NewRecorder()
	s.handleCommunitySearch(rec, req)

	var res semconv.Result
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Total == 0 {
		t.Fatal("expected metric results")
	}
	for _, it := range res.Entries {
		if it.Type != semconv.ItemMetric {
			t.Fatalf("non-metric item: %s", it.Name)
		}
	}
}

func TestCommunitySearchLimitClamp(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community?limit=999999", nil)
	rec := httptest.NewRecorder()
	s.handleCommunitySearch(rec, req)

	var res semconv.Result
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Limit > 1000 {
		t.Errorf("limit not clamped: %d", res.Limit)
	}
}

func TestCommunityEntryFound(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community/attribute:http.request.method", nil)
	rec := httptest.NewRecorder()
	s.handleCommunityEntry(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var item semconv.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.Name != "http.request.method" {
		t.Errorf("name = %q", item.Name)
	}
}

func TestCommunityEntryNotFound(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community/attribute:does.not.exist", nil)
	rec := httptest.NewRecorder()
	s.handleCommunityEntry(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCommunityEntryMethodNotAllowed(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semconv/community/attribute:http.request.method", nil)
	rec := httptest.NewRecorder()
	s.handleCommunityEntry(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestCommunityEntryRegistryUnavailable(t *testing.T) {
	s := &Server{} // no registry loaded
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community/attribute:http.request.method", nil)
	rec := httptest.NewRecorder()
	s.handleCommunityEntry(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// TestCommunityEntryEmptyKey exercises the bare-prefix path (no item key),
// which the router can hand off when the trailing segment is empty.
func TestCommunityEntryEmptyKey(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community/", nil)
	rec := httptest.NewRecorder()
	s.handleCommunityEntry(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCommunityRegistryUnavailable(t *testing.T) {
	s := &Server{} // no registry loaded
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/community", nil)
	rec := httptest.NewRecorder()
	s.handleCommunitySearch(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestCommunityMethodNotAllowed(t *testing.T) {
	s := newSemconvServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semconv/community", nil)
	rec := httptest.NewRecorder()
	s.handleCommunitySearch(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}
