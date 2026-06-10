package api

import (
	"net/http"
	"strings"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/semconv"
)

// Compare classification buckets. Each observed attribute lands in exactly one,
// evaluated in this priority order: a name miss wins over everything, then
// deprecation, then a type mismatch, otherwise it matched cleanly.
const (
	classMatched       = "matched"
	classTypeMismatch  = "type-mismatch"
	classDeprecated    = "deprecated"
	classNotInRegistry = "not-in-registry"
)

// compareClasses fixes the bucket order so the response (and the UI) is stable.
var compareClasses = []string{classMatched, classTypeMismatch, classDeprecated, classNotInRegistry}

type compareItem struct {
	Name           string                  `json:"name"`
	Classification string                  `json:"classification"`
	TelemetryType  string                  `json:"telemetry_type"`
	RegistryType   string                  `json:"registry_type,omitempty"`
	SignalTypes    []dictionary.SignalType `json:"signal_types"`
	Cardinality    int64                   `json:"cardinality"`
	RegistryKey    string                  `json:"registry_key,omitempty"`
	Deprecation    *semconv.Deprecation    `json:"deprecation,omitempty"`
}

type compareBucket struct {
	Count   int           `json:"count"`
	Entries []compareItem `json:"entries"`
}

// handleCompare cross-references the user's observed telemetry attributes
// (the dictionary) against the embedded official registry.
//
//	GET /api/v1/semconv/compare?q=&limit=
//
// q narrows by attribute-name substring. limit caps the number of entries
// returned per bucket; bucket counts always reflect the full match set.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
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
	limit := parseIntDefault(q.Get("limit"), 200, 1000)

	entries := s.dict.List(&dictionary.Filter{Pattern: q.Get("q")})

	buckets := map[string]*compareBucket{}
	for _, c := range compareClasses {
		buckets[c] = &compareBucket{Entries: []compareItem{}}
	}

	total := 0
	for _, e := range entries {
		class, regItem, found := s.classifyAttribute(e)
		b := buckets[class]
		b.Count++
		total++
		if len(b.Entries) >= limit {
			continue
		}

		card := e.Cardinality
		if s.tracker != nil {
			card = s.tracker.Cardinality(e.Name)
		}
		item := compareItem{
			Name:           e.Name,
			Classification: class,
			TelemetryType:  e.Type,
			SignalTypes:    e.SignalTypes,
			Cardinality:    card,
		}
		if found {
			item.RegistryType = regItem.ValueType
			item.RegistryKey = regItem.Key
			item.Deprecation = regItem.Deprecated
		}
		b.Entries = append(b.Entries, item)
	}

	out := make(map[string]compareBucket, len(buckets))
	for c, b := range buckets {
		out[c] = *b
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":   total,
		"limit":   limit,
		"buckets": out,
	})
}

// classifyAttribute resolves an observed attribute into a compare bucket and,
// when the attribute exists in the registry, returns the matched item.
func (s *Server) classifyAttribute(e *dictionary.AttributeEntry) (string, semconv.Item, bool) {
	item, ok := s.semconv.Get("attribute:" + e.Name)
	if !ok {
		return classNotInRegistry, semconv.Item{}, false
	}
	if item.Deprecated != nil {
		return classDeprecated, item, true
	}
	if typesConflict(e.Type, item.ValueType) {
		return classTypeMismatch, item, true
	}
	return classMatched, item, true
}

// typesConflict reports whether an observed scalar type contradicts the
// registry's declared value type. It is deliberately conservative: a conflict
// is only reported when both sides resolve to a known, differing scalar.
// Indeterminate types (empty, enum without a scalar mapping, maps, slices,
// templates) never produce a mismatch, so we don't cry wolf on shapes we can't
// reliably compare in this MVP.
func typesConflict(telemetry, registry string) bool {
	t := canonicalScalar(telemetry)
	r := canonicalScalar(registry)
	if t == "" || r == "" {
		return false
	}
	return t != r
}

// canonicalScalar maps both telemetry- and registry-side type labels onto a
// shared scalar vocabulary, returning "" for anything not confidently scalar.
func canonicalScalar(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	t = strings.TrimSuffix(t, "[]") // compare arrays by element type
	switch t {
	case "string", "enum": // enum members are strings on the wire
		return "string"
	case "int":
		return "int"
	case "double":
		return "double"
	case "bool", "boolean":
		return "bool"
	default:
		return ""
	}
}
