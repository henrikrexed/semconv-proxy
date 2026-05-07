# Epics & Stories — Collector Semantic Convention Proxy

**Generated:** 2026-05-07
**Source:** PRD (`_bmad-output/prd.md`) + Architecture (`_bmad-output/planning-artifacts/architecture.md`)
**Total:** 10 Epics, 40 Stories

---

## Epic 1: Project Foundation & Configuration

**Goal:** Initialize the Go module, wire configuration, and establish the project skeleton.

**Maps to:** FR50–FR54 (Deployment config), NFR22–NFR26 (Operability), Architecture Decision D8

### Stories

**E1-S1: Initialize Go module and project structure**
- Create `github.com/henrikrexed/semconv-proxy` module
- Set up directory structure per architecture doc: `cmd/`, `internal/config|receiver|exporter|analysis|dictionary|storage|cardinality|api|export|health|metrics|lifecycle|testutil/`, `deployments/`, `docs/`, `scripts/`, `tests/integration/`
- Add `go.mod`, `Makefile` (build/test/lint targets), `.gitignore`, `.golangci.yml`
- Add `LICENSE` (Apache 2.0), `README.md` (placeholder), `CONTRIBUTING.md`
- **FRs:** FR50, FR51
- **AC:** `go build ./...` succeeds, `make lint` passes with zero findings

**E1-S2: Configuration layer with Cobra + Viper**
- Define `Config` struct with all fields from architecture (ports, backend endpoint, shard count, ring buffer size, cardinality caps, TTL intervals, log level)
- CLI flags via Cobra (`--backend-endpoint`, `--otlp-http-port`, `--api-port`, `--log-level`, `--config`, etc.)
- Environment variable binding with `SEMCONV_PROXY_` prefix
- YAML config file support via Viper
- Priority order: CLI flags → env vars → config file → defaults
- **FRs:** FR54
- **AC:** All config values settable via flag, env var, and config file; defaults work with zero configuration beyond backend endpoint

**E1-S3: Structured logging with slog**
- Set up `log/slog` with JSON handler
- Configurable log level via `--log-level` flag
- Create logger factory that injects component name as default attribute
- **NFRs:** NFR23
- **AC:** All log output is structured JSON; level configurable at runtime via SIGHUP

**E1-S4: CI pipeline with GitHub Actions**
- `.github/workflows/ci.yaml`: lint (golangci-lint), test, build on push/PR
- Build for Linux amd64 + arm64
- Test with `-race` flag
- Code coverage report
- **AC:** CI runs on every PR; lint + test + build must pass

---

## Epic 2: OTLP Receiver & Forwarder (Core Data Path)

**Goal:** Receive OTLP signals and forward them to the backend with zero data loss. This is the critical hot path.

**Maps to:** FR1–FR5, NFR1, NFR2, NFR8, NFR11, NFR12, NFR27, NFR28

### Stories

**E2-S1: OTLP/HTTP receiver**
- Implement OTLP/HTTP receiver on configurable port (default 4318)
- Accept metrics, traces, and logs endpoints (`/v1/metrics`, `/v1/traces`, `/v1/logs`)
- Parse incoming protobuf payloads using `pdata`
- Forward raw bytes to backend; pass reference to analysis pipeline
- **FRs:** FR1, FR3, FR4
- **AC:** Receiver accepts OTLP/HTTP from OTel Collector exporter; all signals forwarded unmodified

**E2-S2: OTLP/gRPC receiver**
- Implement OTLP/gRPC receiver on configurable port (default 4317)
- Register OTLP service handlers for metrics, traces, logs
- Forward raw protobuf to backend; pass reference to analysis pipeline
- **FRs:** FR2, FR3, FR4
- **AC:** Receiver accepts OTLP/gRPC from OTel Collector exporter; all signals forwarded unmodified

**E2-S3: Backend forwarder with retry**
- Implement OTLP exporter that forwards to configured backend endpoint
- Exponential backoff retry (max 3 retries) on transient failures
- Track forwarding metrics: success/failure/dropped counts by signal type and protocol
- If backend unreachable, proxy continues receiving and analyzing; logs warning
- **FRs:** FR3, FR4, FR5
- **NFRs:** NFR8, NFR11
- **AC:** Zero signal loss on forwarding path at 10K signals/sec sustained; metrics track all outcomes

