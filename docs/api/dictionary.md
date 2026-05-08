# Dictionary API

Query the live dictionary of discovered semantic conventions.

## List Entries

```
GET /api/v1/dictionary
```

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `type` | string | all | Filter by signal type: `metric`, `trace`, `log` |
| `q` | string | all | Attribute name glob pattern (e.g., `http.*`) |
| `sort` | string | `name` | Sort field: `cardinality`, `first_seen`, `last_seen`, `name` |
| `order` | string | `asc` | Sort direction: `asc` or `desc` |
| `limit` | int | `100` | Max entries (max: 1000) |
| `offset` | int | `0` | Pagination offset |

### Response

```json
{
  "total": 342,
  "offset": 0,
  "limit": 100,
  "entries": [
    {
      "name": "http.request.method",
      "type": "string",
      "signal_types": ["metric", "trace"],
      "cardinality": 6,
      "first_seen": "2026-05-08T10:00:00Z",
      "last_seen": "2026-05-08T15:30:00Z",
      "status": "active"
    }
  ]
}
```

### Examples

Sort by cardinality (highest first):

```bash
curl "http://localhost:8080/api/v1/dictionary?sort=cardinality&order=desc&limit=10"
```

Filter to metric attributes:

```bash
curl "http://localhost:8080/api/v1/dictionary?type=metric"
```

Search for HTTP attributes:

```bash
curl "http://localhost:8080/api/v1/dictionary?q=http.*"
```

Paginate through results:

```bash
curl "http://localhost:8080/api/v1/dictionary?offset=100&limit=100"
```

## Get Single Attribute

```
GET /api/v1/dictionary/:attribute_name
```

### Response

```json
{
  "name": "http.request.method",
  "type": "string",
  "signal_types": ["metric", "trace"],
  "cardinality": 6,
  "top_values": [
    {"value": "GET", "approximate_count": 45230},
    {"value": "POST", "approximate_count": 12340},
    {"value": "PUT", "approximate_count": 3200}
  ],
  "first_seen": "2026-05-08T10:00:00Z",
  "last_seen": "2026-05-08T15:30:00Z",
  "status": "active"
}
```

### Error Response

```json
{
  "error": "attribute not found",
  "code": "NOT_FOUND"
}
```

### Examples

```bash
curl http://localhost:8080/api/v1/dictionary/http.request.method | jq .
```

## Cardinality Endpoint

```
GET /api/v1/cardinality
```

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `threshold` | int | `100` | Only show attributes above this cardinality |

### Response

```json
{
  "global_budget": {
    "used": 342,
    "limit": 10000,
    "utilization_pct": 3.42
  },
  "attributes": [
    {
      "name": "k8s.pod.name",
      "cardinality": 847,
      "cap": 1000,
      "utilization_pct": 84.7
    }
  ]
}
```

### Examples

Show attributes with more than 50 unique values:

```bash
curl "http://localhost:8080/api/v1/cardinality?threshold=50" | jq .
```
