---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - _bmad-output/prd.md
  - _bmad-output/brainstorming-report.md
  - _bmad-output/party-mode-results.md
workflowType: 'architecture'
project_name: 'collector-semantic-convention'
user_name: 'Henrik.rexed'
date: '2026-05-07'
lastStep: 8
status: 'complete'
completedAt: '2026-05-07'
---

# Architecture Decision Document

_Collector Semantic Convention Proxy — comprehensive architecture for consistent AI agent implementation._

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**

The PRD defines 54 functional requirements across 8 capability areas:

| Category | FRs | Description |
|----------|-----|-------------|
| OTLP Proxy | FR1–FR5 | Receive and forward OTLP signals (HTTP + gRPC) |
| Auto-Discovery Engine | FR6–FR12 | Extract metric/span/log definitions from signals |
| Live Dictionary | FR13–FR19 | In-memory sharded dictionary with cardinality tracking |
| Persistence & Recovery | FR20–FR23 | Pebble-based async write-behind with crash recovery |
| REST API | FR24–FR31 | Dictionary queries, filtering, sorting, pagination |
| Weaver Integration | FR32–FR36 | Export in OTel Weaver semantic convention YAML format |
| Cardinality Management | FR37–FR41 | Per-attr cap, global budget, Top-K, TTL expiry |
| Self-Observability | FR42–FR49 | 30+ metrics across 6 categories |
| Deployment | FR50–FR54 | Docker, standalone binary, Helm chart, CLI flags |

**Non-Functional Requirements:**

30 NFRs covering 6 areas:

| Category | NFRs | Key Targets |
|----------|------|-------------|
| Performance | NFR1–NFR7 | <1ms p99 overhead, 50K burst, 512MB memory |
| Reliability | NFR8–NFR12 | Zero data loss on forwarding, <5s crash recovery |
| Scalability | NFR13–NFR16 | 10K–100K attributes, sharded concurrent reads |
| Security | NFR17–NFR21 | Internal-only API, no value storage, optional TLS/mTLS |
| Operability | NFR22–NFR26 | 10s cold start, health/readiness probes, SIGHUP reload |
| Compatibility | NFR27–NFR30 | OTLP HTTP/gRPC, Weaver YAML validation, Linux amd64/arm64 |

**Scale & Complexity:**

- Primary domain: Infrastructure / Observability tooling
- Complexity level: Medium
- Estimated architectural components: 10 distinct packages/modules
- Estimated LOC: 2,000–3,000 (MVP)
- No UI in MVP — API-only product

### Technical Constraints & Dependencies

- **Go language:** Required for OTel ecosystem compatibility and performance
- **OTLP protocol:** Must support both HTTP (port 4318) and gRPC (port 4317)
- **Pebble storage engine:** Confirmed over BadgerDB for lower memory/CPU overhead
- **OTel Weaver YAML format:** Export format must pass `weaver registry check`
- **Apache 2.0 license:** Open source, community-driven
- **No external dependencies for MVP:** Single binary, embedded storage
- **Any OTel backend:** Dynatrace, Grafana, Jaeger, New Relic, etc.

### Cross-Cutting Concerns Identified

1. **Zero data loss guarantee** — Forwarding path must never be blocked by analysis or storage
2. **Async everywhere** — Analysis pipeline, persistence, API all async relative to hot path
3. **Cardinality management** — Layered defense across entire data flow
4. **Self-observability** — Every component must instrument itself
5. **Configuration consistency** — CLI flags, env vars, config file must all use same names
6. **Graceful lifecycle** — Startup, shutdown, crash recovery must all be handled cleanly

---

## Starter Template Evaluation

### Primary Technology Domain

Go backend / infrastructure component — no frontend, no database server, no web framework. This is a network proxy with embedded storage.

### Starter Approach: Go Module Scaffold

No heavyweight starter template needed. The project is a Go module with a clear structure. We use the standard Go project layout with opinionated choices:

**Selected Approach: Go Module with Standard Layout**

**Rationale:**
- Go has a well-established project layout for CLI/server tools
- No ORM, no web framework needed — keep dependencies minimal
- OTel Go SDK provides the protocol layer
- Standard library + minimal dependencies aligns with community expectations

**Initialization Command:**

```bash
mkdir semconv-proxy && cd semconv-proxy
go mod init github.com/henrikrexed/semconv-proxy
```

**Key Dependencies (verified current versions):**

