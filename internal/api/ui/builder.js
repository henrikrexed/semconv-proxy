// Weaver Asset Builder — Definitions surface (Phase 2 · S2).
//
// A self-contained Preact + htm island (no JSX, no build step) mounted on demand
// by app.js when the Builder scope is opened. It seeds the editable attribute
// table from GET /api/v1/builder/seed and drives a live generated-YAML preview
// through POST /api/v1/builder/generate.
import { html, render, useState, useEffect, useRef } from "./vendor/preact-htm.standalone.module.js";

const STABILITIES = ["stable", "development", "release_candidate", "deprecated"];
const REQUIREMENTS = ["required", "recommended", "opt_in", "conditionally_required"];

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

// buildState turns the row model into the BuilderState payload that the
// generate endpoint (and WeaverExporter) consume.
function buildState(manifest, rows) {
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

function Preview({ files, error, loading }) {
  const paths = Object.keys(files).sort();
  return html`
    <div class="bld-preview">
      <div class="bld-preview-head">
        <h3>Generated assets ${loading ? html`<span class="bld-muted">· generating…</span>` : null}</h3>
      </div>
      ${error ? html`<p class="bld-error">${error}</p>` : null}
      ${paths.length === 0 && !error
        ? html`<p class="bld-muted">Set a valid schema URL to preview the generated registry.</p>`
        : null}
      ${paths.map(
        (p) => html`
          <div class="bld-file" key=${p}>
            <div class="bld-file-head">
              <code>${p}</code>
              <button class="bld-btn bld-btn-sm" onClick=${() => downloadFile(p, files[p])}>Download</button>
            </div>
            <pre>${files[p]}</pre>
          </div>
        `
      )}
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
  const debounceRef = useRef(null);

  // Seed once on mount.
  useEffect(() => {
    let cancelled = false;
    getJSON("/api/v1/builder/seed")
      .then((data) => {
        if (cancelled) return;
        setRows((data.attributes || []).map(seedToRow));
        setLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        setSeedError(err.message);
        setLoading(false);
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
          body: JSON.stringify(buildState(manifest, rows)),
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
  }, [manifest, rows]);

  if (loading) return html`<p class="bld-muted">Loading discovered attributes…</p>`;
  if (seedError) return html`<p class="bld-error">Could not seed the builder: ${seedError}</p>`;

  return html`
    <div class="bld">
      <header class="bld-header">
        <h2>Weaver Asset Builder — Definitions</h2>
        <p class="bld-muted">
          Author a custom semantic-convention registry from the attributes this proxy has observed,
          cross-referenced against the official registry. Edit the rows below; the generated assets update live.
        </p>
      </header>
      <${ManifestForm} manifest=${manifest} setManifest=${setManifest} />
      <${GroupTools} rows=${rows} setRows=${setRows} />
      <${AttrTable} rows=${rows} setRows=${setRows} />
      <${Preview} files=${files} error=${genError} loading=${generating} />
    </div>
  `;
}

export function mount(container) {
  render(html`<${Builder} />`, container);
}
