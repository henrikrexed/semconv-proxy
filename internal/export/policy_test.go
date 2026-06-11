package export

import (
	"strings"
	"testing"
)

// sampleParams returns valid params for every parameterised template, so the
// catalog can be emitted end-to-end in tests.
func sampleParams(id string) map[string]any {
	switch id {
	case "naming_prefix_required":
		return map[string]any{"prefix": "acme."}
	case "namespace_allow_list":
		return map[string]any{"allowed": "acme.\ninternal."}
	case "type_consistency":
		return map[string]any{"attribute": "http.response.status_code", "expected_type": "int"}
	default:
		return nil
	}
}

// TestPolicyCatalogHasBuilders guards against a catalog entry shipping without a
// matching emitter (which would 500 at generate time).
func TestPolicyCatalogHasBuilders(t *testing.T) {
	for _, tmpl := range PolicyCatalog() {
		if policyBuilders[tmpl.ID] == nil {
			t.Errorf("catalog template %q has no builder", tmpl.ID)
		}
	}
}

// TestEmitPolicyRegoConformsToG0 asserts every catalog template emits modern rego
// (§10.2): the declared stage as package, import rego.v1, deny contains v if, and
// a closed semconv_attribute Violation carrying the full {type,id,category,group,
// attr} field set.
func TestEmitPolicyRegoConformsToG0(t *testing.T) {
	for _, tmpl := range PolicyCatalog() {
		name, rego, err := EmitPolicyRego(PolicyInput{TemplateID: tmpl.ID, Params: sampleParams(tmpl.ID)})
		if err != nil {
			t.Fatalf("%s: emit error: %v", tmpl.ID, err)
		}
		if name == "" {
			t.Errorf("%s: empty file name", tmpl.ID)
		}

		// Stage = package (acceptance: each template's stage is asserted).
		if want := "package " + tmpl.Stage + "\n"; !strings.Contains(rego, want) {
			t.Errorf("%s: missing %q\n%s", tmpl.ID, want, rego)
		}
		// Modern rego is mandatory; partial-set deny is rejected by weaver.
		if !strings.Contains(rego, "import rego.v1") {
			t.Errorf("%s: missing import rego.v1\n%s", tmpl.ID, rego)
		}
		if !strings.Contains(rego, "deny contains v if {") {
			t.Errorf("%s: missing modern deny rule\n%s", tmpl.ID, rego)
		}
		if strings.Contains(rego, "deny[") {
			t.Errorf("%s: emitted forbidden partial-set deny\n%s", tmpl.ID, rego)
		}
		// Closed Violation variant + full field set.
		if !strings.Contains(rego, `"type": "semconv_attribute"`) {
			t.Errorf("%s: missing semconv_attribute type tag\n%s", tmpl.ID, rego)
		}
		for _, field := range []string{`"id":`, `"category":`, `"group":`, `"attr":`} {
			if !strings.Contains(rego, field) {
				t.Errorf("%s: missing required Violation field %s\n%s", tmpl.ID, field, rego)
			}
		}
		// Category matches the catalog declaration.
		if !strings.Contains(rego, `"category": "`+tmpl.Category+`"`) {
			t.Errorf("%s: category %q not emitted\n%s", tmpl.ID, tmpl.Category, rego)
		}
	}
}

func TestEmitPolicyRegoRawPassthrough(t *testing.T) {
	raw := "package after_resolution\nimport rego.v1\n\ndeny contains v if { false }\n"
	name, content, err := EmitPolicyRego(PolicyInput{Name: "My Custom Check!", Raw: raw})
	if err != nil {
		t.Fatalf("emit raw: %v", err)
	}
	if content != raw {
		t.Errorf("raw rego was modified:\ngot:  %q\nwant: %q", content, raw)
	}
	if name != "my_custom_check" {
		t.Errorf("sanitized name = %q, want my_custom_check", name)
	}
}

