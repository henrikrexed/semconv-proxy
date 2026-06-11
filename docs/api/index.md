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
| `/api/v1/semconv/community` | GET | Search the official OTel semantic-convention registry |
| `/api/v1/semconv/community/:key` | GET | Get a single registry item by key |
| `/api/v1/semconv/compare` | GET | Bucket live telemetry against the registry |
| `/api/v1/cardinality` | GET | Budget utilization and high-cardinality attributes |
| `/api/v1/export` | GET | Export as Weaver YAML (one-shot) |
| `/api/v1/builder/seed` | GET | Seed the Weaver Asset Builder definitions table |
| `/api/v1/builder/policy-templates` | GET | Builder policy-check catalog |
| `/api/v1/builder/generate` | POST | Emit a Weaver registry file set from Builder state |
| `/api/v1/builder/export.zip` | POST | Bundle the generated registry as a zip |
| `/healthz` | GET | Liveness probe |
| `/readyz` | GET | Readiness probe |
| `/metrics` | GET | Prometheus metrics |

## Sections

- [Dictionary API](dictionary.md) — query and browse discovered conventions
- [Community SemConv API](community.md) — search the official OTel registry snapshot
- [Builder & Compare API](builder.md) — Weaver Asset Builder + telemetry/registry comparison
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
| `REGISTRY_UNAVAILABLE` | 503 | Embedded semconv registry failed to load |
| `DICTIONARY_UNAVAILABLE` | 503 | Dictionary subsystem not ready |
| `EXPORT_ERROR` | 500 | Export generation failed |
| `GENERATE_ERROR` | 500 | Builder registry generation failed |
| `BUNDLE_ERROR` | 500 | Builder zip bundling failed |
