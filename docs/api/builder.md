# Builder & Compare API

These endpoints back the [Weaver Asset Builder](../operations/weaver-builder.md)
and the **Compare** scope of the [web UI](../operations/web-ui.md). All are
served on the API port (`:8080`).

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/semconv/compare` | GET | Bucket live telemetry against the official registry |
| `/api/v1/builder/seed` | GET | Seed the Definitions table from live telemetry |
| `/api/v1/builder/policy-templates` | GET | Policy-check catalog |
| `/api/v1/builder/generate` | POST | Emit the Weaver registry file set |
| `/api/v1/builder/export.zip` | POST | Bundle the registry file set as a zip |

## Compare

```
GET /api/v1/semconv/compare
```

Classifies every observed attribute against the embedded official registry into
buckets — `matched`, `type-mismatch`, `deprecated`, `not-in-registry` — with
per-bucket counts and a total. Drives the **Compare** scope in the UI.

Returns `503 REGISTRY_UNAVAILABLE` if the embedded registry failed to load.

## Seed

```
GET /api/v1/builder/seed?q=&limit=
```

Seeds the Builder's Definitions table from the live dictionary,
cross-referenced against the embedded registry.

### Query parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | — | Filter discovered attributes by name pattern |
| `limit` | int | `500` (max `2000`) | Maximum rows returned |

### Response

```json
{
  "total": 128,
  "limit": 500,
  "attributes": [
    {
      "id": "http.request.method",
      "namespace": "http",
      "type": "string",
      "observed_type": "string",
      "cross_ref": "matched",
      "signal_types": ["span"],
      "cardinality": 6,
      "registry_key": "http.request.method",
      "registry_type": "string",
      "brief": "HTTP request method.",
      "stability": "stable",
      "requirement_level": "recommended",
      "examples": ["GET", "POST"]
    }
  ],
  "default_dependency": { "name": "otel", "registry": "https://..." },
  "known_signal_types": ["span", "metric", "log"]
}
```

- `type` is pre-filled from the *observed* telemetry type.
- `cross_ref` is one of `matched`, `type-mismatch`, `deprecated`,
  `not-in-registry`.
- For matched attributes, the official `brief` / `stability` /
  `requirement_level` / `examples` are surfaced as suggested enrichment.
- `known_signal_types` sources the Config tab's finding-filter signal-type
  dropdown.

Returns `503 REGISTRY_UNAVAILABLE` or `503 DICTIONARY_UNAVAILABLE` if those
subsystems are not ready.

## Policy templates

```
GET /api/v1/builder/policy-templates
```

Returns the static, proxy-versioned catalog of policy-check templates that
drives the Builder's **Checks** tab. Each template carries its parameter schema
and its Weaver stage (Rego package).

```json
{
  "templates": [
    {
      "id": "naming_prefix_required",
      "title": "Naming prefix required",
      "description": "Every attribute name must start with a required prefix …",
      "stage": "after_resolution",
      "params": [
        { "name": "prefix", "label": "Required prefix", "type": "string", "required": true }
      ]
    }
  ]
}
```

## Generate

```
POST /api/v1/builder/generate
```

Turns posted builder state into a Weaver registry file set (`path -> content`).

### Request body

A `BuilderState` object:

```json
{
  "manifest": {
    "schema_url": "https://acme.com/schemas/0.1.0",
    "description": "Acme custom semantic conventions."
  },
  "groups": [
    {
      "namespace": "acme.payments",
      "attributes": [
        {
          "id": "acme.payments.provider",
          "type": "string",
          "brief": "Payment provider.",
          "stability": "development",
          "requirement_level": "recommended",
          "examples": ["stripe", "adyen"]
        }
      ]
    }
  ],
  "policies": [
    { "template_id": "naming_prefix_required", "params": { "prefix": "acme." } }
  ],
  "config": {
    "registry_path": ".",
    "policy_paths": ["policies"],
    "finding_filters": [
      { "min_level": "violation", "signal_type": "span" }
    ]
  }
}
```

Validation rules (shared with `export.zip`):

- `manifest.schema_url` is **required** and must be a versioned OTel schema URL.
- `manifest.dependencies` allows at most one entry.
- `config.finding_filters[].min_level`, when set, must be one of
  `information` / `improvement` / `violation`.

When the state declares no dependencies, the pinned OTel dependency derived from
the embedded registry is injected as the default.

### Response

```json
{
  "files": {
    "registry_manifest.yaml": "...",
    "groups/acme.payments.yaml": "...",
    "policies/naming_prefix_required.rego": "...",
    ".weaver.toml": "..."
  }
}
```

## Export zip

```
POST /api/v1/builder/export.zip
```

Takes the same `BuilderState` body as `/generate` and returns the generated file
set bundled into a single archive. The response is
`Content-Type: application/zip` with
`Content-Disposition: attachment; filename="registry.zip"`. The archive unpacks
to a directory that passes `weaver registry check -r <dir>`.

> Served over `POST` (not `GET`) because the full builder state cannot be carried
> in a URL.

## Error responses

In addition to the [shared error codes](index.md#error-responses), the Builder
endpoints can return:

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `REGISTRY_UNAVAILABLE` | 503 | Embedded semconv registry failed to load |
| `DICTIONARY_UNAVAILABLE` | 503 | Dictionary subsystem not ready |
| `GENERATE_ERROR` | 500 | Registry generation failed |
| `BUNDLE_ERROR` | 500 | Zip bundling failed |

## Designed, not yet shipped

A server-side `/api/v1/builder/validate` endpoint and an in-proxy
`weaver registry check` validation loop are designed but gated. Validate the
exported zip with the Weaver CLI for now.