**E2-S4: Zero-loss forwarding integration test**
- End-to-end test: send 10K signals through proxy, verify all arrive at backend
- Test with proxy analysis pipeline saturated (ring buffer full) — forwarding still succeeds
- Test with backend temporarily unreachable — proxy retries and eventually delivers
- **NFRs:** NFR1, NFR2, NFR8
- **AC:** 100% of signals forwarded; <1ms p99 overhead at 10K signals/sec

---

## Epic 3: Async Analysis Pipeline

**Goal:** Build the ring buffer + worker pool that extracts semantic conventions from signals without blocking the forwarding path.

**Maps to:** FR6–FR12, NFR5, NFR12, Architecture Decisions D3, D7

### Stories

**E3-S1: Ring buffer implementation**
- Fixed-size ring buffer with configurable capacity (default 10,000 slots)
- Drop-oldest overflow policy — never block the forwarding goroutine
- `AnalysisTask` struct: signal type, timestamp, byte slice of serialized signal data
- Thread-safe write (single producer) and read (multiple consumers)
- Metric: `semconv_proxy_pipeline_ring_buffer_size` gauge
- **NFRs:** NFR5, NFR12
- **AC:** Ring buffer accepts writes without blocking sender; overflow drops oldest task; benchmark <100ns per write

**E3-S2: Worker pool**
- Configurable number of workers (default: `runtime.NumCPU()`)
- Workers read from ring buffer channel, extract attributes, update dictionary
- Graceful drain on shutdown via context cancellation
- Metrics: `semconv_proxy_pipeline_lag`, `semconv_proxy_pipeline_drops_total`, `semconv_proxy_pipeline_processing_duration_seconds`
- **NFRs:** NFR5
- **AC:** Workers process tasks concurrently; no goroutine leaks; processing duration metric emitted

**E3-S3: Signal attribute extractor**
- Extract from **metrics**: name, type (counter/gauge/histogram/summary/exponential), unit, temporality, attribute keys + value types
- Extract from **traces**: span names, attribute keys, status codes, parent-child relationships, resource attributes
- Extract from **logs**: attribute keys, severity levels, body field patterns, resource attributes
- Track first-seen/last-seen timestamps per discovered entity
- Classify source signal type for each attribute
- **FRs:** FR6–FR12
- **AC:** Extractor correctly identifies all attribute types from OTLP metric/trace/log signals; timestamps tracked

---

## Epic 4: In-Memory Sharded Dictionary

**Goal:** Build the core data structure that stores all discovered semantic conventions with O(1) reads.

**Maps to:** FR13–FR19, NFR3, NFR7, NFR13–NFR16, Architecture Decisions D1, D4

### Stories

**E4-S1: Sharded dictionary core**
- 64 shards (configurable), each a `map[string]*AttributeEntry` protected by `sync.RWMutex`
- FNV-1a hash for shard selection
- `AttributeEntry` struct: name, type, signal types, first_seen, last_seen, status (active/expired), classification field (null for MVP)
- Operations: Upsert (add or update), Get, Delete, List (with pagination)
- **FRs:** FR13, FR14
- **NFRs:** NFR16
- **AC:** Concurrent reads/writes without contention; benchmark <1μs per read

**E4-S2: TTL-based entry expiry**
- Background sweeper goroutine at configurable interval
- Two-tier TTL: stale (default 24h — marks inactive) and purge (default 7d — removes entry)
- Sweeper updates entry status and reports via metrics
- **FRs:** FR17
- **AC:** Entries not seen for 24h marked stale; entries not seen for 7d purged; sweeper doesn't block reads

**E4-S3: Cardinality tracking integration**
- Per-attribute unique value counter (exact up to cap, then HyperLogLog)
- Top-K values via count-min sketch + min-heap (top ~50 values)
- Global attribute budget (configurable, default 10K) with utilization tracking
- Per-attribute cardinality cap (configurable, default 1,000)
- **FRs:** FR15, FR16, FR18, FR19
- **AC:** Cardinality tracked for all attributes; global budget enforced; Top-K query returns correct results within 95% accuracy

**E4-S4: Change detection**
- Detect and classify: new attribute, attribute type changed, attribute removed
- Store change reason as label on dictionary operation metrics
- **FRs:** FR11, FR12
- **AC:** New/changed/removed attributes detected and reported via metrics with reason labels

---

## Epic 5: Pebble Persistence & Crash Recovery

**Goal:** Persist the dictionary asynchronously to Pebble and recover on restart.

**Maps to:** FR20–FR23, NFR6, NFR9, NFR10, Architecture Decision D2

