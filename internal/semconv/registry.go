// Package semconv provides an in-memory, faceted query layer over a build-time
// pinned snapshot of the official OpenTelemetry semantic-convention registry.
//
// The snapshot (data/registry.json) is produced by `weaver registry resolve`
// against a pinned semconv release and embedded via go:embed, so the proxy has
// no runtime dependency on Weaver or network access.
package semconv

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed data/registry.json
var registryJSON []byte

// ItemType is the kind of semantic-convention signal an Item represents.
type ItemType string

const (
	ItemAttribute ItemType = "attribute"
	ItemMetric    ItemType = "metric"
	ItemSpan      ItemType = "span"
	ItemEvent     ItemType = "event"
	ItemEntity    ItemType = "entity"
)

// Deprecation captures the deprecation metadata attached to an attribute or group.
type Deprecation struct {
	Reason    string `json:"reason,omitempty"`
	RenamedTo string `json:"renamed_to,omitempty"`
	Note      string `json:"note,omitempty"`
}

// EnumMember is one allowed value of an enum-typed attribute.
type EnumMember struct {
	ID        string `json:"id,omitempty"`
	Value     string `json:"value"`
	Brief     string `json:"brief,omitempty"`
	Stability string `json:"stability,omitempty"`
}

// Item is a normalized, searchable semantic-convention entry. Attributes,
// metrics, spans, events and entities are flattened into this single shape so
// the UI can search across all signal types uniformly.
type Item struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Type        ItemType     `json:"type"`
	Namespace   string       `json:"namespace"`
	Brief       string       `json:"brief,omitempty"`
	Note        string       `json:"note,omitempty"`
	Stability   string       `json:"stability,omitempty"`
	Deprecated  *Deprecation `json:"deprecated,omitempty"`
	ValueType   string       `json:"value_type,omitempty"` // attribute value type (string, int, enum, ...)
	Enum        []EnumMember `json:"enum,omitempty"`       // members when ValueType == "enum"
	Examples    []string     `json:"examples,omitempty"`
	Requirement string       `json:"requirement_level,omitempty"`
	Unit        string       `json:"unit,omitempty"`       // metric
	Instrument  string       `json:"instrument,omitempty"` // metric
	SpanKind    string       `json:"span_kind,omitempty"`  // span
	Attributes  []string     `json:"attributes,omitempty"` // referenced attribute names (groups)
}

// raw JSON shapes for the resolved registry. Only fields we consume are declared.
type rawRegistry struct {
	RegistryURL string     `json:"registry_url"`
	Groups      []rawGroup `json:"groups"`
}

type rawGroup struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Brief      string         `json:"brief"`
	Note       string         `json:"note"`
	Stability  string         `json:"stability"`
	Deprecated *Deprecation   `json:"deprecated"`
	Attributes []rawAttribute `json:"attributes"`
	MetricName string         `json:"metric_name"`
	Instrument string         `json:"instrument"`
	Unit       string         `json:"unit"`
	SpanKind   string         `json:"span_kind"`
	Name       string         `json:"name"` // event / entity name
}

type rawAttribute struct {
	Name             string          `json:"name"`
	Type             json.RawMessage `json:"type"`
	Brief            string          `json:"brief"`
	Note             string          `json:"note"`
	Examples         json.RawMessage `json:"examples"`
	RequirementLevel json.RawMessage `json:"requirement_level"`
	Stability        string          `json:"stability"`
	Deprecated       *Deprecation    `json:"deprecated"`
}

// Registry is the parsed, indexed snapshot.
type Registry struct {
	url   string
	items []Item
	byKey map[string]int
}

// Load parses the embedded snapshot into a queryable Registry.
func Load() (*Registry, error) {
	return parse(registryJSON)
}

