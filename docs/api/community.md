# Community SemConv API

Faceted search over the official OpenTelemetry semantic-convention registry. The
registry is a build-time pinned Weaver snapshot embedded in the proxy binary, so
these endpoints have no runtime dependency on Weaver or network access.

Attributes, metrics, spans, events, and entities are flattened into a single
searchable `item` shape so a query spans all signal types uniformly.

## Search

```
GET /api/v1/semconv/community
```

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | all | Keyword search. Whitespace-separated terms are AND-matched as case-insensitive substrings of an item's name or brief (e.g. `http method` matches `http.request.method`). Results are ranked by relevance. |
| `type` | string | all | Filter by signal type: `attribute`, `metric`, `span`, `event`, `entity`. Repeat or comma-separate to OR multiple values. |
| `stability` | string | all | Filter by stability: `stable`, `development`, `release_candidate`. Repeat or comma-separate to OR multiple values. |
| `namespace` | string | all | Filter by leading namespace segment (e.g. `http`). |
| `limit` | int | `50` | Max entries (max: 1000). |
| `offset` | int | `0` | Pagination offset. |

### Response

The `total`/`offset`/`limit`/`entries` envelope mirrors the dictionary endpoint.
`facets` is an additive field carrying counts over the text-matched set
**before** facet selections are applied, so the UI can show how many results
each facet value would yield.

```json
{
  "total": 3,
  "offset": 0,
  "limit": 50,
  "entries": [
    {
      "key": "attribute:http.request.method",
      "name": "http.request.method",
      "type": "attribute",
      "namespace": "http",
      "brief": "HTTP request method.",
      "stability": "stable",
      "value_type": "enum",
      "enum": [
        {"id": "connect", "value": "CONNECT", "brief": "CONNECT method.", "stability": "stable"},
        {"id": "get", "value": "GET", "brief": "GET method.", "stability": "stable"}
      ],
      "examples": ["GET", "POST", "HEAD"],
      "requirement_level": "recommended"
    }
  ],
  "facets": {
    "type": [
      {"value": "attribute", "count": 2},
      {"value": "metric", "count": 1}
    ],
    "stability": [
      {"value": "stable", "count": 3}
    ],
    "namespace": [
      {"value": "http", "count": 3}
    ]
  }
}
```

### Item Fields

| Field | Type | Notes |
|-------|------|-------|
| `key` | string | Stable identifier, e.g. `attribute:http.request.method`, `metric:http.server.request.duration`. Used to fetch a single entry. |
| `name` | string | Item name. |
| `type` | string | One of `attribute`, `metric`, `span`, `event`, `entity`. |
| `namespace` | string | Leading dotted segment of the name. |
| `brief` | string | Short description. |
| `note` | string | Longer description (optional). |
| `stability` | string | `stable`, `development`, `release_candidate` (optional). |
| `deprecated` | object | `{reason, renamed_to, note}` when deprecated (optional). |
| `value_type` | string | Attribute value type: `string`, `int`, `enum`, … (attributes). |
| `enum` | array | Allowed members when `value_type` is `enum`: `{id, value, brief, stability}`. |
| `examples` | array | Example values, flattened from nested arrays. |
| `requirement_level` | string | `required`, `recommended`, `conditionally_required`, … (optional). |
| `unit` | string | Metric unit (metrics). |
| `instrument` | string | Metric instrument kind (metrics). |
| `span_kind` | string | Span kind (spans). |
| `attributes` | array | Referenced attribute names (groups). |

### Examples

Keyword search (AND-matched, ranked):

```bash
curl "http://localhost:8080/api/v1/semconv/community?q=http+method&limit=10" | jq .
```

Filter to stable metrics in the `http` namespace:

```bash
curl "http://localhost:8080/api/v1/semconv/community?type=metric&stability=stable&namespace=http" | jq .
```

## Get Single Entry

```
GET /api/v1/semconv/community/:key
```

`key` is the item key from a search result, such as
`attribute:http.request.method` or `metric:http.server.request.duration`.

### Response

Returns a single `item` object (same shape as a search `entries[]` element).

```bash
curl http://localhost:8080/api/v1/semconv/community/attribute:http.request.method | jq .
```

### Error Response

```json
{
  "error": "item not found",
  "code": "NOT_FOUND"
}
```

## Errors

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `REGISTRY_UNAVAILABLE` | 503 | The embedded registry failed to load at startup. |
| `NOT_FOUND` | 404 | Requested item key does not exist. |
| `BAD_REQUEST` | 400 | Missing item key. |
| `METHOD_NOT_ALLOWED` | 405 | Wrong HTTP method (only GET is supported). |
