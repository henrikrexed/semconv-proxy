package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/export"
	"github.com/henrikrexed/semconv-proxy/internal/semconv"
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
	if state.Config != nil {
		for _, f := range state.Config.FindingFilters {
			if f.MinLevel != "" && !export.ValidFindingLevel(f.MinLevel) {
				writeError(w, http.StatusBadRequest, "config.finding_filters min_level must be one of information|improvement|violation", "BAD_REQUEST")
				return
			}
		}
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

// handleBuilderPolicyTemplates serves the static, proxy-versioned policy check
// catalog that drives the Checks UI.
//
//	GET /api/v1/builder/policy-templates
//
// Each template carries its parameter schema and its Weaver stage (rego package);
// the UI renders a form per template and the emitter (POST /builder/generate)
// turns the filled form into a `policies/<name>.rego` file.
func (s *Server) handleBuilderPolicyTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"templates": export.PolicyCatalog(),
	})
}

// seedAttribute is one discovered attribute shaped for the Definitions table. It
// carries both the editable authoring fields (pre-filled from the observed
// telemetry and, when matched, the official registry) and the community
// cross-reference state that drives the table's status chip.
type seedAttribute struct {
	ID           string                  `json:"id"`
	Namespace    string                  `json:"namespace"`
	Type         string                  `json:"type"` // pre-fill = observed dictionary type
	ObservedType string                  `json:"observed_type"`
	CrossRef     string                  `json:"cross_ref"` // matched | type-mismatch | deprecated | not-in-registry
	SignalTypes  []dictionary.SignalType `json:"signal_types"`
	Cardinality  int64                   `json:"cardinality"`
	RegistryKey  string                  `json:"registry_key,omitempty"`
	RegistryType string                  `json:"registry_type,omitempty"`
	// Suggested enrichment, surfaced from the registry when the attribute matched
	// so the user can accept the official wording instead of typing it.
	Brief            string               `json:"brief,omitempty"`
	Stability        string               `json:"stability,omitempty"`
	RequirementLevel string               `json:"requirement_level,omitempty"`
	Examples         []string             `json:"examples,omitempty"`
	Deprecation      *semconv.Deprecation `json:"deprecation,omitempty"`
}

// handleBuilderSeed seeds the Definitions-table state from the live dictionary,
// cross-referenced against the embedded official registry.
//
//	GET /api/v1/builder/seed?q=&limit=
//
// Each observed attribute is returned with its type pre-filled from the observed
// telemetry type and tagged with its community cross-ref bucket (matched /
// type-mismatch / deprecated / not-in-registry), reusing the Phase 1 compare
// classification. For matched attributes, the official brief/stability/
// requirement-level/examples are surfaced as suggested enrichment.
func (s *Server) handleBuilderSeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}
	if s.semconv == nil {
		writeError(w, http.StatusServiceUnavailable, "semconv registry unavailable", "REGISTRY_UNAVAILABLE")
		return
	}
	if s.dict == nil {
		writeError(w, http.StatusServiceUnavailable, "dictionary unavailable", "DICTIONARY_UNAVAILABLE")
		return
	}

	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 500, 2000)

	entries := s.dict.List(&dictionary.Filter{Pattern: q.Get("q")})

	// Collect the distinct signal types observed across the full result set
	// (before the row limit) so the Config tab's finding-filter signal_type
	// dropdown is sourced from the user's known signals (S4 acceptance).
	knownSignals := distinctSignalTypes(entries)

	attrs := make([]seedAttribute, 0, len(entries))
	for _, e := range entries {
		if len(attrs) >= limit {
			break
		}
		class, regItem, found := s.classifyAttribute(e)

		card := e.Cardinality
		if s.tracker != nil {
			card = s.tracker.Cardinality(e.Name)
		}

		a := seedAttribute{
			ID:           e.Name,
			Namespace:    namespaceFromAttr(e.Name),
			Type:         firstNonEmptyStr(e.Type, defaultAttributeTypeSeed),
			ObservedType: e.Type,
			CrossRef:     class,
			SignalTypes:  e.SignalTypes,
			Cardinality:  card,
		}
		if found {
			a.RegistryKey = regItem.Key
			a.RegistryType = regItem.ValueType
			a.Brief = regItem.Brief
			a.Stability = regItem.Stability
			a.RequirementLevel = regItem.Requirement
			a.Examples = regItem.Examples
			a.Deprecation = regItem.Deprecated
		}
		attrs = append(attrs, a)
	}

	var defaultDep *export.DependencySpec
	if s.semconv != nil {
		dep := export.DefaultOTelDependency(s.semconv.URL())
		defaultDep = &dep
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":              len(entries),
		"limit":              limit,
		"attributes":         attrs,
		"default_dependency": defaultDep,
		"known_signal_types": knownSignals,
	})
}

// distinctSignalTypes returns the unique signal types across the entries,
// preserving first-seen order so the dropdown is stable.
func distinctSignalTypes(entries []*dictionary.AttributeEntry) []dictionary.SignalType {
	seen := make(map[dictionary.SignalType]bool)
	out := make([]dictionary.SignalType, 0, 3)
	for _, e := range entries {
		for _, st := range e.SignalTypes {
			if st != "" && !seen[st] {
				seen[st] = true
				out = append(out, st)
			}
		}
	}
	return out
}

const defaultAttributeTypeSeed = "string"

// namespaceFromAttr returns the leading dotted segment of an attribute name as
// its suggested namespace (e.g. "http.request.method" -> "http"), falling back
// to "custom" for un-namespaced names.
func namespaceFromAttr(name string) string {
	if i := strings.IndexByte(name, '.'); i >= 0 && i > 0 {
		return name[:i]
	}
	return "custom"
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
