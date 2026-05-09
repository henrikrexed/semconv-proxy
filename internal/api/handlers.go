package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

func (s *Server) handleDictionary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	filter := &dictionary.Filter{
		SignalType: dictionary.SignalType(r.URL.Query().Get("type")),
		Pattern:    r.URL.Query().Get("q"),
		Sort:       r.URL.Query().Get("sort"),
		Order:      r.URL.Query().Get("order"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	} else {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	total := int(s.dict.Count())
	filtered := s.dict.List(filter)

	type entryResponse struct {
		Name        string                  `json:"name"`
		Type        string                  `json:"type"`
		SignalTypes []dictionary.SignalType `json:"signal_types"`
		Cardinality int64                   `json:"cardinality"`
		FirstSeen   time.Time               `json:"first_seen"`
		LastSeen    time.Time               `json:"last_seen"`
		Status      dictionary.EntryStatus  `json:"status"`
	}

	entries := make([]entryResponse, len(filtered))
	for i, e := range filtered {
		card := e.Cardinality
		if s.tracker != nil {
			card = s.tracker.Cardinality(e.Name)
		}
		entries[i] = entryResponse{
			Name:        e.Name,
			Type:        e.Type,
			SignalTypes: e.SignalTypes,
			Cardinality: card,
			FirstSeen:   e.FirstSeen,
			LastSeen:    e.LastSeen,
			Status:      e.Status,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":   total,
		"offset":  filter.Offset,
		"limit":   filter.Limit,
		"entries": entries,
	})
}

func (s *Server) handleDictionaryEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/dictionary/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "attribute name required", "BAD_REQUEST")
		return
	}

	entry, ok := s.dict.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "attribute not found", "NOT_FOUND")
		return
	}

	cardinality := entry.Cardinality
	if s.tracker != nil {
		cardinality = s.tracker.Cardinality(name)
	}

	response := map[string]interface{}{
		"name":         entry.Name,
		"type":         entry.Type,
		"signal_types": entry.SignalTypes,
		"cardinality":  cardinality,
		"first_seen":   entry.FirstSeen,
		"last_seen":    entry.LastSeen,
		"status":       entry.Status,
	}

	if s.tracker != nil {
		topValues := s.tracker.TopValues(name)
		type topVal struct {
			Value            string `json:"value"`
			ApproximateCount uint64 `json:"approximate_count"`
		}
		var tvs []topVal
		for _, tv := range topValues {
			tvs = append(tvs, topVal{Value: tv.Value, ApproximateCount: tv.Count})
		}
		response["top_values"] = tvs
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCardinality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	used, limit, pct := s.tracker.GlobalUtilization()

	threshold := int64(100)
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.ParseInt(t, 10, 64); err == nil {
			threshold = v
		}
	}

	highAttrs := s.tracker.HighCardinality(threshold)
	type attrResp struct {
		Name           string  `json:"name"`
		Cardinality    int64   `json:"cardinality"`
		Cap            int     `json:"cap"`
		UtilizationPct float64 `json:"utilization_pct"`
	}
	var attrs []attrResp
	for _, a := range highAttrs {
		utilPct := float64(0)
		if a.Cap > 0 {
			utilPct = float64(a.Cardinality) / float64(a.Cap) * 100
		}
		attrs = append(attrs, attrResp{
			Name:           a.Name,
			Cardinality:    a.Cardinality,
			Cap:            a.Cap,
			UtilizationPct: utilPct,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"global_budget": map[string]interface{}{
			"used":            used,
			"limit":           limit,
			"utilization_pct": pct,
		},
		"attributes": attrs,
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.ready || (s.healthAgg != nil && !s.healthAgg.IsReady()) {
		status := "loading"
		if s.healthAgg != nil {
			components := s.healthAgg.GetAll()
			type compStatus struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			}
			var comps []compStatus
			for _, c := range components {
				comps = append(comps, compStatus{Name: c.Name, Status: c.Status.String()})
			}
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"status":             status,
				"dictionary_entries": s.dict.Count(),
				"components":         comps,
			})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":             status,
			"dictionary_entries": s.dict.Count(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "ready",
		"dictionary_entries": s.dict.Count(),
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	format := r.URL.Query().Get("format")
	if format != "weaver" {
		writeError(w, http.StatusBadRequest, "only 'weaver' format is supported", "BAD_REQUEST")
		return
	}

	signalType := r.URL.Query().Get("type")
	prefix := r.URL.Query().Get("prefix")

	entries := s.dict.List(&dictionary.Filter{
		SignalType: dictionary.SignalType(signalType),
		Pattern:    prefix,
	})

	yaml, err := s.exporter.Export(entries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export failed", "EXPORT_ERROR")
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(yaml)
}
