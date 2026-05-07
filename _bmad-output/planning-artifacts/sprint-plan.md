# Sprint Plan — Collector Semantic Convention Proxy

**Generated:** 2026-05-07
**Source:** Epics & Stories (`_bmad-output/planning-artifacts/epics-and-stories.md`)
**Total:** 5 Sprints, 10 Epics, 36 Stories, ~4,300 LOC, ~21 developer-days

---

## Sprint Overview

| Sprint | Epics | Theme | Stories | Est. Days | Key Milestone |
|--------|-------|-------|---------|-----------|---------------|
| Sprint 1 | E1, E2 | Foundation + Core Data Path | 8 | 5 | Proxy receives and forwards OTLP signals with zero loss |
| Sprint 2 | E3, E4 | Analysis & Dictionary | 7 | 5 | Proxy extracts and stores semantic conventions in memory |
| Sprint 3 | E5, E6 | Persistence & API | 7 | 4 | Dictionary persisted to disk and queryable via REST API |
| Sprint 4 | E7, E8 | Export & Observability | 7 | 4 | Weaver YAML export and full self-observability metrics |
| Sprint 5 | E9, E10 | Lifecycle & Distribution | 7 | 3 | Production-ready with Docker, Helm, and lifecycle coordination |

---

## Dependency Graph

```
Sprint 1 (E1 → E2)
    │
    ▼
Sprint 2 (E3 → E4)
    │
    ├──────────────┐
    ▼              ▼
Sprint 3 (E5)   Sprint 3 (E6)
    │              │
    │              ▼
    │           Sprint 4 (E7 ← depends on E4 + E6)
    ▼
Sprint 4 (E8 ← instruments E2–E7)
    │
    ▼
Sprint 5 (E9 → E10)
```

---

## Sprint 1: Foundation + Core Data Path

**Duration:** 5 days
**Epics:** E1 (Project Foundation), E2 (OTLP Receiver & Forwarder)
**Goal:** Establish project skeleton and prove zero-loss OTLP signal forwarding.

### Prerequisites
- Go 1.22+ toolchain installed
- Access to `github.com/henrikrexed/semconv-proxy` repository
- OTel Collector SDK dependency resolved

### Stories

| Story | Title | Est. Effort | Depends On |
|-------|-------|-------------|------------|
| E1-S1 | Initialize Go module and project structure | 0.5 day | — |
| E1-S2 | Configuration layer with Cobra + Viper | 0.5 day | E1-S1 |
| E1-S3 | Structured logging with slog | 0.5 day | E1-S1 |
| E1-S4 | CI pipeline with GitHub Actions | 0.5 day | E1-S1 |
| E2-S1 | OTLP/HTTP receiver | 1 day | E1-S2 |
| E2-S2 | OTLP/gRPC receiver | 1 day | E1-S2 |
| E2-S3 | Backend forwarder with retry | 0.5 day | E2-S1, E2-S2 |
| E2-S4 | Zero-loss forwarding integration test | 0.5 day | E2-S3 |

### Execution Order

```
Day 1: E1-S1 (project structure) → E1-S3 (logging) + E1-S4 (CI)
Day 2: E1-S2 (config layer)
Day 3: E2-S1 (HTTP receiver) + E2-S2 (gRPC receiver)
Day 4: E2-S3 (forwarder with retry)
Day 5: E2-S4 (integration test) + buffer
```

### Acceptance Criteria
- [ ] `go build ./...` succeeds with no errors
- [ ] `make lint` passes with zero findings
- [ ] CI pipeline runs on PR (lint + test + build)
- [ ] All config values settable via CLI flag, env var, and config file
- [ ] Proxy accepts OTLP/HTTP and OTLP/gRPC signals from an OTel Collector
- [ ] All signals forwarded unmodified to backend
- [ ] Zero signal loss at 10K signals/sec sustained load
- [ ] Forwarding retry works on transient backend failures

### Deliverables
- Complete Go module with all project scaffolding
- Working OTLP proxy that receives and forwards all signal types
- CI pipeline running lint + test + build
- Integration test proving zero-loss forwarding

### Risk
- **OTel Collector SDK API changes:** Pin dependency version in `go.mod`
- **Performance target miss:** Profile early, adjust ring buffer capacity

---

## Sprint 2: Analysis & Dictionary

**Duration:** 5 days
**Epics:** E3 (Async Analysis Pipeline), E4 (In-Memory Sharded Dictionary)
**Goal:** Extract semantic conventions from signals and store them in a high-performance in-memory dictionary.

### Prerequisites
- Sprint 1 complete (receivers forwarding signals)
- Ring buffer design finalized (Architecture Decision D3)