### Stories

**E5-S1: Pebble write-behind persister**
- Background goroutine drains write channel, batches entries, writes to Pebble
- Key scheme: `{signal_type}:{attribute_name}`
- Value format: MessagePack-encoded `AttributeEntry`
- Batch size: 1,000 entries or 100ms interval
- Metrics: `semconv_proxy_storage_persist_duration_seconds`, `semconv_proxy_storage_disk_size_bytes`
- **FRs:** FR20, FR22
- **NFRs:** NFR6
- **AC:** Dictionary mutations persisted to Pebble within 100ms; persistence doesn't impact forwarding latency

**E5-S2: Crash recovery from Pebble**
- On startup, load all entries from Pebble into in-memory dictionary
- Report loading progress via readiness endpoint (`/readyz` returns 503 during load)
- Target: <5 seconds for dictionary with 10K entries
- **FRs:** FR21
- **NFRs:** NFR9
- **AC:** After crash, proxy recovers full dictionary in <5s; readiness probe reflects loading state

**E5-S3: Graceful shutdown with persistence**
- SIGTERM/SIGINT trigger ordered shutdown: stop receivers → drain ring buffer → flush Pebble → stop API → exit
- Configurable shutdown timeout (default 30s), force exit after timeout
- **NFRs:** NFR10
- **AC:** On SIGTERM, dictionary fully persisted before exit; no data loss on forwarding path

---

## Epic 6: REST API

**Goal:** Expose the dictionary, cardinality data, and health via a REST API.

**Maps to:** FR24–FR31, NFR3, NFR17, NFR24, Architecture Decisions D5, D6

### Stories

**E6-S1: API server setup and middleware**
- HTTP server using Go 1.22+ `net/http` with `ServeMux` enhanced routing
- Structured logging middleware (request ID, method, path, duration)
- Panic recovery middleware
- Separate internal-only port (default 8080)
- **FRs:** FR24
- **NFRs:** NFR17
- **AC:** Server starts on configured port; all requests logged in structured format; panics recovered

**E6-S2: Dictionary query endpoints**
- `GET /api/v1/dictionary` — full dictionary with filters (type, pattern, sort, order, limit, offset)
- `GET /api/v1/dictionary/:attribute_name` — single attribute with cardinality details and Top-K values
- Pagination support (limit/offset)
- Standard error responses (`{"error": "...", "code": "..."}`)
- **FRs:** FR25, FR26, FR27, FR28, FR31
- **NFRs:** NFR3
- **AC:** Queries respond in <10ms p95 for up to 1K entries; filters work correctly; 404 for missing attributes

**E6-S3: Cardinality endpoint**
- `GET /api/v1/cardinality` — global budget utilization + high-cardinality attributes
- Query params: threshold, sort
- Returns per-attribute cardinality, cap, utilization %, and Top-K values
- **FRs:** FR37, FR38, FR40, FR41
- **AC:** Returns accurate cardinality data; budget utilization percentage correct; high-cardinality attributes flagged

**E6-S4: Health and readiness endpoints**
- `GET /healthz` — liveness (200 if alive)
- `GET /readyz` — readiness (200 if dictionary loaded, 503 during loading)
- Component-level health aggregation
- **FRs:** FR30
- **NFRs:** NFR24
- **AC:** Kubernetes probes work correctly; readiness reflects dictionary loading state

---

## Epic 7: Weaver YAML Export

**Goal:** Export the live dictionary in OTel Weaver-compatible YAML format.

**Maps to:** FR28, FR29, FR32–FR36, NFR4, NFR29

### Stories

**E7-S1: Weaver YAML generator**
- Transform dictionary entries into OTel Weaver semantic convention YAML format
- Groups organized by signal type and metric name
- Include: group id, type, metric_name, brief, instrument, unit, attributes, stability, annotations
- Annotations: `semconv.proxy.status`, `semconv.proxy.first_seen`, `semconv.proxy.last_seen`, `semconv.proxy.cardinality`
- **FRs:** FR32, FR33, FR34, FR35
- **AC:** Generated YAML passes `weaver registry check` validation with zero errors

**E7-S2: Export API endpoint**
- `GET /api/v1/export?format=weaver` — full Weaver YAML export
- Optional filters: `type` (metric/trace/log), `prefix` (attribute/metric name prefix)
- Response content type: `text/yaml`
- **FRs:** FR28, FR29
- **NFRs:** NFR4
- **AC:** Export completes in <5s for 10K attributes; filtered exports work correctly; output is valid Weaver YAML

