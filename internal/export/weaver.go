package export

import (
	"fmt"
	"strings"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"gopkg.in/yaml.v3"
)

type WeaverExporter struct{}

func NewWeaverExporter() *WeaverExporter {
	return &WeaverExporter{}
}

type WeaverRegistry struct {
	Groups []WeaverGroup `yaml:"groups"`
}

type WeaverGroup struct {
	ID          string            `yaml:"id"`
	Type        string            `yaml:"type,omitempty"`
	MetricName  string            `yaml:"metric_name,omitempty"`
	Brief       string            `yaml:"brief"`
	Instrument  string            `yaml:"instrument,omitempty"`
	Unit        string            `yaml:"unit,omitempty"`
	Attributes  []WeaverAttribute `yaml:"attributes,omitempty"`
	Stability   string            `yaml:"stability"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type WeaverAttribute struct {
	ID               string `yaml:"id"`
	Type             string `yaml:"type"`
	RequirementLevel string `yaml:"requirement_level"`
}

// Defaults applied when builder state omits a field. These mirror the modern
// Weaver v0.23 vocabulary (§10): manifest/group/attribute stability defaults to
// `development` (not the legacy `experimental`).
const (
	defaultStability        = "development"
	defaultRequirementLevel = "recommended"
	defaultAttributeType    = "string"
)

// BuilderState is the request payload for parameterised generation: the manifest
// fields plus enriched, namespace-grouped convention definitions.
type BuilderState struct {
	Manifest ManifestSpec `json:"manifest"`
	Groups   []GroupInput `json:"groups"`
}

// ManifestSpec maps to Weaver's DefinitionRegistryManifest (§10.1). schema_url is
// the only required field; there is no `name` field.
type ManifestSpec struct {
	SchemaURL    string           `json:"schema_url"`
	Description  string           `json:"description,omitempty"`
	Stability    string           `json:"stability,omitempty"`
	Dependencies []DependencySpec `json:"dependencies,omitempty"`
}

// DependencySpec is one manifest dependency. schema_url is required per
// dependency; registry_path is optional (§10.1).
type DependencySpec struct {
	SchemaURL    string `json:"schema_url"`
	RegistryPath string `json:"registry_path,omitempty"`
}

// GroupInput is one authored group. Attributes with the same Namespace are
// emitted into a single groups/<namespace>.yaml file.
type GroupInput struct {
	ID         string           `json:"id,omitempty"`
	Namespace  string           `json:"namespace,omitempty"`
	Type       string           `json:"type,omitempty"`
	Brief      string           `json:"brief,omitempty"`
	Stability  string           `json:"stability,omitempty"`
	Attributes []AttributeInput `json:"attributes,omitempty"`
}

// AttributeInput is one authored attribute with overridable enrichment fields.
type AttributeInput struct {
	ID               string   `json:"id"`
	Type             string   `json:"type,omitempty"`
	Brief            string   `json:"brief,omitempty"`
	Stability        string   `json:"stability,omitempty"`
	RequirementLevel string   `json:"requirement_level,omitempty"`
	Examples         []string `json:"examples,omitempty"`
	Note             string   `json:"note,omitempty"`
}

// genManifest is the YAML shape of DefinitionRegistryManifest. Stability has no
// omitempty so the default `development` is always emitted; there is no `name`.
type genManifest struct {
	SchemaURL    string   `yaml:"schema_url"`
	Description  string   `yaml:"description,omitempty"`
	Stability    string   `yaml:"stability"`
	Dependencies []genDep `yaml:"dependencies,omitempty"`
}

type genDep struct {
	SchemaURL    string `yaml:"schema_url"`
	RegistryPath string `yaml:"registry_path,omitempty"`
}

type genGroupFile struct {
	Groups []genGroup `yaml:"groups"`
}

type genGroup struct {
	ID         string    `yaml:"id"`
	Type       string    `yaml:"type,omitempty"`
	Brief      string    `yaml:"brief"`
	Stability  string    `yaml:"stability,omitempty"`
	Attributes []genAttr `yaml:"attributes,omitempty"`
}

type genAttr struct {
	ID               string   `yaml:"id"`
	Type             string   `yaml:"type,omitempty"`
	Brief            string   `yaml:"brief,omitempty"`
	Stability        string   `yaml:"stability,omitempty"`
	RequirementLevel string   `yaml:"requirement_level,omitempty"`
	Examples         []string `yaml:"examples,omitempty"`
	Note             string   `yaml:"note,omitempty"`
}

// DefaultOTelDependency derives the pinned OTel dependency from the embedded
// registry URL (e.g. ".../semantic-conventions.git@v1.41.1[model]"): the URL
// becomes registry_path and its version becomes the OTel schema_url.
func DefaultOTelDependency(registryURL string) DependencySpec {
	dep := DependencySpec{RegistryPath: registryURL}
	if v := versionFromRegistryURL(registryURL); v != "" {
		dep.SchemaURL = "https://opentelemetry.io/schemas/" + v
	}
	return dep
}

func versionFromRegistryURL(u string) string {
	at := strings.LastIndex(u, "@")
	if at < 0 {
		return ""
	}
	rest := u[at+1:]
	if i := strings.IndexByte(rest, '['); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimPrefix(rest, "v")
}

// Generate turns builder state into a Weaver registry file set (path -> content):
// `manifest.yaml` plus one `groups/<namespace>.yaml` per namespace. When the
// state declares no dependencies and defaultDep is non-nil, defaultDep is
// injected as the single dependency.
func (e *WeaverExporter) Generate(state BuilderState, defaultDep *DependencySpec) (map[string]string, error) {
	files := make(map[string]string)

	man := genManifest{
		SchemaURL:   state.Manifest.SchemaURL,
		Description: state.Manifest.Description,
		Stability:   firstNonEmpty(state.Manifest.Stability, defaultStability),
	}
	deps := state.Manifest.Dependencies
	if len(deps) == 0 && defaultDep != nil {
		deps = []DependencySpec{*defaultDep}
	}
	for _, d := range deps {
		man.Dependencies = append(man.Dependencies, genDep{SchemaURL: d.SchemaURL, RegistryPath: d.RegistryPath})
	}
	manYAML, err := yaml.Marshal(man)
	if err != nil {
		return nil, fmt.Errorf("export: marshal manifest: %w", err)
	}
	files["manifest.yaml"] = string(manYAML)

	// Group by namespace: each namespace becomes exactly one group in its own
	// file. Weaver requires unique group ids, so collapsing same-namespace inputs
	// into a single group (concatenating attributes) keeps the output valid.
	order := []string{}
	byNS := make(map[string]*genGroup)
	for _, g := range state.Groups {
		ns := g.Namespace
		if ns == "" {
			ns = namespaceFromID(g.ID)
		}
		if ns == "" {
			ns = "custom"
		}

		grp, ok := byNS[ns]
		if !ok {
			grp = &genGroup{
				ID:        firstNonEmpty(g.ID, ns),
				Type:      firstNonEmpty(g.Type, "attribute_group"),
				Brief:     firstNonEmpty(g.Brief, fmt.Sprintf("Attributes for the %s namespace.", ns)),
				Stability: firstNonEmpty(g.Stability, defaultStability),
			}
			byNS[ns] = grp
			order = append(order, ns)
		}

		for _, a := range g.Attributes {
			grp.Attributes = append(grp.Attributes, genAttr{
				ID:               a.ID,
				Type:             firstNonEmpty(a.Type, defaultAttributeType),
				Brief:            firstNonEmpty(a.Brief, fmt.Sprintf("The %s attribute.", a.ID)),
				Stability:        firstNonEmpty(a.Stability, defaultStability),
				RequirementLevel: firstNonEmpty(a.RequirementLevel, defaultRequirementLevel),
				Examples:         a.Examples,
				Note:             a.Note,
			})
		}
	}

	for _, ns := range order {
		data, err := yaml.Marshal(genGroupFile{Groups: []genGroup{*byNS[ns]}})
		if err != nil {
			return nil, fmt.Errorf("export: marshal group %q: %w", ns, err)
		}
		files["groups/"+ns+".yaml"] = string(data)
	}

	return files, nil
}

func namespaceFromID(id string) string {
	if i := strings.IndexByte(id, '.'); i >= 0 {
		return id[:i]
	}
	return id
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func (e *WeaverExporter) Export(entries []*dictionary.AttributeEntry) ([]byte, error) {
	registry := WeaverRegistry{Groups: []WeaverGroup{}}
	grouped := make(map[string][]*dictionary.AttributeEntry)

	for _, entry := range entries {
		for _, st := range entry.SignalTypes {
			key := string(st) + ":" + entry.Name
			grouped[key] = append(grouped[key], entry)
		}
	}

	for key, entries := range grouped {
		parts := strings.SplitN(key, ":", 2)
		signalType := parts[0]
		name := parts[1]
		if len(entries) == 0 {
			continue
		}
		entry := entries[0]

		group := WeaverGroup{
			ID:        fmt.Sprintf("%s.%s", signalType, strings.ReplaceAll(name, ".", "_")),
			Brief:     "Auto-discovered attribute",
			Stability: "experimental",
			Annotations: map[string]string{
				"semconv.proxy.status":     string(entry.Status),
				"semconv.proxy.first_seen": entry.FirstSeen.Format("2006-01-02T15:04:05Z"),
				"semconv.proxy.last_seen":  entry.LastSeen.Format("2006-01-02T15:04:05Z"),
			},
		}

		switch signalType {
		case "metric":
			group.Type = "metric_group"
			group.MetricName = name
		case "trace":
			group.Type = "span"
		case "log":
			group.Type = "log_record"
		}

		group.Attributes = append(group.Attributes, WeaverAttribute{
			ID:               name,
			Type:             entry.Type,
			RequirementLevel: "recommended",
		})

		registry.Groups = append(registry.Groups, group)
	}

	data, err := yaml.Marshal(registry)
	if err != nil {
		return nil, fmt.Errorf("export: marshal yaml: %w", err)
	}
	return data, nil
}
