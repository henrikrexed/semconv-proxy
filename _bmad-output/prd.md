# Product Requirements Document — Collector Semantic Convention Proxy

**Author:** Henrik (Product Owner), Alfred (CTO)
**Date:** 2026-05-07
**Status:** Draft

---

## Executive Summary

The Collector Semantic Convention Proxy is a standalone Go OTLP proxy that sits between OpenTelemetry Collectors and observability backends. It auto-discovers semantic conventions from live telemetry signals (metrics, traces, logs), builds a live dictionary, and exports in OTel Weaver-compatible YAML format.

### Product Differentiator

OTel Weaver requires manual, top-down semantic convention definition. This proxy provides the inverse: bottom-up discovery from actual telemetry. It answers "what conventions does my system actually use?" — bridging the gap between real-world telemetry and OTel's schema-first tooling. The proxy is the data collection layer; the product is "the linter for your telemetry."

### Target Users

| Persona | Role | Primary Need |
|---------|------|-------------|
| Platform Engineer | Primary | Discover and standardize conventions across services; export Weaver YAML for CI/CD enforcement |
| SRE / On-Call | Secondary | Identify cardinality explosions, convention drift, and attribute anomalies before they cause incidents |
| Application Developer | Secondary | Understand what telemetry their service emits; catch non-standard attributes early in dev |
| Developer Advocate | Tertiary | Demonstrate OTel best practices using real-world convention data |

### Project Context

- **Type:** Open source, community-driven (Apache 2.0)
- **Timeline:** Pure community initiative — no fixed deadline
- **Deployment:** Kubernetes (primary) with Helm chart, VM (secondary), standalone binary
- **Backend Compatibility:** Any OTel-compatible backend (Dynatrace, Grafana, Jaeger, New Relic, etc.)
- **Estimated Size:** ~2,000–3,000 LOC MVP, 2–3 weeks initial build

---

## Success Criteria

### User Success

- Platform Engineers can deploy the proxy in a dev environment, observe live telemetry for several hours, and export a complete Weaver-compatible semantic convention YAML in under 60 seconds
- SREs can identify the top-10 highest-cardinality attributes across all services within 30 seconds via the REST API
- Application Developers can see all attributes their service emits, with OTel standard match/near-miss/custom classification, without reading documentation
- The exported Weaver YAML feeds directly into `weaver registry check` and `weaver registry generate` with zero manual editing

### Business Success

- Community adoption: 100+ GitHub stars within 3 months of launch
- At least 3 organizations running the proxy in production within 6 months
- Integration into at least 1 community OTel demo or tutorial
- Active contribution from non-founding contributors within 6 months

### Technical Success

- Proxy adds <1ms p99 latency to the OTLP data path at 10,000 signals/second
- Dictionary rebuild from Pebble persistence completes in <5 seconds after crash recovery
- Memory usage stays under 512MB with 10,000 unique attributes under observation
- Zero telemetry data loss — all signals are forwarded to the backend even if the analysis pipeline is saturated
- Self-observability: 30+ metrics under `semconv.proxy.*` namespace exposed via Prometheus endpoint

### Measurable Outcomes

| Metric | Target | Measurement |
|--------|--------|-------------|
| Proxy latency overhead (p99) | <1ms | OTLP export request duration comparison with/without proxy |
| Dictionary query latency (p95) | <10ms | REST API response time |
| Cardinality tracking accuracy | >95% of top-K values correct | Count-min sketch validation against exact counting |
| Crash recovery time | <5 seconds | Time from process start to dictionary fully loaded |
| Memory under 10K attributes | <512MB | Process RSS monitoring |
| Telemetry forwarding reliability | 100% pass-through | Zero dropped signals on forwarding path |

---

## Product Scope

### MVP (P0) — Minimum Viable Product

- OTLP proxy with zero-copy pass-through forwarding
- Auto-discovery engine for metrics, traces, and logs
- In-memory sharded dictionary (64 shards, O(1) reads)
- Pebble-based async write-behind persistence with crash recovery
- REST API for dictionary queries and Weaver YAML export
- Cardinality tracking with per-attribute cap (1,000), global budget (10K), Top-K with count-min sketch, TTL expiry (24h stale / 7d purge)
- Self-observability metrics (30+ metrics across 6 categories)
- Docker container image
- Helm chart for Kubernetes deployment
- CLI flags for configuration

