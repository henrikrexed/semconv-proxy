# Self-Observability Metrics

SemConv Proxy exposes 30+ Prometheus metrics at `/metrics` for monitoring the proxy itself.

## Endpoint

```
GET /metrics
```

Returns Prometheus text exposition format on the API port (default: 8080).

## Metric Categories

### Signal Throughput

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `semconv_proxy_signals_received_total` | counter | `signal_type`, `protocol` | Total OTLP signals received |
| `semconv_proxy_signals_forwarded_total` | counter | `signal_type`, `protocol` | Total signals forwarded to backend |
| `semconv_proxy_signals_dropped_total` | counter | `signal_type`, `protocol` | Total signals dropped (backend failure) |

**Label values:**

- `signal_type`: `metric`, `trace`, `log`
- `protocol`: `http`, `grpc`

**Example query — received vs forwarded:**

```promql
rate(semconv_proxy_signals_received_total[5m])
-
rate(semconv_proxy_signals_dropped_total[5m])
```

### Dictionary Operations

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `semconv_proxy_dictionary_entries` | gauge | — | Current number of dictionary entries |
| `semconv_proxy_dictionary_attributes_added_total` | counter | — | Total attributes added |
| `semconv_proxy_dictionary_attributes_changed_total` | counter | — | Total attributes changed |
| `semconv_proxy_dictionary_attributes_removed_total` | counter | — | Total attributes removed |

**Example query — attribute growth rate:**

```promql
rate(semconv_proxy_dictionary_attributes_added_total[1h])
```

### Cardinality

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `semconv_proxy_cardinality_high_attributes` | gauge | — | Number of high-cardinality attributes |
| `semconv_proxy_cardinality_budget_utilization` | gauge | — | Global budget utilization percentage |

**Example alert — budget approaching limit:**

```promql
semconv_proxy_cardinality_budget_utilization > 80
```

### Analysis Pipeline

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `semconv_proxy_pipeline_ring_buffer_size` | gauge | — | Current ring buffer fill level |
| `semconv_proxy_pipeline_lag` | gauge | — | Analysis pipeline lag |
| `semconv_proxy_pipeline_drops_total` | counter | — | Total dropped analysis tasks |
| `semconv_proxy_pipeline_processing_duration_seconds` | histogram | `stage` | Processing time per stage |

**Example query — p99 processing time:**

```promql
histogram_quantile(0.99, rate(semconv_proxy_pipeline_processing_duration_seconds_bucket[5m]))
```

### Storage

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `semconv_proxy_storage_persist_duration_seconds` | histogram | `operation` | Pebble persist duration |
| `semconv_proxy_storage_disk_size_bytes` | gauge | — | Pebble disk size in bytes |

### API

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `semconv_proxy_api_request_total` | counter | `path`, `method`, `status` | Total API requests |
| `semconv_proxy_api_request_duration_seconds` | histogram | `path` | API request duration |

**Example query — API error rate:**

```promql
rate(semconv_proxy_api_request_total{status=~"5.."}[5m])
/
rate(semconv_proxy_api_request_total[5m])
```

## Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: semconv-proxy
    static_configs:
      - targets:
          - semconv-proxy.observability:8080
    metrics_path: /metrics
    scrape_interval: 15s
```

## Grafana Dashboard

A minimal dashboard might include these panels:

| Panel | Query | Visualization |
|-------|-------|---------------|
| Signal Throughput | `rate(semconv_proxy_signals_received_total[5m])` | Time series |
| Forwarding Success Rate | `rate(semconv_proxy_signals_forwarded_total[5m]) / rate(semconv_proxy_signals_received_total[5m])` | Stat |
| Dictionary Size | `semconv_proxy_dictionary_entries` | Stat |
| Cardinality Budget | `semconv_proxy_cardinality_budget_utilization` | Gauge |
| Pipeline Drops | `rate(semconv_proxy_pipeline_drops_total[5m])` | Time series |
| API Latency p99 | `histogram_quantile(0.99, rate(semconv_proxy_api_request_duration_seconds_bucket[5m]))` | Time series |
