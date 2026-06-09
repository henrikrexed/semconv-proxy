"use strict";

const PAGE = 50;

const state = {
  scope: "community", // "community" | "mine"
  q: "",
  filters: { type: new Set(), stability: new Set(), namespace: new Set() },
  offset: 0,
  total: 0,
  items: [],
  selected: null,
};

const el = (id) => document.getElementById(id);
const results = el("results-list");
const meta = el("results-meta");

// ---- URL hash (deep links) ----
function readHash() {
  const p = new URLSearchParams(location.hash.slice(1));
  if (p.get("scope") === "mine" || p.get("scope") === "community") state.scope = p.get("scope");
  state.q = p.get("q") || "";
  state.selected = p.get("sel") || null;
  for (const dim of ["type", "stability", "namespace"]) {
    state.filters[dim] = new Set((p.get(dim) || "").split(",").filter(Boolean));
  }
}
function writeHash() {
  const p = new URLSearchParams();
  p.set("scope", state.scope);
  if (state.q) p.set("q", state.q);
  for (const dim of ["type", "stability", "namespace"]) {
    if (state.filters[dim].size) p.set(dim, [...state.filters[dim]].join(","));
  }
  if (state.selected) p.set("sel", state.selected);
  const next = "#" + p.toString();
  if (next !== location.hash) history.replaceState(null, "", next);
}

// ---- Normalization ----
function normCommunity(it) {
  return { key: it.key, name: it.name, type: it.type, stability: it.stability, deprecated: it.deprecated, raw: it };
}
function normMine(e) {
  const t = (e.signal_types && e.signal_types[0]) || e.type || "attribute";
  return { key: "mine:" + e.name, name: e.name, type: "attribute", stability: e.status, signal: t, deprecated: null, raw: e };
}

// ---- Fetching ----
async function fetchPage(reset) {
  if (reset) { state.offset = 0; state.items = []; }
  toast("Loading…");
  try {
    let data;
    if (state.scope === "community") {
      const p = new URLSearchParams();
      if (state.q) p.set("q", state.q);
      for (const dim of ["type", "stability", "namespace"]) {
        for (const v of state.filters[dim]) p.append(dim, v);
      }
      p.set("limit", PAGE); p.set("offset", state.offset);
      data = await getJSON("/api/v1/semconv/community?" + p);
      state.total = data.total;
      state.items.push(...(data.entries || []).map(normCommunity));
      renderFacets(data.facets);
    } else {
      const p = new URLSearchParams();
      if (state.q) p.set("q", state.q);
      const t = [...state.filters.type][0];
      if (t) p.set("type", t);
      p.set("limit", PAGE); p.set("offset", state.offset);
      data = await getJSON("/api/v1/dictionary?" + p);
      state.total = data.total;
      state.items.push(...(data.entries || []).map(normMine));
      renderFacets(mineFacets());
    }
    renderResults();
    clearToast();
  } catch (err) {
    toast("Error: " + err.message, true);
  }
}

async function getJSON(url) {
  const r = await fetch(url);
  if (!r.ok) throw new Error(r.status + " " + r.statusText);
  return r.json();
}

// Client-side facets for the My-Telemetry scope (over loaded entries only).
function mineFacets() {
  const stab = {};
  for (const it of state.items) if (it.stability) stab[it.stability] = (stab[it.stability] || 0) + 1;
  const sig = {};
  for (const it of state.items) if (it.signal) sig[it.signal] = (sig[it.signal] || 0) + 1;
  const toArr = (o) => Object.entries(o).map(([value, count]) => ({ value, count })).sort((a, b) => b.count - a.count);
  return { type: toArr(sig), stability: toArr(stab), namespace: [] };
}

// ---- Rendering ----
function renderResults() {
  results.innerHTML = "";
  if (state.items.length === 0) {
    results.innerHTML = '<li class="empty">No conventions match your search.</li>';
    meta.textContent = "0 results";
    el("results-more").hidden = true;
    return;
  }
  meta.textContent = `${state.total} result${state.total === 1 ? "" : "s"}` +
    (state.scope === "mine" ? " in your telemetry" : " in the official registry");
  for (const it of state.items) {
    const li = document.createElement("li");
    li.className = "result" + (it.key === state.selected ? " selected" : "");
    li.tabIndex = 0;
    const badges = [`<span class="badge type">${esc(it.type)}</span>`];
    if (it.deprecated) badges.push(`<span class="badge deprecated">deprecated</span>`);
    else if (it.stability) badges.push(`<span class="badge ${cls(it.stability)}">${esc(it.stability)}</span>`);
    li.innerHTML =
      `<div class="result-head"><span class="result-name">${esc(it.name)}</span>${badges.join("")}</div>` +
      (briefOf(it) ? `<div class="result-brief">${esc(briefOf(it))}</div>` : "");
    li.addEventListener("click", () => select(it.key));
    li.addEventListener("keydown", (e) => { if (e.key === "Enter") select(it.key); });
    results.appendChild(li);
  }
  el("results-more").hidden = state.items.length >= state.total;
}