### Growth (P1) — Post-MVP

- OTel standard semantic convention comparison (match / near-miss / custom classification)
- Dictionary diff and changelog between exports
- GitHub Actions integration for cardinality alerts and drift detection
- Attribute recommendations based on OTel standard conventions
- Snapshot and restore API endpoints
- Multi-service namespace isolation
- Sampling strategy for high-volume environments

### Vision (P2) — Future

- MCP server for AI agent queries ("What metrics does my service emit?")
- IDE plugin (VS Code) for local convention browsing and autocomplete
- Web dashboard for visual dictionary exploration
- Multi-collector / multi-cluster aggregation
- Enforcement mode (reject non-conforming telemetry)
- Cost attribution analysis ("user.email costs you $X/month in cardinality")

---

## User Journeys

### Journey 1: Platform Engineer — Convention Discovery

**Goal:** Discover what semantic conventions services use and export Weaver YAML

1. Engineer adds the proxy Helm chart to their dev Kubernetes cluster
2. Configures OTel Collectors to export to the proxy endpoint instead of directly to the backend
3. Proxy receives OTLP signals, analyzes them in the async pipeline, and builds the live dictionary
4. After running for several hours/days, engineer queries `GET /api/v1/dictionary` to browse discovered conventions
5. Engineer exports Weaver YAML via `GET /api/v1/export?format=weaver`
6. YAML is stored in the service repository under `semconv/` directory
7. CI/CD pipeline runs `weaver registry check` against the exported YAML for validation
8. Engineer reviews match/near-miss/custom classification to identify non-standard attributes
9. Standard-compliant conventions are enforced via `weaver registry generate` for type-safe code

**Success Moment:** Engineer exports a complete, validated semantic convention registry without manually writing a single YAML line.

### Journey 2: SRE — Cardinality Investigation

**Goal:** Identify and address cardinality explosions before they cause incidents

1. SRE notices rising backend costs or slow queries
2. Checks proxy self-observability dashboard (Prometheus metrics at `semconv.proxy.*`)
3. Identifies high-cardinality attributes via `semconv.proxy.cardinality.high_attributes` metric
4. Queries `GET /api/v1/dictionary?type=attribute&sort=cardinality&order=desc` for details
5. Reviews Top-K values for the offending attributes
6. Checks cardinality budget utilization: `semconv.proxy.cardinality.budget_utilization`
7. Takes action: either standardizes the attribute (reduces unique values) or sets TTL expiry
8. Monitors cardinality reduction over time via proxy metrics

**Success Moment:** SRE identifies the root cause of a cardinality explosion in under 5 minutes using proxy data.

### Journey 3: Application Developer — Dev-Time Convention Awareness

**Goal:** Understand what telemetry their service emits during development

1. Developer deploys the proxy in their local dev environment (Docker container)
2. Configures their OTel SDK to export to the proxy
3. Runs their application and exercises key code paths
4. Queries `GET /api/v1/dictionary?service=my-service` to see all emitted attributes
5. Reviews OTel standard comparison to find non-standard attributes
6. Fixes convention violations before merging code
7. Optionally exports Weaver YAML for their service to commit to the repo

**Success Moment:** Developer catches a `k8s.pod.name` vs `kubernetes.pod.name` drift in development, not production.

### Journey 4: Dev Advocate — Demo and Workshop

**Goal:** Demonstrate OTel best practices using real convention data

1. Deploys proxy alongside OTel demo application
2. Generates traffic through the demo
3. Exports discovered conventions and shows live dictionary in workshop
4. Demonstrates comparison against OTel standard
5. Shows CI/CD integration with Weaver for automated validation
6. Participants leave with a clear understanding of bottom-up convention discovery

**Success Moment:** Workshop participants can replicate the workflow in their own environments within 30 minutes.

---

## Domain Requirements

This project operates in the OpenTelemetry / Cloud-Native Observability domain. Key domain requirements:

### OpenTelemetry Compliance

- Must accept standard OTLP/HTTP and OTLP/gRPC protocols
- Must handle all OTel signal types: metrics (counter, gauge, histogram, summary, exponential histogram), traces, and logs
- Export format must be compatible with OTel Weaver semantic convention YAML schema
- Must respect OTel resource conventions for service identification
- Must handle both cumulative and delta temporality for metrics

### Telemetry Data Sensitivity

