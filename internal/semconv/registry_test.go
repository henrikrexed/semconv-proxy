package semconv

import (
	"encoding/json"
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg.Len() == 0 {
		t.Fatal("expected non-empty registry")
	}

	// All five signal types should be represented in the official registry.
	counts := map[ItemType]int{}
	for _, it := range reg.Items() {
		counts[it.Type]++
		if it.Name == "" || it.Key == "" {
			t.Fatalf("item with empty name/key: %+v", it)
		}
	}
	for _, typ := range []ItemType{ItemAttribute, ItemMetric, ItemSpan, ItemEvent, ItemEntity} {
		if counts[typ] == 0 {
			t.Errorf("expected at least one %s item", typ)
		}
	}
}

func TestAttributesAreDeduplicated(t *testing.T) {
	reg := mustLoad(t)
	seen := map[string]bool{}
	for _, it := range reg.Items() {
		if it.Type != ItemAttribute {
			continue
		}
		if seen[it.Name] {
			t.Fatalf("duplicate attribute item: %s", it.Name)
		}
		seen[it.Name] = true
	}
}

func TestKnownAttributePresent(t *testing.T) {
	reg := mustLoad(t)
	it, ok := reg.Get("attribute:http.request.method")
	if !ok {
		t.Fatal("expected http.request.method attribute")
	}
	if it.Namespace != "http" {
		t.Errorf("namespace = %q, want http", it.Namespace)
	}
	if it.ValueType == "" {
		t.Error("expected a value type")
	}
	if it.Stability != "stable" {
		t.Errorf("stability = %q, want stable", it.Stability)
	}
}

func TestParseNormalization(t *testing.T) {
	const data = `{
	  "registry_url": "test://reg",
	  "groups": [
	    {
	      "id": "attr.http",
	      "type": "attribute_group",
	      "attributes": [
	        {
	          "name": "http.request.method",
	          "type": {"members": [{"id": "get", "value": "GET"}]},
	          "brief": "HTTP method",
	          "examples": ["GET", "POST"],
	          "requirement_level": {"conditionally_required": "always"},
	          "stability": "stable"
	        },
	        {
	          "name": "net.peer.port",
	          "type": "int",
	          "examples": 8080,
	          "requirement_level": "recommended",
	          "stability": "development",
	          "deprecated": {"reason": "renamed", "renamed_to": "server.port"}
	        }
	      ]
	    },
	    {
	      "id": "metric.http.server.duration",
	      "type": "metric",
	      "metric_name": "http.server.request.duration",
	      "instrument": "histogram",
	      "unit": "s",
	      "stability": "stable",
	      "brief": "Duration",
	      "attributes": [{"name": "http.request.method", "type": "string"}]
	    }
	  ]
	}`
	reg, err := parse([]byte(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	method, ok := reg.Get("attribute:http.request.method")
	if !ok {
		t.Fatal("missing http.request.method")
	}
	if method.ValueType != "enum" {
		t.Errorf("enum type = %q, want enum", method.ValueType)
	}
	if method.Requirement != "conditionally_required" {
		t.Errorf("requirement = %q", method.Requirement)
	}
	if len(method.Examples) != 2 || method.Examples[0] != "GET" {
		t.Errorf("examples = %v", method.Examples)
	}

	port, _ := reg.Get("attribute:net.peer.port")
	if port.ValueType != "int" {
		t.Errorf("int type = %q", port.ValueType)
	}
	if port.Requirement != "recommended" {
		t.Errorf("requirement = %q", port.Requirement)
	}
	if len(port.Examples) != 1 || port.Examples[0] != "8080" {
		t.Errorf("scalar example = %v", port.Examples)
	}
	if port.Deprecated == nil || port.Deprecated.RenamedTo != "server.port" {
		t.Errorf("deprecated = %+v", port.Deprecated)
	}

	metric, ok := reg.Get("metric:http.server.request.duration")
	if !ok {
		t.Fatal("missing metric")
	}
	if metric.Type != ItemMetric || metric.Unit != "s" || metric.Instrument != "histogram" {
		t.Errorf("metric = %+v", metric)
	}
	if metric.Namespace != "http" {
		t.Errorf("metric namespace = %q", metric.Namespace)
	}
}

// Ensure Item marshals cleanly for the API layer (no unexpected panics on the
// polymorphic fields).
func TestItemJSONRoundTrip(t *testing.T) {
	reg := mustLoad(t)
	b, err := json.Marshal(reg.Items()[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Item
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func mustLoad(t *testing.T) *Registry {
	t.Helper()
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}