function briefOf(it) {
  if (state.scope === "community") return it.raw.brief || "";
  return `${it.raw.type} · cardinality ${it.raw.cardinality}`;
}

function renderFacets(facets) {
  for (const dim of ["type", "stability", "namespace"]) {
    const group = document.querySelector(`.facet-group[data-dim="${dim}"]`);
    const list = group.querySelector(".facet-list");
    const vals = facets[dim] || [];
    group.hidden = vals.length === 0;
    list.innerHTML = "";
    for (const fc of vals.slice(0, dim === "namespace" ? 25 : 12)) {
      const id = `f-${dim}-${fc.value}`;
      const label = document.createElement("label");
      label.className = "facet-item";
      label.innerHTML =
        `<input type="checkbox" id="${esc(id)}" ${state.filters[dim].has(fc.value) ? "checked" : ""}/>` +
        `<span>${esc(fc.value)}</span><span class="count">${fc.count}</span>`;
      label.querySelector("input").addEventListener("change", (e) => {
        if (e.target.checked) state.filters[dim].add(fc.value);
        else state.filters[dim].delete(fc.value);
        writeHash();
        fetchPage(true);
      });
      list.appendChild(label);
    }
  }
}

async function select(key) {
  state.selected = key;
  writeHash();
  document.querySelectorAll(".result").forEach((r) => r.classList.remove("selected"));
  renderResults();
  const detail = el("detail");
  const body = el("detail-body");
  detail.hidden = false;
  body.innerHTML = '<p class="note">Loading…</p>';
  try {
    let item;
    if (state.scope === "community") {
      item = await getJSON("/api/v1/semconv/community/" + encodeURIComponent(key));
      body.innerHTML = renderCommunityDetail(item);
    } else {
      const name = key.replace(/^mine:/, "");
      item = await getJSON("/api/v1/dictionary/" + encodeURIComponent(name));
      body.innerHTML = renderMineDetail(item);
    }
  } catch (err) {
    body.innerHTML = `<p class="note">Could not load detail: ${esc(err.message)}</p>`;
  }
}

function renderCommunityDetail(it) {
  const badges = [`<span class="badge type">${esc(it.type)}</span>`];
  if (it.deprecated) badges.push(`<span class="badge deprecated">deprecated</span>`);
  else if (it.stability) badges.push(`<span class="badge ${cls(it.stability)}">${esc(it.stability)}</span>`);
  let html = `<h2>${esc(it.name)}</h2><div class="badges">${badges.join("")}</div>`;
  if (it.brief) html += `<p>${esc(it.brief)}</p>`;
  if (it.deprecated) {
    const d = it.deprecated;
    html += `<div class="dep-note"><strong>Deprecated</strong>${d.renamed_to ? ` — renamed to <code>${esc(d.renamed_to)}</code>` : ""}${d.note ? `<br/>${esc(d.note)}` : ""}</div>`;
  }
  const rows = [];
  if (it.value_type) rows.push(["Value type", `<code>${esc(it.value_type)}</code>`]);
  if (it.unit) rows.push(["Unit", `<code>${esc(it.unit)}</code>`]);
  if (it.instrument) rows.push(["Instrument", esc(it.instrument)]);
  if (it.span_kind) rows.push(["Span kind", esc(it.span_kind)]);
  if (it.requirement_level) rows.push(["Requirement", esc(it.requirement_level)]);
  if (it.namespace) rows.push(["Namespace", esc(it.namespace)]);
  if (it.examples && it.examples.length) rows.push(["Examples", it.examples.map((e) => `<code>${esc(e)}</code>`).join(" ")]);
  if (rows.length) html += dl(rows);
  if (it.enum && it.enum.length) {
    html += `<h3 style="font-size:13px;color:var(--muted);margin:14px 0 6px">Allowed values</h3><div class="attrs">` +
      it.enum.map((m) => `<code title="${esc(m.brief || "")}">${esc(m.value)}</code>`).join("") + `</div>`;
  }
  if (it.note) html += `<p class="note">${esc(it.note)}</p>`;
  if (it.attributes && it.attributes.length) {
    html += `<h3 style="font-size:13px;color:var(--muted);margin:14px 0 6px">Attributes</h3><div class="attrs">` +
      it.attributes.map((a) => `<code>${esc(a)}</code>`).join("") + `</div>`;
  }
  return html;
}

