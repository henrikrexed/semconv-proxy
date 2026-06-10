package api

// Integration coverage for the ISI-1179 SemConv Explorer SPA (story S3).
//
// These tests boot the fully wired server (NewServer -> real mux + middleware)
// and exercise it through an httptest.Server, asserting the three things the S3
// acceptance criteria require that ARE verifiable without a browser:
//
//   1. The SPA shell + its assets load from the binary with no external assets.
//   2. Both scopes return the contract the SPA actually consumes:
//        - Community  -> /api/v1/semconv/community(/{key})   (app.js normCommunity)
//        - My Telemetry -> /api/v1/dictionary(/{name})        (app.js normMine)
//   3. The served CSS carries the responsive contract (mobile collapse markers).
//
// LIMITATION: pixel layout / live DOM rendering / touch-target sizing at a
// phone-width viewport cannot be asserted here -- that needs a real browser, and
// this headless box lacks the Chrome system libs (libatk et al.). The Playwright
// recipe for that layer is documented in _bmad/tests/test-summary.md for CI /
// local runs where a browser shell is available.

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/export"
	"github.com/henrikrexed/semconv-proxy/internal/health"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// newWiredTestServer builds the full server exactly as production does and seeds
// one dictionary entry so the "My Telemetry" scope has something to return. It
// returns an httptest.Server fronting the real mux+middleware chain.
func newWiredTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	dict, err := dictionary.New(&dictionary.Config{ShardCount: 4, GlobalBudget: 100, PerAttrCap: 10}, slog.Default())
	if err != nil {
		t.Fatalf("dictionary.New: %v", err)
	}
	now := time.Now()
	dict.Upsert(&dictionary.AttributeEntry{
		Name:        "http.request.method",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
		FirstSeen:   now,
		LastSeen:    now,
		Status:      dictionary.StatusActive,
		Cardinality: 7,
	})

	tracker := cardinality.NewTracker(100, 10, slog.Default())
	weaverExporter := export.NewWeaverExporter()
	healthAgg := health.NewAggregator(slog.Default())
	registry := prometheus.NewRegistry()
	m := metrics.New(registry)

	srv := NewServer(0, dict, tracker, weaverExporter, slog.Default(), healthAgg, registry, m)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func getBody(t *testing.T, ts *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	resp, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return resp, string(b)
}

// AC: "UI loads from the binary with no external assets."
func TestSPA_ShellAndAssetsServedFromBinary(t *testing.T) {
	ts := newWiredTestServer(t)

	resp, body := getBody(t, ts, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", resp.StatusCode)
	}
	if !strings.Contains(body, "SemConv Explorer") {
		t.Error("shell did not render the app title")
	}
	// Responsive contract entry point: the viewport meta is mandatory for a
	// phone-usable layout. Its absence silently breaks the HARD mobile AC.
	if !strings.Contains(body, `name="viewport"`) {
		t.Error("shell missing viewport meta -- responsive layout would break on phones")
	}
	// "No external assets": every script/style must be same-origin relative.
	if strings.Contains(body, `src="http`) || strings.Contains(body, `href="http`) || strings.Contains(body, `src="//`) {
		t.Error("shell references an external (off-origin) asset")
	}

	for _, asset := range []string{"/app.js", "/styles.css"} {
		r, b := getBody(t, ts, asset)
		if r.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d", asset, r.StatusCode)
		}
		if len(b) == 0 {
			t.Errorf("GET %s: empty body", asset)
		}
	}
}

// AC: "verified responsive at narrow and wide viewports (single-column collapse,
// no horizontal scroll)." We can't compute layout without a browser, but we can
// guarantee the responsive contract the layout depends on is actually shipped in
// the embedded CSS -- a regression here (e.g. someone drops the @media block)
// silently reverts the mobile experience.
func TestSPA_ResponsiveContractInServedCSS(t *testing.T) {
	ts := newWiredTestServer(t)
	resp, css := getBody(t, ts, "/styles.css")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /styles.css: status %d", resp.StatusCode)
	}

	checks := map[string]string{
		"phone-width media query":    "@media (max-width: 820px)",
		"single-column collapse":     "grid-template-columns: 1fr",
		"no-horizontal-scroll guard": "word-break",
	}
	for name, marker := range checks {
		if !strings.Contains(css, marker) {
			t.Errorf("responsive contract missing %s (expected %q in styles.css)", name, marker)
		}
	}
}