### Stories

| Story | Title | Est. Effort | Depends On |
|-------|-------|-------------|------------|
| E3-S1 | Ring buffer implementation | 1 day | Sprint 1 (E2) |
| E3-S2 | Worker pool | 1 day | E3-S1 |
| E3-S3 | Signal attribute extractor | 1 day | E3-S2 |
| E4-S1 | Sharded dictionary core | 1.5 days | E3-S3 |
| E4-S2 | TTL-based entry expiry | 0.5 day | E4-S1 |
| E4-S3 | Cardinality tracking integration | 1 day | E4-S1 |
| E4-S4 | Change detection | 0.5 day | E4-S1 |

### Execution Order

```
Day 1: E3-S1 (ring buffer) — benchmark <100ns per write
Day 2: E3-S2 (worker pool)
Day 3: E3-S3 (signal attribute extractor) — extract from metrics, traces, logs
Day 4: E4-S1 (sharded dictionary core) — benchmark <1μs per read
Day 5: E4-S2 (TTL) + E4-S3 (cardinality) + E4-S4 (change detection)
```

### Acceptance Criteria
- [ ] Ring buffer accepts writes without blocking sender
- [ ] Ring buffer overflow drops oldest task (never blocks forwarding)
- [ ] Worker pool processes tasks concurrently with no goroutine leaks
- [ ] Extractor correctly identifies all attribute types from OTLP metric/trace/log signals
- [ ] Dictionary supports concurrent reads/writes without contention
- [ ] Dictionary read benchmark <1μs
- [ ] TTL sweeper marks stale entries after 24h and purges after 7d
- [ ] Cardinality tracked with HyperLogLog for high-count attributes
- [ ] Top-K values query returns results within 95% accuracy
- [ ] Global attribute budget enforced
- [ ] New/changed/removed attributes detected and reported via metrics

### Deliverables
- Async analysis pipeline (ring buffer → workers → extractor)
- In-memory sharded dictionary with full CRUD operations
- Cardinality tracking with HyperLogLog, count-min sketch, and Top-K
- TTL-based entry lifecycle management
- Benchmark tests for ring buffer and dictionary

### Risk
- **Ring buffer contention:** Benchmark early, adjust capacity and overflow policy
- **Dictionary memory under high cardinality:** Enforce global budget early (E4-S3)

---

## Sprint 3: Persistence & API

**Duration:** 4 days
**Epics:** E5 (Pebble Persistence), E6 (REST API)
**Goal:** Persist the dictionary for crash recovery and expose it via a REST API.

### Prerequisites
- Sprint 2 complete (dictionary with cardinality tracking)
- Pebble dependency added to `go.mod`

### Stories

| Story | Title | Est. Effort | Depends On |
|-------|-------|-------------|------------|
| E5-S1 | Pebble write-behind persister | 1 day | E4 (Sprint 2) |
| E5-S2 | Crash recovery from Pebble | 0.5 day | E5-S1 |
| E5-S3 | Graceful shutdown with persistence | 0.5 day | E5-S2 |
| E6-S1 | API server setup and middleware | 0.5 day | Sprint 2 (E4) |
| E6-S2 | Dictionary query endpoints | 1 day | E6-S1, E4 |
| E6-S3 | Cardinality endpoint | 0.5 day | E6-S1, E4-S3 |
| E6-S4 | Health and readiness endpoints | 0.5 day | E6-S1 |

### Execution Order

```
Day 1: E5-S1 (Pebble persister) — runs parallel with E6-S1
Day 2: E5-S2 (crash recovery) + E6-S1 (API server) + E6-S2 (dictionary endpoints, start)
Day 3: E6-S2 (dictionary endpoints, finish) + E6-S3 (cardinality endpoint)
Day 4: E5-S3 (graceful shutdown) + E6-S4 (health endpoints)
```

**Note:** E5 and E6 can be developed in parallel since both depend on E4 from Sprint 2 but not on each other.

### Acceptance Criteria
- [ ] Dictionary mutations persisted to Pebble within 100ms
- [ ] Persistence does not impact forwarding latency
- [ ] After crash, proxy recovers full dictionary in <5s for 10K entries
- [ ] Readiness probe returns 503 during dictionary loading, 200 after
- [ ] On SIGTERM, dictionary fully persisted before exit
- [ ] API server starts on configured internal port
- [ ] All API requests logged in structured format
- [ ] `GET /api/v1/dictionary` returns filtered, paginated results in <10ms p95
- [ ] `GET /api/v1/dictionary/:name` returns single attribute with cardinality details
- [ ] `GET /api/v1/cardinality` returns budget utilization and high-cardinality attributes
- [ ] `GET /healthz` returns 200 when alive
- [ ] `GET /readyz` returns 200 when dictionary loaded, 503 during startup