func TestEmitPolicyRegoUnknownTemplate(t *testing.T) {
	if _, _, err := EmitPolicyRego(PolicyInput{TemplateID: "does_not_exist"}); err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestEmitPolicyRegoMissingRequiredParam(t *testing.T) {
	// naming_prefix_required without its prefix param must error rather than emit
	// a rego that references an empty string.
	if _, _, err := EmitPolicyRego(PolicyInput{TemplateID: "naming_prefix_required"}); err == nil {
		t.Fatal("expected error for missing required param")
	}
}

// TestEmitNamespaceAllowListArray confirms list params render a rego array and a
// uniquely-named helper rule.
func TestEmitNamespaceAllowListArray(t *testing.T) {
	_, rego, err := EmitPolicyRego(PolicyInput{
		TemplateID: "namespace_allow_list",
		Name:       "ns_check",
		Params:     map[string]any{"allowed": []any{"acme.", "internal."}},
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !strings.Contains(rego, `["acme.", "internal."]`) {
		t.Errorf("allow-list array not emitted\n%s", rego)
	}
	if !strings.Contains(rego, "namespace_allowed_ns_check(name) if {") {
		t.Errorf("uniquely-named helper rule not emitted\n%s", rego)
	}
}

// TestEmitNamespaceAllowListMissingParam guards the required-list error path: an
// allow-list template with no prefixes must error rather than emit a helper that
// matches nothing.
func TestEmitNamespaceAllowListMissingParam(t *testing.T) {
	if _, _, err := EmitPolicyRego(PolicyInput{TemplateID: "namespace_allow_list"}); err == nil {
		t.Fatal("expected error for empty allow-list")
	}
	// Key present in the map but not the expected one -> still treated as empty.
	if _, _, err := EmitPolicyRego(PolicyInput{
		TemplateID: "namespace_allow_list",
		Params:     map[string]any{"unrelated": "x"},
	}); err == nil {
		t.Fatal("expected error when allowed param key is absent")
	}
}

// TestEmitNamespaceAllowListStringSlice covers the []string param form (as opposed
// to the []any and newline-string forms exercised elsewhere).
func TestEmitNamespaceAllowListStringSlice(t *testing.T) {
	_, rego, err := EmitPolicyRego(PolicyInput{
		TemplateID: "namespace_allow_list",
		Params:     map[string]any{"allowed": []string{"acme.", "internal."}},
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !strings.Contains(rego, `["acme.", "internal."]`) {
		t.Errorf("[]string allow-list not emitted\n%s", rego)
	}
}

// TestEmitTypeConsistencyMissingParam covers the two-required-param error path and,
// via paramStr, the non-string param value branch (a numeric attribute is ignored,
// leaving the param empty).
func TestEmitTypeConsistencyMissingParam(t *testing.T) {
	if _, _, err := EmitPolicyRego(PolicyInput{TemplateID: "type_consistency"}); err == nil {
		t.Fatal("expected error for missing type_consistency params")
	}
	// expected_type supplied but attribute is a non-string value -> paramStr drops
	// it, so the required-param guard still trips.
	if _, _, err := EmitPolicyRego(PolicyInput{
		TemplateID: "type_consistency",
		Params:     map[string]any{"attribute": 123, "expected_type": "int"},
	}); err == nil {
		t.Fatal("expected error when attribute param is not a string")
	}
}

// TestSanitizePolicyNameFallback drives the all-symbol name down to the "check"
// fallback (sanitize strips every rune, leaving an empty token).
func TestSanitizePolicyNameFallback(t *testing.T) {
	name, _, err := EmitPolicyRego(PolicyInput{
		Name: "!!!",
		Raw:  "package after_resolution\nimport rego.v1\ndeny contains v if { false }\n",
	})
	if err != nil {
		t.Fatalf("emit raw: %v", err)
	}
	if name != "check" {
		t.Errorf("sanitized name = %q, want fallback \"check\"", name)
	}
}

// TestGenerateEmitsPolicyFiles wires policies through Generate and asserts each
// becomes a uniquely-named policies/<name>.rego file alongside the registry.
func TestGenerateEmitsPolicyFiles(t *testing.T) {
	state := sampleState()
	state.Policies = []PolicyInput{
		{TemplateID: "stability_required"},
		{TemplateID: "naming_prefix_required", Params: map[string]any{"prefix": "acme."}},
		{TemplateID: "stability_required"}, // duplicate base name -> _2 suffix
		{Name: "hand_written", Raw: "package after_resolution\nimport rego.v1\ndeny contains v if { false }\n"},
	}

	files, err := NewWeaverExporter().Generate(state, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, want := range []string{
		"policies/stability_required.rego",
		"policies/stability_required_2.rego",
		"policies/naming_prefix_required.rego",
		"policies/hand_written.rego",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s; got %v", want, keysOf(files))
		}
	}
}