| Dependency | Purpose | Import Path |
|------------|---------|-------------|
| OTel Go SDK | OTLP receiver/exporter | `go.opentelemetry.io/collector/...` |
| Pebble | Embedded storage | `github.com/cockroachdb/pebble` |
| gRPC | OTLP gRPC protocol | `google.golang.org/grpc` |
| Chi or net/http | REST API router | Standard library preferred |
| Prometheus client | Self-observability metrics | `github.com/prometheus/client_golang` |
| Zap or slog | Structured logging | `go.uber.org/zap` or `log/slog` |
| Count-min sketch | Cardinality tracking | `github.com/seiflotfy/count-min-sketch` or custom |
| YAML | Weaver export | `gopkg.in/yaml.v3` |
| Cobra | CLI flags and commands | `github.com/spf13/cobra` |
| Viper | Config file parsing | `github.com/spf13/viper` |

**Note:** Prefer standard library `log/slog` over Zap to reduce dependencies. Prefer `net/http` standard library router for MVP simplicity — can upgrade to Chi if middleware needs grow.

---

## Core Architectural Decisions

### Decision Priority Analysis

**Already Decided (from brainstorming):**

- Language: Go
- Architecture pattern: Standalone OTLP proxy
- Storage engine: Pebble
- Dictionary: In-memory sharded (64 shards)
- Analysis: Async ring buffer + worker pool
- Deployment: Docker + Helm + standalone binary
- License: Apache 2.0

**Critical Decisions (Block Implementation):**

1. Go module structure and package boundaries
2. Data flow architecture (signal → analysis → dictionary → storage)
3. Ring buffer design and sizing
4. Dictionary sharding strategy and key scheme
5. Pebble schema and write-behind pattern
6. Configuration management approach
7. Error handling and propagation strategy

**Important Decisions (Shape Architecture):**

8. REST API framework choice
9. Structured logging library choice
10. Metrics exposition approach
11. Graceful shutdown coordination
12. Health check design

**Deferred Decisions (Post-MVP):**

13. OTel standard comparison engine architecture (P1)
14. GitHub Actions integration design (P1)
15. MCP server protocol (P2)
16. Web dashboard (P2)
17. Enforcement mode (P2)

### Data Architecture

**Decision D1: Two-Tier Dictionary Storage**

- **Hot tier:** In-memory sharded `map[string]*AttributeEntry` with 64 shards, each protected by `sync.RWMutex`
- **Cold tier:** Pebble LSM-tree for async write-behind persistence
- **Rationale:** Hot tier provides O(1) reads for API queries. Cold tier provides crash recovery without blocking the hot path.
- **Sharding key:** FNV-1a hash of attribute name, modulo 64 — ensures even distribution

**Decision D2: Pebble Write-Behind Pattern**

- **Pattern:** Background goroutine drains a write channel, batches entries, writes to Pebble
- **Batch size:** 1,000 entries or 100ms interval, whichever comes first
- **Key scheme:** `{signal_type}:{attribute_name}` (e.g., `metric:http.request.method`)
- **Value format:** MessagePack-encoded `AttributeEntry` struct (compact, fast serialization)
- **Rationale:** Batching reduces write amplification. MessagePack over JSON for 30-50% size reduction.

**Decision D3: Ring Buffer Design**

- **Capacity:** Configurable, default 10,000 analysis tasks
- **Entry type:** `AnalysisTask` struct with signal type, timestamp, and a byte slice of the serialized signal data
- **Overflow policy:** Drop oldest — never block the forwarding goroutine
- **Consumer:** Worker pool of configurable goroutines (default: runtime.NumCPU())
- **Rationale:** Fixed-size ring buffer provides bounded memory usage. Drop-oldest guarantees forwarding is never blocked.

**Decision D4: Cardinality Data Structures**

- **Per-attribute cardinality:** Exact counting via `map[string]struct{}` up to per-attr cap (1,000), then approximate via HyperLogLog
- **Top-K values:** Count-min sketch with conservative update, min-heap for top-K extraction
- **Global budget:** Atomic counter with configurable limit (default 10K)
- **TTL tracking:** Each entry has `lastSeen` timestamp; background sweeper runs at configurable interval

### API & Communication Patterns

**Decision D5: REST API Framework**

- **Choice:** Go standard library `net/http` with `http.ServeMux` (Go 1.22+ enhanced routing)
- **Rationale:** MVP has <10 endpoints. Standard library suffices and avoids dependency bloat. Can upgrade to Chi router if middleware needs grow in P1.
- **Fallback:** If Go version <1.22, use `github.com/go-chi/chi/v5`

**Decision D6: API Response Format**

- **Success:** Direct JSON body with appropriate HTTP status code (200, 201, 204)
- **Error:** `{"error": "<human-readable message>", "code": "<machine-readable code>"}`
- **Lists:** `{"total": N, "offset": N, "limit": N, "entries": [...]}`
- **Rationale:** Simple, predictable, no envelope wrapping needed for internal API

