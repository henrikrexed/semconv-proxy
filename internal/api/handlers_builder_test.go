package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
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

func postExportZip(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/builder/export.zip", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleBuilderExportZip(w, req)
	return w
}

func TestBuilderExportZipOK(t *testing.T) {
	s := newTestServer(t)
	body := `{
		"manifest": {"schema_url": "https://acme.com/schemas/0.1.0"},
		"groups": [
			{"namespace": "http", "attributes": [{"id": "http.request.method", "type": "string"}]}
		]
	}`

	w := postExportZip(t, s, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition = %q, want attachment filename", cd)
	}

	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	got := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(data)
	}
	if _, ok := got["manifest.yaml"]; !ok {
		t.Errorf("zip missing manifest.yaml, got %v", keysOf(got))
	}
	if _, ok := got["groups/http.yaml"]; !ok {
		t.Errorf("zip missing groups/http.yaml, got %v", keysOf(got))
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBuilderExportZipMissingSchemaURL(t *testing.T) {
	s := newTestServer(t)
	w := postExportZip(t, s, `{"groups": []}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestBuilderExportZipMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/builder/export.zip", nil)
	w := httptest.NewRecorder()
	s.handleBuilderExportZip(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
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

func TestBuilderGenerateUnversionedSchemaURL(t *testing.T) {
	s := newTestServer(t)
	// No version segment -> rejected (L3, §10.1).
	w := postGenerate(t, s, `{"manifest": {"schema_url": "https://acme.com/schemas"}, "groups": []}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestBuilderGenerateTooManyDependencies(t *testing.T) {
	s := newTestServer(t)
	body := `{
		"manifest": {
			"schema_url": "https://acme.com/schemas/0.1.0",
			"dependencies": [
				{"schema_url": "https://opentelemetry.io/schemas/1.41.1"},
				{"schema_url": "https://example.com/schemas/2.0.0"}
			]
		},
		"groups": []
	}`
	w := postGenerate(t, s, body)
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

func TestIsVersionedSchemaURL(t *testing.T) {
	cases := map[string]bool{
		"https://acme.com/schemas/0.1.0": true,
		"http://host:8080/path/1.2.3":    true,
		"https://acme.com/v2":            true,  // digit in final segment
		"https://acme.com/schemas":       false, // no digit in segment
		"https://acme.com":               false, // empty path
		"ftp://acme.com/1.0.0":           false, // wrong scheme
		"/schemas/1.0.0":                 false, // no host
		"":                               false, // empty / unparseable host
		"://bad":                         false, // parse error
	}
	for in, want := range cases {
		if got := isVersionedSchemaURL(in); got != want {
			t.Errorf("isVersionedSchemaURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFirstNonEmptyStr(t *testing.T) {
	if got := firstNonEmptyStr("", "b", "c"); got != "b" {
		t.Errorf("firstNonEmptyStr = %q, want b", got)
	}
	if got := firstNonEmptyStr("", ""); got != "" {
		t.Errorf("firstNonEmptyStr(all empty) = %q, want empty", got)
	}
}

func TestBuilderPolicyTemplatesOK(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/builder/policy-templates", nil)
	w := httptest.NewRecorder()
	s.handleBuilderPolicyTemplates(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var res struct {
		Templates []struct {
			ID    string `json:"id"`
			Stage string `json:"stage"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(res.Templates) == 0 {
		t.Fatal("expected a non-empty policy template catalog")
	}
	for _, tmpl := range res.Templates {
		if tmpl.ID == "" || tmpl.Stage == "" {
			t.Errorf("template missing id/stage: %+v", tmpl)
		}
	}
}

func TestBuilderPolicyTemplatesMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/builder/policy-templates", nil)
	w := httptest.NewRecorder()
	s.handleBuilderPolicyTemplates(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

// TestBuilderGenerateWithPolicies covers the S3 path through the handler: a
// selected template renders a policies/<name>.rego file in the response set.
func TestBuilderGenerateWithPolicies(t *testing.T) {
	s := newTestServer(t)
	body := `{
		"manifest": {"schema_url": "https://acme.com/schemas/0.1.0"},
		"groups": [{"namespace": "http", "attributes": [{"id": "http.request.method"}]}],
		"policies": [{"template_id": "naming_prefix_required", "params": {"prefix": "acme."}}]
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
	rego, ok := res.Files["policies/naming_prefix_required.rego"]
	if !ok {
		t.Fatalf("expected policies/naming_prefix_required.rego, got %v", res.Files)
	}
	if !strings.Contains(rego, "deny contains v if {") || !strings.Contains(rego, `"type": "semconv_attribute"`) {
		t.Errorf("emitted rego is not §10.2-conformant:\n%s", rego)
	}
}

// TestBuilderGenerateUnknownPolicyTemplate ensures an unknown template id is a
// generation error, not a silent skip.
func TestBuilderGenerateUnknownPolicyTemplate(t *testing.T) {
	s := newTestServer(t)
	body := `{
		"manifest": {"schema_url": "https://acme.com/schemas/0.1.0"},
		"groups": [],
		"policies": [{"template_id": "nope"}]
	}`
	w := postGenerate(t, s, body)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
}
