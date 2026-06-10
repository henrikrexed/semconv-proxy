package export

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Policy stages are the rego package names Weaver evaluates (§10.2). The catalog
// standardises on after_resolution — the stage where attribute checks see the
// fully resolved registry (imports/extends applied), confirmed against the
// bundled v0.23 binary. The Stage field stays on the template so future
// templates can target comparison_after_resolution / before_resolution / advice.
const (
	stageAfterResolution = "after_resolution"

	// violationType is the only closed-enum Violation variant the catalog emits
	// (§10.2). Its required field set is {type, id, category, group, attr}.
	violationType = "semconv_attribute"
)

// PolicyParam describes one user-supplied input on a template form.
type PolicyParam struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"` // "string" | "string_list"
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
	Help     string `json:"help,omitempty"`
}

// PolicyTemplate is one entry in the parameterised check catalog. The metadata
// here drives the Checks UI form; the rego itself is produced by the matching
// builder in policyBuilders.
type PolicyTemplate struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Stage       string        `json:"stage"`    // rego package name
	Category    string        `json:"category"` // Violation.category value
	Params      []PolicyParam `json:"params,omitempty"`
}

// PolicyInput is one authored check in a BuilderState. Either TemplateID (+Params)
// selects a catalog template, or Raw carries hand-written rego that is passed
// through unmodified (the advanced escape hatch).
type PolicyInput struct {
	TemplateID string         `json:"template_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
	Raw        string         `json:"raw,omitempty"`
}

// policyBuilder produces the rego pieces for a template given resolved params:
// optional top-level helper rules, the condition lines inside the deny body
// (which may reference `group` and `attr`), and the rego expression for the
// finding's id field.
type policyBuilder func(p map[string]any, token string) (helpers []string, cond []string, idExpr string, err error)

// policyTemplates is the static, proxy-versioned catalog. Order is the display
// order in the Checks UI.
var policyTemplates = []PolicyTemplate{
	{
		ID:          "naming_prefix_required",
		Title:       "Naming prefix required",
		Description: "Every attribute name must start with a required prefix (e.g. your vendor namespace).",
		Stage:       stageAfterResolution,
		Category:    "naming",
		Params: []PolicyParam{
			{Name: "prefix", Label: "Required prefix", Type: "string", Required: true, Default: "acme.", Help: "Attribute names must start with this string, e.g. \"acme.\"."},
		},
	},
	{
		ID:          "namespace_allow_list",
		Title:       "Namespace allow-list",
		Description: "Attribute names must start with one of an allowed set of namespace prefixes.",
		Stage:       stageAfterResolution,
		Category:    "naming",
		Params: []PolicyParam{
			{Name: "allowed", Label: "Allowed prefixes", Type: "string_list", Required: true, Help: "One prefix per line (or comma-separated), e.g. \"acme.\", \"internal.\"."},
		},
	},
	{
		ID:          "stability_required",
		Title:       "Stability required",
		Description: "Every attribute must declare a stability level.",
		Stage:       stageAfterResolution,
		Category:    "stability",
	},
	{
		ID:          "requirement_level_required",
		Title:       "Requirement level required",
		Description: "Every attribute must declare a requirement_level.",
		Stage:       stageAfterResolution,
		Category:    "requirement_level",
	},
	{
		ID:          "no_deprecated_without_replacement",
		Title:       "No deprecation without replacement",
		Description: "A deprecated attribute must point at a replacement (renamed_to).",
		Stage:       stageAfterResolution,
		Category:    "deprecation",
	},
	{
		ID:          "type_consistency",
		Title:       "Attribute type consistency",
		Description: "A named attribute must use an expected type.",
		Stage:       stageAfterResolution,
		Category:    "type",
		Params: []PolicyParam{
			{Name: "attribute", Label: "Attribute name", Type: "string", Required: true, Help: "The attribute id to constrain, e.g. \"http.response.status_code\"."},
			{Name: "expected_type", Label: "Expected type", Type: "string", Required: true, Default: "int", Help: "The required Weaver type, e.g. \"int\", \"string\", \"boolean\"."},
		},
	},
}

var policyBuilders = map[string]policyBuilder{
	"naming_prefix_required": func(p map[string]any, _ string) ([]string, []string, string, error) {
		prefix := paramStr(p, "prefix")
		if prefix == "" {
			return nil, nil, "", fmt.Errorf("naming_prefix_required: param %q is required", "prefix")
		}
		cond := []string{fmt.Sprintf("not startswith(attr.name, %s)", regoStr(prefix))}
		id := fmt.Sprintf("sprintf(\"attribute '%%s' must use the required prefix %%s\", [attr.name, %s])", regoStr(prefix))
		return nil, cond, id, nil
	},
	"namespace_allow_list": func(p map[string]any, token string) ([]string, []string, string, error) {
		allowed := paramList(p, "allowed")
		if len(allowed) == 0 {
			return nil, nil, "", fmt.Errorf("namespace_allow_list: param %q requires at least one prefix", "allowed")
		}
		fn := "namespace_allowed_" + token
		helpers := []string{
			fmt.Sprintf("%s(name) if {", fn),
			fmt.Sprintf("\tallowed := %s", regoStrArray(allowed)),
			"\tstartswith(name, allowed[_])",
			"}",
		}
		cond := []string{fmt.Sprintf("not %s(attr.name)", fn)}
		id := "sprintf(\"attribute '%s' is outside the namespace allow-list\", [attr.name])"
		return helpers, cond, id, nil
	},
	"stability_required": func(_ map[string]any, _ string) ([]string, []string, string, error) {
		return nil, []string{"not attr.stability"}, "sprintf(\"attribute '%s' is missing a stability level\", [attr.name])", nil
	},
	"requirement_level_required": func(_ map[string]any, _ string) ([]string, []string, string, error) {
		return nil, []string{"not attr.requirement_level"}, "sprintf(\"attribute '%s' is missing a requirement_level\", [attr.name])", nil
	},
	"no_deprecated_without_replacement": func(_ map[string]any, _ string) ([]string, []string, string, error) {
		cond := []string{"attr.deprecated", "not attr.deprecated.renamed_to"}
		return nil, cond, "sprintf(\"deprecated attribute '%s' has no replacement (renamed_to)\", [attr.name])", nil
	},
	"type_consistency": func(p map[string]any, _ string) ([]string, []string, string, error) {
		name := paramStr(p, "attribute")
		typ := paramStr(p, "expected_type")
		if name == "" || typ == "" {
			return nil, nil, "", fmt.Errorf("type_consistency: params %q and %q are required", "attribute", "expected_type")
		}
		cond := []string{
			fmt.Sprintf("attr.name == %s", regoStr(name)),
			fmt.Sprintf("attr.type != %s", regoStr(typ)),
		}
		id := fmt.Sprintf("sprintf(\"attribute '%%s' must have type %%s\", [attr.name, %s])", regoStr(typ))
		return nil, cond, id, nil
	},
}

// PolicyCatalog returns the static parameterised check catalog.
func PolicyCatalog() []PolicyTemplate {
	out := make([]PolicyTemplate, len(policyTemplates))
	copy(out, policyTemplates)
	return out
}

func policyTemplateByID(id string) (PolicyTemplate, bool) {
	for _, t := range policyTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return PolicyTemplate{}, false
}

// EmitPolicyRego turns one PolicyInput into a (filename-base, rego-content) pair.
// Raw rego is returned unmodified (only the file name is derived). Template rego
// conforms to §10.2: modern rego (import rego.v1 + deny contains v if), a closed
// semconv_attribute Violation with the full {type,id,category,group,attr} field
// set. The returned name is the file base (without the .rego extension).
func EmitPolicyRego(in PolicyInput) (string, string, error) {
	if strings.TrimSpace(in.Raw) != "" {
		name := sanitizePolicyName(firstNonEmpty(in.Name, "custom_check"))
		return name, in.Raw, nil
	}

	tmpl, ok := policyTemplateByID(in.TemplateID)
	if !ok {
		return "", "", fmt.Errorf("export: unknown policy template %q", in.TemplateID)
	}
	build := policyBuilders[tmpl.ID]
	if build == nil {
		return "", "", fmt.Errorf("export: policy template %q has no emitter", tmpl.ID)
	}

	name := sanitizePolicyName(firstNonEmpty(in.Name, tmpl.ID))
	helpers, cond, idExpr, err := build(in.Params, name)
	if err != nil {
		return "", "", err
	}

	content := renderRego(tmpl.Stage, tmpl.Category, helpers, cond, idExpr)
	return name, content, nil
}

// renderRego assembles a complete policy file from its pieces. The deny rule
// iterates input.groups[_].attributes[_]; cond lines are ANDed inside the body;
// idExpr is the rego expression assigned to the finding's id field.
func renderRego(stage, category string, helpers, cond []string, idExpr string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n", stage)
	b.WriteString("import rego.v1\n\n")

	for _, line := range helpers {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(helpers) > 0 {
		b.WriteByte('\n')
	}

	b.WriteString("deny contains v if {\n")
	b.WriteString("\tgroup := input.groups[_]\n")
	b.WriteString("\tattr := group.attributes[_]\n")
	for _, c := range cond {
		fmt.Fprintf(&b, "\t%s\n", c)
	}
	b.WriteString("\tv := {\n")
	fmt.Fprintf(&b, "\t\t\"type\": %s,\n", regoStr(violationType))
	fmt.Fprintf(&b, "\t\t\"id\": %s,\n", idExpr)
	fmt.Fprintf(&b, "\t\t\"category\": %s,\n", regoStr(category))
	b.WriteString("\t\t\"group\": group.id,\n")
	b.WriteString("\t\t\"attr\": attr.name,\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

// sanitizePolicyName reduces an arbitrary name to a rego-safe file/identifier
// token: lower-case, [a-z0-9_] only, never empty.
func sanitizePolicyName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == ' ':
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "check"
	}
	return out
}

func regoStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func regoStrArray(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = regoStr(v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func paramStr(p map[string]any, key string) string {
	if p == nil {
		return ""
	}
	if v, ok := p[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func paramList(p map[string]any, key string) []string {
	if p == nil {
		return nil
	}
	v, ok := p[key]
	if !ok {
		return nil
	}
	var raw []string
	switch t := v.(type) {
	case []string:
		raw = t
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				raw = append(raw, s)
			}
		}
	case string:
		raw = strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == '\n' })
	}
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// uniquePolicyName ensures a policy file base name is unique within a registry by
// suffixing _2, _3, … on collision.
func uniquePolicyName(base string, used map[string]bool) string {
	name := base
	for i := 2; used[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	used[name] = true
	return name
}