- The proxy processes observability data that may contain sensitive attributes (user IDs, IP addresses, etc.)
- The proxy must not persist attribute values — only attribute keys, types, and cardinality metadata
- The proxy must not log or expose actual telemetry payload data through its API
- All proxy API endpoints should be restricted to internal network access only

### Community and Open Source

- Apache 2.0 license
- Code must be idiomatic Go with comprehensive documentation
- API design should follow OTel community conventions for naming and structure
- Contribution guidelines and code of conduct required at launch

---

## Innovation Analysis

### Core Innovation: Bottom-Up Convention Discovery

Traditional OTel workflow is schema-first (define conventions → instrument → validate). This proxy inverts the flow: observe what exists → discover conventions → export schema. This reduces the barrier to OTel semantic convention adoption from "read the spec first" to "deploy and see what you have."

### Competitive Landscape

| Tool | Approach | Gap |
|------|----------|-----|
| OTel Weaver | Top-down schema validation and code generation | No discovery — requires manual YAML authoring |
| OTel Collector processors | Transform/filter in pipeline | No convention awareness, no export capability |
| Datadog/Grafana convention checks | Vendor-specific, proprietary | Not portable, not OTel-native |
| Custom scripts | Ad-hoc analysis | No live dictionary, no Weaver integration |

### Differentiation Strategy

- **Community-first:** Open source, OTel-native, not vendor-locked
- **Zero-config discovery:** No manual convention definition required to start
- **Weaver integration:** Direct export to the standard OTel tooling ecosystem
- **Developer workflow:** Dev → observe → export → repo → CI/CD pipeline
- **Not just linting — the data layer for telemetry conventions**

---

## Project-Type Requirements

### Infrastructure Component Requirements

This is an infrastructure/DevOps tool, not an end-user application. Specific requirements:

**Deployment:**
- Kubernetes deployment via Helm chart (primary)
- Standalone binary for VM/bare-metal (secondary)
- Docker container image published to GitHub Container Registry
- Configuration via CLI flags, environment variables, and optional config file

**Operability:**
- Health check endpoint (`/healthz`)
- Readiness endpoint (`/readyz`) — reports ready when dictionary is loaded
- Graceful shutdown with dictionary persistence
- Signal handling for configuration reload (SIGHUP)
- Structured logging (JSON format, zap or slog)

**Observability:**
- 30+ self-observability metrics under `semconv.proxy.*` namespace
- Prometheus `/metrics` endpoint
- Optional OTLP self-export of proxy metrics
- Metrics cover: signal throughput, dictionary operations, cardinality, analysis pipeline, storage, API/health

**Compatibility:**
- OTLP/HTTP (port 4318) and OTLP/gRPC (port 4317) receivers
- Configurable backend export endpoint
- Compatible with any OTel Collector version that supports OTLP export

---

## Functional Requirements

### OTLP Proxy

- FR1: The proxy can receive OTLP/HTTP signals on a configurable port
- FR2: The proxy can receive OTLP/gRPC signals on a configurable port
- FR3: The proxy forwards all received OTLP signals to the configured backend endpoint without modification
- FR4: The proxy continues forwarding signals even when the analysis pipeline is saturated or encountering errors
- FR5: The proxy reports forwarding status (success/failure) via self-observability metrics

### Auto-Discovery Engine

- FR6: The proxy can extract metric definitions (name, type, unit, temporality) from received OTLP metric signals
- FR7: The proxy can extract attribute keys, value types, and cardinality from metric attributes
- FR8: The proxy can extract span names, attribute keys, status codes, and parent-child relationships from trace signals
- FR9: The proxy can extract attribute keys, severity levels, and body field patterns from log signals
- FR10: The proxy can track first-seen and last-seen timestamps for each discovered entity
- FR11: The proxy can track the source signal type (metric/trace/log) for each discovered attribute
- FR12: The proxy can detect new, changed, and removed attributes with reason classification

### Live Dictionary

- FR13: The proxy maintains an in-memory dictionary of all discovered semantic conventions
- FR14: The dictionary is partitioned by signal type (metrics, traces, logs) and queryable via API
- FR15: The dictionary tracks per-attribute cardinality (unique value count) using approximate counting
- FR16: The dictionary tracks Top-K values for each attribute using count-min sketch
- FR17: The dictionary supports TTL-based entry expiry (configurable stale and purge intervals)
- FR18: The dictionary enforces a configurable per-attribute cardinality cap
- FR19: The dictionary enforces a configurable global attribute budget

