package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postGenerate(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/builder/generate", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleBuilderGenerate(w, req)
	return w
}

func TestBuilderGenerateOK(t *testing.T) {
	s := newTestServer(t)
	body := `{
		"manifest": {"schema_url": "https://acme.com/schemas/0.1.0"},
		"groups": [
			{"namespace": "http", "attributes": [{"id": "http.request.method", "type": "string"}]}
		]
	}`

	w := postGenerate(t, s, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var res struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := res.Files["manifest.yaml"]; !ok {
		t.Errorf("expected manifest.yaml in file set, got %v", res.Files)
	}
	if _, ok := res.Files["groups/http.yaml"]; !ok {
		t.Errorf("expected groups/http.yaml in file set, got %v", res.Files)
	}
	// The server injects the pinned OTel dependency from the embedded registry
	// when the request declares none.
	if s.semconv != nil && !strings.Contains(res.Files["manifest.yaml"], "opentelemetry.io/schemas/") {
		t.Errorf("expected default OTel dependency injected; manifest:\n%s", res.Files["manifest.yaml"])
	}
}

func TestBuilderGenerateMissingSchemaURL(t *testing.T) {
	s := newTestServer(t)
	w := postGenerate(t, s, `{"groups": []}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestBuilderGenerateBadJSON(t *testing.T) {
	s := newTestServer(t)
	w := postGenerate(t, s, `{not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBuilderGenerateMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/builder/generate", nil)
	w := httptest.NewRecorder()
	s.handleBuilderGenerate(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}