**Decision D7: OTLP Signal Handling**

- **Receiver:** Use OTel Collector receiver interfaces (`go.opentelemetry.io/collector/receiver`)
- **Forwarding:** Use OTel Collector exporter interfaces (`go.opentelemetry.io/collector/exporter`)
- **Zero-copy:** Pass `pdata` resource/signal references to analysis pipeline without cloning
- **Rationale:** Using OTel Collector SDK components ensures protocol compliance and reduces custom parsing code

### Infrastructure & Deployment

**Decision D8: Configuration Management**

- **Priority order (highest wins):** CLI flags → Environment variables → Config file → Defaults
- **Library:** Cobra (CLI) + Viper (config binding)
- **Config file format:** YAML (aligned with OTel Collector convention)
- **Reload:** SIGHUP triggers Viper config re-read; changes applied to mutable components only (cardinality limits, TTL, log level)
- **Rationale:** Matches OTel Collector configuration patterns for familiarity

**Decision D9: Graceful Shutdown**

- **Signal handling:** SIGTERM/SIGINT trigger graceful shutdown sequence
- **Shutdown order:**
  1. Stop accepting new OTLP signals (close receivers)
  2. Drain ring buffer (process remaining analysis tasks)
  3. Flush Pebble writes (persist dictionary)
  4. Stop API server
  5. Exit
- **Timeout:** Configurable, default 30 seconds. Force exit after timeout.
- **Rationale:** Ensures no data loss on the forwarding path. Dictionary persistence is best-effort (crash recovery handles incomplete writes).

**Decision D10: Health Check Design**

- **Liveness (`/healthz`):** Returns 200 if process is alive and not deadlocked
- **Readiness (`/readyz`):** Returns 200 if dictionary is loaded and receivers are started; 503 during startup/crash recovery
- **Components health:** Each component (receiver, analyzer, persister, API) reports health to a central health aggregator
- **Rationale:** Kubernetes-compatible probes for Helm deployment

**Decision D11: Structured Logging**

- **Choice:** Go standard library `log/slog` (Go 1.21+)
- **Format:** JSON structured logs
- **Levels:** DEBUG, INFO, WARN, ERROR
- **Default level:** INFO (configurable via CLI flag `--log-level`)
- **Rationale:** Standard library, no external dependency, Go community best practice since 1.21

**Decision D12: Metrics Exposition**

- **Endpoint:** `/metrics` in Prometheus text exposition format
- **Library:** `github.com/prometheus/client_golang/prometheus`
- **Namespace:** All metrics prefixed with `semconv_proxy_` (underscores for Prometheus convention)
- **Optional self-export:** Configurable OTLP export of proxy metrics to the same backend
- **Rationale:** Prometheus is the de facto standard for Kubernetes metric collection

### Decision Impact Analysis

**Implementation Sequence:**

1. Go module init + dependency setup
2. Configuration layer (Cobra + Viper)
3. OTLP receiver + forwarder (core data path)
4. Ring buffer + analysis worker pool
5. In-memory sharded dictionary
6. Pebble persistence layer
7. Cardinality tracking (count-min sketch, HyperLogLog, Top-K)
8. REST API endpoints
9. Weaver YAML export
10. Self-observability metrics
11. Health checks + graceful shutdown
12. Docker image + Helm chart

**Cross-Component Dependencies:**

- Receiver depends on: Configuration, Ring buffer
- Ring buffer depends on: Analysis workers
- Analysis workers depend on: Dictionary, Cardinality tracker
- Dictionary depends on: Pebble persister
- API depends on: Dictionary, Cardinality tracker, Weaver exporter
- Health depends on: All components
- Metrics depend on: All components

---

## Implementation Patterns & Consistency Rules

### Naming Patterns

**Go Package Naming:**

- All lowercase, single word or short compound: `receiver`, `exporter`, `dictionary`, `analysis`, `storage`, `api`, `cardinality`, `config`, `health`, `metrics`
- No underscores or camelCase in package names
- Internal packages under `internal/` to prevent external imports

**Go File Naming:**

- `snake_case.go`: `ring_buffer.go`, `count_min_sketch.go`, `pebble_store.go`
- Test files: `ring_buffer_test.go`, `count_min_sketch_test.go`
- One primary type per file (with helpers)

**Go Type/Function Naming:**

- Exported types: PascalCase — `Dictionary`, `AnalysisTask`, `AttributeEntry`
- Unexported types: camelCase — `shard`, `workerPool`
- Interface names: PascalCase with `-er` suffix where applicable — `Persister`, `Analyzer`
- Constants: PascalCase (exported) or camelCase (unexported) — `DefaultShardCount`, `defaultRingBufferSize`
- Config struct: `{Component}Config` — `DictionaryConfig`, `AnalysisConfig`