### Persistence and Recovery

- FR20: The proxy persists the dictionary to disk asynchronously using Pebble storage engine
- FR21: The proxy can reload the full dictionary from Pebble on startup after a crash or restart
- FR22: The proxy persists dictionary snapshots periodically (configurable interval)
- FR23: The proxy reports persistence status and timing via self-observability metrics

### REST API

- FR24: The proxy exposes a REST API for querying the live dictionary
- FR25: The API supports querying the full dictionary with optional filters (signal type, attribute name pattern)
- FR26: The API supports querying individual attributes by name with cardinality details
- FR27: The API supports sorting dictionary entries by cardinality, first-seen, or last-seen timestamps
- FR28: The API supports exporting the complete dictionary in OTel Weaver semantic convention YAML format
- FR29: The API supports exporting filtered subsets of the dictionary (by signal type, service, or attribute prefix)
- FR30: The API provides health check and readiness endpoints
- FR31: The API returns standard HTTP status codes with structured error responses

### Weaver Integration

- FR32: The proxy exports semantic convention YAML compatible with OTel Weaver's `registry check` command
- FR33: The exported YAML includes metric definitions with name, type, unit, instrument, and attributes
- FR34: The exported YAML includes attribute definitions with type and requirement level
- FR35: The exported YAML includes span and log convention definitions
- FR36: The proxy classifies each discovered attribute as match, near-miss, or custom against the OTel standard registry (P1 — post-MVP, classification data stored from MVP for future use)

### Cardinality Management

- FR37: The proxy tracks per-attribute unique value counts with configurable eviction policies
- FR38: The proxy tracks global unique attribute count and exposes budget utilization percentage
- FR39: The proxy evicts stale entries based on configurable TTL (default: 24h stale, 7d purge)
- FR40: The proxy reports high-cardinality attributes (exceeding configurable threshold) via metrics and API
- FR41: The proxy reports Top-K values for high-cardinality attributes for diagnostic purposes

### Self-Observability

- FR42: The proxy exposes Prometheus-compatible metrics at a configurable endpoint
- FR43: The proxy reports signal throughput metrics (received, forwarded, dropped by signal type)
- FR44: The proxy reports dictionary operation metrics (entries gauge, attributes added/changed/removed)
- FR45: The proxy reports cardinality metrics (high attributes, top-K, budget utilization)
- FR46: The proxy reports analysis pipeline metrics (ring buffer lag, drops, processing duration)
- FR47: The proxy reports storage metrics (persist duration, disk size, snapshot status)
- FR48: The proxy reports API metrics (request rate, response time, error rate)
- FR49: The proxy can optionally self-export its metrics as OTLP signals

### Deployment

- FR50: The proxy is distributed as a Docker container image
- FR51: The proxy is distributed as a standalone Go binary for Linux (amd64, arm64)
- FR52: The proxy ships a Helm chart for Kubernetes deployment
- FR53: The proxy Helm chart supports configurable resource limits, service types, and ingress
- FR54: The proxy supports configuration via CLI flags, environment variables, and config file

---

## Non-Functional Requirements

### Performance

- NFR1: Proxy adds <1ms p99 latency to OTLP signal forwarding at 10,000 signals/second sustained throughput
- NFR2: Proxy supports burst throughput of 50,000 signals/second without data loss on the forwarding path
- NFR3: Dictionary reads (API queries) respond in <10ms at p95 for queries returning up to 1,000 entries
- NFR4: Weaver YAML export completes in <5 seconds for a dictionary with 10,000 attributes
- NFR5: Analysis pipeline (ring buffer + workers) processes signals without blocking the forwarding path
- NFR6: Pebble write-behind persistence does not impact forwarding latency (async, separate goroutine pool)
- NFR7: Memory usage remains under 512MB with 10,000 unique attributes under active observation

### Reliability

- NFR8: Zero telemetry data loss on the forwarding path — signals are forwarded even if analysis pipeline fails
- NFR9: Crash recovery completes in <5 seconds — dictionary fully loaded from Pebble and ready to serve
- NFR10: Proxy performs graceful shutdown on SIGTERM/SIGINT, persisting dictionary before exit
- NFR11: If the backend is unreachable, the proxy continues analyzing signals and serving API requests; forwarding retries with exponential backoff
- NFR12: Ring buffer overflow drops oldest analysis tasks, never blocks the forwarding path

