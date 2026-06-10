package export

import (
	"strings"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"gopkg.in/yaml.v3"
)

func TestWeaverExportBasic(t *testing.T) {
	exporter := NewWeaverExporter()
	now := time.Now()

	entries := []*dictionary.AttributeEntry{
		{
			Name:        "http.request.method",
			Type:        "string",
			SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
			FirstSeen:   now,
			LastSeen:    now,
			Status:      dictionary.StatusActive,
			Cardinality: 5,
		},
		{
			Name:        "http.response.status_code",
			Type:        "int",
			SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
			FirstSeen:   now,
			LastSeen:    now,
			Status:      dictionary.StatusActive,
			Cardinality: 10,
		},
	}

	yaml, err := exporter.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	output := string(yaml)
	if !strings.Contains(output, "http.request.method") {
		t.Error("expected http.request.method in output")
	}
	if !strings.Contains(output, "http.response.status_code") {
		t.Error("expected http.response.status_code in output")
	}
}

// sampleState exercises overrides, defaults, multi-attribute groups, and
// namespace grouping in one payload.
func sampleState() BuilderState {
	return BuilderState{
		Manifest: ManifestSpec{
			SchemaURL: "https://acme.com/schemas/0.1.0",
		},
		Groups: []GroupInput{
			{
				Namespace: "http",
				Brief:     "HTTP attributes.",
				Stability: "stable",
				Attributes: []AttributeInput{
					{ID: "http.request.method", Type: "string", Brief: "Method.", RequirementLevel: "required", Examples: []string{"GET", "POST"}},
					{ID: "http.response.status_code", Type: "int"}, // defaults applied
				},
			},
			{
				Namespace: "http", // same namespace -> same file
				Brief:     "More HTTP attributes.",
				Attributes: []AttributeInput{
					{ID: "http.route"},
				},
			},
			{
				Namespace:  "db",
				Attributes: []AttributeInput{{ID: "db.system"}},
			},
		},
	}
}

func parseManifest(t *testing.T, content string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &m); err != nil {
		t.Fatalf("manifest does not parse as YAML: %v", err)
	}
	return m
}

