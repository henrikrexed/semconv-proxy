package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/henrikrexed/semconv-proxy/internal/export"
)

// isVersionedSchemaURL enforces §10.1: schema_url MUST be a versioned OTel schema
// URL (http[s]://server[:port]/path/<version>). The check is intentionally
// lenient — it requires an http(s) scheme, a host, and a trailing path segment
// that looks like a version (contains a digit).
func isVersionedSchemaURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return false
	}
	seg := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		seg = p[i+1:]
	}
	return strings.ContainsAny(seg, "0123456789")
}

// handleBuilderGenerate turns posted builder state into a Weaver registry file
// set (path -> content).
//
//	POST /api/v1/builder/generate
//
// The body is an export.BuilderState. schema_url on the manifest is required.
// When the state declares no dependencies, the pinned OTel dependency derived
// from the embedded registry is injected as the default.
func (s *Server) handleBuilderGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	var state export.BuilderState
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}

	if state.Manifest.SchemaURL == "" {
		writeError(w, http.StatusBadRequest, "manifest.schema_url is required", "BAD_REQUEST")
		return
	}
	if !isVersionedSchemaURL(state.Manifest.SchemaURL) {
		writeError(w, http.StatusBadRequest, "manifest.schema_url must be a versioned OTel schema URL (e.g. https://host/path/1.2.3)", "BAD_REQUEST")
		return
	}
	if len(state.Manifest.Dependencies) > 1 {
		writeError(w, http.StatusBadRequest, "manifest.dependencies allows at most one entry (weaver v0.23, weaver#604)", "BAD_REQUEST")
		return
	}

	var defaultDep *export.DependencySpec
	if s.semconv != nil {
		dep := export.DefaultOTelDependency(s.semconv.URL())
		defaultDep = &dep
	}

	files, err := s.exporter.Generate(state, defaultDep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation failed", "GENERATE_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
	})
}
