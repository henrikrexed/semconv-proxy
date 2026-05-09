# Troubleshooting

## Common Issues

### Proxy Won't Start

**Symptom:** Container exits immediately.

**Check logs:**

```bash
docker logs semconv-proxy
# or
kubectl logs -l app=semconv-proxy
```

**Common causes:**

| Error | Fix |
|-------|-----|
| `config: backend-endpoint is required` | Set `--backend-endpoint` or `SEMCONV_PROXY_BACKEND_ENDPOINT` |
| `failed to create data directory` | Ensure `--data-dir` is writable by the proxy user (UID 65532) |
| `api: listen: address already in use` | Change the port with `--api-port` or check for conflicting services |

### Signals Not Reaching Backend

**Check forwarding metrics:**

```bash
curl http://localhost:8080/metrics | grep semconv_proxy_signals
```

| Symptom | Check | Fix |
|---------|-------|-----|
| `received` > 0 but `forwarded` = 0 | Backend unreachable | Verify `--backend-endpoint` is correct and reachable |
| `dropped` > 0 | Backend returning errors | Check backend health, TLS settings |
| `received` = 0 | No signals arriving | Verify Collector exporter points at the proxy |

**Test backend connectivity:**

```bash
# From inside the proxy container
docker exec semconv-proxy wget -qO- http://backend-host:4318/
```

### Dictionary Is Empty

**Possible causes:**

1. No signals have been sent through the proxy yet
2. Signals are being forwarded but not analyzed (ring buffer full)
3. Persistence is not loading on restart

**Check pipeline metrics:**

```bash
curl http://localhost:8080/metrics | grep -E "pipeline_lag|pipeline_drops|dictionary_entries"
```

If `pipeline_drops_total` is increasing, the ring buffer is overflowing. Increase `--ring-buffer-size` or `--worker-count`.

### High Memory Usage

**Check cardinality:**

```bash
curl http://localhost:8080/api/v1/cardinality | jq .
```

If `utilization_pct` is high:

1. Identify high-cardinality attributes: `GET /api/v1/cardinality?threshold=50`
2. Add OTel Collector processors to drop or aggregate those attributes
3. Reduce `--global-budget` to force more aggressive eviction

### Readiness Probe Failing

```bash
curl http://localhost:8080/readyz
```

If status is `loading`:

- Dictionary is still recovering from Pebble — wait for it to complete
- If stuck, check Pebble data directory for corruption

If a component shows `DEGRADED` or `FAILED`:

- Check logs for that component
- The proxy continues operating in degraded mode for non-critical component failures

### Export Produces Empty YAML

**Check dictionary state:**

```bash
curl http://localhost:8080/api/v1/dictionary | jq '.total'
```

If `total` is 0, the dictionary is empty. Ensure signals have been flowing through the proxy.

### Persistence Not Working

**Verify the data directory:**

```bash
# Check if Pebble files exist
ls -la /data/pebble/
```

**Common issues:**

- Volume not mounted correctly in Docker/Kubernetes
- Permissions: proxy runs as UID 65532, directory must be writable
- `--data-dir` pointing to wrong location

## Debug Mode

Run with debug logging for detailed component-level output:

```bash
semconv-proxy --log-level=debug --backend-endpoint=localhost:4317
```

Debug logs include:

- Every signal received and forwarded
- Dictionary upsert details
- Ring buffer and worker pool state
- Pebble batch writes
- API request details

## Getting Help

- [GitHub Issues](https://github.com/henrikrexed/semconv-proxy/issues) — bug reports and feature requests
- [Discussions](https://github.com/henrikrexed/semconv-proxy/discussions) — questions and community
