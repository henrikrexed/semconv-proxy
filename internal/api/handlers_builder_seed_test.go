package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

func getSeed(t *testing.T, s *Server, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/builder/seed"+query, nil)
	w := httptest.NewRecorder()
	s.handleBuilderSeed(w, req)
	return w
}

type seedResponse struct {
	Total      int             `json:"total"`
	Limit      int             `json:"limit"`
	Attributes []seedAttribute `json:"attributes"`
	DefaultDep *struct {
		SchemaURL    string `json:"schema_url"`
		RegistryPath string `json:"registry_path"`
	} `json:"default_dependency"`
}

func decodeSeed(t *testing.T, w *httptest.ResponseRecorder) seedResponse {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res seedResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return res
}

func findSeed(attrs []seedAttribute, id string) (seedAttribute, bool) {
	for _, a := range attrs {
		if a.ID == id {
			return a, true
		}
	}
	return seedAttribute{}, false
}

// The default test server seeds http.request.method (a known registry
// attribute). The seed endpoint must classify it as matched, pre-fill its type
// from the observed telemetry type, and surface a suggested namespace.
func TestBuilderSeedMatched(t *testing.T) {
	s := newTestServer(t)
	res := decodeSeed(t, getSeed(t, s, ""))

	a, ok := findSeed(res.Attributes, "http.request.method")
	if !ok {
		t.Fatalf("expected http.request.method in seed, got %+v", res.Attributes)
	}
	if a.CrossRef != classMatched {
		t.Errorf("cross_ref = %q, want %q", a.CrossRef, classMatched)
	}
	if a.Namespace != "http" {
		t.Errorf("namespace = %q, want http", a.Namespace)
	}
	if a.Type != "string" || a.ObservedType != "string" {
		t.Errorf("type pre-fill = %q (observed %q), want string from observed type", a.Type, a.ObservedType)
	}
	if a.RegistryKey == "" {
		t.Errorf("expected registry_key populated for a matched attribute")
	}
	if res.DefaultDep == nil || res.DefaultDep.RegistryPath == "" {
		t.Errorf("expected default_dependency populated from embedded registry")
	}
}

// An attribute the registry has never heard of must land in not-in-registry
// with no suggested enrichment, while its type is still pre-filled from the
// observed telemetry.
func TestBuilderSeedNotInRegistry(t *testing.T) {
	s := newTestServer(t)
	s.dict.Upsert(&dictionary.AttributeEntry{
		Name:        "acme.tenant.id",
		Type:        "int",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 3,
	})

	res := decodeSeed(t, getSeed(t, s, ""))
	a, ok := findSeed(res.Attributes, "acme.tenant.id")
	if !ok {
		t.Fatalf("expected acme.tenant.id in seed, got %+v", res.Attributes)
	}
	if a.CrossRef != classNotInRegistry {
		t.Errorf("cross_ref = %q, want %q", a.CrossRef, classNotInRegistry)
	}
	if a.Namespace != "acme" {
		t.Errorf("namespace = %q, want acme", a.Namespace)
	}
	if a.Type != "int" {
		t.Errorf("type pre-fill = %q, want int (from observed type)", a.Type)
	}
	if a.RegistryKey != "" || a.Brief != "" {
		t.Errorf("expected no registry enrichment for unknown attr, got key=%q brief=%q", a.RegistryKey, a.Brief)
	}
}

// A type that conflicts with the registry's declared value type lands in the
// type-mismatch bucket.
func TestBuilderSeedTypeMismatch(t *testing.T) {
	s := newTestServer(t)
	// http.request.method is a string in the registry; observe it as an int.
	s.dict.Upsert(&dictionary.AttributeEntry{
		Name:        "http.request.method",
		Type:        "int",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 10,
	})

	res := decodeSeed(t, getSeed(t, s, ""))
	a, ok := findSeed(res.Attributes, "http.request.method")
	if !ok {
		t.Fatalf("expected http.request.method in seed")
	}
	if a.CrossRef != classTypeMismatch {
		t.Errorf("cross_ref = %q, want %q", a.CrossRef, classTypeMismatch)
	}
	if a.RegistryType == "" {
		t.Errorf("expected registry_type populated on a mismatch")
	}
}

func TestBuilderSeedQueryFilter(t *testing.T) {
	s := newTestServer(t)
	s.dict.Upsert(&dictionary.AttributeEntry{
		Name:        "db.system",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
	})

	res := decodeSeed(t, getSeed(t, s, "?q=db."))
	if _, ok := findSeed(res.Attributes, "db.system"); !ok {
		t.Errorf("expected db.system to match q=db., got %+v", res.Attributes)
	}
	if _, ok := findSeed(res.Attributes, "http.request.method"); ok {
		t.Errorf("did not expect http.request.method when filtering q=db.")
	}
}

func TestBuilderSeedMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/builder/seed", nil)
	w := httptest.NewRecorder()
	s.handleBuilderSeed(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}