**API Naming (HTTP):**

- Endpoints: `lowercase` with hyphens for multi-word — `/api/v1/dictionary`, `/api/v1/export`
- Query parameters: `snake_case` — `?signal_type=metric`, `?sort=cardinality`
- JSON field names: `snake_case` — `{"signal_types": [...], "first_seen": "..."}`

**Metric Naming (Prometheus):**

- Prefix: `semconv_proxy_`
- Subsystem pattern: `semconv_proxy_{subsystem}_{metric_name}`
- Units suffix: `_seconds`, `_bytes`, `_total`
- Examples: `semconv_proxy_signals_received_total`, `semconv_proxy_dictionary_entries`, `semconv_proxy_pipeline_processing_duration_seconds`

**Environment Variable Naming:**

- Prefix: `SEMCONV_PROXY_`
- Uppercase, underscore-separated: `SEMCONV_PROXY_LOG_LEVEL`, `SEMCONV_PROXY_BACKEND_ENDPOINT`
- Nested config: `SEMCONV_PROXY_DICTIONARY_SHARD_COUNT`

**CLI Flag Naming:**

- Lowercase, hyphen-separated: `--log-level`, `--backend-endpoint`, `--shard-count`
- Short flags for common options: `-l` (log-level), `-c` (config)

### Structure Patterns

**Project Organization:**

```
semconv-proxy/
├── cmd/
│   └── semconv-proxy/      # Main entry point
│       └── main.go
├── internal/
│   ├── config/             # Configuration (Cobra + Viper)
│   ├── receiver/           # OTLP signal receiver
│   ├── exporter/           # OTLP signal forwarder
│   ├── analysis/           # Ring buffer + worker pool
│   ├── dictionary/         # Sharded in-memory dictionary
│   ├── storage/            # Pebble persistence layer
│   ├── cardinality/        # Cardinality tracking (Top-K, CMS, HLL)
│   ├── api/                # REST API handlers
│   ├── export/             # Weaver YAML export
│   ├── health/             # Health check aggregator
│   ├── metrics/            # Self-observability metrics
│   └── lifecycle/          # Graceful shutdown coordinator
├── pkg/                    # (empty for MVP — no public API)
├── deployments/
│   ├── docker/             # Dockerfile + docker-compose
│   └── helm/               # Helm chart
├── docs/                   # Documentation
├── scripts/                # Build, test, release scripts
├── .goreleaser.yml         # GoReleaser config
├── Dockerfile
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

**Test Organization:**

- Unit tests: Co-located with source — `internal/dictionary/dictionary_test.go`
- Integration tests: `tests/integration/`
- Test utilities: `internal/testutil/`
- Build tag: `//go:build integration` for integration tests

### Format Patterns

**API Response Formats:**

```json
{
  "total": 342,
  "offset": 0,
  "limit": 100,
  "entries": [...]
}
```

**Error Response Format:**

```json
{
  "error": "attribute not found",
  "code": "NOT_FOUND"
}
```

**Date/Time Format:**

- All timestamps: RFC 3339 — `"2026-05-07T10:00:00Z"`
- Stored internally as `time.Time`

**Weaver YAML Export Format:**

- Follows OTel Weaver semantic convention schema exactly
- Groups organized by signal type and metric name
- Annotations include `semconv.proxy.*` metadata

### Communication Patterns

**Internal Communication:**

- Components communicate via Go channels (typed, buffered)
- No event bus or message queue — direct channel passing
- Component interfaces defined in each package, implementations are unexported

**Channel Types:**

- `OTLP signals → Ring buffer`: `chan *AnalysisTask` (buffered, drops oldest)
- `Analysis results → Dictionary`: Direct method calls (synchronous within worker goroutine)
- `Dictionary mutations → Persister`: `chan *PersistEntry` (buffered, batch writes)
- `Health status → Aggregator`: `chan ComponentHealth` (unbuffered, non-blocking send)

**Error Propagation:**

- Components return `error` from constructors (`New*` functions)
- Runtime errors: logged via `slog.Error` + increment error metric
- Fatal errors: returned to `main()` → graceful shutdown
- No panic/recover in production code

### Process Patterns

**Error Handling:**

- All errors wrapped with `fmt.Errorf("component: operation: %w", err)`
- Errors logged once at the point of handling (not at every level)
- Metrics incremented for every error path
- Forwarding errors: retry with exponential backoff (max 3 retries, then drop + metric)

**Concurrency Patterns:**

