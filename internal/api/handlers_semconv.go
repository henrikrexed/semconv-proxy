package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/henrikrexed/semconv-proxy/internal/semconv"
)

// handleCommunitySearch serves faceted search over the embedded official OTel
// semantic-convention registry.
//
//	GET /api/v1/semconv/community?q=&type=&stability=&namespace=&limit=&offset=
//
// type and stability may be repeated to OR multiple values.
func (s *Server) handleCommunitySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}
	if s.semconv == nil {
		writeError(w, http.StatusServiceUnavailable, "semconv registry unavailable", "REGISTRY_UNAVAILABLE")
		return
	}

	q := r.URL.Query()
	query := semconv.Query{
		Text:      q.Get("q"),
		Types:     parseItemTypes(q["type"]),
		Stability: splitMulti(q["stability"]),
		Namespace: q.Get("namespace"),
		Limit:     parseIntDefault(q.Get("limit"), 50, 1000),
		Offset:    parseIntDefault(q.Get("offset"), 0, -1),
	}

	writeJSON(w, http.StatusOK, s.semconv.Search(query))
}

// handleCommunityEntry returns a single registry item by key.
//
//	GET /api/v1/semconv/community/{key}
//
// key is the item key such as "attribute:http.request.method" or
// "metric:http.server.request.duration".
func (s *Server) handleCommunityEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}
	if s.semconv == nil {
		writeError(w, http.StatusServiceUnavailable, "semconv registry unavailable", "REGISTRY_UNAVAILABLE")
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/api/v1/semconv/community/")
	if key == "" {
		writeError(w, http.StatusBadRequest, "item key required", "BAD_REQUEST")
		return
	}

	item, ok := s.semconv.Get(key)
	if !ok {
		writeError(w, http.StatusNotFound, "item not found", "NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// parseItemTypes maps repeated/comma-separated type params to ItemType values.
func parseItemTypes(vals []string) []semconv.ItemType {
	strs := splitMulti(vals)
	if len(strs) == 0 {
		return nil
	}
	out := make([]semconv.ItemType, 0, len(strs))
	for _, s := range strs {
		out = append(out, semconv.ItemType(s))
	}
	return out
}

// splitMulti flattens repeated query params and comma-separated values into a
// trimmed, non-empty slice.
func splitMulti(vals []string) []string {
	var out []string
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// parseIntDefault parses s as a non-negative int, falling back to def. When max
// is >= 0 the result is clamped to max.
func parseIntDefault(s string, def, max int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	if max >= 0 && n > max {
		return max
	}
	return n
}