### Deliverables
- Pebble write-behind persistence layer with batch writes
- Crash recovery mechanism with <5s target
- Graceful shutdown with ordered component stop
- REST API with dictionary query, cardinality, and health endpoints
- API middleware (structured logging, panic recovery)

### Risk
- **Pebble write amplification:** Monitor batch sizes, adjust interval if needed
- **Recovery time exceeds 5s target:** Profile Pebble scan, consider parallel shard loading

---

## Sprint 4: Export & Observability

**Duration:** 4 days
**Epics:** E7 (Weaver YAML Export), E8 (Self-Observability Metrics)
**Goal:** Add Weaver YAML export capability and full self-observability metrics.

### Prerequisites
- Sprint 3 complete (REST API with dictionary endpoints)
- Sprint 2 complete (all components to instrument)
- OTel Weaver CLI installed for validation testing

### Stories

| Story | Title | Est. Effort | Depends On |
|-------|-------|-------------|------------|
| E7-S1 | Weaver YAML generator | 1.5 days | E4 (Sprint 2), E6 (Sprint 3) |
| E7-S2 | Export API endpoint | 0.5 day | E7-S1, E6-S1 |
| E8-S1 | Prometheus metrics registry and endpoint | 0.5 day | Sprint 1 (E1) |
| E8-S2 | Signal throughput metrics | 0.5 day | E2 (Sprint 1) |
| E8-S3 | Dictionary and pipeline metrics | 0.5 day | E3, E4 (Sprint 2) |
| E8-S4 | Storage and API metrics | 0.5 day | E5, E6 (Sprint 3) |
| E8-S5 | Optional OTLP self-export | 0.5 day | E8-S1 |

### Execution Order

```
Day 1: E7-S1 (Weaver YAML generator, start) + E8-S1 (metrics registry)
Day 2: E7-S1 (Weaver YAML generator, finish) + E8-S2 (signal throughput metrics)
Day 3: E7-S2 (export endpoint) + E8-S3 (dictionary/pipeline metrics)
Day 4: E8-S4 (storage/API metrics) + E8-S5 (OTLP self-export)
```

**Note:** E7 and E8 can be developed in parallel. E8 stories are incremental and can be spread across the sprint.

### Acceptance Criteria
- [ ] Generated Weaver YAML passes `weaver registry check` with zero errors
- [ ] YAML includes groups organized by signal type with all required annotations
- [ ] `GET /api/v1/export?format=weaver` returns valid YAML in <5s for 10K attributes
- [ ] Filtered exports by type and prefix work correctly
- [ ] `/metrics` endpoint returns Prometheus text format
- [ ] All metrics under `semconv_proxy_` prefix
- [ ] Signal throughput metrics count received/forwarded/dropped by type and protocol
- [ ] Dictionary mutation metrics track adds/changes/removes with reason labels
- [ ] Pipeline metrics track ring buffer size, lag, drops, processing duration
- [ ] Storage metrics track persist duration and disk size
- [ ] API metrics track request count and duration by path
- [ ] Optional OTLP self-export works when enabled

### Deliverables
- Weaver YAML generator with full semantic convention format
- Export API endpoint with filter support
- Complete Prometheus metrics suite (30+ metrics)
- Optional OTLP self-export for proxy metrics

### Risk
- **Weaver YAML schema compliance:** Test with `weaver registry check` early and often
- **Metric cardinality explosion:** Use labels judiciously, review before merge

---

## Sprint 5: Lifecycle & Distribution

**Duration:** 3 days
**Epics:** E9 (Lifecycle & Health Coordination), E10 (Docker, Helm & Distribution)
**Goal:** Production-ready lifecycle management and distribution packaging.

### Prerequisites
- Sprint 4 complete (all features implemented)
- Docker build environment available
- Helm 3 installed for chart testing

### Stories

| Story | Title | Est. Effort | Depends On |
|-------|-------|-------------|------------|
| E9-S1 | Lifecycle coordinator | 0.5 day | All E1–E8 |
| E9-S2 | Component health aggregator | 0.5 day | E9-S1 |
| E9-S3 | Configuration hot-reload via SIGHUP | 0.5 day | E1-S2, E9-S1 |
| E10-S1 | Docker image build | 0.5 day | All E1–E9 |
| E10-S2 | GoReleaser configuration | 0.5 day | E10-S1 |
| E10-S3 | Helm chart | 0.5 day | E10-S1 |
| E10-S4 | Makefile and developer tooling | 0.5 day | E10-S1 |