func TestGenerateManifestConformsToG0(t *testing.T) {
	e := NewWeaverExporter()
	files, err := e.Generate(BuilderState{Manifest: ManifestSpec{SchemaURL: "https://acme.com/schemas/0.1.0"}}, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	content, ok := files["manifest.yaml"]
	if !ok {
		t.Fatalf("expected file manifest.yaml, got keys %v", keysOf(files))
	}
	if _, bad := files["registry_manifest.yaml"]; bad {
		t.Error("must not emit registry_manifest.yaml (G0 §10.1)")
	}

	m := parseManifest(t, content)
	if m["schema_url"] != "https://acme.com/schemas/0.1.0" {
		t.Errorf("schema_url = %v, want acme schema", m["schema_url"])
	}
	if _, has := m["name"]; has {
		t.Error("manifest must not contain a name field (G0 §10.1)")
	}
	if m["stability"] != "development" {
		t.Errorf("stability default = %v, want development", m["stability"])
	}
}

func TestGenerateManifestOverridesAndDependencies(t *testing.T) {
	e := NewWeaverExporter()
	files, err := e.Generate(BuilderState{
		Manifest: ManifestSpec{
			SchemaURL:   "https://acme.com/schemas/0.1.0",
			Description: "Acme conventions.",
			Stability:   "stable",
			Dependencies: []DependencySpec{
				{SchemaURL: "https://opentelemetry.io/schemas/1.41.1", RegistryPath: "git@v1.41.1[model]"},
			},
		},
	}, &DependencySpec{SchemaURL: "https://should-not-appear"})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	m := parseManifest(t, files["manifest.yaml"])
	if m["stability"] != "stable" {
		t.Errorf("stability override = %v, want stable", m["stability"])
	}
	if m["description"] != "Acme conventions." {
		t.Errorf("description = %v", m["description"])
	}
	deps, ok := m["dependencies"].([]interface{})
	if !ok || len(deps) != 1 {
		t.Fatalf("expected 1 explicit dependency, got %v", m["dependencies"])
	}
	if strings.Contains(files["manifest.yaml"], "should-not-appear") {
		t.Error("default dependency must not be injected when dependencies are provided")
	}
}

func TestGenerateInjectsDefaultDependency(t *testing.T) {
	e := NewWeaverExporter()
	dep := DefaultOTelDependency("https://github.com/open-telemetry/semantic-conventions.git@v1.41.1[model]")
	files, err := e.Generate(BuilderState{Manifest: ManifestSpec{SchemaURL: "https://acme.com/schemas/0.1.0"}}, &dep)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	content := files["manifest.yaml"]
	if !strings.Contains(content, "https://opentelemetry.io/schemas/1.41.1") {
		t.Errorf("default dependency schema_url missing; got:\n%s", content)
	}
	if !strings.Contains(content, "v1.41.1[model]") {
		t.Errorf("default dependency registry_path missing; got:\n%s", content)
	}
}

func TestDefaultOTelDependency(t *testing.T) {
	dep := DefaultOTelDependency("https://github.com/open-telemetry/semantic-conventions.git@v1.41.1[model]")
	if dep.SchemaURL != "https://opentelemetry.io/schemas/1.41.1" {
		t.Errorf("schema_url = %q", dep.SchemaURL)
	}
	if dep.RegistryPath != "https://github.com/open-telemetry/semantic-conventions.git@v1.41.1[model]" {
		t.Errorf("registry_path = %q", dep.RegistryPath)
	}

	// No version token -> no schema_url, registry_path preserved.
	bare := DefaultOTelDependency("https://example.com/registry")
	if bare.SchemaURL != "" {
		t.Errorf("expected empty schema_url for versionless URL, got %q", bare.SchemaURL)
	}
}

func TestGenerateNamespaceGrouping(t *testing.T) {
	e := NewWeaverExporter()
	files, err := e.Generate(sampleState(), nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if _, ok := files["groups/http.yaml"]; !ok {
		t.Fatalf("expected groups/http.yaml, got %v", keysOf(files))
	}
	if _, ok := files["groups/db.yaml"]; !ok {
		t.Fatalf("expected groups/db.yaml, got %v", keysOf(files))
	}

	var httpFile genGroupFile
	if err := yaml.Unmarshal([]byte(files["groups/http.yaml"]), &httpFile); err != nil {
		t.Fatalf("groups/http.yaml does not parse: %v", err)
	}
	// Two http GroupInputs share the namespace -> merged into one group.
	if len(httpFile.Groups) != 1 {
		t.Fatalf("expected 1 merged http group, got %d", len(httpFile.Groups))
	}
	// Merged group accumulates all three attributes (multi-attribute coverage).
	if len(httpFile.Groups[0].Attributes) != 3 {
		t.Errorf("expected 3 attributes in merged http group, got %d", len(httpFile.Groups[0].Attributes))
	}
}

func TestGenerateFieldOverridesVsDefaults(t *testing.T) {
	e := NewWeaverExporter()
	files, err := e.Generate(sampleState(), nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	var httpFile genGroupFile
	if err := yaml.Unmarshal([]byte(files["groups/http.yaml"]), &httpFile); err != nil {
		t.Fatalf("parse http file: %v", err)
	}

	g0 := httpFile.Groups[0]
	if g0.Brief != "HTTP attributes." || g0.Stability != "stable" || g0.Type != "attribute_group" {
		t.Errorf("group override/default mismatch: %+v", g0)
	}

	overridden := g0.Attributes[0]
	if overridden.RequirementLevel != "required" || overridden.Type != "string" || len(overridden.Examples) != 2 {
		t.Errorf("attribute overrides not preserved: %+v", overridden)
	}

	defaulted := g0.Attributes[1] // only id+type provided
	if defaulted.Type != "int" {
		t.Errorf("explicit type lost: %q", defaulted.Type)
	}
	if defaulted.RequirementLevel != "recommended" || defaulted.Stability != "development" {
		t.Errorf("attribute defaults not applied: %+v", defaulted)
	}
	// Weaver hard-errors on attributes without a brief, so a default must fill in.
	if defaulted.Brief == "" {
		t.Error("attribute brief default must be non-empty (weaver requires brief)")
	}

	// Group with no brief gets a derived default; db group id defaults to namespace.
	var dbFile genGroupFile
	_ = yaml.Unmarshal([]byte(files["groups/db.yaml"]), &dbFile)
	if dbFile.Groups[0].ID != "db" {
		t.Errorf("group id default = %q, want db", dbFile.Groups[0].ID)
	}
	if dbFile.Groups[0].Brief == "" {
		t.Error("group brief default must be non-empty (GroupSpec requires brief)")
	}
}

// TestGenerateRoundTrip checks every emitted group satisfies the SemConvSpecV1
// required fields (id + brief on each group) after a YAML round trip.
func TestGenerateRoundTrip(t *testing.T) {
	e := NewWeaverExporter()
	files, err := e.Generate(sampleState(), nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	for path, content := range files {
		if !strings.HasPrefix(path, "groups/") {
			continue
		}
		var f genGroupFile
		if err := yaml.Unmarshal([]byte(content), &f); err != nil {
			t.Fatalf("%s does not parse: %v", path, err)
		}
		if len(f.Groups) == 0 {
			t.Errorf("%s has no groups", path)
		}
		for _, g := range f.Groups {
			if g.ID == "" || g.Brief == "" {
				t.Errorf("%s: group missing required id/brief: %+v", path, g)
			}
			for _, a := range g.Attributes {
				if a.ID == "" {
					t.Errorf("%s: attribute missing id in group %s", path, g.ID)
				}
			}
		}
	}
}

func keysOf(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestWeaverExportEmpty(t *testing.T) {
	exporter := NewWeaverExporter()
	yaml, err := exporter.Export([]*dictionary.AttributeEntry{})
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if len(yaml) == 0 {
		t.Error("expected non-empty output for empty entries")
	}
}

func TestWeaverExportNil(t *testing.T) {
	exporter := NewWeaverExporter()
	yaml, err := exporter.Export(nil)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if len(yaml) == 0 {
		t.Error("expected non-empty output for nil entries")
	}
}