- One goroutine per component, managed by `main()` lifecycle coordinator
- Worker pool pattern for analysis: N goroutines reading from ring buffer channel
- Context cancellation for shutdown propagation
- `sync.WaitGroup` for graceful drain of worker pools
- `sync/atomic` for counters (signal counts, dictionary size)

**Configuration Hot-Reload:**

- SIGHUP → Viper re-reads config file
- Changes applied to: log level, cardinality limits, TTL intervals
- Changes NOT applied: port bindings, backend endpoint (requires restart)
- Changes logged at INFO level

### Enforcement Guidelines

**All AI Agents MUST:**

1. Follow the package structure defined above — no new top-level packages without updating this document
2. Use `slog` for all logging — never `fmt.Println` or `log` package
3. Use `context.Context` as the first parameter in all functions that can be cancelled
4. Return `error` as the last return value — no panic in library code
5. Use `internal/` for all implementation packages — no public API packages in MVP
6. Follow Prometheus metric naming conventions under `semconv_proxy_` namespace
7. Use RFC 3339 for all timestamp serialization
8. Use `snake_case` for JSON API fields
9. Write table-driven tests for all exported functions
10. Include a benchmark test for hot-path functions (ring buffer, dictionary reads)

**Pattern Enforcement:**

- `golangci-lint` with project config enforced in CI
- `go vet` + `staticcheck` in Makefile
- Test coverage threshold: 80% for MVP
- `gofmt` + `goimports` enforced via pre-commit hook or CI

### Pattern Examples

**Good Example — Creating a Component:**

```go
package dictionary

type Dictionary struct {
    shards []*shard
    config *Config
    logger *slog.Logger
}

type Config struct {
    ShardCount int
    GlobalBudget int
    PerAttrCap int
}

func New(cfg *Config, logger *slog.Logger) (*Dictionary, error) {
    if cfg.ShardCount <= 0 {
        return nil, fmt.Errorf("dictionary: shard count must be positive, got %d", cfg.ShardCount)
    }
    d := &Dictionary{config: cfg, logger: logger}
    d.shards = make([]*shard, cfg.ShardCount)
    for i := range d.shards {
        d.shards[i] = newShard(cfg.PerAttrCap)
    }
    return d, nil
}
```

**Anti-Pattern — Avoid:**

```go
// WRONG: No context, no error wrapping, mixed naming
func getAttr(name string) *AttrEntry {
    log.Println("getting " + name) // should use slog
    return globalDict[name] // no sharding, no concurrency safety
}
```

---

## Project Structure & Boundaries

### Complete Project Directory Structure

```
semconv-proxy/
├── cmd/
│   └── semconv-proxy/
│       └── main.go                    # Entry point: config, wiring, lifecycle
├── internal/
│   ├── config/
│   │   ├── config.go                  # Config struct + Viper binding
│   │   └── config_test.go
│   ├── receiver/
│   │   ├── otlp.go                    # OTLP HTTP + gRPC receiver
│   │   └── otlp_test.go
│   ├── exporter/
│   │   ├── forwarder.go               # OTLP signal forwarding to backend
│   │   └── forwarder_test.go
│   ├── analysis/
│   │   ├── ring_buffer.go             # Ring buffer implementation
│   │   ├── worker_pool.go             # Worker pool for analysis
│   │   ├── extractor.go               # Signal attribute extraction
│   │   ├── ring_buffer_test.go
│   │   ├── worker_pool_test.go
│   │   └── extractor_test.go
│   ├── dictionary/
│   │   ├── dictionary.go              # Sharded dictionary interface
│   │   ├── shard.go                   # Single shard (map + RWMutex)
│   │   ├── entry.go                   # AttributeEntry struct
│   │   ├── dictionary_test.go
│   │   └── shard_test.go
│   ├── storage/
│   │   ├── pebble.go                  # Pebble write-behind persister
│   │   ├── schema.go                  # Key/value encoding
│   │   ├── pebble_test.go
│   │   └── schema_test.go
│   ├── cardinality/
│   │   ├── tracker.go                 # Cardinality tracking coordinator
│   │   ├── count_min_sketch.go        # CMS implementation
│   │   ├── hyperloglog.go             # HLL for approximate counting
│   │   ├── topk.go                    # Top-K extraction
│   │   ├── tracker_test.go
│   │   ├── count_min_sketch_test.go
│   │   ├── hyperloglog_test.go
│   │   └── topk_test.go
│   ├── api/
│   │   ├── server.go                  # HTTP server setup
│   │   ├── handlers.go                # API endpoint handlers
│   │   ├── middleware.go              # Logging, recovery middleware
│   │   ├── server_test.go
│   │   └── handlers_test.go
│   ├── export/
│   │   ├── weaver.go                  # Weaver YAML export logic
│   │   ├── weaver_test.go
│   │   └── testdata/                  # Example Weaver YAML fixtures
│   │       └── expected_export.yaml
│   ├── health/
│   │   ├── aggregator.go              # Component health aggregation
│   │   └── aggregator_test.go
│   ├── metrics/
│   │   ├── metrics.go                 # Metric definitions (prometheus)
│   │   └── metrics_test.go
│   ├── lifecycle/
│   │   ├── coordinator.go             # Graceful shutdown coordination
│   │   └── coordinator_test.go
│   └── testutil/
│       └── helpers.go                 # Shared test utilities
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── docker-compose.yaml
│   └── helm/
│       └── semconv-proxy/
│           ├── Chart.yaml
│           ├── values.yaml
│           ├── templates/
│           │   ├── deployment.yaml
│           │   ├── service.yaml
│           │   ├── configmap.yaml
│           │   ├── ingress.yaml
│           │   └── _helpers.tpl
│           └── .helmignore
├── docs/
│   ├── architecture.md                # This document
│   ├── configuration.md               # Config reference
│   └── deployment.md                  # Deployment guide
├── scripts/
│   ├── build.sh
│   ├── test.sh
│   └── release.sh
├── tests/
│   └── integration/
│       ├── proxy_test.go              # End-to-end proxy test
│       ├── dictionary_test.go         # Dictionary lifecycle test
│       └── export_test.go             # Weaver export integration test
├── .github/
│   └── workflows/
│       └── ci.yaml                    # CI: lint, test, build
├── .gitignore
├── .golangci.yml                      # Linter config
├── .goreleaser.yml                    # Release automation
├── Dockerfile
├── Makefile                           # Build, test, lint targets
├── go.mod
├── go.sum
├── LICENSE                            # Apache 2.0
├── README.md
└── CONTRIBUTING.md
```