function renderMineDetail(e) {
  let html = `<h2>${esc(e.name)}</h2><div class="badges"><span class="badge type">${esc(e.type)}</span>` +
    (e.status ? `<span class="badge ${cls(e.status)}">${esc(e.status)}</span>` : "") + `</div>`;
  const rows = [
    ["Type", `<code>${esc(e.type)}</code>`],
    ["Signals", esc((e.signal_types || []).join(", "))],
    ["Cardinality", String(e.cardinality)],
    ["First seen", fmt(e.first_seen)],
    ["Last seen", fmt(e.last_seen)],
  ];
  html += dl(rows);
  if (e.top_values && e.top_values.length) {
    html += `<h3 style="font-size:13px;color:var(--muted);margin:14px 0 6px">Top values</h3><div class="attrs">` +
      e.top_values.map((v) => `<code>${esc(v.value)} (${v.approximate_count})</code>`).join("") + `</div>`;
  }
  return html;
}

function dl(rows) {
  return "<dl>" + rows.map(([k, v]) => `<dt>${esc(k)}</dt><dd>${v}</dd>`).join("") + "</dl>";
}

// ---- Helpers ----
function esc(s) {
  return String(s == null ? "" : s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}
function cls(s) { return String(s).replace(/[^a-z_]/gi, "_"); }
function fmt(t) { try { return new Date(t).toLocaleString(); } catch { return esc(t); } }

let toastTimer;
function toast(msg, sticky) {
  const s = el("status");
  s.textContent = msg; s.classList.add("show");
  clearTimeout(toastTimer);
  if (!sticky) toastTimer = setTimeout(() => s.classList.remove("show"), 1500);
}
function clearToast() { el("status").classList.remove("show"); }

// ---- Events ----
function setScope(scope) {
  if (state.scope === scope) return;
  state.scope = scope;
  state.filters = { type: new Set(), stability: new Set(), namespace: new Set() };
  el("scope-community").classList.toggle("active", scope === "community");
  el("scope-mine").classList.toggle("active", scope === "mine");
  el("scope-community").setAttribute("aria-selected", scope === "community");
  el("scope-mine").setAttribute("aria-selected", scope === "mine");
  el("detail").hidden = true;
  state.selected = null;
  writeHash();
  fetchPage(true);
}

let debounce;
function init() {
  readHash();
  el("q").value = state.q;
  el("scope-community").classList.toggle("active", state.scope === "community");
  el("scope-mine").classList.toggle("active", state.scope === "mine");

  el("scope-community").addEventListener("click", () => setScope("community"));
  el("scope-mine").addEventListener("click", () => setScope("mine"));
  el("search-form").addEventListener("submit", (e) => e.preventDefault());
  el("q").addEventListener("input", (e) => {
    state.q = e.target.value.trim();
    clearTimeout(debounce);
    debounce = setTimeout(() => { writeHash(); fetchPage(true); }, 250);
  });
  el("load-more").addEventListener("click", () => { state.offset += PAGE; fetchPage(false); });
  el("facets-toggle").addEventListener("click", () => {
    const f = el("facets");
    f.hidden = !f.hidden;
    el("facets-toggle").setAttribute("aria-expanded", String(!f.hidden));
  });
  el("facets-clear").addEventListener("click", () => {
    state.filters = { type: new Set(), stability: new Set(), namespace: new Set() };
    writeHash(); fetchPage(true);
  });
  el("detail-close").addEventListener("click", () => { el("detail").hidden = true; state.selected = null; writeHash(); renderResults(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") { el("detail").hidden = true; } });

  // Facets visible by default on wide screens.
  if (window.matchMedia("(min-width: 821px)").matches) el("facets").hidden = false;

  fetchPage(true).then(() => { if (state.selected) select(state.selected); });
}

document.addEventListener("DOMContentLoaded", init);
