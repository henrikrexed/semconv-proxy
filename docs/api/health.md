# Health & Readiness

Kubernetes-compatible health check endpoints for liveness and readiness probes.

## Liveness Probe

```
GET /healthz
```

Returns `200` when the proxy process is alive and not deadlocked.

```json
{"status": "alive"}
```

**Kubernetes configuration:**

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Readiness Probe

```
GET /readyz
```

Returns `200` when the proxy is ready to receive traffic (dictionary loaded, receivers started). Returns `503` during startup or crash recovery.

### Ready Response (200)

```json
{
  "status": "ready",
  "dictionary_entries": 342
}
```

### Not Ready Response (503)

```json
{
  "status": "loading",
  "dictionary_entries": 128,
  "components": [
    {"name": "dictionary", "status": "OK"},
    {"name": "receiver", "status": "STARTING"},
    {"name": "api", "status": "OK"},
    {"name": "storage", "status": "OK"}
  ]
}
```

**Kubernetes configuration:**

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

## Component Health

The readiness endpoint reports per-component health status. Each component reports one of:

| Status | Meaning |
|--------|---------|
| `OK` | Component is healthy and running |
| `STARTING` | Component is initializing |
| `DEGRADED` | Component encountered errors but is still running |
| `FAILED` | Component has failed |

Components tracked:

- **dictionary** — in-memory sharded dictionary
- **receiver** — OTLP HTTP/gRPC receivers
- **api** — REST API server
- **storage** — Pebble persistence layer