### Scalability

- NFR13: Proxy handles 10,000 unique attributes within default memory budget (512MB)
- NFR14: Proxy handles up to 100,000 unique attributes with configurable memory budget increase
- NFR15: Cardinality management caps prevent unbounded memory growth regardless of attribute value diversity
- NFR16: Sharded dictionary (64 shards) allows concurrent reads without lock contention

### Security

- NFR17: Proxy API endpoints listen on a separate, internal-only port (not exposed to public internet)
- NFR18: Proxy does not persist, log, or expose actual telemetry attribute values — only keys, types, and cardinality metadata
- NFR19: Proxy supports optional TLS for both OTLP receiver and backend exporter connections
- NFR20: Proxy supports mTLS authentication for OTLP/gRPC connections
- NFR21: Helm chart deploys with restricted RBAC, non-root container, and read-only root filesystem

### Operability

- NFR22: Proxy starts and reports ready within 10 seconds on a cold start (no existing dictionary)
- NFR23: Proxy exposes structured JSON logs with configurable log level
- NFR24: Proxy exposes `/healthz` (liveness) and `/readyz` (readiness) endpoints for Kubernetes probes
- NFR25: Configuration reload via SIGHUP signal without restart
- NFR26: All configuration defaults support a "works out of the box" deployment with zero required configuration beyond backend endpoint

### Compatibility

- NFR27: OTLP/HTTP receiver compatible with OTel SDK and Collector exporters (OTLP HTTP protocol)
- NFR28: OTLP/gRPC receiver compatible with OTel SDK and Collector exporters (OTLP gRPC protocol)
- NFR29: Exported Weaver YAML passes `weaver registry check` validation with zero errors
- NFR30: Proxy binary supports Linux amd64 and arm64 architectures

---

## API Specification

### Endpoints

#### Signal Ingestion

```
POST /v1/metrics      (OTLP/HTTP metrics)
POST /v1/traces       (OTLP/HTTP traces)
POST /v1/logs         (OTLP/HTTP logs)
```
OTLP/gRPC on standard port 4317.

All ingestion endpoints forward signals to the backend and enqueue analysis tasks asynchronously.

#### Dictionary Queries

```
GET /api/v1/dictionary
  ?type=metric|trace|log   (filter by signal type)
  &q=<pattern>             (attribute name glob pattern)
  &sort=cardinality|first_seen|last_seen|name
  &order=asc|desc
  &limit=<n>               (max entries, default 100, max 1000)
  &offset=<n>              (pagination)

Response 200:
{
  "total": 342,
  "offset": 0,
  "limit": 100,
  "entries": [
    {
      "name": "http.request.method",
      "type": "string",
      "signal_types": ["metric", "trace"],
      "cardinality": 6,
      "first_seen": "2026-05-07T10:00:00Z",
      "last_seen": "2026-05-07T15:30:00Z",
      "status": "active",
      "classification": "match" | "near_miss" | "custom" | null
    }
  ]
}
```

```
GET /api/v1/dictionary/:attribute_name

Response 200:
{
  "name": "http.request.method",
  "type": "string",
  "signal_types": ["metric", "trace"],
  "cardinality": 6,
  "top_values": [
    {"value": "GET", "approximate_count": 45230},
    {"value": "POST", "approximate_count": 12340},
    ...
  ],
  "first_seen": "2026-05-07T10:00:00Z",
  "last_seen": "2026-05-07T15:30:00Z",
  "status": "active",
  "classification": "match",
  "ttl_expires_at": "2026-05-08T10:00:00Z"
}

Response 404:
{
  "error": "attribute not found",
  "attribute_name": "..."
}
```

#### Cardinality

```
GET /api/v1/cardinality
  ?threshold=<n>           (only attributes above this cardinality)
  &sort=count|name

Response 200:
{
  "global_budget": {"used": 342, "limit": 10000, "utilization_pct": 3.42},
  "attributes": [
    {
      "name": "k8s.pod.name",
      "cardinality": 847,
      "cap": 1000,
      "utilization_pct": 84.7,
      "top_values": [...]
    }
  ]
}
```

#### Weaver Export

