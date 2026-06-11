package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
)

// BundleZip packs a generated file set (path -> content, as produced by
// WeaverExporter.Generate) into a deterministic zip archive whose entries are the
// registry-root-relative paths (`manifest.yaml`, `groups/<ns>.yaml`,
// `policies/<name>.rego`, `.weaver.toml`). Unpacking the archive yields a
// directory that `weaver registry check -r <dir>` can consume directly.
//
// Entries are written in sorted-path order so the archive bytes are stable for a
// given file set (test-friendly, cache-friendly).
func BundleZip(files map[string]string) ([]byte, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range paths {
		fw, err := zw.Create(p)
		if err != nil {
			return nil, fmt.Errorf("export: zip create %q: %w", p, err)
		}
		if _, err := fw.Write([]byte(files[p])); err != nil {
			return nil, fmt.Errorf("export: zip write %q: %w", p, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("export: zip close: %w", err)
	}
	return buf.Bytes(), nil
}
