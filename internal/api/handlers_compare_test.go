package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/semconv"
)

type compareResp struct {
	Total   int                      `json:"total"`
	Limit   int                      `json:"limit"`
	Buckets map[string]compareBucket `json:"buckets"`
}

// newCompareServer builds a server backed by the embedded registry and a
// dictionary seeded with the given attributes. Anchors are real registry
// entries: app.build_id (string), az.service_request_id (deprecated).
func newCompareServer(t *testing.T, entries []*dictionary.AttributeEntry) *Server {
	t.Helper()
	reg, err := semconv.Load()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	dict, err := dictionary.New(&dictionary.Config{ShardCount: 4, PerAttrCap: 100}, slog.Default())
	if err != nil {
		t.Fatalf("new dict: %v", err)
	}
	for _, e := range entries {
		dict.Upsert(e)
	}
	return &Server{semconv: reg, dict: dict}
}

func doCompare(t *testing.T, s *Server, query string) compareResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/compare"+query, nil)
	rec := httptest.NewRecorder()
	s.handleCompare(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var res compareResp
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return res
}

// findItem returns the entry with the given name from a bucket, if present.
func findItem(b compareBucket, name string) (compareItem, bool) {
	for _, it := range b.Entries {
		if it.Name == name {
			return it, true
		}
	}
	return compareItem{}, false
}

func TestCompareMatched(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "string", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace}},
	})
	res := doCompare(t, s, "")
	if res.Buckets[classMatched].Count != 1 {
		t.Fatalf("matched count = %d, want 1", res.Buckets[classMatched].Count)
	}
	it, ok := findItem(res.Buckets[classMatched], "app.build_id")
	if !ok {
		t.Fatal("app.build_id not in matched bucket")
	}
	if it.RegistryKey != "attribute:app.build_id" {
		t.Errorf("registry_key = %q", it.RegistryKey)
	}
}

func TestCompareTypeMismatch(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "int", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric}},
	})
	res := doCompare(t, s, "")
	if res.Buckets[classTypeMismatch].Count != 1 {
		t.Fatalf("type-mismatch count = %d, want 1", res.Buckets[classTypeMismatch].Count)
	}
	it, ok := findItem(res.Buckets[classTypeMismatch], "app.build_id")
	if !ok {
		t.Fatal("app.build_id not in type-mismatch bucket")
	}
	if it.TelemetryType != "int" || it.RegistryType != "string" {
		t.Errorf("types: telemetry=%q registry=%q", it.TelemetryType, it.RegistryType)
	}
}

func TestCompareDeprecated(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "az.service_request_id", Type: "string", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeLog}},
	})
	res := doCompare(t, s, "")
	if res.Buckets[classDeprecated].Count != 1 {
		t.Fatalf("deprecated count = %d, want 1", res.Buckets[classDeprecated].Count)
	}
	it, ok := findItem(res.Buckets[classDeprecated], "az.service_request_id")
	if !ok {
		t.Fatal("az.service_request_id not in deprecated bucket")
	}
	if it.Deprecation == nil {
		t.Fatal("expected deprecation metadata")
	}
}

func TestCompareNotInRegistry(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "my.custom.attribute", Type: "string", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace}},
	})
	res := doCompare(t, s, "")
	if res.Buckets[classNotInRegistry].Count != 1 {
		t.Fatalf("not-in-registry count = %d, want 1", res.Buckets[classNotInRegistry].Count)
	}
	if it, ok := findItem(res.Buckets[classNotInRegistry], "my.custom.attribute"); !ok || it.RegistryKey != "" {
		t.Errorf("unexpected entry: %+v ok=%v", it, ok)
	}
}

func TestCompareAllBuckets(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "string"},
		{Name: "app.build_id_int", Type: "int"}, // not in registry — distinct name
		{Name: "az.service_request_id", Type: "string"},
		{Name: "totally.made.up", Type: "string"},
	})
	res := doCompare(t, s, "")
	if res.Total != 4 {
		t.Fatalf("total = %d, want 4", res.Total)
	}
	wantNonEmpty := []string{classMatched, classDeprecated, classNotInRegistry}
	for _, c := range wantNonEmpty {
		if res.Buckets[c].Count == 0 {
			t.Errorf("bucket %q empty", c)
		}
	}
}