```
GET /api/v1/export
  ?format=weaver           (required — only supported format in MVP)
  &type=metric|trace|log   (optional filter)
  &prefix=<string>         (optional attribute/metric name prefix filter)

Response 200 (Content-Type: text/yaml):
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
      semconv.proxy.first_seen: "2026-05-07T10:00:00Z"
      semconv.proxy.last_seen: "2026-05-07T15:30:00Z"
      semconv.proxy.cardinality: 6
```

#### Health and Readiness

```
GET /healthz    → 200 {"status": "alive"}
GET /readyz     → 200 {"status": "ready", "dictionary_entries": 342}
                   503 {"status": "loading", "dictionary_entries": 128}
GET /metrics    → Prometheus text format
```

### Self-Observability Metrics

All metrics under `semconv.proxy.*` namespace:

| Category | Metrics |
|----------|---------|
| Signal Throughput | `signals.received`, `signals.forwarded`, `signals.dropped` (by type, protocol) |
| Dictionary Operations | `dictionary.entries` (gauge), `dictionary.attributes_added`, `dictionary.attributes_changed`, `dictionary.attributes_removed` (with reason label) |
| Cardinality | `cardinality.high_attributes`, `cardinality.top_k_values`, `cardinality.budget_utilization` |
| Analysis Pipeline | `pipeline.ring_buffer_size`, `pipeline.lag`, `pipeline.drops`, `pipeline.processing_duration_seconds` |
| Storage | `storage.persist_duration_seconds`, `storage.disk_size_bytes`, `storage.snapshots_total` |
| API/Health | `api.request_total`, `api.request_duration_seconds`, `backend.connection_status`, `process.memory_rss` |

---

## Scope Boundaries — Explicitly Out of Scope for MVP

The following are explicitly excluded from the MVP:

1. **OTel standard comparison engine** — Classification data structure is present but comparison logic is P1
2. **GitHub Actions integration** — P1 after Weaver export is validated
3. **Web dashboard / UI** — P2, REST API is the only interface in MVP
4. **IDE plugin** — P2
5. **MCP server** — P2
6. **Multi-collector aggregation** — Single proxy instance only in MVP
7. **Enforcement mode** — Read-only discovery, no rejection of non-conforming telemetry
8. **Attribute value storage** — Only metadata (keys, types, cardinality), never actual values
9. **Authentication/Authorization** — Internal network deployment assumed for MVP
10. **Multi-tenancy** — Single global dictionary in MVP
11. **Built-in Prometheus scraper** — Collection handled by upstream OTel Collectors
12. **Sampling** — All signals analyzed in MVP, sampling is P1

---

## Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| Proxy latency on hot path | Critical | Async ring buffer + worker pool; zero-copy pass-through forwarding; <1ms p99 target with load testing |
| Cardinality explosion / unbounded memory | Critical | Layered defense: per-attr cap (1K), global budget (10K), Top-K with count-min sketch, TTL expiry (24h/7d) |
| Single point of failure | High | Proxy never drops forwarded signals; crash recovery from Pebble <5s; document deployment patterns with redundancy |
| Analysis pipeline saturation | Medium | Ring buffer overflow drops analysis tasks, never blocks forwarding; configurable buffer size and worker pool |
| Pebble storage corruption | Medium | Periodic snapshot mechanism; proxy can operate in memory-only mode if disk is unavailable |
| Competitive displacement by OTel native tooling | Medium | Ship fast, build community, integrate deeply with Weaver; open source reduces adoption barrier |
| Backward-incompatible OTel spec changes | Low | Pin OTel SDK version; Weaver format is stable; proxy data model is internal |
| Low community adoption | Low | Focus on developer experience; "works out of the box" defaults; comprehensive documentation; OTel community engagement |

---

## Appendix: Confirmed Architecture Decisions

From brainstorming phase ([ISI-906](/ISI/issues/ISI-906)):

- **Architecture:** Option A — standalone proxy between Collector and backend
- **Language:** Go (~2,000–3,000 LOC MVP)
- **Storage:** Two-tier — in-memory sharded dictionary (64 shards) + Pebble async write-behind
- **Analysis:** Async ring buffer + worker pool, zero-copy pass-through
- **Cardinality:** Per-attr cap, global budget, Top-K with count-min sketch, TTL expiry
- **Export:** OTel Weaver semantic convention YAML format
- **Deployment:** Docker image, Helm chart, standalone binary
- **License:** Apache 2.0
