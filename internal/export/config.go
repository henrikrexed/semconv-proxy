package export

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// validFindingLevels is the closed FindingLevel enum a finding filter's min_level
// may take (verified via `weaver registry json-schema -j weaver-config` → FindingLevel).
var validFindingLevels = map[string]bool{
	"information": true,
	"improvement": true,
	"violation":   true,
}

// ValidFindingLevel reports whether s is a valid FindingLevel for a finding
// filter's min_level. Empty is treated as invalid; callers should special-case
// the unset value.
func ValidFindingLevel(s string) bool {
	return validFindingLevels[s]
}

// FindingFilter mirrors Weaver's FindingFilter (`[[live_check.finding_filters]]`).
// It drops live-check findings by id exclusion, sample name, minimum level, and/or
// an optional signal-type scope (§2d). New in Weaver v0.23 (#1247/#1256).
type FindingFilter struct {
	Exclude        []string `json:"exclude,omitempty"`
	ExcludeSamples []string `json:"exclude_samples,omitempty"`
	MinLevel       string   `json:"min_level,omitempty"`
	SignalType     string   `json:"signal_type,omitempty"`
}

// ConfigSpec is the builder's `.weaver.toml` input. Presence of a non-nil
// ConfigSpec on a BuilderState turns on config emission; the fields map onto the
// WeaverConfig sections registry / policy / live_check / diagnostics (§2d/§10).
type ConfigSpec struct {
	RegistryPath       string          `json:"registry_path,omitempty"`
	PolicyPaths        []string        `json:"policy_paths,omitempty"`
	PolicySkip         bool            `json:"policy_skip,omitempty"`
	AdvicePolicies     string          `json:"advice_policies,omitempty"`
	AdvicePreprocessor string          `json:"advice_preprocessor,omitempty"`
	FindingFilters     []FindingFilter `json:"finding_filters,omitempty"`
	DiagnosticsFormat  string          `json:"diagnostics_format,omitempty"`
}

// tomlConfig and friends are the on-disk TOML shape. Sections are pointers so an
// unset section is omitted entirely rather than emitted empty (which would still
// parse but adds noise). Field names match the WeaverConfig schema exactly.
type tomlConfig struct {
	Registry    *tomlRegistry    `toml:"registry,omitempty"`
	Policy      *tomlPolicy      `toml:"policy,omitempty"`
	LiveCheck   *tomlLiveCheck   `toml:"live_check,omitempty"`
	Diagnostics *tomlDiagnostics `toml:"diagnostics,omitempty"`
}

type tomlRegistry struct {
	Path string `toml:"path"`
}

type tomlPolicy struct {
	Paths []string `toml:"paths,omitempty"`
	Skip  bool     `toml:"skip"`
}

type tomlLiveCheck struct {
	AdvicePolicies     string              `toml:"advice_policies,omitempty"`
	AdvicePreprocessor string              `toml:"advice_preprocessor,omitempty"`
	FindingFilters     []tomlFindingFilter `toml:"finding_filters,omitempty"`
}

type tomlFindingFilter struct {
	Exclude        []string `toml:"exclude,omitempty"`
	ExcludeSamples []string `toml:"exclude_samples,omitempty"`
	MinLevel       string   `toml:"min_level,omitempty"`
	SignalType     string   `toml:"signal_type,omitempty"`
}

type tomlDiagnostics struct {
	Format string `toml:"format,omitempty"`
}

// EmitWeaverConfig renders a ConfigSpec to `.weaver.toml` content. The output
// conforms to the WeaverConfig schema and is accepted by `weaver registry
// live-check --config` (v0.23). A section is emitted only when it carries at
// least one set field. min_level values are validated against the closed
// FindingLevel enum so a bad value fails here rather than at weaver runtime.
func EmitWeaverConfig(spec ConfigSpec) (string, error) {
	cfg := tomlConfig{}

	if spec.RegistryPath != "" {
		cfg.Registry = &tomlRegistry{Path: spec.RegistryPath}
	}

	if len(spec.PolicyPaths) > 0 || spec.PolicySkip {
		cfg.Policy = &tomlPolicy{Paths: spec.PolicyPaths, Skip: spec.PolicySkip}
	}

	filters := make([]tomlFindingFilter, 0, len(spec.FindingFilters))
	for i, f := range spec.FindingFilters {
		if f.MinLevel != "" && !validFindingLevels[f.MinLevel] {
			return "", fmt.Errorf("export: finding_filter[%d].min_level %q is not a valid level (information|improvement|violation)", i, f.MinLevel)
		}
		// Skip a filter that carries no constraint at all — emitting it would be
		// a no-op table that only adds noise.
		if len(f.Exclude) == 0 && len(f.ExcludeSamples) == 0 && f.MinLevel == "" && f.SignalType == "" {
			continue
		}
		filters = append(filters, tomlFindingFilter{
			Exclude:        f.Exclude,
			ExcludeSamples: f.ExcludeSamples,
			MinLevel:       f.MinLevel,
			SignalType:     f.SignalType,
		})
	}
	if spec.AdvicePolicies != "" || spec.AdvicePreprocessor != "" || len(filters) > 0 {
		cfg.LiveCheck = &tomlLiveCheck{
			AdvicePolicies:     spec.AdvicePolicies,
			AdvicePreprocessor: spec.AdvicePreprocessor,
			FindingFilters:     filters,
		}
	}

	if spec.DiagnosticsFormat != "" {
		cfg.Diagnostics = &tomlDiagnostics{Format: spec.DiagnosticsFormat}
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("export: marshal weaver config: %w", err)
	}
	return string(data), nil
}