### Execution Order

```
Day 1: E9-S1 (lifecycle coordinator) + E9-S2 (health aggregator)
Day 2: E9-S3 (config hot-reload) + E10-S1 (Docker image)
Day 3: E10-S2 (GoReleaser) + E10-S3 (Helm chart) + E10-S4 (Makefile/tooling)
```

### Acceptance Criteria
- [ ] Components start in correct order: config → storage → dictionary → analysis → receiver/exporter → API → health
- [ ] Shutdown completes within 30s timeout with no goroutine leaks
- [ ] Health endpoints accurately reflect component states
- [ ] Component failure doesn't crash the proxy (degraded mode)
- [ ] SIGHUP reloads config; mutable values take effect immediately
- [ ] Immutable config changes (ports, backend endpoint) warn and require restart
- [ ] `docker build` succeeds for linux/amd64 and linux/arm64
- [ ] Docker image size <50MB, runs with non-root user
- [ ] `goreleaser release` produces binaries with checksums and SBOM
- [ ] `helm install` deploys proxy; health probes work
- [ ] All make targets (`build`, `test`, `lint`, `integration-test`, `docker-build`) work
- [ ] Docker Compose local dev environment works

### Deliverables
- Lifecycle coordinator with ordered startup/shutdown
- Component health aggregator driving Kubernetes probes
- SIGHUP configuration hot-reload
- Multi-arch Docker image (<50MB)
- GoReleaser configuration for automated releases
- Helm chart with full Kubernetes deployment
- Makefile with all development targets
- Docker Compose for local development

### Risk
- **Docker image size exceeds 50MB:** Use distroless base, strip debug info
- **Helm chart security hardening:** Review RBAC, security context before merge

---

## Effort Summary

### By Sprint

| Sprint | Days | Cumulative | % Complete |
|--------|------|------------|------------|
| Sprint 1 | 5 | 5 | 24% |
| Sprint 2 | 5 | 10 | 48% |
| Sprint 3 | 4 | 14 | 67% |
| Sprint 4 | 4 | 18 | 86% |
| Sprint 5 | 3 | 21 | 100% |

### By Epic

| Epic | Sprint | Stories | Est. LOC | Est. Days |
|------|--------|---------|----------|-----------|
| E1: Project Foundation | Sprint 1 | 4 | ~400 | 2 |
| E2: Receiver/Forwarder | Sprint 1 | 4 | ~600 | 3 |
| E3: Analysis Pipeline | Sprint 2 | 3 | ~500 | 2 |
| E4: Dictionary | Sprint 2 | 4 | ~600 | 3 |
| E5: Persistence | Sprint 3 | 3 | ~400 | 2 |
| E6: REST API | Sprint 3 | 4 | ~500 | 2 |
| E7: Weaver Export | Sprint 4 | 2 | ~400 | 2 |
| E8: Metrics | Sprint 4 | 5 | ~300 | 2 |
| E9: Lifecycle | Sprint 5 | 3 | ~300 | 1 |
| E10: Distribution | Sprint 5 | 4 | ~300 | 2 |
| **Total** | | **36** | **~4,300** | **21** |

---

## Sprint Transitions & Gates

Each sprint has a **completion gate** that must pass before the next sprint begins:

### Sprint 1 → Sprint 2 Gate
- Zero-loss forwarding integration test passes at 10K signals/sec
- CI pipeline green on main branch
- Config layer fully functional (flags + env + file)

### Sprint 2 → Sprint 3 Gate
- Ring buffer write benchmark <100ns
- Dictionary read benchmark <1μs
- Attribute extraction works for all three signal types (metrics, traces, logs)
- Cardinality tracking operational with Top-K

### Sprint 3 → Sprint 4 Gate
- Crash recovery completes in <5s for 10K entries
- REST API responds in <10ms p95 for dictionary queries
- Graceful shutdown persists full dictionary
- Health probes working correctly

### Sprint 4 → Sprint 5 Gate
- Weaver YAML passes `weaver registry check`
- All 30+ Prometheus metrics present on `/metrics`
- Export endpoint completes in <5s for 10K attributes

### Sprint 5 Completion Gate (Release Readiness)
- `docker build` succeeds for both architectures, image <50MB
- `helm install` deploys working proxy
- All make targets pass
- Full integration test suite passes (end-to-end)
- No goroutine leaks on shutdown

---

## Testing Strategy by Sprint