func TestCompareQueryFilter(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "string"},
		{Name: "az.service_request_id", Type: "string"},
	})
	res := doCompare(t, s, "?q=app.build")
	if res.Total != 1 {
		t.Fatalf("filtered total = %d, want 1", res.Total)
	}
}

func TestCompareLimitCapsEntriesNotCounts(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "nope.one", Type: "string"},
		{Name: "nope.two", Type: "string"},
		{Name: "nope.three", Type: "string"},
	})
	res := doCompare(t, s, "?limit=1")
	b := res.Buckets[classNotInRegistry]
	if b.Count != 3 {
		t.Errorf("count = %d, want 3 (full set)", b.Count)
	}
	if len(b.Entries) != 1 {
		t.Errorf("entries = %d, want 1 (capped)", len(b.Entries))
	}
}

// errorCode decodes a writeError body ({"error","code"}) and returns its code.
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode error body: %v (raw=%s)", err, body)
	}
	return e.Code
}

func TestCompareRegistryUnavailable(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/compare", nil)
	rec := httptest.NewRecorder()
	s.handleCompare(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "REGISTRY_UNAVAILABLE" {
		t.Errorf("code = %q, want REGISTRY_UNAVAILABLE", code)
	}
}

// TestCompareDictionaryUnavailable exercises the dict-nil branch, which the
// registry-unavailable test never reaches (the semconv check returns first).
func TestCompareDictionaryUnavailable(t *testing.T) {
	reg, err := semconv.Load()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	s := &Server{semconv: reg} // semconv present, dict nil
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/compare", nil)
	rec := httptest.NewRecorder()
	s.handleCompare(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "DICTIONARY_UNAVAILABLE" {
		t.Errorf("code = %q, want DICTIONARY_UNAVAILABLE", code)
	}
}

func TestCompareLimitClampedToMax(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "string"},
	})
	res := doCompare(t, s, "?limit=5000")
	if res.Limit != 1000 {
		t.Errorf("limit = %d, want 1000 (clamped to max)", res.Limit)
	}
}

// TestCompareCardinalityFromTracker verifies the response surfaces the live
// tracker cardinality, overriding the static value stored on the dict entry.
func TestCompareCardinalityFromTracker(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "string", Cardinality: 1},
	})
	tracker := cardinality.NewTracker(1000, 100, slog.Default())
	for _, v := range []string{"v1", "v2", "v3"} {
		tracker.TrackValue("app.build_id", v)
	}
	s.tracker = tracker

	res := doCompare(t, s, "")
	it, ok := findItem(res.Buckets[classMatched], "app.build_id")
	if !ok {
		t.Fatal("app.build_id not in matched bucket")
	}
	if it.Cardinality != 3 {
		t.Errorf("cardinality = %d, want 3 (from tracker, not dict entry's 1)", it.Cardinality)
	}
}

func TestCompareInvalidLimitFallsToDefault(t *testing.T) {
	s := newCompareServer(t, []*dictionary.AttributeEntry{
		{Name: "app.build_id", Type: "string"},
	})
	for _, q := range []string{"?limit=abc", "?limit=-5"} {
		res := doCompare(t, s, q)
		if res.Limit != 200 {
			t.Errorf("limit for %q = %d, want 200 (default)", q, res.Limit)
		}
	}
}

func TestCompareMethodNotAllowed(t *testing.T) {
	s := newCompareServer(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semconv/compare", nil)
	rec := httptest.NewRecorder()
	s.handleCompare(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "METHOD_NOT_ALLOWED" {
		t.Errorf("code = %q, want METHOD_NOT_ALLOWED", code)
	}
}

func TestTypesConflict(t *testing.T) {
	cases := []struct {
		telemetry, registry string
		want                bool
	}{
		{"string", "string", false},
		{"int", "string", true},
		{"bool", "boolean", false},
		{"string", "enum", false},
		{"string", "string[]", false},
		{"int", "", false},       // indeterminate registry
		{"map", "string", false}, // indeterminate telemetry
		{"double", "int", true},
	}
	for _, c := range cases {
		if got := typesConflict(c.telemetry, c.registry); got != c.want {
			t.Errorf("typesConflict(%q,%q) = %v, want %v", c.telemetry, c.registry, got, c.want)
		}
	}
}
