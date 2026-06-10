package export

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func TestBundleZipRoundTrip(t *testing.T) {
	files := map[string]string{
		"manifest.yaml":    "schema_url: https://acme.com/schemas/0.1.0\n",
		"groups/http.yaml": "groups: []\n",
		"policies/x.rego":  "package after_resolution\n",
		".weaver.toml":     "[registry]\npath = \".\"\n",
	}

	archive, err := BundleZip(files)
	if err != nil {
		t.Fatalf("BundleZip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	got := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(data)
	}

	if len(got) != len(files) {
		t.Fatalf("entry count = %d, want %d", len(got), len(files))
	}
	for path, want := range files {
		if got[path] != want {
			t.Errorf("entry %q = %q, want %q", path, got[path], want)
		}
	}
}

// TestBundleZipDeterministic verifies the archive bytes are stable across calls
// for the same input (entries written in sorted-path order).
func TestBundleZipDeterministic(t *testing.T) {
	files := map[string]string{
		"b.yaml":        "b\n",
		"a.yaml":        "a\n",
		"sub/c.yaml":    "c\n",
		"manifest.yaml": "m\n",
	}
	a1, err := BundleZip(files)
	if err != nil {
		t.Fatalf("BundleZip #1: %v", err)
	}
	a2, err := BundleZip(files)
	if err != nil {
		t.Fatalf("BundleZip #2: %v", err)
	}
	if !bytes.Equal(a1, a2) {
		t.Errorf("archives differ across calls for identical input")
	}
}

func TestBundleZipEmpty(t *testing.T) {
	archive, err := BundleZip(map[string]string{})
	if err != nil {
		t.Fatalf("BundleZip empty: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open empty zip: %v", err)
	}
	if len(zr.File) != 0 {
		t.Errorf("empty bundle has %d entries, want 0", len(zr.File))
	}
}
