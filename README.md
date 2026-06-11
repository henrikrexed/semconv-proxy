# SemConv Proxy

[![CI](https://github.com/henrikrexed/semconv-proxy/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/henrikrexed/semconv-proxy/actions/workflows/ci.yaml)
[![Docs](https://github.com/henrikrexed/semconv-proxy/actions/workflows/docs.yaml/badge.svg?branch=main)](https://github.com/henrikrexed/semconv-proxy/actions/workflows/docs.yaml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**A transparent OTLP proxy that discovers, tracks, and exports OpenTelemetry semantic conventions in real time.**

SemConv Proxy sits between your OpenTelemetry Collectors and your observability backend, observing every signal that flows through it. It builds a live dictionary of every attribute your services actually emit — across metrics, traces, and logs — so platform teams can audit cardinality, detect convention drift, and export OTel Weaver-compatible YAML directly from production telemetry.

📖 **Documentation:** **<https://henrikrexed.github.io/semconv-proxy/>**

---

## Why SemConv Proxy?

OpenTelemetry semantic conventions are how teams agree on attribute names, types, and meaning across services. As OTel adoption scales, organizations hit three recurring problems:

1. **You don't know what you're emitting.** Conventions live in code; what hits the backend often doesn't match.
2. **Cardinality explodes silently.** A single rogue attribute (e.g. `user.id` on a metric) can blow up your bill.
3. **Conventions drift over time.** New attributes appear, types change, services diverge from the registry.

SemConv Proxy solves this by sitting transparently in the pipeline — `App → OTel Collector → SemConv Proxy → Backend` — and giving you:

- **Zero-loss forwarding** — sub-millisecond p99 overhead, backpressure-aware
- **Real-time discovery** — auto-catalog every attribute across metrics, traces, and logs
- **Cardinality intelligence** — HyperLogLog, Count-Min Sketch, and Top-K per attribute
- **Change detection** — track when attributes are added, modified, or removed
- **SemConv search UI** — browse and compare your live telemetry against the official OTel registry from an embedded web UI
- **Weaver export** — generate Weaver-compatible YAML straight from live data
- **Weaver Asset Builder** — scaffold a complete, validatable Weaver registry (manifest, attribute groups, `.rego` policy checks, `.weaver.toml`) from observed telemetry and export it as a zip ready to commit
- **Self-observability** — 30+ Prometheus metrics about the proxy itself

## How It Works

```mermaid
flowchart LR
    App["Your App"] --> Coll["OTel Collector"]
    Coll --> Proxy["SemConv Proxy"]
    Proxy --> Backend["Backend"]
    Proxy --> Surface["REST API +<br/>Weaver YAML +<br/>Prom metrics"]
```

Internally: OTLP receivers (HTTP/gRPC) → ring buffer → worker pool → 64-shard concurrent dictionary → Pebble persistence. See the [Architecture docs](https://henrikrexed.github.io/semconv-proxy/architecture/) for the full diagram and data flow.

## Getting Started

### Run with Docker

```bash
docker run -p 4317:4317 -p 4318:4318 -p 8080:8080 \
  ghcr.io/henrikrexed/semconv-proxy:latest \
  --backend-endpoint=otel-collector:4317
```

### Build from source

```bash
git clone https://github.com/henrikrexed/semconv-proxy
cd semconv-proxy
make build
./bin/semconv-proxy --backend-endpoint=localhost:4317
```

### Deploy on Kubernetes

From the published OCI chart on GHCR:

```bash
helm install semconv-proxy \
  oci://ghcr.io/henrikrexed/semconv-proxy-chart \
  --version 0.1.0 \
  --set config.backendEndpoint=otel-collector.observability.svc.cluster.local:4317
```

Or from the in-tree source:

```bash
helm install semconv-proxy ./deployments/helm/semconv-proxy-chart \
  --set config.backendEndpoint=otel-collector.observability.svc.cluster.local:4317
```

The chart artifact is published as `semconv-proxy-chart` (distinct from the
container image `semconv-proxy`) to avoid GHCR namespace collisions.

Once running, point your OTel Collectors at the proxy's OTLP endpoints (`:4317` gRPC, `:4318` HTTP), then open the web UI or query the dictionary:

```bash
open http://localhost:8080/                                  # SemConv Explorer + Weaver Asset Builder
curl http://localhost:8080/api/v1/dictionary                 # live attribute dictionary
curl "http://localhost:8080/api/v1/export?format=weaver" > my-conventions.yaml
```

➡️ Full guide: [Getting Started](https://henrikrexed.github.io/semconv-proxy/getting-started/)

## Web UI

The proxy serves an embedded, responsive single-page UI on the API port (`:8080`) — nothing extra to deploy. It has four scopes, switched from the header:

- **Community** — search the official OTel semantic-convention registry (build-time pinned snapshot).
- **My Telemetry** — search the attributes the proxy has actually observed on the wire.
- **Compare** — bucket your live telemetry against the registry (matched / type-mismatch / deprecated / not-in-registry).
- **Weaver Asset Builder** — author and export a custom Weaver registry from observed telemetry.

### Weaver Asset Builder

The Builder turns discovered telemetry into a complete, validatable [OTel Weaver](https://github.com/open-telemetry/weaver) registry you can commit to a repo. The workflow runs across three tabs with a live preview:

1. **Definitions** — an attribute table seeded from your live telemetry and cross-referenced against the official registry. Edit type, stability, requirement level, brief, and examples; group attributes under a namespace.
2. **Checks** — add policy checks from a parameterised catalog (naming-prefix, namespace allow-list, stability/requirement-level required, type consistency, deprecation-replacement), or hand-write raw Rego. Each emits a `policies/<name>.rego`.
3. **Config** — optionally emit a `.weaver.toml` wiring the registry, policy paths, and live-check finding filters.

The preview pane renders the generated file tree live (`registry_manifest.yaml`, `groups/`, `policies/`, `.weaver.toml`); download individual files or the whole registry as a zip whose contents pass `weaver registry check`.

➡️ Full walkthrough: [Weaver Asset Builder](https://henrikrexed.github.io/semconv-proxy/operations/weaver-builder/)

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--backend-endpoint` | `SEMCONV_PROXY_BACKEND_ENDPOINT` | *(required)* | OTLP backend endpoint |
| `--otlp-http-port` | `SEMCONV_PROXY_OTLP_HTTP_PORT` | `4318` | OTLP/HTTP listen port |
| `--otlp-grpc-port` | `SEMCONV_PROXY_OTLP_GRPC_PORT` | `4317` | OTLP/gRPC listen port |
| `--api-port` | `SEMCONV_PROXY_API_PORT` | `8080` | REST API listen port |
| `--log-level` | `SEMCONV_PROXY_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `--config` | — | — | Path to YAML config file |

Full reference (storage, ring buffer sizing, retention, TLS, etc.): [Configuration docs](https://henrikrexed.github.io/semconv-proxy/getting-started/configuration/).

## API at a Glance

All endpoints are served on the API port (`:8080`). See the [API reference](https://henrikrexed.github.io/semconv-proxy/api/) for full schemas.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/dictionary` | GET | List and filter discovered attributes |
| `/api/v1/dictionary/:name` | GET | Single attribute detail |
| `/api/v1/cardinality` | GET | Cardinality budget + high-cardinality attributes |
| `/api/v1/semconv/community` | GET | Search the official OTel registry snapshot |
| `/api/v1/semconv/compare` | GET | Bucket live telemetry vs. the registry |
| `/api/v1/export?format=weaver` | GET | One-shot Weaver YAML export of live data |
| `/api/v1/builder/seed` | GET | Seed the Builder definitions table |
| `/api/v1/builder/policy-templates` | GET | Policy-check catalog for the Builder |
| `/api/v1/builder/generate` | POST | Emit the Weaver registry file set from Builder state |
| `/api/v1/builder/export.zip` | POST | Bundle the generated registry as a downloadable zip |
| `/healthz`, `/readyz`, `/metrics` | GET | Probes and Prometheus metrics |

> **Designed, not yet shipped:** a server-side `/api/v1/builder/validate` and an in-proxy `weaver registry check` validation loop are designed but gated. Today, validate the exported zip with the Weaver CLI (`weaver registry check`).

## Use Cases

- **Semantic convention discovery** — what does this service actually emit?
- **Cardinality budget monitoring** — catch high-cardinality attributes before they hit your bill
- **Migration tracking** — verify a SemConv migration before/after deployment
- **Observability pipeline auditing** — confirm what reaches the backend
- **Multi-source aggregation** — unify conventions across teams and clusters

Each use case has a dedicated walkthrough with diagrams and code in the [Use Cases section](https://henrikrexed.github.io/semconv-proxy/use-cases/).

## Project Structure

```
.
├── cmd/                # Binary entrypoints
├── internal/           # Core proxy implementation (private)
├── docs/               # MkDocs documentation source
├── deployments/        # Docker, Helm chart, Kubernetes manifests
├── tests/              # Integration and load tests
├── mkdocs.yml          # Documentation site config
└── Makefile
```

## Development

```bash
make build          # Build the binary
make test           # Run unit tests
make lint           # Run golangci-lint
make docker         # Build Docker image
make docs-serve     # Serve docs locally on http://127.0.0.1:8000
```

Contribution guide: [development/contributing](https://henrikrexed.github.io/semconv-proxy/development/contributing/).

## Documentation

The full documentation site is published to GitHub Pages and rebuilt automatically on every push to `main` that touches `docs/` or `mkdocs.yml`.

- **Site:** <https://henrikrexed.github.io/semconv-proxy/>
- **Source:** [`docs/`](./docs/)
- **CI:** [`.github/workflows/docs.yaml`](./.github/workflows/docs.yaml)

To preview locally:

```bash
pip install -r docs/requirements.txt
mkdocs serve
```

## License

Licensed under the [Apache License 2.0](LICENSE).
