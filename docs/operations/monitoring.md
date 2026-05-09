# Monitoring

## Prometheus Scrape Configuration

Add the proxy to your Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: semconv-proxy
    static_configs:
      - targets:
          - semconv-proxy.observability:8080
    metrics_path: /metrics
    scrape_interval: 15s
```

Or with service discovery in Kubernetes:

```yaml
scrape_configs:
  - job_name: semconv-proxy
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - observability
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name]
        action: keep
        regex: semconv-proxy
```

## Key Metrics to Monitor

| Priority | Metric | Alert Condition |
|----------|--------|-----------------|
| Critical | `semconv_proxy_signals_dropped_total` | `rate() > 0` |
| High | `semconv_proxy_cardinality_budget_utilization` | `> 80%` |
| High | `semconv_proxy_pipeline_drops_total` | `rate() > 0` |
| Medium | `semconv_proxy_pipeline_lag` | Continuously increasing |
| Medium | `semconv_proxy_dictionary_entries` | Sudden large spikes |
| Low | `semconv_proxy_api_request_duration_seconds` | p99 > 100ms |

## Recommended Alerts

```yaml
groups:
  - name: semconv-proxy
    rules:
      - alert: SemConvProxySignalDrops
        expr: rate(semconv_proxy_signals_dropped_total[5m]) > 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "SemConv Proxy is dropping signals"
          description: "The proxy is dropping signals/sec. Check backend connectivity."

      - alert: SemConvProxyHighCardinality
        expr: semconv_proxy_cardinality_high_attributes > 10
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "High cardinality detected"
          description: "Attributes exceed the cardinality threshold."

      - alert: SemConvProxyBudgetExhaustion
        expr: semconv_proxy_cardinality_budget_utilization > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Cardinality budget approaching limit"
          description: "Budget utilization is above 80%."

      - alert: SemConvProxyPipelineDrops
        expr: rate(semconv_proxy_pipeline_drops_total[5m]) > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Analysis pipeline is dropping tasks"
          description: "The ring buffer is overflowing. Consider increasing ring-buffer-size."

      - alert: SemConvProxyHighMemory
        expr: process_resident_memory_bytes{job="semconv-proxy"} > 536870912
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Proxy memory exceeds 512MB"
```

## Grafana Dashboard

Import the key panels:

1. **Signal Throughput** — stacked area chart of `semconv_proxy_signals_received_total` by `signal_type`
2. **Forwarding Health** — success rate percentage
3. **Dictionary Size** — `semconv_proxy_dictionary_entries` over time
4. **Cardinality Budget** — gauge for `semconv_proxy_cardinality_budget_utilization`
5. **Pipeline Lag** — `semconv_proxy_pipeline_lag` time series
6. **API Latency** — histogram quantile of `semconv_proxy_api_request_duration_seconds`

## Log Analysis

Logs are structured JSON. Useful queries:

```bash
# All errors
kubectl logs -l app=semconv-proxy | jq 'select(.level=="ERROR")'

# Signal forwarding issues
kubectl logs -l app=semconv-proxy | jq 'select(.msg | contains("forward"))'

# Dictionary changes
kubectl logs -l app=semconv-proxy | jq 'select(.msg | contains("dictionary"))'
```
