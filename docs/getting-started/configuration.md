# Configuration

SemConv Proxy supports three configuration methods, listed in priority order (highest wins):

1. **CLI flags** — override everything
2. **Environment variables** — prefixed with `SEMCONV_PROXY_`
3. **YAML config file** — set via `--config`
4. **Defaults** — sensible values for all settings

## Quick Reference

| Setting | CLI Flag | Env Variable | Default |
|---------|----------|-------------|---------|
| Backend endpoint | `--backend-endpoint` | `SEMCONV_PROXY_BACKEND_ENDPOINT` | *(required)* |
| OTLP/HTTP port | `--otlp-http-port` | `SEMCONV_PROXY_OTLP_HTTP_PORT` | `4318` |
| OTLP/gRPC port | `--otlp-grpc-port` | `SEMCONV_PROXY_OTLP_GRPC_PORT` | `4317` |
| REST API port | `--api-port` | `SEMCONV_PROXY_API_PORT` | `8080` |
| Log level | `--log-level` / `-l` | `SEMCONV_PROXY_LOG_LEVEL` | `info` |
| Data directory | `--data-dir` | `SEMCONV_PROXY_DATA_DIR` | `./data` |
| Ring buffer size | `--ring-buffer-size` | — | `10000` |
| Worker count | `--worker-count` | — | *(NumCPU)* |
| Shard count | `--shard-count` | — | `64` |
| Global attribute budget | `--global-budget` | — | `10000` |
| Per-attribute cap | `--per-attr-cap` | — | `1000` |
| Backend insecure | `--backend-insecure` | — | `true` |

## Required Settings

The only required setting is `--backend-endpoint` — the `host:port` of your OTLP backend (or another OTel Collector).

```bash
semconv-proxy --backend-endpoint=otel-collector.observability:4317
```

## Config File

Create a YAML config file:

```yaml
backend-endpoint: otel-collector.observability:4317
log-level: debug
api-port: 8080
otlp-http-port: 4318
otlp-grpc-port: 4317
data-dir: /data
ring-buffer-size: 20000
worker-count: 8
shard-count: 64
global-budget: 10000
per-attr-cap: 1000
```

Point to it:

```bash
semconv-proxy --config=/etc/semconv-proxy/config.yaml
```

## Environment Variables

All settings are available as environment variables with the `SEMCONV_PROXY_` prefix:

```bash
export SEMCONV_PROXY_BACKEND_ENDPOINT=otel-collector:4317
export SEMCONV_PROXY_LOG_LEVEL=debug
export SEMCONV_PROXY_OTLP_HTTP_PORT=4318
semconv-proxy
```

## Dictionary Tuning

### Shard Count

The dictionary is partitioned into shards (default: 64) using FNV-1a hashing. Each shard has its own `sync.RWMutex`, allowing concurrent reads without contention.

- Increase to 128 for high-concurrency workloads (>50K signals/sec)
- Decrease to 32 for low-throughput environments to reduce memory overhead

### Global Attribute Budget

The maximum number of unique attribute keys the dictionary tracks across all signal types. When the budget is exceeded, the least-recently-used attributes are evicted.

```bash
semconv-proxy --global-budget=50000 ...
```

### Per-Attribute Cardinality Cap

Maximum number of unique values tracked per individual attribute. Attributes exceeding this cap switch from exact counting to approximate HyperLogLog estimation.

```bash
semconv-proxy --per-attr-cap=2000 ...
```

## Analysis Pipeline

### Ring Buffer Size

The ring buffer sits between the forwarding path and the analysis workers. It decouples signal ingestion from analysis processing.

- Default: 10,000 tasks
- If the buffer fills, oldest analysis tasks are dropped (forwarding is never blocked)
- Monitor with `semconv_proxy_pipeline_lag` and `semconv_proxy_pipeline_drops_total` metrics

```bash
semconv-proxy --ring-buffer-size=50000 ...
```

### Worker Count

Number of goroutines processing analysis tasks from the ring buffer.

- Default: matches the number of CPU cores
- Increase for environments with high signal diversity
- Monitor with `semconv_proxy_pipeline_processing_duration_seconds`

```bash
semconv-proxy --worker-count=16 ...
```

## Persistence

The dictionary is persisted to Pebble storage in the `data-dir` for crash recovery.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `--data-dir` | `./data` | Directory for Pebble storage |
| `--persist-interval` | `100ms` | How often batches are flushed to disk |
| `--persist-batch-size` | `1000` | Maximum entries per batch write |

After a crash, the proxy reloads the full dictionary from Pebble on startup. Recovery completes in under 5 seconds for 10,000 entries.

## Log Levels

| Level | Use Case |
|-------|----------|
| `error` | Production — only errors |
| `warn` | Production — errors and warnings |
| `info` | Default — startup, shutdown, key events |
| `debug` | Development — detailed component logging |

Logs are structured JSON via `slog`:

```json
{"time":"2026-05-08T10:00:00Z","level":"INFO","msg":"starting semconv-proxy","backend_endpoint":"localhost:4317","http_port":4318,"grpc_port":4317,"api_port":8080}
```

## Hot Reload

Send `SIGHUP` to reload mutable configuration from the config file without restarting:

```bash
kill -HUP $(pidof semconv-proxy)
```

Mutable settings (reloaded on SIGHUP):

- `log-level`

Immutable settings (require restart):

- `backend-endpoint`, `otlp-http-port`, `otlp-grpc-port`, `api-port`
