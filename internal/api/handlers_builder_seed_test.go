package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/semconv"
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

// A matched attribute must surface the official registry enrichment — brief,
// stability, requirement level, and examples — so the table can offer the
// canonical wording instead of making the user retype it (AC §4.2/§5).
func TestBuilderSeedMatchedSurfacesEnrichment(t *testing.T) {
	s := newTestServer(t)
	res := decodeSeed(t, getSeed(t, s, ""))

	a, ok := findSeed(res.Attributes, "http.request.method")
	if !ok {
		t.Fatalf("expected http.request.method in seed, got %+v", res.Attributes)
	}
	if a.Brief == "" {
		t.Errorf("expected official brief surfaced for matched attr")
	}
	if a.Stability == "" {
		t.Errorf("expected official stability surfaced for matched attr")
	}
	if a.RequirementLevel == "" {
		t.Errorf("expected official requirement_level surfaced for matched attr")
	}
	if len(a.Examples) == 0 {
		t.Errorf("expected official examples surfaced for matched attr")
	}
	if a.Deprecation != nil {
		t.Errorf("matched (non-deprecated) attr should carry no deprecation, got %+v", a.Deprecation)
	}
}

// A deprecated registry attribute must land in the deprecated bucket and carry
// its deprecation metadata, distinct from the matched/type-mismatch buckets
// (AC §4.2 lists deprecated as one of the four cross-ref states).
func TestBuilderSeedDeprecated(t *testing.T) {
	s := newTestServer(t)
	// az.service_request_id is deprecated in the embedded registry.
	s.dict.Upsert(&dictionary.AttributeEntry{
		Name:        "az.service_request_id",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeLog},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
	})

	res := decodeSeed(t, getSeed(t, s, "?q=az.service_request_id"))
	a, ok := findSeed(res.Attributes, "az.service_request_id")
	if !ok {
		t.Fatalf("expected az.service_request_id in seed, got %+v", res.Attributes)
	}
	if a.CrossRef != classDeprecated {
		t.Errorf("cross_ref = %q, want %q", a.CrossRef, classDeprecated)
	}
	if a.Deprecation == nil {
		t.Errorf("expected deprecation metadata surfaced for a deprecated attr")
	}
	if a.RegistryKey == "" {
		t.Errorf("expected registry_key populated for a deprecated (known) attr")
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

func TestBuilderSeedRegistryUnavailable(t *testing.T) {
	s := &Server{} // semconv nil
	w := getSeed(t, s, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "REGISTRY_UNAVAILABLE") {
		t.Errorf("expected REGISTRY_UNAVAILABLE code, got %s", w.Body.String())
	}
}

func TestBuilderSeedDictionaryUnavailable(t *testing.T) {
	reg, err := semconv.Load()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	s := &Server{semconv: reg} // dict nil
	w := getSeed(t, s, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "DICTIONARY_UNAVAILABLE") {
		t.Errorf("expected DICTIONARY_UNAVAILABLE code, got %s", w.Body.String())
	}
}

// The limit caps the returned attributes while total reflects the full match
// count, so the table can page without losing the true size.
func TestBuilderSeedLimitCaps(t *testing.T) {
	s := newTestServer(t)
	// newTestServer already seeds http.request.method; add a second entry so the
	// dictionary holds >1 and limit=1 forces the cap break.
	s.dict.Upsert(&dictionary.AttributeEntry{
		Name:        "db.system",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
	})

	res := decodeSeed(t, getSeed(t, s, "?limit=1"))
	if res.Limit != 1 {
		t.Errorf("limit = %d, want 1", res.Limit)
	}
	if len(res.Attributes) != 1 {
		t.Errorf("returned %d attributes, want 1 (capped)", len(res.Attributes))
	}
	if res.Total < 2 {
		t.Errorf("total = %d, want >=2 (full match count, not capped)", res.Total)
	}
}

// An attribute name with no dotted prefix falls back to the "custom" namespace.
func TestBuilderSeedUnNamespacedFallsBackToCustom(t *testing.T) {
	s := newTestServer(t)
	s.dict.Upsert(&dictionary.AttributeEntry{
		Name:        "singleword",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
	})

	res := decodeSeed(t, getSeed(t, s, "?q=singleword"))
	a, ok := findSeed(res.Attributes, "singleword")
	if !ok {
		t.Fatalf("expected singleword in seed, got %+v", res.Attributes)
	}
	if a.Namespace != "custom" {
		t.Errorf("namespace = %q, want custom for un-namespaced name", a.Namespace)
	}
}
