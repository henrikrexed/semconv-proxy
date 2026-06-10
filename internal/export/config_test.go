package export

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// parseTOML decodes emitted config back into a generic map for round-trip checks.
func parseTOML(t *testing.T, content string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		t.Fatalf("emitted .weaver.toml does not parse: %v\n%s", err, content)
	}
	return m
}

func TestValidFindingLevel(t *testing.T) {
	cases := map[string]bool{
		"information": true,
		"improvement": true,
		"violation":   true,
		"warning":     false, // not a FindingLevel
		"Violation":   false, // case-sensitive
		"":            false, // unset is invalid; callers special-case it
	}
	for in, want := range cases {
		if got := ValidFindingLevel(in); got != want {
			t.Errorf("ValidFindingLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestEmitWeaverConfigSections(t *testing.T) {
	spec := ConfigSpec{
		RegistryPath:       ".",
		PolicyPaths:        []string{"policies"},
		PolicySkip:         false,
		AdvicePolicies:     "advice",
		AdvicePreprocessor: ".groups",
		DiagnosticsFormat:  "ansi",
		FindingFilters: []FindingFilter{
			{Exclude: []string{"missing_attribute"}, MinLevel: "improvement", SignalType: "trace"},
			{ExcludeSamples: []string{"trace.span_id"}},
		},
	}

	out, err := EmitWeaverConfig(spec)
	if err != nil {
		t.Fatalf("EmitWeaverConfig: %v", err)
	}
	m := parseTOML(t, out)

	reg, _ := m["registry"].(map[string]any)
	if reg == nil || reg["path"] != "." {
		t.Errorf("registry.path = %v, want \".\"", m["registry"])
	}

	pol, _ := m["policy"].(map[string]any)
	if pol == nil {
		t.Fatalf("policy section missing")
	}
	paths, _ := pol["paths"].([]any)
	if len(paths) != 1 || paths[0] != "policies" {
		t.Errorf("policy.paths = %v, want [policies]", pol["paths"])
	}

	lc, _ := m["live_check"].(map[string]any)
	if lc == nil || lc["advice_policies"] != "advice" || lc["advice_preprocessor"] != ".groups" {
		t.Errorf("live_check section wrong: %v", m["live_check"])
	}
	filters, _ := lc["finding_filters"].([]map[string]any)
	if filters == nil {
		// go-toml decodes arrays of tables as []any of maps.
		raw, _ := lc["finding_filters"].([]any)
		if len(raw) != 2 {
			t.Fatalf("finding_filters round-trip = %v, want 2 entries", lc["finding_filters"])
		}
		f0, _ := raw[0].(map[string]any)
		if f0["min_level"] != "improvement" || f0["signal_type"] != "trace" {
			t.Errorf("finding_filters[0] round-trip lost fields: %v", f0)
		}
	}

	diag, _ := m["diagnostics"].(map[string]any)
	if diag == nil || diag["format"] != "ansi" {
		t.Errorf("diagnostics.format = %v, want ansi", m["diagnostics"])
	}
}

// Empty/no-op sections must be omitted so the file stays minimal.
func TestEmitWeaverConfigOmitsEmptySections(t *testing.T) {
	out, err := EmitWeaverConfig(ConfigSpec{RegistryPath: "."})
	if err != nil {
		t.Fatalf("EmitWeaverConfig: %v", err)
	}
	for _, section := range []string{"[policy]", "[live_check]", "[diagnostics]", "finding_filters"} {
		if strings.Contains(out, section) {
			t.Errorf("expected %q to be omitted; got:\n%s", section, out)
		}
	}
}

// A no-constraint finding filter is dropped rather than emitted as an empty table.
func TestEmitWeaverConfigDropsEmptyFilter(t *testing.T) {
	out, err := EmitWeaverConfig(ConfigSpec{FindingFilters: []FindingFilter{{}}})
	if err != nil {
		t.Fatalf("EmitWeaverConfig: %v", err)
	}
	if strings.Contains(out, "finding_filters") {
		t.Errorf("empty filter should be dropped; got:\n%s", out)
	}
}

func TestEmitWeaverConfigRejectsBadLevel(t *testing.T) {
	_, err := EmitWeaverConfig(ConfigSpec{FindingFilters: []FindingFilter{{MinLevel: "bogus"}}})
	if err == nil {
		t.Fatal("expected error for invalid min_level, got nil")
	}
}

// Generate emits `.weaver.toml` only when Config is set, and defaults policy.paths
// to the generated policies dir when checks are present.
func TestGenerateEmitsConfigWithDefaults(t *testing.T) {
	state := sampleState()
	state.Policies = []PolicyInput{{TemplateID: "stability_required"}}
	state.Config = &ConfigSpec{} // enabled, all defaults

	files, err := NewWeaverExporter().Generate(state, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content, ok := files[".weaver.toml"]
	if !ok {
		t.Fatal(".weaver.toml not emitted")
	}
	m := parseTOML(t, content)
	reg, _ := m["registry"].(map[string]any)
	if reg == nil || reg["path"] != "." {
		t.Errorf("default registry.path = %v, want \".\"", m["registry"])
	}
	pol, _ := m["policy"].(map[string]any)
	paths, _ := pol["paths"].([]any)
	if len(paths) != 1 || paths[0] != "policies" {
		t.Errorf("default policy.paths = %v, want [policies]", pol)
	}
}

func TestGenerateNoConfigByDefault(t *testing.T) {
	files, err := NewWeaverExporter().Generate(sampleState(), nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, ok := files[".weaver.toml"]; ok {
		t.Error(".weaver.toml should not be emitted when Config is nil")
	}
}