---

## Epic 8: Self-Observability Metrics

**Goal:** Expose 30+ Prometheus metrics for proxy monitoring.

**Maps to:** FR42–FR49, Architecture Decision D12

### Stories

**E8-S1: Prometheus metrics registry and endpoint**
- Set up Prometheus client registry with `semconv_proxy_` namespace
- `/metrics` endpoint on API server port
- Metric naming follows Prometheus conventions (underscores, `_total` suffix for counters, `_seconds` for histograms)
- **FRs:** FR42
- **AC:** `/metrics` returns Prometheus text format; all metrics under `semconv_proxy_` prefix

**E8-S2: Signal throughput metrics**
- Counters: `signals_received_total`, `signals_forwarded_total`, `signals_dropped_total`
- Labels: signal_type (metric/trace/log), protocol (http/grpc)
- **FRs:** FR43
- **AC:** Metrics accurately count received/forwarded/dropped signals by type and protocol

**E8-S3: Dictionary and pipeline metrics**
- Dictionary: `entries` gauge, `attributes_added_total`, `attributes_changed_total`, `attributes_removed_total` (with reason label)
- Pipeline: `ring_buffer_size` gauge, `lag` gauge, `drops_total`, `processing_duration_seconds` histogram
- **FRs:** FR44, FR46
- **AC:** All dictionary mutations and pipeline operations produce metrics

**E8-S4: Storage and API metrics**
- Storage: `persist_duration_seconds`, `disk_size_bytes`, `snapshots_total`
- API: `request_total` (by path, method, status), `request_duration_seconds` (by path)
- Cardinality: `high_attributes` gauge, `budget_utilization` gauge
- **FRs:** FR45, FR47, FR48
- **AC:** All storage operations, API requests, and cardinality states produce metrics

**E8-S5: Optional OTLP self-export**
- Configurable OTLP export of proxy metrics to the same backend
- Uses OTel Go SDK meter provider with OTLP exporter
- **FRs:** FR49
- **AC:** When enabled, proxy metrics appear in the configured backend as OTLP metrics

---

## Epic 9: Lifecycle & Health Coordination

**Goal:** Orchestrate startup, shutdown, health checks, and configuration reload.

**Maps to:** NFR10, NFR22, NFR24, NFR25, Architecture Decisions D9, D10

### Stories

**E9-S1: Lifecycle coordinator**
- Ordered component startup: config → storage (load dictionary) → dictionary → cardinality → analysis (ring buffer + workers) → receiver + exporter → API → health
- Ordered shutdown: receivers → drain ring buffer → flush storage → API → exit
- Context propagation for cancellation
- Configurable shutdown timeout (default 30s)
- **NFRs:** NFR10, NFR22
- **AC:** Components start in correct order; shutdown completes within timeout; no goroutine leaks

**E9-S2: Component health aggregator**
- Each component reports health status (ok/degraded/failed) to central aggregator
- Health aggregator drives `/healthz` and `/readyz` responses
- Failed component triggers degraded state (not crash)
- **NFRs:** NFR24
- **AC:** Health endpoints accurately reflect component states; component failure doesn't crash proxy

**E9-S3: Configuration hot-reload via SIGHUP**
- SIGHUP signal handler triggers Viper config re-read
- Mutable changes applied: log level, cardinality limits, TTL intervals
- Immutable changes logged as requiring restart: port bindings, backend endpoint
- All reloads logged at INFO level
- **NFRs:** NFR25
- **AC:** SIGHUP reloads config; mutable values take effect immediately; immutable values warn

---

## Epic 10: Docker, Helm Chart & Distribution

**Goal:** Package and distribute the proxy for Kubernetes and standalone deployment.

**Maps to:** FR50–FR54, NFR21, NFR30

### Stories

**E10-S1: Docker image build**
- Multi-stage Dockerfile: builder (Go build) → minimal runtime image (distroless or alpine)
- Multi-arch: linux/amd64, linux/arm64
- Docker Compose file for local development (proxy + OTel Collector + mock backend)
- **FRs:** FR50
- **AC:** `docker build` succeeds for both architectures; image size <50MB; container runs with non-root user

**E10-S2: GoReleaser configuration**
- `.goreleaser.yml` for automated releases
- Build targets: linux/amd64, linux/arm64
- Archive formats: tar.gz with binary + README + LICENSE
- SBOM generation
- **FRs:** FR51
- **NFRs:** NFR30
- **AC:** `goreleaser release` produces binaries for both architectures with checksums and SBOM

