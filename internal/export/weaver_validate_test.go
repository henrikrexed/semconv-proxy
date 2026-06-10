package export

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findWeaverBinary walks up from the package dir looking for the bundled
// .spike-weaver/bin/weaver. Returns "" if not found.
func findWeaverBinary() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".spike-weaver", "bin", "weaver")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// TestGenerateWeaverRegistryCheck materialises the generated file set on disk and
// runs `weaver registry check` against it, asserting the manifest is recognised.
// Skips when the bundled v0.23 binary is unavailable (e.g. CI without the spike).
func TestGenerateWeaverRegistryCheck(t *testing.T) {
	bin := findWeaverBinary()
	if bin == "" {
		t.Skip("bundled weaver binary not found; skipping registry check")
	}

	// Pass nil defaultDep so the registry has no remote dependency: the check
	// stays hermetic (no network clone into the weaver vdir cache).
	e := NewWeaverExporter()
	files, err := e.Generate(sampleState(), nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	regDir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(regDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	cmd := exec.Command(bin, "registry", "check", "-r", regDir)
	out, runErr := cmd.CombinedOutput()
	combined := string(out)
	t.Logf("weaver registry check exit=%v output:\n%s", runErr, combined)

	// Primary acceptance: our manifest.yaml is discovered by weaver.
	if !strings.Contains(combined, "Found registry manifest: "+filepath.Join(regDir, "manifest.yaml")) {
		t.Errorf("weaver did not report finding our manifest; output:\n%s", combined)
	}
	// The emitted groups must be valid: no invalid-attribute resolution errors.
	if strings.Contains(combined, "Invalid attribute definition") {
		t.Errorf("weaver rejected an emitted attribute definition; output:\n%s", combined)
	}
	// And no duplicate group id warnings from namespace merging.
	if strings.Contains(combined, "declared multiple times") {
		t.Errorf("weaver reported duplicate group ids; output:\n%s", combined)
	}
	if runErr != nil {
		t.Errorf("weaver registry check failed: %v\n%s", runErr, combined)
	}
}

// TestGeneratePolicyRegoLoadsUnderWeaver is the S3 acceptance check: every
// catalog template (plus a raw policy) emits a `.rego` that loads under
// `weaver registry check --policy` with no parse/deserialize error. Exit status
// is ignored — a non-zero exit from a *found* violation is expected and fine;
// what must not appear are rego parse / Violation-deserialize errors.
func TestGeneratePolicyRegoLoadsUnderWeaver(t *testing.T) {
	bin := findWeaverBinary()
	if bin == "" {
		t.Skip("bundled weaver binary not found; skipping policy check")
	}

	state := sampleState()
	for _, tmpl := range PolicyCatalog() {
		state.Policies = append(state.Policies, PolicyInput{TemplateID: tmpl.ID, Params: sampleParams(tmpl.ID)})
	}
	state.Policies = append(state.Policies, PolicyInput{
		Name: "raw_passthrough",
		Raw:  "package after_resolution\nimport rego.v1\n\ndeny contains v if {\n\tfalse\n}\n",
	})

	files, err := NewWeaverExporter().Generate(state, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	regDir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(regDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	cmd := exec.Command(bin, "registry", "check", "-r", regDir, "--policy", filepath.Join(regDir, "policies"))
	out, runErr := cmd.CombinedOutput()
	combined := string(out)
	t.Logf("weaver registry check --policy exit=%v output:\n%s", runErr, combined)

	// These substrings are the §10.2 failure modes the emitter must never produce.
	for _, bad := range []string{
		"rego_parse_error",
		"'if' keyword is required",
		"expected A policy violation",
		"missing field",
		"var v is unsafe",
	} {
		if strings.Contains(combined, bad) {
			t.Errorf("weaver reported %q — emitted rego is non-conformant; output:\n%s", bad, combined)
		}
	}
	// The policies must actually be loaded/evaluated, not silently ignored.
	if !strings.Contains(combined, "policies checked") {
		t.Errorf("weaver did not report checking any policies; output:\n%s", combined)
	}
}