func parse(data []byte) (*Registry, error) {
	var raw rawRegistry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("semconv: parse registry: %w", err)
	}

	reg := &Registry{url: raw.RegistryURL}
	seenAttr := make(map[string]int) // attribute name -> index in items

	for _, g := range raw.Groups {
		switch g.Type {
		case "metric":
			reg.items = append(reg.items, Item{
				Key:        "metric:" + g.MetricName,
				Name:       g.MetricName,
				Type:       ItemMetric,
				Namespace:  namespaceOf(g.MetricName),
				Brief:      g.Brief,
				Note:       g.Note,
				Stability:  g.Stability,
				Deprecated: g.Deprecated,
				Unit:       g.Unit,
				Instrument: g.Instrument,
				Attributes: attrNames(g.Attributes),
			})
		case "span":
			reg.items = append(reg.items, Item{
				Key:        g.ID,
				Name:       g.ID,
				Type:       ItemSpan,
				Namespace:  namespaceOf(strings.TrimPrefix(g.ID, "span.")),
				Brief:      g.Brief,
				Note:       g.Note,
				Stability:  g.Stability,
				Deprecated: g.Deprecated,
				SpanKind:   g.SpanKind,
				Attributes: attrNames(g.Attributes),
			})
		case "event":
			name := g.Name
			if name == "" {
				name = strings.TrimPrefix(g.ID, "event.")
			}
			reg.items = append(reg.items, Item{
				Key:        "event:" + name,
				Name:       name,
				Type:       ItemEvent,
				Namespace:  namespaceOf(name),
				Brief:      g.Brief,
				Note:       g.Note,
				Stability:  g.Stability,
				Deprecated: g.Deprecated,
				Attributes: attrNames(g.Attributes),
			})
		case "entity":
			name := g.Name
			if name == "" {
				name = strings.TrimPrefix(g.ID, "entity.")
			}
			reg.items = append(reg.items, Item{
				Key:        "entity:" + name,
				Name:       name,
				Type:       ItemEntity,
				Namespace:  namespaceOf(name),
				Brief:      g.Brief,
				Note:       g.Note,
				Stability:  g.Stability,
				Deprecated: g.Deprecated,
				Attributes: attrNames(g.Attributes),
			})
		}

		// Attributes are flattened and de-duplicated across all groups. The first
		// definition with a brief wins; later occurrences only backfill detail so
		// reference-only mentions don't clobber a richer definition.
		for _, a := range g.Attributes {
			if a.Name == "" {
				continue
			}
			item := attributeItem(a)
			if idx, ok := seenAttr[a.Name]; ok {
				backfill(&reg.items[idx], item)
				continue
			}
			seenAttr[a.Name] = len(reg.items)
			reg.items = append(reg.items, item)
		}
	}

	sort.SliceStable(reg.items, func(i, j int) bool {
		if reg.items[i].Type != reg.items[j].Type {
			return reg.items[i].Type < reg.items[j].Type
		}
		return reg.items[i].Name < reg.items[j].Name
	})

	reg.byKey = make(map[string]int, len(reg.items))
	for i, it := range reg.items {
		reg.byKey[it.Key] = i
	}
	return reg, nil
}

func attributeItem(a rawAttribute) Item {
	valueType, enum := attrType(a.Type)
	return Item{
		Key:         "attribute:" + a.Name,
		Name:        a.Name,
		Type:        ItemAttribute,
		Namespace:   namespaceOf(a.Name),
		Brief:       a.Brief,
		Note:        a.Note,
		Stability:   a.Stability,
		Deprecated:  a.Deprecated,
		ValueType:   valueType,
		Enum:        enum,
		Examples:    examples(a.Examples),
		Requirement: requirementLevel(a.RequirementLevel),
	}
}

// backfill fills empty fields on dst from src without overwriting existing data.
func backfill(dst *Item, src Item) {
	if dst.Brief == "" {
		dst.Brief = src.Brief
	}
	if dst.Note == "" {
		dst.Note = src.Note
	}
	if dst.ValueType == "" {
		dst.ValueType = src.ValueType
	}
	if len(dst.Examples) == 0 {
		dst.Examples = src.Examples
	}
	if dst.Stability == "" {
		dst.Stability = src.Stability
	}
	if dst.Deprecated == nil {
		dst.Deprecated = src.Deprecated
	}
}

func attrNames(attrs []rawAttribute) []string {
	if len(attrs) == 0 {
		return nil
	}
	names := make([]string, 0, len(attrs))
	for _, a := range attrs {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return names
}

// namespaceOf returns the leading dotted segment of a name (e.g. "http" from
// "http.request.method"). Names without a dot are their own namespace.
func namespaceOf(name string) string {
	if i := strings.IndexByte(name, '.'); i > 0 {
		return name[:i]
	}
	return name
}

// attrType resolves an attribute's type. It is either a string ("string",
// "int", ...) or an enum object carrying members. For enums we return the type
// label "enum" plus the parsed members so callers can surface allowed values.
func attrType(raw json.RawMessage) (string, []EnumMember) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var enum struct {
		Members []EnumMember `json:"members"`
	}
	if err := json.Unmarshal(raw, &enum); err == nil {
		return "enum", enum.Members
	}
	return "enum", nil
}

// examples normalizes the polymorphic examples field (scalar, array of scalars,
// or array of arrays) into a flat slice of display strings. Nested arrays are
// flattened so a value like [["a","b"],["c"]] renders as "a", "b", "c" rather
// than raw JSON.
func examples(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return flattenExamples(v)
}

func flattenExamples(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		var out []string
		for _, e := range t {
			out = append(out, flattenExamples(e)...)
		}
		return out
	default:
		return []string{scalarString(t)}
	}
}

func scalarString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// render integers without trailing .0
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// requirementLevel normalizes the requirement_level field, which is either a
// string ("required", "recommended", ...) or a single-key object such as
// {"conditionally_required": "if ..."}.
func requirementLevel(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		for k := range obj {
			return k
		}
	}
	return ""
}