**E10-S3: Helm chart**
- Chart: `deployments/helm/semconv-proxy/`
- Templates: Deployment, Service, ConfigMap, ServiceAccount, Ingress (optional)
- Values: image config, resource limits, ports, backend endpoint, cardinality settings, TLS
- Health probes wired to `/healthz` and `/readyz`
- Restricted RBAC, non-root container, read-only root filesystem
- **FRs:** FR52, FR53
- **NFRs:** NFR21
- **AC:** `helm install` deploys proxy; health probes work; proxy receives and forwards OTLP signals

**E10-S4: Makefile and developer tooling**
- Targets: `build`, `test`, `lint`, `integration-test`, `docker-build`, `helm-package`, `coverage`
- Development convenience: `make run` (build + run with dev config)
- Pre-commit hooks or CI enforcement for `gofmt`, `goimports`, `go vet`
- **FRs:** FR54
- **AC:** All make targets work; CI pipeline uses make targets

---

## Epic Dependency Graph

```
E1 (Foundation)
├── E2 (Receiver/Forwarder) ← depends on E1 config
│   └── E3 (Analysis Pipeline) ← depends on E2 for signal data
│       └── E4 (Dictionary) ← depends on E3 for extracted attributes
│           ├── E5 (Persistence) ← depends on E4
│           ├── E6 (REST API) ← depends on E4
│           └── E7 (Weaver Export) ← depends on E4, E6
├── E8 (Metrics) ← instruments E2–E7
├── E9 (Lifecycle) ← coordinates E1–E8
└── E10 (Distribution) ← packages E1–E9
```

## Recommended Implementation Order

1. **E1** → Project foundation (must be first)
2. **E2** → Core data path (verify zero-loss forwarding ASAP)
3. **E3** → Analysis pipeline (adds observation without blocking data path)
4. **E4** → Dictionary (stores discoveries)
5. **E5** → Persistence (crash recovery)
6. **E6** → REST API (expose dictionary)
7. **E7** → Weaver export (killer feature)
8. **E8** → Metrics (instrument everything — can be done incrementally alongside E2–E7)
9. **E9** → Lifecycle coordination (polish)
10. **E10** → Distribution (package for deployment)

## Estimated Effort

| Epic | Stories | Est. LOC | Est. Time |
|------|---------|----------|-----------|
| E1: Foundation | 4 | ~400 | 2 days |
| E2: Receiver/Forwarder | 4 | ~600 | 3 days |
| E3: Analysis Pipeline | 3 | ~500 | 2 days |
| E4: Dictionary | 4 | ~600 | 3 days |
| E5: Persistence | 3 | ~400 | 2 days |
| E6: REST API | 4 | ~500 | 2 days |
| E7: Weaver Export | 2 | ~400 | 2 days |
| E8: Metrics | 5 | ~300 | 2 days |
| E9: Lifecycle | 3 | ~300 | 1 day |
| E10: Distribution | 4 | ~300 | 2 days |
| **Total** | **36** | **~4,300** | **~21 days** |

Note: LOC estimate is higher than original 2K–3K due to expanded PRD scope (54 FRs, 30 NFRs). The 3-week estimate aligns with the original brainstorming projection.

---

## FR Coverage Matrix

| FR | Epic(s) | Story(ies) |
|----|---------|------------|
| FR1–FR2 | E2 | E2-S1, E2-S2 |
| FR3–FR5 | E2 | E2-S1, E2-S2, E2-S3 |
| FR6–FR12 | E3 | E3-S3 |
| FR13–FR14 | E4 | E4-S1 |
| FR15–FR16, FR18–FR19 | E4 | E4-S3 |
| FR17 | E4 | E4-S2 |
| FR20, FR22 | E5 | E5-S1 |
| FR21 | E5 | E5-S2 |
| FR23 | E5 | E5-S1 |
| FR24–FR29, FR31 | E6 | E6-S1, E6-S2, E6-S3 |
| FR30 | E6 | E6-S4 |
| FR32–FR35 | E7 | E7-S1 |
| FR36 | Deferred (P1) | — |
| FR37–FR41 | E4, E6 | E4-S3, E6-S3 |
| FR42–FR49 | E8 | E8-S1–E8-S5 |
| FR50–FR54 | E1, E10 | E1-S1, E10-S1–E10-S4 |