| Sprint | Unit Tests | Integration Tests | Benchmarks |
|--------|------------|-------------------|------------|
| Sprint 1 | Config, logging | Zero-loss forwarding (E2-S4) | — |
| Sprint 2 | Ring buffer, dictionary, extractor | — | Ring buffer write, dictionary read |
| Sprint 3 | Storage, API handlers | Crash recovery (E5-S2), API queries | Pebble batch write |
| Sprint 4 | Weaver generator, metrics | Weaver export validation | Export for 10K attrs |
| Sprint 5 | Lifecycle, hot-reload | Docker run, Helm deploy, full E2E | Cold start time |

---

## Risk Register

| Risk | Probability | Impact | Mitigation | Sprint |
|------|-------------|--------|------------|--------|
| OTel Collector SDK API instability | Low | High | Pin version in go.mod, lock to stable release | Sprint 1 |
| Ring buffer contention under load | Medium | High | Benchmark early in Sprint 2, adjust capacity | Sprint 2 |
| Pebble recovery exceeds 5s target | Low | Medium | Profile scan, consider parallel shard loading | Sprint 3 |
| Weaver YAML schema changes | Medium | Medium | Test with `weaver registry check` from Sprint 4 day 1 | Sprint 4 |
| Docker image size exceeds 50MB | Low | Low | Use distroless base, strip debug symbols | Sprint 5 |
| Forwarding latency exceeds 1ms p99 | Low | High | Profile hot path early, async by design mitigates | Sprint 1 |

---

## FR/NFR Coverage by Sprint

### Functional Requirements

| FR Range | Category | Sprint | Epic |
|----------|----------|--------|------|
| FR1–FR5 | OTLP Proxy | Sprint 1 | E2 |
| FR6–FR12 | Auto-Discovery | Sprint 2 | E3 |
| FR13–FR19 | Live Dictionary + Cardinality | Sprint 2 | E4 |
| FR20–FR23 | Persistence & Recovery | Sprint 3 | E5 |
| FR24–FR31 | REST API | Sprint 3 | E6 |
| FR32–FR36 | Weaver Integration | Sprint 4 | E7 |
| FR37–FR41 | Cardinality Management | Sprint 2–3 | E4, E6 |
| FR42–FR49 | Self-Observability | Sprint 4 | E8 |
| FR50–FR54 | Deployment | Sprint 1, 5 | E1, E10 |

### Non-Functional Requirements

| NFR Range | Category | Sprint | Covered By |
|-----------|----------|--------|------------|
| NFR1–NFR2 | Performance (latency/throughput) | Sprint 1 | E2 (zero-loss test) |
| NFR3 | Performance (API response) | Sprint 3 | E6 |
| NFR4 | Performance (export speed) | Sprint 4 | E7 |
| NFR5 | Performance (async processing) | Sprint 2 | E3 |
| NFR6 | Performance (persistence overhead) | Sprint 3 | E5 |
| NFR7 | Performance (memory budget) | Sprint 2 | E4 |
| NFR8, NFR11 | Reliability (zero loss, retry) | Sprint 1 | E2 |
| NFR9, NFR10 | Reliability (recovery, shutdown) | Sprint 3, 5 | E5, E9 |
| NFR12 | Reliability (no data blocking) | Sprint 2 | E3 |
| NFR13–NFR16 | Scalability | Sprint 2 | E4 |
| NFR17 | Security (internal API) | Sprint 3 | E6 |
| NFR18–NFR21 | Security (TLS, RBAC, etc.) | Sprint 5 | E10 |
| NFR22, NFR25 | Operability (start, reload) | Sprint 5 | E9 |
| NFR23 | Operability (structured logging) | Sprint 1 | E1 |
| NFR24 | Operability (health probes) | Sprint 3, 5 | E6, E9 |
| NFR26 | Operability (cold start) | Sprint 5 | E9 |
| NFR27–NFR28 | Compatibility (OTLP) | Sprint 1 | E2 |
| NFR29–NFR30 | Compatibility (Weaver, multi-arch) | Sprint 4, 5 | E7, E10 |

---

## Notes for Implementation Agents

1. **Follow dependency order strictly.** Each sprint builds on the previous one. Do not skip ahead.
2. **Write tests alongside implementation.** Table-driven tests for all exported functions. Benchmarks for hot-path code.
3. **Verify sprint gates before declaring a sprint complete.** Each gate has specific performance targets.
4. **Use the architecture document** (`_bmad-output/planning-artifacts/architecture.md`) as the authoritative reference for all design decisions.
5. **No new top-level packages** without updating the architecture document.
6. **All code under `internal/`** — no public Go API in MVP.
7. **Commit co-author:** Add `Co-Authored-By: Paperclip <noreply@paperclip.ing>` to all commits.