// communityResult mirrors the JSON fields app.js normCommunity()/renderResults()
// read. If the server stops emitting any of these, the Community scope renders
// blank rows even though it returns 200.
type communityResult struct {
	Total   int `json:"total"`
	Entries []struct {
		Key       string `json:"key"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		Stability string `json:"stability"`
	} `json:"entries"`
	Facets struct {
		Type []struct {
			Value string `json:"value"`
			Count int    `json:"count"`
		} `json:"type"`
	} `json:"facets"`
}

// AC: "both scopes return and render results" -- Community scope, through the
// fully wired mux (not the handler in isolation).
func TestSPA_CommunityScopeReturnsRenderableContract(t *testing.T) {
	ts := newWiredTestServer(t)

	resp, body := getBody(t, ts, "/api/v1/semconv/community?q=http&limit=50")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("community search: status %d", resp.StatusCode)
	}
	var res communityResult
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("decode community result: %v", err)
	}
	if res.Total == 0 || len(res.Entries) == 0 {
		t.Fatal("community scope returned no results for q=http")
	}
	if len(res.Facets.Type) == 0 {
		t.Error("community scope returned no type facets (facet rail would be empty)")
	}
	for _, e := range res.Entries {
		if e.Key == "" || e.Name == "" || e.Type == "" {
			t.Fatalf("entry missing render fields: %+v", e)
		}
	}

	// Detail panel deep-dive: the key returned by search must resolve.
	key := res.Entries[0].Key
	dResp, dBody := getBody(t, ts, "/api/v1/semconv/community/"+key)
	if dResp.StatusCode != http.StatusOK {
		t.Fatalf("community detail %q: status %d", key, dResp.StatusCode)
	}
	var item struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(dBody), &item); err != nil {
		t.Fatalf("decode community detail: %v", err)
	}
	if item.Name == "" {
		t.Error("community detail returned no name")
	}
}

// mineResult mirrors the JSON fields app.js normMine() reads off /api/v1/dictionary.
type mineResult struct {
	Total   int `json:"total"`
	Entries []struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		SignalTypes []string `json:"signal_types"`
		Status      string   `json:"status"`
	} `json:"entries"`
}

// AC: "both scopes return and render results" -- My Telemetry scope, backed by
// the live dictionary, through the fully wired mux.
func TestSPA_MyTelemetryScopeReturnsRenderableContract(t *testing.T) {
	ts := newWiredTestServer(t)

	resp, body := getBody(t, ts, "/api/v1/dictionary?limit=50")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dictionary search: status %d", resp.StatusCode)
	}
	var res mineResult
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("decode dictionary result: %v", err)
	}
	if res.Total == 0 || len(res.Entries) == 0 {
		t.Fatal("My Telemetry scope returned no entries from seeded dictionary")
	}
	first := res.Entries[0]
	if first.Name == "" || len(first.SignalTypes) == 0 {
		t.Fatalf("entry missing fields normMine() needs: %+v", first)
	}

	dResp, dBody := getBody(t, ts, "/api/v1/dictionary/"+first.Name)
	if dResp.StatusCode != http.StatusOK {
		t.Fatalf("dictionary detail %q: status %d", first.Name, dResp.StatusCode)
	}
	var item struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(dBody), &item); err != nil {
		t.Fatalf("decode dictionary detail: %v", err)
	}
	if item.Name != first.Name {
		t.Errorf("dictionary detail name = %q, want %q", item.Name, first.Name)
	}
}

// compareResult mirrors the JSON shape app.js renderCompare()/renderCompareBucket
// consume off /api/v1/semconv/compare.
type compareResult struct {
	Total   int `json:"total"`
	Buckets map[string]struct {
		Count   int `json:"count"`
		Entries []struct {
			Name           string `json:"name"`
			Classification string `json:"classification"`
			TelemetryType  string `json:"telemetry_type"`
			RegistryType   string `json:"registry_type"`
			RegistryKey    string `json:"registry_key"`
		} `json:"entries"`
	} `json:"buckets"`
}

// AC (S4): "Compare tab renders the four buckets with counts + drill-in." The
// seeded http.request.method (telemetry string vs registry enum) is a known
// member of the official registry, so it lands in the matched bucket — and the
// drill-in entry carries the registry_key the Compare detail deep-links from.
func TestSPA_CompareScopeReturnsRenderableContract(t *testing.T) {
	ts := newWiredTestServer(t)

	resp, body := getBody(t, ts, "/api/v1/semconv/compare")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("compare: status %d", resp.StatusCode)
	}
	var res compareResult
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatalf("decode compare result: %v", err)
	}
	if res.Total == 0 {
		t.Fatal("compare returned zero attributes for seeded dictionary")
	}
	// All four buckets the SPA renders must be present so the bucket cards show
	// a count (even zero) rather than rendering undefined.
	for _, c := range []string{"matched", "type-mismatch", "deprecated", "not-in-registry"} {
		if _, ok := res.Buckets[c]; !ok {
			t.Errorf("compare response missing bucket %q", c)
		}
	}
	matched := res.Buckets["matched"]
	if matched.Count == 0 {
		t.Fatal("http.request.method should land in the matched bucket")
	}
	var found bool
	for _, e := range matched.Entries {
		if e.Name == "http.request.method" {
			found = true
			if e.RegistryKey != "attribute:http.request.method" {
				t.Errorf("matched entry registry_key = %q", e.RegistryKey)
			}
			if e.TelemetryType != "string" {
				t.Errorf("matched entry telemetry_type = %q", e.TelemetryType)
			}
		}
	}
	if !found {
		t.Error("http.request.method not present in matched bucket entries")
	}
}
