# Brainstorming Report: Collector Semantic Convention

**Project:** Collector Semantic Convention  
**Date:** 2026-05-07  
**Method:** Interactive brainstorming (Alfred + Henrik)

## 1. Problem Statement

Currently, OpenTelemetry Weaver requires users to manually define semantic conventions in YAML, then validates/enforces them. This is a **top-down (schema-first)** approach.

Henrik wants the **inverse**: a tool that **discovers** what semantic conventions are actually in use from live telemetry signals, builds a live dictionary, and exports it in Weaver-compatible format.

### Core Requirements
- Intercept telemetry signals (metrics, logs, traces) from any source
- Auto-discover semantic conventions (attribute names, metric names, units, types, span patterns, log fields)
- Maintain a live dictionary in memory
- Expose an API to query/export the dictionary
- Export in OTel Weaver-compatible YAML format
- Support developer workflow: deploy in dev → observe → export → store in repo → CI/CD validation

## 2. Architecture Decision: OTLP Proxy (Confirmed)

### What Was Evaluated
| Component | Verdict | Reason |
|-----------|---------|--------|
| OTel Extension | ❌ No | Cannot access pipeline data |
| OTel Processor | ⚠️ Possible but awkward | In hot path, can't expose API independently |
| OTel Connector | ⚠️ Partial | Routing/transform only, not analysis + API |
| **Standalone OTLP Proxy** | ✅ **Best fit** | Full control, API surface, not tied to collector lifecycle |

### Confirmed Architecture: Option A — Proxy Between Collector and Backend

```
Applications → OTel Collector → [Semantic Conv Proxy] → Backend (Dynatrace, etc.)
                                    │
                              Prometheus scraping handled by
                              collector's prometheus receiver
                              → converts to OTLP → proxy sees uniform data
```

**Henrik confirmed:** "Option A where the proxy is between the collector and the final destination should be the right model."

### Key Architectural Points
- **All protocol-specific collection** (Prometheus scraping, Fluent Bit, etc.) happens in upstream collectors
- Proxy **always receives OTLP** — uniform input
- For Prometheus: Collector scrapes → prometheus receiver → OTLP → Proxy
- Proxy: receive OTLP → analyze → build dictionary → expose API → generate Weaver YAML → forward to backend

## 3. Developer Workflow (Henrik's Vision)

1. Developer builds a tooling project
2. Deploys it in dev environment
3. Adds the proxy to the pipeline
4. After several minutes/hours, requests the proxy to export the dictionary
5. Stores the Weaver-compatible YAML in the repository
6. Adds a GitHub Actions workflow based on the dictionary to:
   - Detect high cardinality attributes
   - Detect major diffs in attribute counts
7. IDE plugin could pull the dictionary to the local repo

## 4. Dictionary Content

### Metrics
- Metric name, type (gauge/counter/histogram/summary/exponential)
- Unit, temporality (cumulative vs delta)
- All observed attribute keys + value types + cardinality
- First seen / last seen timestamps
- Source (OTLP push vs Prometheus scrape)

### Traces
- Span names + patterns
- Attributes per span name
- Status codes, parent-child relationships
- Resource attributes

### Logs
- Attribute keys + types
- Severity levels observed
- Body field patterns (if structured)
- Resource attributes

## 5. Weaver-Compatible Output

The proxy exports in OTel Weaver semantic convention YAML format:

```yaml
groups:
  - id: metric.http.server.request.duration
    type: metric
    metric_name: http.server.request.duration
    brief: "Auto-discovered metric"
    instrument: histogram
    unit: s
    attributes:
      - id: http.request.method
        type: string
        requirement_level: recommended
    stability: experimental
    annotations:
      semconv.proxy.status: "matches_standard"
      semconv.proxy.standard_ref: "http.request.method"
```

Includes comparison against OTel standard semantic conventions: ✅ match, ⚠️ near-match, ❌ custom.

## 6. User Interface Layers (Phased)

| Phase | Interface | Purpose |
|-------|-----------|---------|
| v1 | REST API | Query dictionary, export Weaver YAML |
| v1 | Weaver YAML export | Feed into Weaver for validation/code-gen |
| v2 | MCP Server | AI agent queries ("What metrics does my service emit?") |
| v2 | GitHub Actions integration | Cardinality alerts, attribute diffs |
| v3 | Web Dashboard | Browse dictionary, compare vs OTel standard |
| v3 | IDE Plugin | Pull dictionary to local repo |

## 7. OTel Weaver Integration Loop

```
1. Deploy proxy, observe telemetry (hours/days)
2. Export: GET /api/v1/export?format=weaver → semconv.yaml
3. Store in repository
4. weaver registry check --registry ./semconv.yaml
5. weaver registry generate → type-safe code
6. Feed into collector processors for enforcement
7. GitHub Actions: cardinality monitoring, drift detection
```

## 8. Open Design Decisions (for BMAD next phases)

1. **Storage engine:** In-memory with persistence (SQLite? BadgerDB? BoltDB?)
2. **Prometheus scraping:** Collector handles it (confirmed), but offer built-in scraper as v2?
3. **Deployment model:** Standalone binary? Docker? Helm chart? Sidecar vs gateway mode?
4. **Language:** Go (natural OTel ecosystem fit) — needs confirmation
5. **Cardinality limits:** How to handle high-cardinality without memory explosion
6. **Retention:** TTL-based? LRU? Size-based eviction?
7. **OTel registry matching:** Local copy of standard semconv? How to keep updated?
8. **Multi-tenancy:** Per-service dictionary or global?
9. **CI/CD integration:** GitHub Action as standalone or proxy API-based?
10. **IDE plugin:** VS Code first? Language Server Protocol?

## 9. Key Themes & Decisions

- **Bottom-up discovery** complements Weaver's top-down enforcement
- **Always OTLP input** keeps proxy focused and protocol-agnostic
- **Weaver YAML as primary export format** enables the full OTel tooling ecosystem
- **Developer-centric workflow**: dev deployment → observe → export → repo → CI
- **Comparison against OTel standard** helps teams adopt best practices incrementally
