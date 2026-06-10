package api

import (
	"encoding/json"
	"net/http"

	"github.com/henrikrexed/semconv-proxy/internal/export"
)

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
