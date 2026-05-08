# API Reference

SemConv Proxy exposes a REST API on port 8080 for querying the live dictionary, checking cardinality, and exporting conventions.

## Base URL

```
http://<proxy-host>:8080
```

## Endpoints Overview

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/dictionary` | GET | List and filter dictionary entries |
| `/api/v1/dictionary/:name` | GET | Get single attribute details |
| `/api/v1/cardinality` | GET | Budget utilization and high-cardinality attributes |
| `/api/v1/export` | GET | Export as Weaver YAML |
| `/healthz` | GET | Liveness probe |
| `/readyz` | GET | Readiness probe |
| `/metrics` | GET | Prometheus metrics |

## Sections

- [Dictionary API](dictionary.md) — query and browse discovered conventions
- [Health & Readiness](health.md) — Kubernetes probe endpoints
- [Metrics](metrics.md) — self-observability Prometheus metrics
- [Weaver Export](weaver-export.md) — generate Weaver-compatible YAML

## Error Responses

All errors follow this format:

```json
{
  "error": "attribute not found",
  "code": "NOT_FOUND"
}
```

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `NOT_FOUND` | 404 | Requested resource does not exist |
| `BAD_REQUEST` | 400 | Invalid query parameters |
| `METHOD_NOT_ALLOWED` | 405 | Wrong HTTP method |
| `EXPORT_ERROR` | 500 | Export generation failed |