### Architectural Boundaries

**API Boundaries:**

| Boundary | Protocol | Port | Direction |
|----------|----------|------|-----------|
| OTLP/HTTP receiver | HTTP/Protobuf | 4318 (configurable) | Inbound from Collectors |
| OTLP/gRPC receiver | gRPC/Protobuf | 4317 (configurable) | Inbound from Collectors |
| REST API | HTTP/JSON | 8080 (configurable) | Inbound internal |
| Prometheus metrics | HTTP/Text | 8080 `/metrics` | Inbound internal |
| Health probes | HTTP/JSON | 8080 `/healthz`, `/readyz` | Inbound Kubernetes |
| OTLP backend export | HTTP or gRPC | Configurable | Outbound to backend |

**Component Boundaries:**

```
┌─────────────────────────────────────────────────────────────────┐
│                         main.go                                  │
│                    (lifecycle coordinator)                        │
├──────────┬──────────┬──────────┬──────────┬──────────┬──────────┤
│ receiver │ analysis │dictionary│ storage  │   api    │ metrics  │
│          │          │          │          │          │          │
│ OTLP     │ ring     │ sharded  │ Pebble   │ REST     │ Prometheus
│ HTTP     │ buffer   │ map      │ write-   │ handlers │ metrics  │
│ gRPC     │ workers  │ 64 shards│ behind   │ export   │ 30+      │
│          │ extractor│          │          │ Weaver   │ counters │
├──────────┴──────────┴──────────┴──────────┴──────────┴──────────┤
│                      config (Cobra + Viper)                      │
└─────────────────────────────────────────────────────────────────┘
```

**Data Flow Boundaries:**

```
Collector ──OTLP──▶ Receiver ──forward──▶ Exporter ──OTLP──▶ Backend
                         │
                         ▼ (async, zero-copy ref)
                    Ring Buffer
                         │
                         ▼ (worker pool)
                    Analyzer (extract attributes)
                         │
                         ▼
                    Dictionary (sharded map)
                    │           │
                    ▼           ▼
              API queries   Pebble persist
                    │
                    ▼
              Weaver export
```

### Requirements to Structure Mapping

**FR Category → Package Mapping:**

| FR Category | Primary Package | Supporting Packages |
|-------------|----------------|---------------------|
| OTLP Proxy (FR1–FR5) | `internal/receiver`, `internal/exporter` | `internal/config` |
| Auto-Discovery (FR6–FR12) | `internal/analysis` | `internal/receiver` |
| Live Dictionary (FR13–FR19) | `internal/dictionary` | `internal/analysis` |
| Persistence (FR20–FR23) | `internal/storage` | `internal/dictionary` |
| REST API (FR24–FR31) | `internal/api` | `internal/dictionary`, `internal/cardinality` |
| Weaver Integration (FR32–FR36) | `internal/export` | `internal/dictionary` |
| Cardinality (FR37–FR41) | `internal/cardinality` | `internal/dictionary` |
| Self-Observability (FR42–FR49) | `internal/metrics` | All packages |
| Deployment (FR50–FR54) | `deployments/`, `Dockerfile`, `Makefile` | `cmd/` |

