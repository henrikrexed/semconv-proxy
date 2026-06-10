// Weaver Asset Builder — Definitions surface (Phase 2 · S2).
//
// A self-contained Preact + htm island (no JSX, no build step) mounted on demand
// by app.js when the Builder scope is opened. It seeds the editable attribute
// table from GET /api/v1/builder/seed and drives a live generated-YAML preview
// through POST /api/v1/builder/generate.
import { html, render, useState, useEffect, useRef } from "./vendor/preact-htm.standalone.module.js";

const STABILITIES = ["stable", "development", "release_candidate", "deprecated"];
const REQUIREMENTS = ["required", "recommended", "opt_in", "conditionally_required"];
// FindingLevel enum for a finding filter's min_level ("" = no minimum).
const FINDING_LEVELS = ["", "information", "improvement", "violation"];

// toList splits a comma/newline-separated textarea/input into a trimmed string[].
function toList(text) {
  return String(text || "")
    .split(/[,\n]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

// Cross-ref classification → status-chip label + reused badge class.
const CROSS_REF = {
  matched: { label: "matched", badge: "stable" },
  "type-mismatch": { label: "type mismatch", badge: "development" },
  deprecated: { label: "deprecated", badge: "deprecated" },
  "not-in-registry": { label: "not in registry", badge: "release_candidate" },
};

function chipFor(crossRef) {
  return CROSS_REF[crossRef] || { label: crossRef || "?", badge: "" };
}

async function getJSON(url) {
  const r = await fetch(url);
  if (!r.ok) throw new Error(r.status + " " + r.statusText);
  return r.json();
}

// examplesToText / textToExamples bridge the string the user types and the
// string[] the generate endpoint expects (one example per line).
function examplesToText(arr) {
  return (arr || []).join("\n");
}
function textToExamples(text) {
  return String(text || "")
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

// seedToRow shapes one /builder/seed attribute into the editable row model.
// Suggested registry enrichment (brief/stability/requirement) pre-fills the
// editable fields so a matched attribute starts from the official wording; the
// observed type is the authoritative type pre-fill (acceptance requirement).
function seedToRow(a) {
  return {
    id: a.id,
    namespace: a.namespace || "custom",
    type: a.type || "string",
    observedType: a.observed_type || a.type || "",
    brief: a.brief || "",
    stability: a.stability || "development",
    requirement_level: a.requirement_level || "recommended",
    examplesText: examplesToText(a.examples),
    crossRef: a.cross_ref,
    registryType: a.registry_type || "",
    selected: false,
  };
}

// checkComplete reports whether a check carries enough input to emit valid rego:
// a raw check needs non-empty rego; a template check needs all required params.
function checkComplete(c, catalog) {
  if (c.rawMode) return String(c.raw || "").trim() !== "";
  const tmpl = (catalog || []).find((t) => t.id === c.template_id);
  if (!tmpl) return false;
  return (tmpl.params || []).every((p) => !p.required || String(c.params[p.name] || "").trim() !== "");
}

// checkToPolicy maps a UI check to a PolicyInput. string_list params stay as the
// raw textarea string — the backend splits on commas/newlines.
function checkToPolicy(c) {
  if (c.rawMode) return { name: c.name || undefined, raw: c.raw };
  return { template_id: c.template_id, name: c.name || undefined, params: { ...c.params } };
}

// configToPayload maps the Config-tab model to the BuilderState.config object.
// Returns undefined when config emission is disabled, so no `.weaver.toml` is
// generated. List/optional fields collapse to undefined when empty so the
// backend defaults (registry.path ".", policy.paths ["policies"]) can apply.
function configToPayload(config) {
  if (!config || !config.enabled) return undefined;
  const policyPaths = toList(config.policy_paths);
  const filters = (config.filters || [])
    .map((f) => ({
      exclude: toList(f.exclude),
      exclude_samples: toList(f.exclude_samples),
      min_level: f.min_level || undefined,
      signal_type: f.signal_type || undefined,
    }))
    .map((f) => ({
      exclude: f.exclude.length ? f.exclude : undefined,
      exclude_samples: f.exclude_samples.length ? f.exclude_samples : undefined,
      min_level: f.min_level,
      signal_type: f.signal_type,
    }))
    .filter((f) => f.exclude || f.exclude_samples || f.min_level || f.signal_type);
  return {
    registry_path: config.registry_path || undefined,
    policy_paths: policyPaths.length ? policyPaths : undefined,
    policy_skip: config.policy_skip || undefined,
    advice_policies: config.advice_policies || undefined,
    advice_preprocessor: config.advice_preprocessor || undefined,
    diagnostics_format: config.diagnostics_format || undefined,
    finding_filters: filters.length ? filters : undefined,
  };
}

// buildState turns the row + check models into the BuilderState payload that the
// generate endpoint (and WeaverExporter) consume. Only complete checks are sent
// so an in-progress check form does not break the whole preview.
function buildState(manifest, rows, checks, catalog, config) {
  return {
    manifest: {
      schema_url: manifest.schema_url,
      description: manifest.description || undefined,
    },
    groups: rows.map((r) => ({
      namespace: r.namespace || "custom",
      attributes: [
        {
          id: r.id,
          type: r.type || undefined,
          brief: r.brief || undefined,
          stability: r.stability || undefined,
          requirement_level: r.requirement_level || undefined,
          examples: textToExamples(r.examplesText),
        },
      ],
    })),
    policies: (checks || []).filter((c) => checkComplete(c, catalog)).map(checkToPolicy),
    config: configToPayload(config),
  };
}

function downloadFile(path, content) {
  const name = path.split("/").pop();
  const blob = new Blob([content], { type: "text/yaml" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function ManifestForm({ manifest, setManifest }) {
  const upd = (k, v) => setManifest({ ...manifest, [k]: v });
  return html`
    <div class="bld-manifest">
      <label class="bld-field">
        <span>Schema URL <em>(required, versioned)</em></span>
        <input
          type="url"
          placeholder="https://acme.com/schemas/0.1.0"
          value=${manifest.schema_url}
          onInput=${(e) => upd("schema_url", e.target.value)}
        />
      </label>
      <label class="bld-field">
        <span>Description</span>
        <input
          placeholder="Acme custom semantic conventions."
          value=${manifest.description}
          onInput=${(e) => upd("description", e.target.value)}
        />
      </label>
    </div>
  `;
}

function GroupTools({ rows, setRows }) {
  const [ns, setNs] = useState("");
  const selectedCount = rows.filter((r) => r.selected).length;
  const allSelected = rows.length > 0 && selectedCount === rows.length;

  const toggleAll = () =>
    setRows(rows.map((r) => ({ ...r, selected: !allSelected })));
  const apply = () => {
    if (!ns.trim()) return;
    setRows(rows.map((r) => (r.selected ? { ...r, namespace: ns.trim(), selected: false } : r)));
    setNs("");
  };

  return html`
    <div class="bld-tools">
      <label class="bld-check">
        <input type="checkbox" checked=${allSelected} onChange=${toggleAll} />
        <span>Select all</span>
      </label>
      <span class="bld-muted">${selectedCount} selected</span>
      <input
        class="bld-ns-input"
        placeholder="namespace (e.g. acme.payments)"
        value=${ns}
        onInput=${(e) => setNs(e.target.value)}
      />
      <button class="bld-btn" disabled=${selectedCount === 0 || !ns.trim()} onClick=${apply}>
        Add selected to group
      </button>
    </div>
  `;
}

function AttrTable({ rows, setRows }) {
  const upd = (i, k, v) => setRows(rows.map((r, j) => (j === i ? { ...r, [k]: v } : r)));
  if (rows.length === 0) {
    return html`<p class="empty">No telemetry attributes observed yet. Send data through the proxy, then reopen the Builder.</p>`;
  }
  return html`
    <div class="bld-table-wrap">
      <table class="bld-table">
        <thead>
          <tr>
            <th class="bld-col-sel"></th>
            <th>Attribute</th>
            <th>Namespace</th>
            <th>Type</th>
            <th>Stability</th>
            <th>Requirement</th>
            <th>Brief</th>
            <th>Examples</th>
            <th>Community</th>
          </tr>
        </thead>
        <tbody>
          ${rows.map((r, i) => {
            const chip = chipFor(r.crossRef);
            return html`
              <tr key=${r.id}>
                <td class="bld-col-sel">
                  <input
                    type="checkbox"
                    checked=${r.selected}
                    onChange=${(e) => upd(i, "selected", e.target.checked)}
                  />
                </td>
                <td class="bld-attr-name">${r.id}</td>
                <td><input value=${r.namespace} onInput=${(e) => upd(i, "namespace", e.target.value)} /></td>
                <td>
                  <input
                    class="bld-type"
                    value=${r.type}
                    title=${r.observedType ? "observed type: " + r.observedType : ""}
                    onInput=${(e) => upd(i, "type", e.target.value)}
                  />
                </td>
                <td>
                  <select value=${r.stability} onChange=${(e) => upd(i, "stability", e.target.value)}>
                    ${STABILITIES.map((s) => html`<option value=${s} selected=${s === r.stability}>${s}</option>`)}
                  </select>
                </td>
                <td>
                  <select value=${r.requirement_level} onChange=${(e) => upd(i, "requirement_level", e.target.value)}>
                    ${REQUIREMENTS.map((s) => html`<option value=${s} selected=${s === r.requirement_level}>${s}</option>`)}
                  </select>
                </td>
                <td><input value=${r.brief} placeholder="Describe this attribute…" onInput=${(e) => upd(i, "brief", e.target.value)} /></td>
                <td>
                  <input
                    value=${r.examplesText.replace(/\n/g, ", ")}
                    placeholder="one, per, comma"
                    onInput=${(e) => upd(i, "examplesText", e.target.value.split(",").map((s) => s.trim()).filter(Boolean).join("\n"))}
                  />
                </td>
                <td>
                  <span class=${"badge " + chip.badge} title=${r.registryType ? "registry type: " + r.registryType : ""}>${chip.label}</span>
                </td>
              </tr>
            `;
          })}
        </tbody>
      </table>
    </div>
  `;
}

// buildTree turns the flat path->content map into a nested directory tree so the
// preview mirrors the registry layout the user will commit (manifest at root,
// groups/ and policies/ as folders).
function buildTree(files) {
  const root = { name: "", dirs: {}, files: [] };
  for (const path of Object.keys(files)) {
    const parts = path.split("/");
    let node = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const seg = parts[i];
      if (!node.dirs[seg]) node.dirs[seg] = { name: seg, dirs: {}, files: [] };
      node = node.dirs[seg];
    }
    node.files.push({ path, name: parts[parts.length - 1] });
  }
  return root;
}

// FileNode renders one generated file: a header (name + per-file download) and a
// collapsible source preview.
function FileNode({ node, content }) {
  const [open, setOpen] = useState(true);
  return html`
    <div class="bld-file" key=${node.path}>
      <div class="bld-file-head">
        <button class="bld-tree-toggle" onClick=${() => setOpen(!open)} aria-expanded=${open} title=${open ? "Collapse" : "Expand"}>
          ${open ? "▾" : "▸"}
        </button>
        <code>${node.name}</code>
        <button class="bld-btn bld-btn-sm" onClick=${() => downloadFile(node.path, content)}>Download</button>
      </div>
      ${open ? html`<pre>${content}</pre>` : null}
    </div>
  `;
}

// TreeNode renders a directory level: nested folders first (sorted), then the
// files at this level (sorted).
function TreeNode({ node, files }) {
  const dirNames = Object.keys(node.dirs).sort();
  const fileNodes = [...node.files].sort((a, b) => a.name.localeCompare(b.name));
  return html`
    <div class="bld-tree-level">
      ${dirNames.map(
        (d) => html`
          <div class="bld-tree-dir" key=${d}>
            <div class="bld-tree-dir-name">📁 <code>${d}/</code></div>
            <div class="bld-tree-children">
              <${TreeNode} node=${node.dirs[d]} files=${files} />
            </div>
          </div>
        `
      )}
      ${fileNodes.map((f) => html`<${FileNode} key=${f.path} node=${f} content=${files[f.path]} />`)}
    </div>
  `;
}

function Preview({ files, error, loading, onDownloadZip }) {
  const paths = Object.keys(files);
  const tree = buildTree(files);
  const hasFiles = paths.length > 0;
  return html`
    <div class="bld-preview">
      <div class="bld-preview-head">
        <h3>Generated registry ${loading ? html`<span class="bld-muted">· generating…</span>` : null}</h3>
        <button class="bld-btn bld-btn-sm" disabled=${!hasFiles} onClick=${onDownloadZip} title="Download the whole registry as a zip">
          Download all (.zip)
        </button>
      </div>
      ${error ? html`<p class="bld-error">${error}</p>` : null}
      ${!hasFiles && !error
        ? html`<p class="bld-muted">Set a valid schema URL to preview the generated registry.</p>`
        : html`<${TreeNode} node=${tree} files=${files} />`}
    </div>
  `;
}

let checkSeq = 0;

// newTemplateCheck instantiates a check from a catalog template, pre-filling
// param defaults.
function newTemplateCheck(tmpl) {
  const params = {};
  (tmpl.params || []).forEach((p) => {
    params[p.name] = p.default || "";
  });
  return { key: "c" + ++checkSeq, rawMode: false, template_id: tmpl.id, title: tmpl.title, stage: tmpl.stage, name: "", params };
}

function newRawCheck() {
  return {
    key: "c" + ++checkSeq,
    rawMode: true,
    name: "",
    raw: "package after_resolution\nimport rego.v1\n\ndeny contains v if {\n\t# your rule here\n\tfalse\n}\n",
  };
}

// ChecksCatalog lists the parameterised templates; each card adds a configurable
// check instance. Raw Rego is the advanced escape hatch.
function ChecksCatalog({ catalog, onAdd, onAddRaw }) {
  return html`
    <div class="bld-cat">
      ${catalog.map(
        (t) => html`
          <div class="bld-cat-card" key=${t.id}>
            <div class="bld-cat-head">
              <strong>${t.title}</strong>
              <span class="badge stable" title="Weaver stage (rego package)">${t.stage}</span>
            </div>
            <p class="bld-muted">${t.description}</p>
            <button class="bld-btn bld-btn-sm" onClick=${() => onAdd(t)}>Add check</button>
          </div>
        `
      )}
      <div class="bld-cat-card bld-cat-raw">
        <div class="bld-cat-head">
          <strong>Raw Rego</strong>
          <span class="badge release_candidate" title="Advanced escape hatch">advanced</span>
        </div>
        <p class="bld-muted">Hand-write a policy. Output is passed through unmodified — you own the §10.2 contract.</p>
        <button class="bld-btn bld-btn-sm" onClick=${onAddRaw}>Add raw policy</button>
      </div>
    </div>
  `;
}

function CheckParam({ param, value, onChange }) {
  if (param.type === "string_list") {
    return html`
      <label class="bld-field">
        <span>${param.label} <em>${param.required ? "(required)" : ""}</em></span>
        <textarea
          class="bld-raw"
          rows="3"
          placeholder=${param.help || "one per line"}
          value=${value}
          onInput=${(e) => onChange(e.target.value)}
        ></textarea>
      </label>
    `;
  }
  return html`
    <label class="bld-field">
      <span>${param.label} <em>${param.required ? "(required)" : ""}</em></span>
      <input
        placeholder=${param.help || ""}
        value=${value}
        onInput=${(e) => onChange(e.target.value)}
      />
    </label>
  `;
}

// CheckCard renders one selected check: its param form (or raw editor) plus a
// remove control.
function CheckCard({ check, catalog, onUpdate, onRemove }) {
  const tmpl = check.rawMode ? null : catalog.find((t) => t.id === check.template_id);
  const setParam = (name, v) => onUpdate({ ...check, params: { ...check.params, [name]: v } });
  return html`
    <div class="bld-check-card">
      <div class="bld-check-head">
        <strong>${check.rawMode ? "Raw Rego" : check.title}</strong>
        ${check.stage ? html`<span class="badge stable">${check.stage}</span>` : null}
        <button class="bld-btn bld-btn-sm bld-btn-ghost" onClick=${onRemove} title="Remove check">Remove</button>
      </div>
      <label class="bld-field">
        <span>File name <em>(optional)</em></span>
        <input
          placeholder=${check.rawMode ? "custom_check" : check.template_id}
          value=${check.name}
          onInput=${(e) => onUpdate({ ...check, name: e.target.value })}
        />
      </label>
      ${check.rawMode
        ? html`
            <label class="bld-field">
              <span>Rego source</span>
              <textarea
                class="bld-raw bld-raw-tall"
                rows="8"
                value=${check.raw}
                onInput=${(e) => onUpdate({ ...check, raw: e.target.value })}
              ></textarea>
            </label>
          `
        : (tmpl && tmpl.params || []).map(
            (p) => html`<${CheckParam} key=${p.name} param=${p} value=${check.params[p.name] || ""} onChange=${(v) => setParam(p.name, v)} />`
          )}
    </div>
  `;
}

function ChecksPanel({ catalog, catalogError, checks, setChecks }) {
  if (catalogError) return html`<p class="bld-error">Could not load the check catalog: ${catalogError}</p>`;
  const add = (t) => setChecks([...checks, newTemplateCheck(t)]);
  const addRaw = () => setChecks([...checks, newRawCheck()]);
  const update = (i, c) => setChecks(checks.map((x, j) => (j === i ? c : x)));
  const remove = (i) => setChecks(checks.filter((_, j) => j !== i));
  return html`
    <div class="bld-checks">
      <p class="bld-muted">
        Add checks from the catalog below; each emits a <code>policies/&lt;name&gt;.rego</code> file in the preview.
        Pick a template (the common path) or hand-write Rego as an advanced escape hatch.
      </p>
      <${ChecksCatalog} catalog=${catalog} onAdd=${add} onAddRaw=${addRaw} />
      ${checks.length === 0
        ? html`<p class="bld-muted">No checks added yet.</p>`
        : html`<div class="bld-check-list">
            ${checks.map((c, i) => html`<${CheckCard} key=${c.key} check=${c} catalog=${catalog} onUpdate=${(x) => update(i, x)} onRemove=${() => remove(i)} />`)}
          </div>`}
    </div>
  `;
}

let filterSeq = 0;

function newFilter() {
  return { key: "f" + ++filterSeq, exclude: "", exclude_samples: "", min_level: "", signal_type: "" };
}

// FindingFilterRow edits one `[[live_check.finding_filters]]` entry. signal_type
// options come from the user's known signal types (seed response).
function FindingFilterRow({ filter, signalTypes, onUpdate, onRemove }) {
  const upd = (k, v) => onUpdate({ ...filter, [k]: v });
  return html`
    <div class="bld-check-card">
      <div class="bld-check-head">
        <strong>Finding filter</strong>
        <button class="bld-btn bld-btn-sm bld-btn-ghost" onClick=${onRemove} title="Remove filter">Remove</button>
      </div>
      <label class="bld-field">
        <span>Exclude finding IDs <em>(comma or newline)</em></span>
        <input placeholder="missing_attribute, deprecated_attribute" value=${filter.exclude} onInput=${(e) => upd("exclude", e.target.value)} />
      </label>
      <label class="bld-field">
        <span>Exclude samples <em>(attribute keys)</em></span>
        <input placeholder="trace.parent_id, trace.span_id" value=${filter.exclude_samples} onInput=${(e) => upd("exclude_samples", e.target.value)} />
      </label>
      <label class="bld-field">
        <span>Minimum level</span>
        <select value=${filter.min_level} onChange=${(e) => upd("min_level", e.target.value)}>
          ${FINDING_LEVELS.map((l) => html`<option value=${l} selected=${l === filter.min_level}>${l || "(any)"}</option>`)}
        </select>
      </label>
      <label class="bld-field">
        <span>Signal type scope</span>
        <select value=${filter.signal_type} onChange=${(e) => upd("signal_type", e.target.value)}>
          <option value="" selected=${filter.signal_type === ""}>(all)</option>
          ${(signalTypes || []).map((s) => html`<option value=${s} selected=${s === filter.signal_type}>${s}</option>`)}
        </select>
      </label>
    </div>
  `;
}

// ConfigPanel drives the `.weaver.toml` emitter: a master enable toggle plus the
// registry / policy / live-check / diagnostics sections and finding-filter rows.
function ConfigPanel({ config, setConfig, signalTypes }) {
  const upd = (k, v) => setConfig({ ...config, [k]: v });
  const updFilter = (i, f) => setConfig({ ...config, filters: config.filters.map((x, j) => (j === i ? f : x)) });
  const addFilter = () => setConfig({ ...config, filters: [...config.filters, newFilter()] });
  const removeFilter = (i) => setConfig({ ...config, filters: config.filters.filter((_, j) => j !== i) });

  return html`
    <div class="bld-checks">
      <label class="bld-check">
        <input type="checkbox" checked=${config.enabled} onChange=${(e) => upd("enabled", e.target.checked)} />
        <span>Emit <code>.weaver.toml</code></span>
      </label>
      <p class="bld-muted">
        Generates the Weaver config that wires your registry, the generated <code>policies/</code> checks,
        and live-check finding filters. Leave a field blank to take the default
        (<code>registry.path = "."</code>, <code>policy.paths = ["policies"]</code>).
      </p>
      ${!config.enabled
        ? null
        : html`
            <div class="bld-cfg">
              <h4>Registry &amp; policy</h4>
              <label class="bld-field">
                <span>Registry path</span>
                <input placeholder="." value=${config.registry_path} onInput=${(e) => upd("registry_path", e.target.value)} />
              </label>
              <label class="bld-field">
                <span>Policy paths <em>(comma or newline)</em></span>
                <input placeholder="policies" value=${config.policy_paths} onInput=${(e) => upd("policy_paths", e.target.value)} />
              </label>
              <label class="bld-check">
                <input type="checkbox" checked=${config.policy_skip} onChange=${(e) => upd("policy_skip", e.target.checked)} />
                <span>Skip policy checks</span>
              </label>

              <h4>Live check</h4>
              <label class="bld-field">
                <span>Advice policies directory</span>
                <input placeholder="advice" value=${config.advice_policies} onInput=${(e) => upd("advice_policies", e.target.value)} />
              </label>
              <label class="bld-field">
                <span>Advice preprocessor <em>(jq)</em></span>
                <input placeholder=".groups" value=${config.advice_preprocessor} onInput=${(e) => upd("advice_preprocessor", e.target.value)} />
              </label>

              <h4>Diagnostics</h4>
              <label class="bld-field">
                <span>Format</span>
                <input placeholder="ansi" value=${config.diagnostics_format} onInput=${(e) => upd("diagnostics_format", e.target.value)} />
              </label>

              <h4>Finding filters</h4>
              <p class="bld-muted">Drop live-check findings by id, sample, minimum level, or signal-type scope.</p>
              <button class="bld-btn bld-btn-sm" onClick=${addFilter}>Add filter</button>
              ${config.filters.length === 0
                ? html`<p class="bld-muted">No filters added.</p>`
                : html`<div class="bld-check-list">
                    ${config.filters.map((f, i) => html`<${FindingFilterRow} key=${f.key} filter=${f} signalTypes=${signalTypes} onUpdate=${(x) => updFilter(i, x)} onRemove=${() => removeFilter(i)} />`)}
                  </div>`}
            </div>
          `}
    </div>
  `;
}

function Builder() {
  const [loading, setLoading] = useState(true);
  const [seedError, setSeedError] = useState("");
  const [manifest, setManifest] = useState({ schema_url: "https://example.com/schemas/0.1.0", description: "" });
  const [rows, setRows] = useState([]);
  const [files, setFiles] = useState({});
  const [genError, setGenError] = useState("");
  const [generating, setGenerating] = useState(false);
  const [tab, setTab] = useState("definitions"); // "definitions" | "checks" | "config"
  const [catalog, setCatalog] = useState([]);
  const [catalogError, setCatalogError] = useState("");
  const [checks, setChecks] = useState([]);
  const [signalTypes, setSignalTypes] = useState([]);
  const [config, setConfig] = useState({
    enabled: false,
    registry_path: "",
    policy_paths: "",
    policy_skip: false,
    advice_policies: "",
    advice_preprocessor: "",
    diagnostics_format: "",
    filters: [],
  });
  const debounceRef = useRef(null);

  // Seed the definitions table and load the check catalog once on mount.
  useEffect(() => {
    let cancelled = false;
    getJSON("/api/v1/builder/seed")
      .then((data) => {
        if (cancelled) return;
        setRows((data.attributes || []).map(seedToRow));
        setSignalTypes(data.known_signal_types || []);
        setLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        setSeedError(err.message);
        setLoading(false);
      });
    getJSON("/api/v1/builder/policy-templates")
      .then((data) => {
        if (!cancelled) setCatalog(data.templates || []);
      })
      .catch((err) => {
        if (!cancelled) setCatalogError(err.message);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Regenerate the preview (debounced) whenever the manifest or rows change.
  useEffect(() => {
    if (!manifest.schema_url) {
      setFiles({});
      setGenError("");
      return;
    }
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      setGenerating(true);
      try {
        const r = await fetch("/api/v1/builder/generate", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(buildState(manifest, rows, checks, catalog, config)),
        });
        const data = await r.json().catch(() => ({}));
        if (!r.ok) {
          setGenError(data.error || r.status + " " + r.statusText);
          setFiles({});
        } else {
          setGenError("");
          setFiles(data.files || {});
        }
      } catch (err) {
        setGenError(err.message);
        setFiles({});
      } finally {
        setGenerating(false);
      }
    }, 400);
    return () => clearTimeout(debounceRef.current);
  }, [manifest, rows, checks, catalog, config]);

  // Bundle the current builder state into a registry.zip and trigger a download.
  // Reuses the same state payload as the live preview so the archive matches what
  // the tree shows.
  async function downloadZip() {
    try {
      const r = await fetch("/api/v1/builder/export.zip", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(buildState(manifest, rows, checks, catalog, config)),
      });
      if (!r.ok) {
        const data = await r.json().catch(() => ({}));
        setGenError(data.error || r.status + " " + r.statusText);
        return;
      }
      const blob = await r.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "registry.zip";
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setGenError(err.message);
    }
  }

  if (loading) return html`<p class="bld-muted">Loading discovered attributes…</p>`;
  if (seedError) return html`<p class="bld-error">Could not seed the builder: ${seedError}</p>`;

  return html`
    <div class="bld">
      <header class="bld-header">
        <h2>Weaver Asset Builder</h2>
        <p class="bld-muted">
          Author a custom semantic-convention registry from the attributes this proxy has observed,
          cross-referenced against the official registry. The generated assets update live.
        </p>
      </header>
      <div class="bld-subtabs" role="tablist">
        <button class=${"bld-subtab" + (tab === "definitions" ? " active" : "")} role="tab" aria-selected=${tab === "definitions"} onClick=${() => setTab("definitions")}>Definitions</button>
        <button class=${"bld-subtab" + (tab === "checks" ? " active" : "")} role="tab" aria-selected=${tab === "checks"} onClick=${() => setTab("checks")}>Checks</button>
        <button class=${"bld-subtab" + (tab === "config" ? " active" : "")} role="tab" aria-selected=${tab === "config"} onClick=${() => setTab("config")}>Config</button>
      </div>
      <${ManifestForm} manifest=${manifest} setManifest=${setManifest} />
      ${tab === "definitions"
        ? html`
            <${GroupTools} rows=${rows} setRows=${setRows} />
            <${AttrTable} rows=${rows} setRows=${setRows} />
          `
        : tab === "checks"
        ? html`<${ChecksPanel} catalog=${catalog} catalogError=${catalogError} checks=${checks} setChecks=${setChecks} />`
        : html`<${ConfigPanel} config=${config} setConfig=${setConfig} signalTypes=${signalTypes} />`}
      <${Preview} files=${files} error=${genError} loading=${generating} onDownloadZip=${downloadZip} />
    </div>
  `;
}

export function mount(container) {
  render(html`<${Builder} />`, container);
}
