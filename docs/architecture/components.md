# Components

SemConv Proxy is organized into distinct Go packages under `internal/`, each responsible for a single capability.

## Package Structure

```
internal/
├── config/         Configuration layer (Cobra + Viper)
├── receiver/       OTLP signal receivers (HTTP + gRPC)
├── exporter/       OTLP signal forwarder to backend
├── analysis/       Ring buffer, worker pool, attribute extractor
├── dictionary/     Sharded in-memory dictionary
├── storage/        Pebble write-behind persistence
├── cardinality/    Cardinality tracking (HLL, CMS, Top-K)
├── api/            REST API server and handlers
├── export/         Weaver YAML export logic
├── health/         Component health aggregation
├── metrics/        Prometheus self-observability metrics
├── lifecycle/      Graceful shutdown coordination
└── testutil/       Shared test utilities
```

## Receiver (`internal/receiver`)

Accepts OTLP signals from OpenTelemetry Collectors or SDKs.

| Protocol | Default Port | Endpoint |
|----------|-------------|----------|
| OTLP/HTTP | 4318 | `POST /v1/metrics`, `/v1/traces`, `/v1/logs` |
| OTLP/gRPC | 4317 | Standard gRPC OTLP export service |

On receiving a signal, the receiver:

1. Decodes the protobuf payload
2. Passes a reference to the forwarder (synchronous)
3. Wraps an `AnalysisTask` and enqueues it to the ring buffer (non-blocking)

The receiver uses the OTel Collector SDK receiver interfaces for protocol compliance.

## Forwarder (`internal/exporter`)

Sends OTLP signals to the configured backend endpoint.

- Supports both HTTP and gRPC backends
- Retry with exponential backoff on transient failures (max 3 retries)
- Reports forwarding status via `semconv_proxy_signals_forwarded_total` and `semconv_proxy_signals_dropped_total`
- Configurable insecure/TLS mode via `--backend-insecure`

## Ring Buffer (`internal/analysis`)

Fixed-capacity circular buffer that decouples signal reception from analysis processing.

```go
type AnalysisTask struct {
    SignalType SignalType
    Timestamp  time.Time
    Data       []byte
}
```

- Default capacity: 10,000 tasks
- Overflow policy: drop oldest — never blocks the forwarding path
- Write latency target: <100ns per write

## Worker Pool (`internal/analysis`)

Pool of goroutines that drain the ring buffer and extract attributes from signals.

- Default worker count: `runtime.NumCPU()`
- Each worker reads from a shared channel fed by the ring buffer
- Workers call the attribute extractor, then update the dictionary

## Attribute Extractor (`internal/analysis`)

Extracts semantic convention data from decoded OTLP signals:

| Signal | Extracted Fields |
|--------|-----------------|
| Metric | Name, type (counter, gauge, histogram, summary, exponential histogram), unit, temporality, all attribute keys and value types |
| Trace | Span name, all attribute keys, status code, parent span context |
| Log | All attribute keys, severity number/text, body field type |

## Sharded Dictionary (`internal/dictionary`)

In-memory attribute store partitioned into 64 shards using FNV-1a hashing.

```go
type AttributeEntry struct {
    Name           string
    Type           string
    SignalTypes    []SignalType
    FirstSeen      time.Time
    LastSeen       time.Time
    Status         EntryStatus    // active, stale, expired
    Cardinality    int64
    Classification *string        // match, near_miss, custom (future)
}
```

Each shard is a `map[string]*AttributeEntry` protected by `sync.RWMutex`. Reads are O(1) and non-blocking when the shard is not being written.

**Cardinality management:**

- Per-attribute cap: 1,000 unique values (configurable)
- Global budget: 10,000 unique attributes (configurable)
- TTL expiry: 24h stale / 7d purge (configurable)
- Budget exceeded: least-recently-used eviction

## Pebble Persistence (`internal/storage`)

Async write-behind persistence layer using the Pebble embedded storage engine.

- **Write channel**: dictionary mutations are enqueued to a buffered channel
- **Batching**: up to 1,000 entries per batch, flushed every 100ms
- **Key scheme**: `{signal_type}:{attribute_name}`
- **Serialization**: MessagePack (compact, fast)
- **Crash recovery**: on startup, the persister loads all entries from Pebble

The persister runs on a separate goroutine. Write failures are logged and retried. If Pebble is unavailable, the proxy operates in memory-only mode.

## Cardinality Tracker (`internal/cardinality`)

Tracks unique value counts and identifies high-cardinality attributes.

| Data Structure | Purpose |
|---------------|---------|
| Exact counting (`map[string]struct{}`) | Precise counting up to per-attribute cap |
| HyperLogLog | Approximate counting after cap exceeded |
| Count-Min Sketch | Frequency estimation for Top-K extraction |
| Min-Heap | Top-K value extraction |

API: `GET /api/v1/cardinality` returns budget utilization and attributes exceeding a configurable threshold.

## REST API (`internal/api`)

HTTP server on port 8080 serving dictionary queries, cardinality info, and export endpoints.

Endpoints:

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/dictionary` | List/filter/paginate dictionary entries |
| `GET /api/v1/dictionary/:name` | Single attribute with cardinality details |
| `GET /api/v1/cardinality` | Budget utilization and high-cardinality attributes |
| `GET /api/v1/export?format=weaver` | Weaver YAML export |
| `GET /healthz` | Liveness probe |
| `GET /readyz` | Readiness probe |
| `GET /metrics` | Prometheus metrics |

Middleware: structured request logging and panic recovery.

## Weaver Exporter (`internal/export`)

Generates OTel Weaver-compatible YAML from the live dictionary.

The exported YAML includes:

- Groups organized by signal type and metric name
- Attribute definitions with type and requirement level
- `semconv.proxy.*` annotations (first_seen, last_seen, cardinality, status)
- Stability field set to `experimental`

## Health Aggregator (`internal/health`)

Central health status collector for all proxy components.

Each component (receiver, dictionary, storage, API) reports its status to the aggregator. The readiness endpoint (`/readyz`) returns 200 only when all components report healthy.

## Lifecycle Coordinator (`internal/lifecycle`)

Manages ordered startup and graceful shutdown.

**Startup order:**

1. Config → Storage → Dictionary (with Pebble recovery)
2. Analysis workers
3. Receiver + Forwarder
4. API server
5. Health aggregator

**Shutdown order:**

1. Stop receiving new signals (close receivers)
2. Drain ring buffer (process remaining analysis tasks)
3. Flush Pebble writes (persist dictionary)
4. Stop API server
5. Exit (configurable timeout: 30s, force-exit after)

## Prometheus Metrics (`internal/metrics`)

30+ metrics under the `semconv_proxy_` namespace. See [Metrics](../api/metrics.md) for the full list.