**Cross-Cutting Concerns:**

| Concern | Primary Package | Used By |
|---------|----------------|---------|
| Configuration | `internal/config` | All packages |
| Health checks | `internal/health` | All packages |
| Graceful shutdown | `internal/lifecycle` | All packages |
| Structured logging | `log/slog` | All packages |
| Error handling | All packages | Consistent pattern |

### Integration Points

**Internal Communication:**

- Components wired in `main.go` via dependency injection (constructor injection, not framework)
- No service locator or DI framework — explicit `New()` function calls
- Each component accepts interfaces, returns concrete types

**External Integrations:**

- OTel Collector: OTLP protocol (HTTP + gRPC)
- Backend: OTLP protocol (HTTP or gRPC, configurable)
- OTel Weaver: YAML file output (filesystem write)
- Prometheus: Standard `/metrics` text endpoint
- Kubernetes: HTTP health probes

**Data Flow:**

1. Collector sends OTLP to Receiver
2. Receiver forwards to Exporter (pass-through)
3. Receiver enqueues AnalysisTask to Ring Buffer (async)
4. Worker drains Ring Buffer → calls Analyzer
5. Analyzer extracts attributes → updates Dictionary
6. Dictionary mutations → enqueued to Persist channel
7. Persister batches → writes to Pebble
8. API reads from Dictionary (concurrent read access)
9. Export reads from Dictionary → generates Weaver YAML

### File Organization Patterns

**Configuration Files:**

- Root: `go.mod`, `go.sum`, `Makefile`, `Dockerfile`, `.goreleaser.yml`, `.golangci.yml`
- Config reference: `docs/configuration.md`
- CI: `.github/workflows/ci.yaml`
- Helm: `deployments/helm/semconv-proxy/`

**Source Organization:**

- All implementation code under `internal/` — no public Go API in MVP
- Each package is self-contained with its own tests
- Test data in `testdata/` subdirectory within each package

**Test Organization:**

- Unit tests: Co-located with source (e.g., `internal/dictionary/dictionary_test.go`)
- Integration tests: `tests/integration/`
- Benchmarks: Co-located, `BenchmarkXxx` functions
- Test fixtures: `testdata/` directories within each package

### Development Workflow Integration

**Development Server:**

```bash
go run ./cmd/semconv-proxy --backend-endpoint=localhost:4317 --log-level=debug
```

**Build Process:**

```bash
make build          # Build binary
make test           # Run unit tests
make lint           # Run golangci-lint
make docker         # Build Docker image
make helm           # Package Helm chart
```

**Deployment:**

```bash
helm install semconv-proxy deployments/helm/semconv-proxy/ \
  --set config.backendEndpoint=otel-collector:4317
```

---

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**

- Go + Pebble + OTel Collector SDK: All compatible, well-maintained Go libraries
- Standard library HTTP server + Prometheus client: No conflicts
- Cobra + Viper: Standard Go CLI config pair
- Ring buffer + sharded dictionary: Complementary concurrency patterns

**Pattern Consistency:**

- All components use same error wrapping pattern (`fmt.Errorf`)
- All components use same logging pattern (`slog`)
- All components use same config pattern (Viper binding)
- All metric names follow same namespace convention

**Structure Alignment:**

- Package structure supports all 8 FR categories
- Each FR category maps to exactly one primary package
- Cross-cutting concerns have dedicated packages
- No circular dependencies between packages

### Requirements Coverage Validation ✅

**Functional Requirements Coverage:**

- OTLP Proxy (FR1–FR5): ✅ Covered by `receiver/` + `exporter/`
- Auto-Discovery (FR6–FR12): ✅ Covered by `analysis/`
- Live Dictionary (FR13–FR19): ✅ Covered by `dictionary/`
- Persistence (FR20–FR23): ✅ Covered by `storage/`
- REST API (FR24–FR31): ✅ Covered by `api/`
- Weaver Integration (FR32–FR36): ✅ Covered by `export/`
- Cardinality (FR37–FR41): ✅ Covered by `cardinality/`
- Self-Observability (FR42–FR49): ✅ Covered by `metrics/`
- Deployment (FR50–FR54): ✅ Covered by `deployments/` + `Dockerfile` + `Makefile`

**Non-Functional Requirements Coverage:**

- Performance (NFR1–NFR7): ✅ Async ring buffer, sharded dictionary, Pebble write-behind
- Reliability (NFR8–NFR12): ✅ Zero-loss forwarding, crash recovery, graceful shutdown
- Scalability (NFR13–NFR16): ✅ 64 shards, configurable memory budget, cardinality caps
- Security (NFR17–NFR21): ✅ Internal API port, no value storage, optional TLS/mTLS
- Operability (NFR22–NFR26): ✅ Health probes, structured logging, SIGHUP reload
- Compatibility (NFR27–NFR30): ✅ OTLP HTTP/gRPC, Weaver YAML, multi-arch builds

### Implementation Readiness Validation ✅

**Decision Completeness:**

- All 12 core decisions documented with rationale
- Technology versions verified
- Dependency list complete
- No unresolved "TBD" items

**Structure Completeness:**

- Complete project tree with all files and directories
- All FR categories mapped to packages
- All integration points specified
- All boundaries defined

**Pattern Completeness:**

- Naming patterns: Go packages, files, types, API, metrics, env vars, CLI flags
- Structure patterns: Project org, test org
- Format patterns: API responses, errors, dates, Weaver YAML
- Communication patterns: Channels, error propagation
- Process patterns: Error handling, concurrency, config reload

### Gap Analysis Results

**No Critical Gaps Found.**

**Important Notes:**

1. **OTel standard comparison (FR36)**: Data structure present for classification field, but comparison engine is P1. Classification field in `AttributeEntry` will default to `null` in MVP.
2. **TLS/mTLS (NFR19–NFR20)**: Configuration fields defined, implementation can be deferred if internal-only deployment is confirmed.
3. **OTLP self-export (FR49)**: Optional feature — may use OTel Collector SDK's own exporter capabilities rather than custom implementation.

**Nice-to-Have Enhancements (Post-MVP):**

- `.editorconfig` for consistent IDE formatting
- `CONTRIBUTING.md` with development setup guide
- Benchmark CI target to track performance regression
- `doc.go` files for package documentation

### Architecture Completeness Checklist

**✅ Requirements Analysis**

- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed
- [x] Technical constraints identified
- [x] Cross-cutting concerns mapped

**✅ Architectural Decisions**

- [x] Critical decisions documented with rationale (12 decisions)
- [x] Technology stack fully specified (Go + dependencies)
- [x] Integration patterns defined (channels, OTLP)
- [x] Performance considerations addressed (async, sharding, zero-copy)

**✅ Implementation Patterns**

- [x] Naming conventions established (7 categories)
- [x] Structure patterns defined (project org, test org)
- [x] Communication patterns specified (channels, errors)
- [x] Process patterns documented (concurrency, config, lifecycle)

**✅ Project Structure**

- [x] Complete directory structure defined (all files)
- [x] Component boundaries established (6 boundary types)
- [x] Integration points mapped (6 external points)
- [x] Requirements to structure mapping complete (9 FR categories)

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High

**Key Strengths:**

- PRD is exceptionally detailed — all requirements are testable
- Brainstorming phase resolved the hard architecture questions early
- Go ecosystem has mature libraries for all required components
- Clear separation of concerns between packages
- Async-by-design ensures the critical <1ms p99 latency target

**Areas for Future Enhancement:**

- P1: OTel standard comparison engine (classification field ready)
- P1: GitHub Actions integration for CI/CD cardinality alerts
- P2: MCP server for AI agent queries
- P2: Web dashboard for visual dictionary exploration

### Implementation Handoff

**AI Agent Guidelines:**

1. Follow all architectural decisions exactly as documented — do not substitute libraries or patterns
2. Use implementation patterns consistently across all components
3. Respect project structure and package boundaries
4. Refer to this document for all architectural questions
5. Start with the data path (receiver → forwarder) and verify zero-loss forwarding before adding analysis
6. Write tests alongside implementation — table-driven tests for all exported functions
7. Benchmark hot-path code (ring buffer write, dictionary read) to verify <1ms p99 target

**First Implementation Priority:**

1. Initialize Go module: `go mod init github.com/henrikrexed/semconv-proxy`
2. Implement `internal/config/` — configuration layer
3. Implement `internal/receiver/` + `internal/exporter/` — OTLP data path
4. Verify zero-loss forwarding with integration test
5. Implement remaining components in dependency order (see Decision Impact Analysis)

**Source PRD:** `_bmad-output/prd.md`
**Source Brainstorming:** `_bmad-output/brainstorming-report.md`
**Source Party Mode:** `_bmad-output/party-mode-results.md`
